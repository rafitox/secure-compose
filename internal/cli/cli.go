package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/rafitox/secure-compose/internal/compose"
	"github.com/rafitox/secure-compose/internal/config"
	"github.com/rafitox/secure-compose/internal/docker"
	"github.com/rafitox/secure-compose/internal/age"
)

// Run is the main entry point for the CLI
func Run() error {
	// Check for version flag first (before any subcommand)
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("secure-compose %s\n", getVersion())
			return nil
		}
	}

	// Check for help flag
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		printUsage()
		return nil
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "encrypt":
		return encryptCmd()
	case "decrypt":
		return decryptCmd()
	case "edit":
		return editCmd()
	case "up":
		return composeCmd("up", args)
	case "down":
		return composeCmd("down", args)
	case "exec":
		if len(args) < 2 {
			return fmt.Errorf("usage: secure-compose exec <service> <command> [args...]")
		}
		return composeCmd("exec", args)
	case "restart":
		return composeCmd("restart", args)
	case "logs":
		return composeCmd("logs", args)
	case "build":
		return composeCmd("build", args)
	default:
		// If unknown, treat as docker compose command
		return composeCmd(cmd, args)
	}
}

func getVersion() string {
	v := os.Getenv("SECURE_COMPOSE_VERSION")
	if v == "" {
		return "dev"
	}
	return v
}

func printUsage() {
	fmt.Printf(`secure-compose - Docker Compose with age-encrypted secrets

Usage:
  secure-compose encrypt               Encrypt .env to .env.age with passphrase
  secure-compose encrypt --secret-file <file>   Encrypt a specific secret file
  secure-compose decrypt               Decrypt .env.age to .env with passphrase
  secure-compose decrypt -o            Decrypt to stdout (for piping)
  secure-compose decrypt --secret-file <file>  Decrypt a specific secret file
  secure-compose edit                  Edit encrypted .env file
  secure-compose up [args]            Decrypt and run docker compose up
  secure-compose down [args]           Run docker compose down and cleanup
  secure-compose exec <svc> <cmd>      Run command in service with decrypted env
  secure-compose restart [args]        Restart services
  secure-compose logs [args]           View logs
  secure-compose build [args]          Build services
  secure-compose -h, --help           Show this help
  secure-compose --version             Show version

Options:
  -o, --stdout                Write decrypted content to stdout (for piping)
  -s, --secret-file <path>    Encrypt/decrypt a specific secret file (Docker secrets)

Security:
  - Uses age with passphrase (scrypt KDF)
  - Team members share the same passphrase
  - No key files to manage
  - .env.age is safe to commit to git

Examples:
  secure-compose encrypt
  secure-compose encrypt --secret-file ./secrets/db_password.txt
  secure-compose decrypt
  secure-compose decrypt --secret-file ./secrets/db_password.txt
  secure-compose decrypt --stdout | jq -r '.DB_PASSWORD'
  secure-compose decrypt --stdout > backup.env
  secure-compose up -d
  secure-compose up --build
  secure-compose exec postgres psql -U postgres
  secure-compose down
  secure-compose logs -f api

Environment Variables:
  SECURE_COMPOSE_ENV_FILE         Path to .env file (default: .env)
  SECURE_COMPOSE_ENCRYPTED_FILE  Path to .env.age file (default: .env.age)
  SECURE_COMPOSE_PASSPHRASE       Passphrase (for automation, avoid in production)
  SECURE_COMPOSE_NO_TEARDOWN      Set to 1 to skip .env cleanup on exit
  SECURE_COMPOSE_SECRET_FILE      Specific secret file for encrypt/decrypt
`)
}

