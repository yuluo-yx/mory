// Package appcli validates CLI requests and delegates them to the native Mory application.
package appcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pathologize"
)

var documentExtensions = map[string]bool{
	".md": true, ".markdown": true, ".mmd": true, ".mdown": true, ".mkd": true,
	".txt": true, ".text": true,
}

var exportExtensions = map[string]string{
	"html": ".html",
	"pdf":  ".pdf",
	"png":  ".png",
	"jpeg": ".jpg",
	"pptx": ".pptx",
}

// ExportRequest describes one non-interactive export delegated to the desktop host.
type ExportRequest struct {
	Source      string
	Destination string
	Format      string
}

// DesktopRequest is the private launch contract accepted by the native application hosts.
type DesktopRequest struct {
	Document string
	Export   *ExportRequest
}

// Client locates and starts the installed native Mory application.
type Client struct {
	AppPath string
}

// ResolveDocument returns an absolute path to a supported readable document.
func ResolveDocument(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("document path is required")
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve document path: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("open document %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("document is not a regular file: %s", path)
	}
	if !documentExtensions[strings.ToLower(filepath.Ext(path))] {
		return "", fmt.Errorf("unsupported document extension %q", filepath.Ext(path))
	}
	return path, nil
}

// ResolveExport validates an export request and derives its destination filename.
func ResolveExport(source, format, outputDirectory string, overwrite bool) (ExportRequest, error) {
	source, err := ResolveDocument(source)
	if err != nil {
		return ExportRequest{}, err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	extension, ok := exportExtensions[format]
	if !ok {
		return ExportRequest{}, fmt.Errorf("unsupported export format %q; use html, pdf, png, jpeg, or pptx", format)
	}
	directory, err := filepath.Abs(outputDirectory)
	if err != nil {
		return ExportRequest{}, fmt.Errorf("resolve output directory: %w", err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		return ExportRequest{}, fmt.Errorf("open output directory %s: %w", directory, err)
	}
	if !info.IsDir() {
		return ExportRequest{}, fmt.Errorf("output path is not a directory: %s", directory)
	}
	name := pathologize.Clean(strings.TrimSuffix(filepath.Base(source), filepath.Ext(source)) + extension)
	destination := filepath.Join(directory, name)
	if !overwrite {
		if _, err := os.Stat(destination); err == nil {
			return ExportRequest{}, fmt.Errorf("output already exists: %s (use --force to replace it)", destination)
		} else if !errors.Is(err, os.ErrNotExist) {
			return ExportRequest{}, fmt.Errorf("inspect output %s: %w", destination, err)
		}
	}
	return ExportRequest{Source: source, Destination: destination, Format: format}, nil
}

// ParseDesktopRequest validates file-association launches and private CLI export arguments.
func ParseDesktopRequest(args []string) (DesktopRequest, error) {
	if len(args) == 0 {
		return DesktopRequest{}, nil
	}
	if args[0] != "--mory-cli-export" {
		document, err := ResolveDocument(args[0])
		return DesktopRequest{Document: document}, err
	}

	flags := flag.NewFlagSet("mory desktop export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := flags.String("format", "", "")
	output := flags.String("output", "", "")
	if err := flags.Parse(args[1:]); err != nil {
		return DesktopRequest{}, fmt.Errorf("parse desktop export arguments: %w", err)
	}
	if flags.NArg() != 1 {
		return DesktopRequest{}, errors.New("desktop export requires one source document")
	}
	source, err := ResolveDocument(flags.Arg(0))
	if err != nil {
		return DesktopRequest{}, err
	}
	formatValue := strings.ToLower(strings.TrimSpace(*format))
	extension, ok := exportExtensions[formatValue]
	if !ok {
		return DesktopRequest{}, fmt.Errorf("unsupported export format %q", formatValue)
	}
	if strings.TrimSpace(*output) == "" {
		return DesktopRequest{}, errors.New("desktop export output path is required")
	}
	destination, err := filepath.Abs(*output)
	if err != nil {
		return DesktopRequest{}, fmt.Errorf("resolve desktop export output: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(destination), extension) {
		return DesktopRequest{}, fmt.Errorf("desktop export output extension must be %s", extension)
	}
	info, err := os.Stat(filepath.Dir(destination))
	if err != nil || !info.IsDir() {
		return DesktopRequest{}, fmt.Errorf("desktop export output directory is unavailable: %s", filepath.Dir(destination))
	}
	export := &ExportRequest{Source: source, Destination: destination, Format: formatValue}
	return DesktopRequest{Document: source, Export: export}, nil
}

// Open asks the desktop application to open a document.
func (client Client) Open(ctx context.Context, document string) error {
	return client.open(ctx, document)
}

// Export starts a hidden desktop host, waits for rendering, and writes the requested output.
func (client Client) Export(ctx context.Context, request ExportRequest) error {
	return client.export(ctx, request)
}
