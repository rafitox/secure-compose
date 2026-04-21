package compose

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Secret represents a docker secret defined in compose file
type Secret struct {
	Name string // "db_password"
	File string // "./secrets/db_password.txt"
}

// SecretFile returns the encrypted counterpart path
func (s Secret) EncryptedFile() string {
	return s.File + ".age"
}

// ParseSecrets parses a docker-compose.yaml file and extracts secret definitions
// Only returns secrets with `file:` references (not external secrets)
func ParseSecrets(composeFile string) ([]Secret, error) {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	var doc struct {
		Secrets map[string]struct {
			File     string `yaml:"file"`
			External bool   `yaml:"external"`
			Name     string `yaml:"name"`
		} `yaml:"secrets"`
	}

	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	var secrets []Secret
	for name, sec := range doc.Secrets {
		// Skip external secrets (managed outside of compose file)
		if sec.External {
			continue
		}

		// Only include secrets with a file reference
		if sec.File != "" {
			// If secret has both name and file, prefer name for display
			displayName := name
			if sec.Name != "" {
				displayName = sec.Name
			}
			secrets = append(secrets, Secret{
				Name: displayName,
				File: sec.File,
			})
		}
	}

	return secrets, nil
}

// FindComposeFile looks for docker-compose files in the project directory
func FindComposeFile(dir string) string {
	candidates := []string{
		filepath.Join(dir, "docker-compose.yaml"),
		filepath.Join(dir, "docker-compose.yml"),
		filepath.Join(dir, "compose.yaml"),
		filepath.Join(dir, "compose.yml"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// ParseSecretsInDir scans the given directory for compose files and extracts secrets
func ParseSecretsInDir(dir string) ([]Secret, string, error) {
	composeFile := FindComposeFile(dir)
	if composeFile == "" {
		return nil, "", fmt.Errorf("no docker-compose file found in %s", dir)
	}

	secrets, err := ParseSecrets(composeFile)
	if err != nil {
		return nil, "", err
	}

	return secrets, composeFile, nil
}

// ResolveSecretFilePath resolves a secret file path relative to the compose file directory
func ResolveSecretFilePath(composeFile, secretFile string) string {
	// If it's an absolute path, return as-is
	if filepath.IsAbs(secretFile) {
		return secretFile
	}

	// Get the directory of the compose file
	composeDir := filepath.Dir(composeFile)

	// Join with the compose file directory
	return filepath.Join(composeDir, secretFile)
}

// GetSecretsDir returns the most common parent directory for all secrets
// If secrets are in different directories, returns the project root
func GetSecretsDir(secrets []Secret, composeFile string) string {
	if len(secrets) == 0 {
		return filepath.Dir(composeFile)
	}

	// Get compose file directory
	composeDir := filepath.Dir(composeFile)

	// Check if all secrets are in or below the compose directory
	allInComposeDir := true
	for _, secret := range secrets {
		resolved := ResolveSecretFilePath(composeFile, secret.File)
		if !isSubPath(composeDir, resolved) {
			allInComposeDir = false
			break
		}
	}

	if allInComposeDir {
		return composeDir
	}

	// Return the most specific common directory
	return composeDir
}

// isSubPath checks if path is equal to or a subdirectory of base
func isSubPath(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	// If the relative path starts with "..", it's outside base
	return rel != ".." && len(rel) > 0 && rel[0] != '.'
}

// FindEncryptedSecrets returns all secrets that have .age files
func FindEncryptedSecrets(secrets []Secret, composeFile string) []Secret {
	var found []Secret

	for _, secret := range secrets {
		encryptedFile := secret.File + ".age"
		resolvedPath := ResolveSecretFilePath(composeFile, encryptedFile)

		if _, err := os.Stat(resolvedPath); err == nil {
			found = append(found, secret)
		}
	}

	return found
}

// MissingSecrets returns secrets that don't have .age files yet
func MissingSecrets(secrets []Secret, composeFile string) []Secret {
	var missing []Secret

	for _, secret := range secrets {
		encryptedFile := secret.File + ".age"
		resolvedPath := ResolveSecretFilePath(composeFile, encryptedFile)

		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			missing = append(missing, secret)
		}
	}

	return missing
}