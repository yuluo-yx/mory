package appcli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func renderExport(ctx context.Context, request ExportRequest, render func(context.Context, ExportRequest) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := inspectExportDestination(request.Source, request.Destination, request.Overwrite); err != nil {
		return err
	}
	// Stage beside the destination so publishing never crosses filesystem boundaries.
	directory, err := os.MkdirTemp(filepath.Dir(request.Destination), ".mory-export-")
	if err != nil {
		return fmt.Errorf("prepare export: %w", err)
	}
	defer os.RemoveAll(directory)
	staged := request
	staged.Destination = filepath.Join(directory, filepath.Base(request.Destination))
	if err := render(ctx, staged); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(staged.Destination)
	if err != nil {
		return fmt.Errorf("inspect rendered output: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("renderer did not produce a non-empty regular file: %s", request.Destination)
	}
	if err := inspectExportDestination(request.Source, request.Destination, request.Overwrite); err != nil {
		return err
	}
	if err := publishExport(staged.Destination, request.Destination, request.Overwrite); err != nil {
		return fmt.Errorf("publish export %s: %w", request.Destination, err)
	}
	return nil
}
