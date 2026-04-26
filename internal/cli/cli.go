package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rafitox/secure-compose/internal/compose"
	"github.com/rafitox/secure-compose/internal/config"
	"github.com/rafitox/secure-compose/internal/docker"
	"github.com/rafitox/secure-compose/internal/orchestrator"
	"github.com/rafitox/secure-compose/internal/secrets"
	"github.com/rafitox/secure-compose/internal/age"
)

// Version is set at build time via -X github.com/rafitox/secure-compose/internal/cli.Version=v0.3.0
var Version = "dev"

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
	case "run":
		return runCmd(args)
	case "rotate":
		return rotateCmd()
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
	// Version can be set at build time via ldflags:
	// -X github.com/rafitox/secure-compose/internal/cli.Version=v0.3.0
	if Version != "dev" {
		return Version
	}
	// Fall back to environment variable SECURE_COMPOSE_VERSION
	if v := os.Getenv("SECURE_COMPOSE_VERSION"); v != "" {
		return v
	}
	return "dev"
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
  secure-compose rotate               Re-encrypt all .age files with new passphrase
  secure-compose run <svc> [cmd...]   Run command with secrets injected (no disk write)
  secure-compose up [args]            Decrypt and run docker compose up
  secure-compose down [args]           Run docker compose down and cleanup
  secure-compose exec <svc> <cmd>      Run command in service with decrypted env
  secure-compose restart [args]        Restart services
  secure-compose logs [args]           View logs
  secure-compose build [args]          Build services
  secure-compose -h, --help           Show this help
  secure-compose --version             Show version

Security (Zero-Disk Architecture):
  - Env vars: decrypted to memory only, injected directly into container process
  - File secrets: decrypted to tmpfs (RAM disk), auto-cleanup on exit
  - SecureZero: sensitive data overwritten after use
  - Constant-time comparison: timing-attack resistant passphrase check

Examples:
  secure-compose encrypt
  secure-compose encrypt --secret-file ./secrets/db_password.txt
  secure-compose decrypt
  secure-compose decrypt --stdout | jq -r '.DB_PASSWORD'
  secure-compose rotate
  secure-compose run postgres psql -U postgres
  secure-compose up -d
  secure-compose up --build
  secure-compose exec postgres psql -U postgres
  secure-compose down
  secure-compose logs -f api

