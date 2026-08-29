package appcli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (client Client) open(ctx context.Context, document string) error {
	executable, err := client.appExecutable()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, executable, document)
	if err := command.Start(); err != nil {
		return fmt.Errorf("open document with Mory: %w", err)
	}
	return command.Process.Release()
}

func (client Client) export(ctx context.Context, request ExportRequest) error {
	executable, err := client.appExecutable()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, executable,
		"--mory-cli-export", "--format", request.Format, "--output", request.Destination, request.Source)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("export with Mory: %w", err)
	}
	return nil
}

func (client Client) appExecutable() (string, error) {
	if client.AppPath != "" {
		return windowsExecutable(client.AppPath)
	}
	if current, err := os.Executable(); err == nil {
		if path, err := windowsExecutable(filepath.Join(filepath.Dir(filepath.Dir(current)), "Mory.exe")); err == nil {
			return path, nil
		}
	}
	candidates := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Mory", "Mory.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Mory Contributors", "Mory", "Mory.exe"),
	}
	for _, candidate := range candidates {
		if path, err := windowsExecutable(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("Mory application was not found; install Mory or pass --app")
}

func windowsExecutable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Mory application executable is unavailable: %s", path)
	}
	return path, nil
}