func encryptCmd() error {
	cfg := config.Load()

	if err := checkDependencies(); err != nil {
		return err
	}

	// Check for --secret-file flag
	secretFile := getFlagValue(os.Args, "--secret-file", "-s")
	if secretFile != "" {
		cfg.SecretFile = secretFile
	}

	// Secret file mode (Phase 2)
	if cfg.IsSecretMode() {
		plainFile, encryptedFile := cfg.SecretFilePaths()
		return encryptSecretFile(plainFile, encryptedFile)
	}

	// Standard .env mode
	if _, err := os.Stat(cfg.EnvFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found. Create it first with your secrets", cfg.EnvFile)
	}

	fmt.Printf("→ Encrypting %s → %s\n", cfg.EnvFile, cfg.EncryptedFile)
	fmt.Printf("→ Enter passphrase (shared with your team)\n")

	passphrase, err := readPassphrase("Passphrase: ")
	if err != nil {
		return err
	}

	confirm, err := readPassphrase("Confirm passphrase: ")
	if err != nil {
		return err
	}

	if passphrase != confirm {
		return fmt.Errorf("passphrases do not match")
	}

	if err := age.EncryptFile(cfg.EnvFile, cfg.EncryptedFile, passphrase); err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	printSuccess("Encrypted successfully with passphrase")
	fmt.Printf("→ You can safely commit %s to git\n", cfg.EncryptedFile)
	fmt.Printf("→ Share the passphrase via 1Password, Vault, or your team's secret manager\n\n")
	fmt.Printf("→ Don't forget to add %s to .gitignore\n", cfg.EnvFile)

	return nil
}

// encryptSecretFile encrypts a specific secret file (Phase 2 feature)
func encryptSecretFile(plainFile, encryptedFile string) error {
	if _, err := os.Stat(plainFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found. Create it first with your secrets", plainFile)
	}

	fmt.Printf("→ Encrypting %s → %s\n", plainFile, encryptedFile)
	fmt.Printf("→ Enter passphrase (shared with your team)\n")

	passphrase, err := readPassphrase("Passphrase: ")
	if err != nil {
		return err
	}

	confirm, err := readPassphrase("Confirm passphrase: ")
	if err != nil {
		return err
	}

	if passphrase != confirm {
		return fmt.Errorf("passphrases do not match")
	}

	if err := age.EncryptFile(plainFile, encryptedFile, passphrase); err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Remove plaintext file after encryption
	if err := os.Remove(plainFile); err != nil {
		fmt.Printf("⚠  Warning: could not remove plaintext file: %v\n", err)
	}

	printSuccess("Secret encrypted successfully")
	fmt.Printf("→ You can safely commit %s to git\n", encryptedFile)
	fmt.Printf("→ Share the passphrase via 1Password, Vault, or your team's secret manager\n")

	return nil
}

