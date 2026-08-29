//go:build !darwin && !windows

package appcli

import (
	"context"
	"errors"
)

func (Client) open(context.Context, string) error {
	return errors.New("Mory CLI is supported on macOS and Windows")
}

func (Client) export(context.Context, ExportRequest) error {
	return errors.New("Mory CLI is supported on macOS and Windows")
}
