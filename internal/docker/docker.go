package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsInstalled checks if docker compose is available (either v1 or v2)
func IsInstalled() bool {
	// Check for docker compose v2 (docker CLI integrated)
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command("docker", "compose", "version")
		if cmd.Run() == nil {
			return true
		}
	}

	// Check for docker-compose v1 standalone
	if _, err := exec.LookPath("docker-compose"); err == nil {
		cmd := exec.Command("docker-compose", "--version")
		if cmd.Run() == nil {
			return true
		}
	}

	return false
}

// Run executes docker compose with the given arguments
// Automatically detects v1 vs v2 and constructs the correct command
func Run(args []string) error {
	ctx := context.Background()

	// Detect docker compose version
	binary, useV2 := detectDockerCompose()
	if binary == "" {
		return fmt.Errorf("docker compose not found. Install Docker with Compose plugin, or docker-compose v1")
	}

	var cmd *exec.Cmd
	if useV2 {
		// Docker Compose v2: "docker compose <args>"
		cmd = exec.CommandContext(ctx, binary, append([]string{"compose"}, args...)...)
	} else {
		// Docker Compose v1: "docker-compose <args>"
		cmd = exec.CommandContext(ctx, binary, args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// detectDockerCompose finds the docker compose binary and version
// Returns (binary, isV2)
func detectDockerCompose() (string, bool) {
	// Try docker compose v2 first (integrated in docker CLI)
	if path, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command(path, "compose", "version")
		if err := cmd.Run(); err == nil {
			return path, true // V2 detected
		}
	}

	// Try standalone docker-compose v1
	if path, err := exec.LookPath("docker-compose"); err == nil {
		cmd := exec.Command(path, "--version")
		if err := cmd.Run(); err == nil {
			return path, false // V1 detected
		}
	}

	return "", false
}

// GetVersion returns docker compose version info
func GetVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	binary, useV2 := detectDockerCompose()
	if binary == "" {
		return "", fmt.Errorf("docker compose not found")
	}

	var cmd *exec.Cmd
	if useV2 {
		cmd = exec.CommandContext(ctx, binary, "compose", "version")
	} else {
		cmd = exec.CommandContext(ctx, binary, "--version")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// IsDockerRunning checks if docker daemon is running
func IsDockerRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// FindComposeFile looks for docker-compose.yaml or docker-compose.yml
func FindComposeFile() (string, error) {
	candidates := []string{
		"docker-compose.yaml",
		"docker-compose.yml",
		"compose.yaml",
		"compose.yml",
	}

	for _, name := range candidates {
		path, err := filepath.Abs(name)
		if err != nil {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no compose file found")
}

// GetProjectName returns the project name from compose file
func GetProjectName(composeFile string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	binary, useV2 := detectDockerCompose()
	if binary == "" {
		return "", fmt.Errorf("docker compose not found")
	}

	var cmd *exec.Cmd
	if useV2 {
		cmd = exec.CommandContext(ctx, binary, "compose", "-f", composeFile, "config", "--project-name")
	} else {
		cmd = exec.CommandContext(ctx, binary, "-f", composeFile, "config", "--project-name")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get project name: %s", string(out))
	}

	return strings.TrimSpace(string(out)), nil
}