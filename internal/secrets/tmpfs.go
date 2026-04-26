// Package secrets provides secure secret handling: environment variable
// injection and tmpfs-backed file secrets.
package secrets

import (
	"crypto/subtle"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// EnvMap represents decrypted environment variables in memory.
type EnvMap map[string]string

// ParseEnvFile parses KEY=VALUE content into an EnvMap.
func ParseEnvFile(content []byte) (EnvMap, error) {
	env := make(EnvMap)
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := line[idx+1:]

		if key != "" {
			env[key] = value
		}
	}

	return env, nil
}

// ToSlice converts EnvMap to []string for Cmd.Env.
func (e EnvMap) ToSlice() []string {
	result := make([]string, 0, len(e))
	for k, v := range e {
		result = append(result, k+"="+v)
	}
	return result
}

// ConstantTimeCompare compares two passphrases in constant time.
func ConstantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SecureZero overwrites a byte slice with zeros (best-effort memory clearing).
func SecureZero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// TmpfsManager handles RAM-disk mounting for file secrets.
type TmpfsManager struct {
	mu        sync.Mutex
	mountPath  string
	sessionID  string
	mounted   bool
}

// NewTmpfsManager creates a new tmpfs manager.
func NewTmpfsManager(sessionID string) *TmpfsManager {
	return &TmpfsManager{
		sessionID: sessionID,
	}
}

// MountPath returns the path where secrets are mounted.
func (t *TmpfsManager) MountPath() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mountPath
}

// IsMounted returns whether the tmpfs is currently mounted.
func (t *TmpfsManager) IsMounted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mounted
}

// Setup creates the mount point directory and mounts tmpfs.
func (t *TmpfsManager) Setup() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.mounted {
		return nil
	}

	uid := os.Geteuid()
	t.mountPath = filepath.Join("/run/user", fmt.Sprintf("%d", uid), "secure-compose", t.sessionID)

	if err := os.MkdirAll(t.mountPath, 0700); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	// Mount tmpfs (requires CAP_SYS_ADMIN - may fail without root)
	if err := syscall.Mount("tmpfs", t.mountPath, "tmpfs", 0, "size=1M"); err != nil {
		fmt.Printf("⚠  Warning: tmpfs mount failed (need root?): %v\n", err)
		fmt.Printf("   Falling back to regular directory: %s\n", t.mountPath)
	}

	t.mounted = true
	return nil
}

// WriteSecret writes content to a file in the tmpfs mount.
// Returns the absolute path to the written file.
func (t *TmpfsManager) WriteSecret(name string, content []byte) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.mounted {
		return "", fmt.Errorf("tmpfs not mounted, call Setup first")
	}

	filePath := filepath.Join(t.mountPath, name)

	if err := os.WriteFile(filePath, content, 0600); err != nil {
		return "", fmt.Errorf("failed to write secret %s: %w", name, err)
	}

	return filePath, nil
}

// Cleanup unmounts tmpfs, removes all files, and removes the mount directory.
func (t *TmpfsManager) Cleanup() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.mounted {
		return nil
	}

	// Remove all files in mount path
	if files, err := os.ReadDir(t.mountPath); err == nil {
		for _, f := range files {
			os.Remove(filepath.Join(t.mountPath, f.Name()))
		}
	}

	// Unmount tmpfs
	syscall.Unmount(t.mountPath, 0)

	// Remove mount directory
	os.RemoveAll(t.mountPath)

	t.mounted = false
	return nil
}

// ExecWithEnv runs a command with environment variables injected from EnvMap.
// Decrypted secrets stay in memory and are never written to disk.
func ExecWithEnv(cmd *exec.Cmd, env EnvMap) error {
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}

	cmd.Env = append(cmd.Env, env.ToSlice()...)
	return cmd.Run()
}
