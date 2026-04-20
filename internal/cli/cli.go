package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/rafaelperet/secure-compose/internal/config"
	"github.com/rafaelperet/secure-compose/internal/docker"
	"github.com/rafaelperet/secure-compose/internal/age"
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
  secure-compose decrypt               Decrypt .env.age to .env with passphrase
  secure-compose edit                  Edit encrypted .env file
  secure-compose up [args]            Decrypt and run docker compose up
  secure-compose down [args]           Run docker compose down and cleanup
  secure-compose exec <svc> <cmd>      Run command in service with decrypted env
  secure-compose restart [args]        Restart services
  secure-compose logs [args]           View logs
  secure-compose build [args]          Build services
  secure-compose -h, --help           Show this help
  secure-compose --version             Show version

Security:
  - Uses age with passphrase (scrypt KDF)
  - Team members share the same passphrase
  - No key files to manage
  - .env.age is safe to commit to git

Examples:
  secure-compose encrypt
  secure-compose up -d
  secure-compose up --build
  secure-compose exec postgres-db psql -U postgres
  secure-compose down
  secure-compose logs -f api

Environment Variables:
  SECURE_COMPOSE_ENV_FILE         Path to .env file (default: .env)
  SECURE_COMPOSE_ENCRYPTED_FILE  Path to .env.age file (default: .env.age)
  SECURE_COMPOSE_PASSPHRASE       Passphrase (for automation, avoid in production)
  SECURE_COMPOSE_NO_TEARDOWN      Set to 1 to skip .env cleanup on exit
`)
}

func encryptCmd() error {
	cfg := config.Load()

	if err := checkDependencies(); err != nil {
		return err
	}

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

func decryptCmd() error {
	cfg := config.Load()

	if err := checkDependencies(); err != nil {
		return err
	}

	if _, err := os.Stat(cfg.EncryptedFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found. Run 'secure-compose encrypt' first", cfg.EncryptedFile)
	}

	fmt.Printf("→ Decrypting %s → %s\n", cfg.EncryptedFile, cfg.EnvFile)
	fmt.Printf("→ Enter passphrase:\n")

	passphrase, err := readPassphrase("Passphrase: ")
	if err != nil {
		return err
	}

	if err := age.DecryptFile(cfg.EncryptedFile, cfg.EnvFile, passphrase); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	if err := os.Chmod(cfg.EnvFile, 0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	printSuccess("Decrypted successfully")

	return nil
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

	if _, err := os.Stat(cfg.EncryptedFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found. Run 'secure-compose encrypt' first", cfg.EncryptedFile)
	}

	// Check if SECURE_COMPOSE_PASSPHRASE is set (for automation)
	passphrase := os.Getenv("SECURE_COMPOSE_PASSPHRASE")
	if passphrase == "" {
		fmt.Printf("→ Enter passphrase:\n")
		var err error
		passphrase, err = readPassphrase("Passphrase: ")
		if err != nil {
			return err
		}
	}

	// Decrypt to .env
	fmt.Printf("→ Decrypting secrets...\n")
	if err := age.DecryptFile(cfg.EncryptedFile, cfg.EnvFile, passphrase); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}
	if err := os.Chmod(cfg.EnvFile, 0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Build docker compose command
	dockerArgs := append([]string{op}, args...)
	fmt.Printf("→ Running: docker compose %s\n", strings.Join(dockerArgs, " "))

	if err := docker.Run(dockerArgs); err != nil {
		return err
	}

	// After successful up, warn about .env persistence
	if op == "up" {
		fmt.Printf("\n⚠  .env file is persisted on disk after 'up'.\n")
		fmt.Printf("   The container reads it on startup — if you restart the container\n")
		fmt.Printf("   without the .env file, it will fail.\n")
		fmt.Printf("   Remove it manually when ready:\n")
		fmt.Printf("   rm %s\n\n", cfg.EnvFile)
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
