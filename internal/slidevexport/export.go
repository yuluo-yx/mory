// Package slidevexport delegates PowerPoint generation to the official Slidev CLI.
package slidevexport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// CodeUnavailable means that no official Slidev CLI installation could be found.
	CodeUnavailable = "slidev_unavailable"
	// CodeFailed means that Slidev started but could not generate a presentation.
	CodeFailed = "slidev_failed"
)

// Request describes one Markdown-to-PowerPoint export.
type Request struct {
	Markdown    string `json:"markdown"`
	SourcePath  string `json:"sourcePath,omitempty"`
	Destination string `json:"destination"`
}

// ExportError exposes a stable code to native hosts while retaining diagnostic detail.
type ExportError struct {
	Code   string
	Detail string
}

func (err *ExportError) Error() string { return err.Detail }

// ErrorCode returns a stable host-facing error code.
func ErrorCode(err error) string {
	var exportError *ExportError
	if errors.As(err, &exportError) {
		return exportError.Code
	}
	return CodeFailed
}

type command struct {
	executable string
	prefix     []string
}

// Runner provides injectable process hooks for deterministic tests.
type Runner struct {
	Getenv        func(string) string
	LookPath      func(string) (string, error)
	UserConfigDir func() (string, error)
	Run           func(context.Context, string, string, []string) ([]byte, error)
}

// Export generates a PPTX by invoking the official Slidev CLI.
func Export(ctx context.Context, request Request) error {
	runner := Runner{
		Getenv:        os.Getenv,
		LookPath:      exec.LookPath,
		UserConfigDir: os.UserConfigDir,
		Run:           runCommand,
	}
	return runner.Export(ctx, request)
}

// Export generates a PPTX with this runner.
func (runner Runner) Export(ctx context.Context, request Request) error {
	if strings.TrimSpace(request.Destination) == "" {
		return &ExportError{Code: CodeFailed, Detail: "PowerPoint destination is required"}
	}
	destination, err := filepath.Abs(request.Destination)
	if err != nil {
		return &ExportError{Code: CodeFailed, Detail: fmt.Sprintf("resolve PowerPoint destination: %v", err)}
	}
	if !strings.EqualFold(filepath.Ext(destination), ".pptx") {
		return &ExportError{Code: CodeFailed, Detail: "PowerPoint destination extension must be .pptx"}
	}

	workingDirectory := filepath.Dir(destination)
	if strings.TrimSpace(request.SourcePath) != "" {
		if source, absoluteErr := filepath.Abs(request.SourcePath); absoluteErr == nil {
			workingDirectory = filepath.Dir(source)
		}
	}
	temporary, err := os.CreateTemp(workingDirectory, ".mory-slidev-*.md")
	if err != nil {
		temporary, err = os.CreateTemp("", "mory-slidev-*.md")
	}
	if err != nil {
		return &ExportError{Code: CodeFailed, Detail: fmt.Sprintf("create temporary Slidev document: %v", err)}
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.WriteString(request.Markdown); err != nil {
		_ = temporary.Close()
		return &ExportError{Code: CodeFailed, Detail: fmt.Sprintf("write temporary Slidev document: %v", err)}
	}
	if err = temporary.Close(); err != nil {
		return &ExportError{Code: CodeFailed, Detail: fmt.Sprintf("close temporary Slidev document: %v", err)}
	}

	cli, err := runner.findCLI(workingDirectory)
	if err != nil {
		return err
	}
	arguments := append([]string{}, cli.prefix...)
	arguments = append(arguments, "export", temporaryPath, "--format", "pptx", "--output", destination, "--with-clicks", "--timeout", "60000")
	if browser := runner.findBrowser(); browser != "" {
		arguments = append(arguments, "--executable-path", browser)
	}
	output, err := runner.Run(ctx, workingDirectory, cli.executable, arguments)
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return &ExportError{Code: CodeFailed, Detail: "Slidev export failed: " + detail}
	}
	info, err := os.Stat(destination)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return &ExportError{Code: CodeFailed, Detail: "Slidev finished without creating a non-empty PPTX file"}
	}
	return nil
}

