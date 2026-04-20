// Package age provides encryption using the age library (filippo.io/age)
// Native Go implementation - no external binaries needed
package age

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
)

// IsInstalled always returns true since we use the native library
func IsInstalled() bool {
	return true
}

// EncryptFile encrypts src file to dst with passphrase
func EncryptFile(src, dst, passphrase string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}

	encrypted, err := Encrypt(content, passphrase)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	if err := os.WriteFile(dst, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}

	return nil
}

// EncryptContent encrypts content and writes to dst
func EncryptContent(dst string, content []byte, passphrase string) error {
	encrypted, err := Encrypt(content, passphrase)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	if err := os.WriteFile(dst, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}

	return nil
}

// DecryptFile decrypts src to dst with passphrase
func DecryptFile(src, dst, passphrase string) error {
	encrypted, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}

	decrypted, err := Decrypt(encrypted, passphrase)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	if err := os.WriteFile(dst, decrypted, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}

	return nil
}

// DecryptContent decrypts file and returns plaintext
func DecryptContent(src string, passphrase string) ([]byte, error) {
	encrypted, err := os.ReadFile(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", src, err)
	}

	return Decrypt(encrypted, passphrase)
}

// Encrypt encrypts data with passphrase using age Scrypt
func Encrypt(data []byte, passphrase string) ([]byte, error) {
	passphrase = strings.TrimSuffix(passphrase, "\n")

	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient: %w", err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close encryptor: %w", err)
	}

	return buf.Bytes(), nil
}

// Decrypt decrypts data with passphrase
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	passphrase = strings.TrimSuffix(passphrase, "\n")

	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	r, err := age.Decrypt(bytes.NewReader(data), identity)
	if err != nil {
		if errors.Is(err, age.ErrIncorrectIdentity) {
			return nil, fmt.Errorf("wrong passphrase")
		}
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted content: %w", err)
	}

	return out, nil
}

// GetVersion returns the age library version
func GetVersion() string {
	return "filippo.io/age v1.3.1"
}

// GuessBinaryPath returns empty since we use native library
func GuessBinaryPath() string {
	return ""
}
