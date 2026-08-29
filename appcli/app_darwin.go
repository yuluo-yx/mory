package appcli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (client Client) open(ctx context.Context, document string) error {
	application := client.AppPath
	if application == "" {
		application = "Mory"
	}
	if !strings.HasSuffix(strings.ToLower(application), ".app") {
		if bundle := enclosingAppBundle(application); bundle != "" {
			application = bundle
		}
	}
	if err := exec.CommandContext(ctx, "open", "-a", application, document).Run(); err != nil {
		return fmt.Errorf("open document with Mory: %w", err)
	}
	return nil
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
		path := client.AppPath
		if strings.HasSuffix(strings.ToLower(path), ".app") {
			path = filepath.Join(path, "Contents", "MacOS", "Mory")
		}
		return existingExecutable(path)
	}
	if current, err := os.Executable(); err == nil {
		directory := filepath.Dir(current)
		candidate := filepath.Join(filepath.Dir(filepath.Dir(directory)), "MacOS", "Mory")
		if path, err := existingExecutable(candidate); err == nil {
			return path, nil
		}
	}
	candidates := []string{
		"/Applications/Mory.app/Contents/MacOS/Mory",
		filepath.Join(userHome(), "Applications", "Mory.app", "Contents", "MacOS", "Mory"),
	}
	for _, candidate := range candidates {
		if path, err := existingExecutable(candidate); err == nil {
			return path, nil
		}
	}
	return "", errorsForMissingApp(candidates)
}

func enclosingAppBundle(path string) string {
	clean := filepath.Clean(path)
	marker := ".app" + string(filepath.Separator)
	if index := strings.Index(strings.ToLower(clean), marker); index >= 0 {
		return clean[:index+len(".app")]
	}
	return ""
}

func existingExecutable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("Mory application executable is unavailable: %s", path)
	}
	return path, nil
}

func errorsForMissingApp(candidates []string) error {
	return fmt.Errorf("Mory application was not found; install Mory.app or pass --app (checked %s)", strings.Join(candidates, ", "))
}

func userHome() string {
	home, _ := os.UserHomeDir()
	return home
}