Environment Variables:
  SECURE_COMPOSE_ENV_FILE         Path to .env file (default: .env)
  SECURE_COMPOSE_ENCRYPTED_FILE  Path to .env.age file (default: .env.age)
  SECURE_COMPOSE_SECRET_FILE      Specific secret file for encrypt/decrypt
  SECURE_COMPOSE_PASSPHRASE       Passphrase (for automation, avoid in production)
  SECURE_COMPOSE_NO_TEARDOWN      Set to 1 to skip .env cleanup on exit
  SECURE_COMPOSE_VERSION          Override version string
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

	// Generate a unique session ID for this invocation
	sessionID := fmt.Sprintf("session-%d", os.Getpid())

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

	// Create orchestrator for this session
	orch := orchestrator.New(cfg, composeFile, passphrase, sessionID)

	// Discover secrets from compose file
	if err := orch.Discover(); err != nil {
		return err
	}

	// Decrypt env vars from .env.age (in memory only - no disk write)
	if err := orch.DecryptEnv(); err != nil {
		return err
	}

	// Decrypt file secrets to tmpfs mount
	if err := orch.DecryptSecrets(); err != nil {
		return err
	}

	// Setup signal handler for cleanup
	orch.SetupSignalHandler()

	// Run docker compose with injected secrets
	dockerArgs := append([]string{op}, args...)
	fmt.Printf("→ Running: docker compose %s\n", strings.Join(dockerArgs, " "))

	// Build the command
	cmd := exec.Command("docker", dockerArgs...)

	// Inject env vars directly into the process (no disk write)
	if len(orch.EnvSecrets()) > 0 {
		cmd.Env = append(os.Environ(), orch.EnvSecrets().ToSlice()...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	// Cleanup tmpfs after command exits
	orch.Cleanup()

	return nil
}

// runCmd runs a command in a service with decrypted secrets injected.
// Unlike "up" which runs docker compose, "run" directly executes a process
// with env vars injected (Infisical-style, no disk write).
// Usage: secure-compose run [--env-file=<path>] <service> [command...]
func runCmd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: secure-compose run [--env-file=<path>] <service> [command...]\n"+
			"Example: secure-compose run postgres psql -U postgres")
	}

	// Parse --env-file flag if provided
	envFile := getFlagValue(args, "--env-file", "-e")
	if envFile == "" {
		envFile = os.Getenv("SECURE_COMPOSE_ENCRYPTED_FILE")
		if envFile == "" {
			envFile = ".env.age"
		}
	}

	cfg := config.Load()
	cfg.EncryptedFile = envFile

	if err := checkDependencies(); err != nil {
		return err
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

	// Decrypt .env.age into memory only (no disk write)
	if _, err := os.Stat(cfg.EncryptedFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found", cfg.EncryptedFile)
	}

	content, err := age.DecryptContent(cfg.EncryptedFile, passphrase)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	envSecrets, err := secrets.ParseEnvFile(content)
	secrets.SecureZero(content) // Clear raw content immediately
	if err != nil {
		return fmt.Errorf("failed to parse env file: %w", err)
	}

	fmt.Printf("→ Running with %d env var(s) (in memory only)\n", len(envSecrets))

	// Parse service name and remaining command
	service := args[0]
	cmdArgs := args[1:]

	// Build docker compose run command
	dockerArgs := append([]string{"run", "--rm", "-e"}, service)
	dockerArgs = append(dockerArgs, cmdArgs...)

	cmd := exec.Command("docker", dockerArgs...)

	// Inject env vars directly (no disk write)
	cmd.Env = append(os.Environ(), envSecrets.ToSlice()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// rotateCmd re-encrypts all .age files with a new passphrase.
// Usage: secure-compose rotate
func rotateCmd() error {
	if err := checkDependencies(); err != nil {
		return err
	}

	fmt.Printf("→ Rotate passphrase for all .age files\n\n")
	fmt.Printf("⚠  This will re-encrypt all .age files with a new passphrase.\n")
	fmt.Printf("   All team members must be notified of the new passphrase.\n\n")

	// Get current passphrase
	fmt.Printf("→ Enter current passphrase:\n")
	currentPass, err := readPassphrase("Current passphrase: ")
	if err != nil {
		return err
	}

	// Get new passphrase
	fmt.Printf("\n→ Enter new passphrase:\n")
	newPass, err := readPassphrase("New passphrase: ")
	if err != nil {
		return err
	}

	confirm, err := readPassphrase("Confirm new passphrase: ")
	if err != nil {
		return err
	}

	if newPass != confirm {
		return fmt.Errorf("passphrases do not match")
	}

	// Constant-time comparison for passphrase
	if secrets.ConstantTimeCompare(currentPass, newPass) {
		secrets.SecureZero([]byte(currentPass))
		secrets.SecureZero([]byte(newPass))
		secrets.SecureZero([]byte(confirm))
		return fmt.Errorf("current and new passphrase must be different")
	}
	secrets.SecureZero([]byte(confirm))

	// Find all .age files
	ageFiles, err := findAgeFiles(".")
	if err != nil {
		return fmt.Errorf("failed to find .age files: %w", err)
	}

	if len(ageFiles) == 0 {
		fmt.Printf("→ No .age files found\n")
		return nil
	}

	fmt.Printf("\n→ Found %d .age file(s) to re-encrypt\n", len(ageFiles))

	rotated := 0
	for _, ageFile := range ageFiles {
		// Decrypt with current passphrase
		content, err := age.DecryptContent(ageFile, currentPass)
		if err != nil {
			fmt.Printf("⚠  Skipping %s: decryption failed (wrong passphrase?)\n", ageFile)
			continue
		}

		// Re-encrypt with new passphrase
		if err := age.EncryptContent(ageFile, content, newPass); err != nil {
			secrets.SecureZero(content)
			return fmt.Errorf("failed to re-encrypt %s: %w", ageFile, err)
		}

		secrets.SecureZero(content)
		fmt.Printf("→ Rotated: %s\n", ageFile)
		rotated++
	}

	secrets.SecureZero([]byte(currentPass))
	secrets.SecureZero([]byte(newPass))

	printSuccess(fmt.Sprintf("Rotated %d file(s)", rotated))
	fmt.Printf("\n→ Share the new passphrase with your team via 1Password/Vault\n")

	return nil
}

// findAgeFiles recursively finds all .age files in directory.
func findAgeFiles(dir string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		path := dir + "/" + entry.Name()

		// Skip hidden directories and common non-project directories
		if entry.IsDir() && entry.Name()[0] != '.' &&
			entry.Name() != "node_modules" &&
			entry.Name() != "vendor" &&
			entry.Name() != ".git" {
			subFiles, err := findAgeFiles(path)
			if err == nil {
				files = append(files, subFiles...)
			}
		} else if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".age") {
			files = append(files, path)
		}
	}

	return files, nil
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
