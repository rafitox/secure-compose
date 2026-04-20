package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readPassphrase reads a passphrase from stdin
func readPassphrase(prompt string) (string, error) {
	fmt.Print(prompt + " ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read passphrase: %w", err)
	}
	return strings.TrimSuffix(line, "\n"), nil
}

// fileChecksum calculates SHA256 checksum of a file
func fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// runEditor opens a file in the user's preferred editor
func runEditor(editor, path string) error {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("empty editor command")
	}
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// validateEnvContent checks if content looks like a valid .env file
func validateEnvContent(content []byte) error {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "=") {
			key := strings.SplitN(line, "=", 2)[0]
			key = strings.TrimSpace(key)
			if key == "" {
				return fmt.Errorf("line %d: empty key name", i+1)
			}
			for _, c := range key {
				if !isEnvKeyChar(c) {
					return fmt.Errorf("line %d: invalid key character '%c'", i+1, c)
				}
			}
		} else if line != "" {
			return fmt.Errorf("line %d: malformed (should be KEY=value): %s", i+1, truncate(line, 50))
		}
	}
	return nil
}

func isEnvKeyChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}