// Package orchestrator handles the secure secret lifecycle:
// 1. Discover secrets from compose file and .env.age
// 2. Decrypt secrets into memory (env vars) or tmpfs (file secrets)
// 3. Inject into docker compose process
// 4. Cleanup on exit
package orchestrator

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rafitox/secure-compose/internal/age"
	"github.com/rafitox/secure-compose/internal/compose"
	"github.com/rafitox/secure-compose/internal/config"
	"github.com/rafitox/secure-compose/internal/secrets"
)

// Orchestrator manages the secret lifecycle during docker compose operations.
type Orchestrator struct {
	cfg          *config.Config
	composeFile  string
	passphrase   string
	envSecrets   secrets.EnvMap // env vars from .env.age (in memory only)
	fileSecrets  []compose.Secret
	tmpfs        *secrets.TmpfsManager
	sessionID    string
}

// New creates a new Orchestrator.
func New(cfg *config.Config, composeFile, passphrase, sessionID string) *Orchestrator {
	return &Orchestrator{
		cfg:         cfg,
		composeFile: composeFile,
		passphrase:  passphrase,
		sessionID:   sessionID,
	}
}

// Discover finds all secrets from compose file.
func (o *Orchestrator) Discover() error {
	if o.composeFile == "" {
		return nil
	}

	secrets, err := compose.ParseSecrets(o.composeFile)
	if err != nil {
		fmt.Printf("⚠  Warning: could not parse secrets from %s: %v\n", o.composeFile, err)
		return nil
	}

	if len(secrets) > 0 {
		o.fileSecrets = secrets
		fmt.Printf("→ Found %d file secret(s) in compose\n", len(secrets))
	}

	return nil
}

// DecryptEnv decrypts .env.age into memory (no disk write).
// Uses the "Infisical-style" approach: secrets stay in RAM only.
func (o *Orchestrator) DecryptEnv() error {
	if _, err := os.Stat(o.cfg.EncryptedFile); os.IsNotExist(err) {
		// No .env.age - that's OK, we might only have file secrets
		return nil
	}

	content, err := age.DecryptContent(o.cfg.EncryptedFile, o.passphrase)
	if err != nil {
		return fmt.Errorf("failed to decrypt .env.age: %w", err)
	}

	o.envSecrets, err = secrets.ParseEnvFile(content)
	if err != nil {
		return fmt.Errorf("failed to parse .env content: %w", err)
	}

	// Securely clear the raw content now that we've parsed it
	secrets.SecureZero(content)

	fmt.Printf("→ Decrypted %d env var(s) from .env.age (in memory only)\n", len(o.envSecrets))
	return nil
}

// DecryptSecrets decrypts file secrets to tmpfs mount.
func (o *Orchestrator) DecryptSecrets() error {
	if len(o.fileSecrets) == 0 {
		return nil
	}

	// Initialize tmpfs manager
	o.tmpfs = secrets.NewTmpfsManager(o.sessionID)
	if err := o.tmpfs.Setup(); err != nil {
		return fmt.Errorf("failed to setup tmpfs: %w", err)
	}

	// Decrypt each secret file to tmpfs
	for _, secret := range o.fileSecrets {
		encryptedFile := compose.ResolveSecretFilePath(o.composeFile, secret.EncryptedFile())
		if _, err := os.Stat(encryptedFile); os.IsNotExist(err) {
			continue // Skip if no encrypted version
		}

		content, err := age.DecryptContent(encryptedFile, o.passphrase)
		if err != nil {
			return fmt.Errorf("failed to decrypt secret %s: %w", secret.Name, err)
		}

		path, err := o.tmpfs.WriteSecret(secret.Name, content)
		if err != nil {
			return err
		}

		// Securely clear the raw content after writing to tmpfs
		secrets.SecureZero(content)

		fmt.Printf("→ Secret '%s' mounted at %s (RAM disk)\n", secret.Name, path)
	}

	return nil
}

// SetupSignalHandler registers cleanup handlers for SIGINT/SIGTERM.
func (o *Orchestrator) SetupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		o.Cleanup()
		os.Exit(1)
	}()
}

// Cleanup tears down tmpfs and clears secrets from memory.
func (o *Orchestrator) Cleanup() {
	if o.tmpfs != nil {
		fmt.Printf("→ Cleaning up tmpfs mount...\n")
		o.tmpfs.Cleanup()
	}

	if o.envSecrets != nil {
		// Clear env map values from memory
		for k := range o.envSecrets {
			delete(o.envSecrets, k)
		}
		o.envSecrets = nil
	}

	fmt.Printf("→ Secrets cleared from memory\n")
}

// EnvSecrets returns the decrypted environment variables map.
func (o *Orchestrator) EnvSecrets() secrets.EnvMap {
	return o.envSecrets
}

// FileSecrets returns the discovered file secrets.
func (o *Orchestrator) FileSecrets() []compose.Secret {
	return o.fileSecrets
}

// TmpfsManager returns the tmpfs manager (nil if no file secrets).
func (o *Orchestrator) TmpfsManager() *secrets.TmpfsManager {
	return o.tmpfs
}
