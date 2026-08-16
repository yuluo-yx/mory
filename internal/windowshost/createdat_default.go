//go:build !windows

package windowshost

import (
	"io/fs"
	"time"
)

func fileCreatedAt(info fs.FileInfo) time.Time { return info.ModTime() }
