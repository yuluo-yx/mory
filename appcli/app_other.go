//go:build !darwin && !windows

package appcli

import (
	"context"
	"errors"
	"os"
)

func publishExport(source, destination string, overwrite bool) error {
	if overwrite {
		return os.Rename(source, destination)
	}
	if err := os.Link(source, destination); err != nil {
		return err
	}
	return os.Remove(source)
}

func (Client) open(context.Context, string) error {
	return errors.New("Mory CLI is supported on macOS and Windows")
}

func (Client) export(context.Context, ExportRequest) error {
	return errors.New("Mory CLI is supported on macOS and Windows")
}
