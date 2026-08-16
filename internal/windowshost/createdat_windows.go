//go:build windows

package windowshost

import (
	"io/fs"
	"syscall"
	"time"
)

func fileCreatedAt(info fs.FileInfo) time.Time {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, data.CreationTime.Nanoseconds())
	}
	return info.ModTime()
}