func (runner Runner) findBrowser() string {
	if runner.Getenv == nil {
		runner.Getenv = os.Getenv
	}
	if override := strings.TrimSpace(runner.Getenv("MORY_SLIDEV_BROWSER")); regularFile(override) {
		return override
	}
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	}
	if programFiles := runner.Getenv("PROGRAMFILES"); programFiles != "" {
		candidates = append(candidates,
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
		)
	}
	if localAppData := runner.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates, filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"))
	}
	for _, candidate := range candidates {
		if regularFile(candidate) {
			return candidate
		}
	}
	return ""
}

func (runner Runner) findCLI(startDirectory string) (command, error) {
	if runner.Getenv == nil {
		runner.Getenv = os.Getenv
	}
	if runner.LookPath == nil {
		runner.LookPath = exec.LookPath
	}
	override := strings.TrimSpace(runner.Getenv("MORY_SLIDEV_BIN"))
	if override != "" {
		return runner.commandForPath(override)
	}
	if runtimeHome := strings.TrimSpace(runner.Getenv("MORY_SLIDEV_HOME")); runtimeHome != "" {
		if cli, ok := runner.runtimeCLI(runtimeHome); ok {
			return cli, nil
		}
	}
	if runner.UserConfigDir == nil {
		runner.UserConfigDir = os.UserConfigDir
	}
	if configDirectory, configErr := runner.UserConfigDir(); configErr == nil {
		if cli, ok := runner.runtimeCLI(filepath.Join(configDirectory, "Mory", "slidev-runtime")); ok {
			return cli, nil
		}
	}

	for directory := startDirectory; ; directory = filepath.Dir(directory) {
		candidate := filepath.Join(directory, "node_modules", "@slidev", "cli", "bin", "slidev.mjs")
		if regularFile(candidate) {
			return runner.nodeCommand(candidate)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}

	if runtime.GOOS != "windows" {
		if executable, err := runner.LookPath("slidev"); err == nil {
			return command{executable: executable}, nil
		}
		for _, candidate := range []string{"/opt/homebrew/bin/slidev", "/usr/local/bin/slidev"} {
			if regularFile(candidate) {
				return command{executable: candidate}, nil
			}
		}
	}
	if appData := runner.Getenv("APPDATA"); appData != "" {
		candidate := filepath.Join(appData, "npm", "node_modules", "@slidev", "cli", "bin", "slidev.mjs")
		if regularFile(candidate) {
			return runner.nodeCommand(candidate)
		}
	}
	return command{}, &ExportError{
		Code:   CodeUnavailable,
		Detail: "official Slidev CLI not found; install @slidev/cli and playwright-chromium",
	}
}

func (runner Runner) runtimeCLI(root string) (command, bool) {
	candidate := filepath.Join(root, "node_modules", "@slidev", "cli", "bin", "slidev.mjs")
	if !regularFile(candidate) {
		return command{}, false
	}
	cli, err := runner.nodeCommand(candidate)
	return cli, err == nil
}

func (runner Runner) commandForPath(value string) (command, error) {
	path, err := filepath.Abs(value)
	if err != nil || !regularFile(path) {
		return command{}, &ExportError{Code: CodeUnavailable, Detail: "MORY_SLIDEV_BIN does not point to a regular file"}
	}
	if extension := strings.ToLower(filepath.Ext(path)); extension == ".js" || extension == ".mjs" || extension == ".cjs" {
		return runner.nodeCommand(path)
	}
	return command{executable: path}, nil
}

func (runner Runner) nodeCommand(script string) (command, error) {
	node, err := runner.LookPath("node")
	if err != nil {
		return command{}, &ExportError{Code: CodeUnavailable, Detail: "Node.js is required to run the installed Slidev CLI"}
	}
	return command{executable: node, prefix: []string{script}}, nil
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func runCommand(ctx context.Context, directory, executable string, arguments []string) ([]byte, error) {
	process := exec.CommandContext(ctx, executable, arguments...)
	process.Dir = directory
	return process.CombinedOutput()
}
