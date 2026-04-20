package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsInstalled checks if docker is available
func IsInstalled() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// Run executes docker compose with the given arguments
func Run(args []string) error {
	ctx := context.Background()

	// Find docker compose binary
	dockerCompose := findDockerCompose()
	if dockerCompose == "" {
		return fmt.Errorf("docker compose not found. Is Docker installed?")
	}

	cmd := exec.CommandContext(ctx, dockerCompose, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// findDockerCompose finds the docker compose binary (v2 or v1)
func findDockerCompose() string {
	// Try docker compose v2 first (integrated in docker CLI)
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command("docker", "compose", "version")
		if err := cmd.Run(); err == nil {
			return "docker"
		}
	}

	// Try standalone docker-compose v1
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return "docker-compose"
	}

	return ""
}

// GetVersion returns docker compose version info
func GetVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	dockerCompose := findDockerCompose()
	if dockerCompose == "" {
		return "", fmt.Errorf("docker compose not found")
	}

	var cmd *exec.Cmd
	if dockerCompose == "docker" {
		cmd = exec.CommandContext(ctx, "docker", "compose", "version")
	} else {
		cmd = exec.CommandContext(ctx, "docker-compose", "version")
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

	dockerCompose := findDockerCompose()
	if dockerCompose == "" {
		return "", fmt.Errorf("docker compose not found")
	}

	var cmd *exec.Cmd
	if dockerCompose == "docker" {
		cmd = exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "config", "--project-name")
	} else {
		cmd = exec.CommandContext(ctx, "docker-compose", "-f", composeFile, "config", "--project-name")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get project name: %s", string(out))
	}

	return strings.TrimSpace(string(out)), nil
}
