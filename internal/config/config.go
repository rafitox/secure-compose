package config

import (
	"os"
	"path/filepath"
)

// Config holds secure-compose configuration
type Config struct {
	EnvFile        string // Path to .env file
	EncryptedFile  string // Path to .env.age file
	Passphrase     string // Passphrase (from env var)
	NoTeardown     bool   // Skip .env cleanup on exit
	ProjectRoot    string // Project root directory
}

// Load returns the current configuration
// Environment variables take precedence over defaults
func Load() *Config {
	// Determine project root (where docker-compose.yaml is)
	projectRoot := detectProjectRoot()

	return &Config{
		EnvFile:        getEnv("SECURE_COMPOSE_ENV_FILE", ".env"),
		EncryptedFile:  getEnv("SECURE_COMPOSE_ENCRYPTED_FILE", ".env.age"),
		Passphrase:     os.Getenv("SECURE_COMPOSE_PASSPHRASE"),
		NoTeardown:     os.Getenv("SECURE_COMPOSE_NO_TEARDOWN") == "1",
		ProjectRoot:    projectRoot,
	}
}

// getEnv returns the value of an environment variable or a default
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// detectProjectRoot finds the project root by looking for docker-compose file
func detectProjectRoot() string {
	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	// Walk up the directory tree looking for docker-compose file
	for {
		candidates := []string{
			filepath.Join(dir, "docker-compose.yaml"),
			filepath.Join(dir, "docker-compose.yml"),
			filepath.Join(dir, "compose.yaml"),
			filepath.Join(dir, "compose.yml"),
		}

		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	// Default to current directory
	wd, _ := os.Getwd()
	return wd
}

// FullPaths returns absolute paths for env and encrypted files
func (c *Config) FullPaths() (envFile, encryptedFile string) {
	envFile = c.EnvFile
	encryptedFile = c.EncryptedFile

	// Make relative paths absolute based on project root
	if !filepath.IsAbs(envFile) {
		envFile = filepath.Join(c.ProjectRoot, c.EnvFile)
	}
	if !filepath.IsAbs(encryptedFile) {
		encryptedFile = filepath.Join(c.ProjectRoot, c.EncryptedFile)
	}

	return
}

// Exists checks if both env files exist
func (c *Config) Exists() (bool, bool) {
	envFile, encryptedFile := c.FullPaths()
	_, envErr := os.Stat(envFile)
	_, encErr := os.Stat(encryptedFile)
	return envErr == nil, encErr == nil
}

// HasEncrypted returns true if the encrypted file exists
func (c *Config) HasEncrypted() bool {
	_, encryptedFile := c.FullPaths()
	_, err := os.Stat(encryptedFile)
	return err == nil
}

// HasEnv returns true if the plain env file exists
func (c *Config) HasEnv() bool {
	envFile, _ := c.FullPaths()
	_, err := os.Stat(envFile)
	return err == nil
}