func decryptCmd() error {
	cfg := config.Load()

	if err := checkDependencies(); err != nil {
		return err
	}

	// Check for --secret-file flag
	secretFile := getFlagValue(os.Args, "--secret-file", "-s")
	if secretFile != "" {
		cfg.SecretFile = secretFile
	}

	// Determine mode: secret file or standard .env
	if cfg.IsSecretMode() {
		plainFile, encryptedFile := cfg.SecretFilePaths()
		return decryptSecretFile(encryptedFile, plainFile)
	}

	// Standard .env mode
	if _, err := os.Stat(cfg.EncryptedFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found. Run 'secure-compose encrypt' first", cfg.EncryptedFile)
	}

	fmt.Printf("→ Enter passphrase:\n")

	passphrase, err := readPassphrase("Passphrase: ")
	if err != nil {
		return err
	}

	// Check for --stdout / -o flag
	stdoutMode := containsFlag(os.Args, "--stdout", "-o")

	if stdoutMode {
		// Decrypt to memory and write to stdout
		fmt.Printf("→ Decrypting %s...\n", cfg.EncryptedFile)
		content, err := age.DecryptContent(cfg.EncryptedFile, passphrase)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		os.Stdout.Write(content)
		return nil
	}

	// Default: decrypt to file
	fmt.Printf("→ Decrypting %s → %s\n", cfg.EncryptedFile, cfg.EnvFile)
	if err := age.DecryptFile(cfg.EncryptedFile, cfg.EnvFile, passphrase); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	if err := os.Chmod(cfg.EnvFile, 0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	printSuccess("Decrypted successfully")

	return nil
}

// decryptSecretFile decrypts a specific secret file (Phase 2 feature)
func decryptSecretFile(encryptedFile, plainFile string) error {
	if _, err := os.Stat(encryptedFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found. Run 'secure-compose encrypt --secret-file %s' first",
			encryptedFile, plainFile)
	}

	fmt.Printf("→ Enter passphrase:\n")

	passphrase, err := readPassphrase("Passphrase: ")
	if err != nil {
		return err
	}

	// Check for --stdout flag
	if containsFlag(os.Args, "--stdout", "-o") {
		fmt.Printf("→ Decrypting %s...\n", encryptedFile)
		content, err := age.DecryptContent(encryptedFile, passphrase)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		os.Stdout.Write(content)
		return nil
	}

	// Decrypt to the specified file
	fmt.Printf("→ Decrypting %s → %s\n", encryptedFile, plainFile)
	if err := age.DecryptFile(encryptedFile, plainFile, passphrase); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	if err := os.Chmod(plainFile, 0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	printSuccess("Secret decrypted successfully")

	return nil
}

// getFlagValue extracts a flag value from command line args
func getFlagValue(args []string, longFlag, shortFlag string) string {
	for i, arg := range args {
		if arg == longFlag || arg == shortFlag {
			if i+1 < len(args) {
				return args[i+1]
			}
		}
		// Handle --flag=value format
		if strings.HasPrefix(arg, longFlag+"=") {
			return strings.TrimPrefix(arg, longFlag+"=")
		}
		if strings.HasPrefix(arg, shortFlag+"=") {
			return strings.TrimPrefix(arg, shortFlag+"=")
		}
	}
	return ""
}

// containsFlag checks if the given flags are present in args
func containsFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag {
				return true
			}
		}
	}
	return false
}

func editCmd() error {
	cfg := config.Load()

	if err := checkDependencies(); err != nil {
		return err
	}

	if _, err := os.Stat(cfg.EncryptedFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found. Run 'secure-compose encrypt' first", cfg.EncryptedFile)
	}

	fmt.Printf("→ Decrypting %s for editing...\n", cfg.EncryptedFile)

	passphrase, err := readPassphrase("Passphrase: ")
	if err != nil {
		return err
	}

	content, err := age.DecryptContent(cfg.EncryptedFile, passphrase)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// Get editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "secure-compose-*.env")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Calculate checksum before
	beforeSum, err := fileChecksum(tmpPath)
	if err != nil {
		return err
	}

	// Open editor
	fmt.Printf("→ Opening %s with %s\n", tmpPath, editor)
	if err := runEditor(editor, tmpPath); err != nil {
		return err
	}

	// Calculate checksum after
	afterSum, err := fileChecksum(tmpPath)
	if err != nil {
		return err
	}

	if beforeSum == afterSum {
		fmt.Printf("→ No changes made, skipping re-encryption\n")
		return nil
	}

	// Read edited content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	// Verify content is valid env format
	if err := validateEnvContent(edited); err != nil {
		return fmt.Errorf("invalid env format: %w", err)
	}

	// Re-encrypt
	fmt.Printf("→ Re-encrypting with passphrase...\n")
	if err := age.EncryptContent(cfg.EncryptedFile, edited, passphrase); err != nil {
		return fmt.Errorf("re-encryption failed: %w", err)
	}

	printSuccess("Changes saved and re-encrypted")

	return nil
}

