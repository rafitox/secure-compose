package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// newUnbufferedReader returns a Reader that reads from stdin without sharing
// a buffer with stdout/stderr — avoids the Go stdin/stdout buffering conflict
// that causes EOF when fmt.Print writes to stdout before ReadString runs.
// Uses syscall.Read directly to bypass Go's buffered stdin.
func newUnbufferedReader() *unbufferedReader {
	return &unbufferedReader{}
}

type unbufferedReader struct{}

func (r *unbufferedReader) ReadString(delim byte) (string, error) {
	var buf [1]byte
	var result []byte
	for {
		n, err := os.Stdin.Read(buf[:])
		_ = err // errors are handled by n==0 check
		if n == 0 {
			if len(result) == 0 {
				return "", fmt.Errorf("EOF")
			}
			break
		}
		if buf[0] == delim {
			break
		}
		result = append(result, buf[0])
	}
	return string(result), nil
}

// readPassphrase reads a passphrase from stdin, hiding the input if a TTY is available.
// Falls back to plain echo input in non-TTY contexts (e.g., piped stdin via `script -qc`).
func readPassphrase(prompt string) (string, error) {
	fmt.Print(prompt + " ")

	// Try to read with echo disabled (only works in a real TTY)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", fmt.Errorf("failed to read passphrase: %w", err)
		}
		fmt.Println() // move to next line after hidden input
		return string(pass), nil
	}

	// Fallback for non-TTY (piped stdin via script -qc or similar)
	reader := newUnbufferedReader()
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