func composeCmd(op string, args []string) error {
	cfg := config.Load()

	if err := checkDependencies(); err != nil {
		return err
	}

	// Check for compose-file flag
	composeFile := getFlagValue(os.Args, "--compose-file", "-f")
	if composeFile == "" {
		composeFile = compose.FindComposeFile(".")
	}

	// Discover secrets from compose file (Phase 3)
	var discoveredSecrets []compose.Secret
	if composeFile != "" {
		secrets, err := compose.ParseSecrets(composeFile)
		if err != nil {
			fmt.Printf("⚠  Warning: could not parse secrets from %s: %v\n", composeFile, err)
		} else if len(secrets) > 0 {
			discoveredSecrets = secrets
			fmt.Printf("→ Found %d secret(s) in compose file\n", len(secrets))
		}
	}

	// Get passphrase
	passphrase := os.Getenv("SECURE_COMPOSE_PASSPHRASE")
	if passphrase == "" {
		fmt.Printf("→ Enter passphrase:\n")
		var err error
		passphrase, err = readPassphrase("Passphrase: ")
		if err != nil {
			return err
		}
	}

	// Decrypt .env (standard mode)
	if _, err := os.Stat(cfg.EncryptedFile); os.IsNotExist(err) {
		// No .env.age, check if we have other secrets
		if len(discoveredSecrets) == 0 {
			return fmt.Errorf("%s not found. Run 'secure-compose encrypt' first", cfg.EncryptedFile)
		}
	} else {
		fmt.Printf("→ Decrypting secrets...\n")
		if err := age.DecryptFile(cfg.EncryptedFile, cfg.EnvFile, passphrase); err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		if err := os.Chmod(cfg.EnvFile, 0600); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	// Phase 3: Decrypt compose secrets
	for _, secret := range discoveredSecrets {
		encryptedFile := compose.ResolveSecretFilePath(composeFile, secret.EncryptedFile())
		plainFile := compose.ResolveSecretFilePath(composeFile, secret.File)

		if _, err := os.Stat(encryptedFile); err == nil {
			fmt.Printf("→ Decrypting secret: %s\n", secret.Name)
			if err := age.DecryptFile(encryptedFile, plainFile, passphrase); err != nil {
				return fmt.Errorf("failed to decrypt secret %s: %w", secret.Name, err)
			}
			if err := os.Chmod(plainFile, 0600); err != nil {
				fmt.Printf("⚠  Warning: could not set permissions on %s: %v\n", plainFile, err)
			}
		}
	}

	// Build docker compose command
	dockerArgs := append([]string{op}, args...)
	fmt.Printf("→ Running: docker compose %s\n", strings.Join(dockerArgs, " "))

	if err := docker.Run(dockerArgs); err != nil {
		return err
	}

	// After successful up, warn about .env persistence
	if op == "up" {
		fmt.Printf("\n⚠  .env and secret files are persisted on disk after 'up'.\n")
		fmt.Printf("   The container reads them on startup — if you restart the container\n")
		fmt.Printf("   without these files, it will fail.\n")
		fmt.Printf("   Remove them manually when ready.\n")
		if len(discoveredSecrets) > 0 {
			fmt.Printf("\n   Secrets found:\n")
			for _, s := range discoveredSecrets {
				resolved := compose.ResolveSecretFilePath(composeFile, s.File)
				fmt.Printf("   - %s (%s)\n", s.Name, resolved)
			}
		}
		fmt.Printf("\n")
	}

	return nil
}

// checkDependencies verifies required tools are installed
func checkDependencies() error {
	missing := []string{}

	if !docker.IsInstalled() {
		missing = append(missing, "docker")
	}
	if !age.IsInstalled() {
		missing = append(missing, "age")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing dependencies: %s\n\nInstall with:\n  brew install age docker  # macOS\n  apt install age docker.io  # Ubuntu/Debian", strings.Join(missing, ", "))
	}

	return nil
}

func printSuccess(msg string) {
	fmt.Printf("\033[0;32m✓ %s\033[0m\n", msg)
}
