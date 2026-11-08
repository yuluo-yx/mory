package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Quick Look uses macOS' own document, video and audio preview providers.
// Thumbnails are decoded once into the native UI and temporary files removed.
func (a *app) fileThumbnail(path string) *ui.Bitmap {
	if a.win == nil {
		return nil
	}
	key := a.assetRoot() + "/" + path
	if image, ok := a.filePreviews[key]; ok {
		return image
	}
	if a.filePreviews == nil {
		a.filePreviews = map[string]*ui.Bitmap{}
	}
	a.filePreviews[key] = nil
	base := a.assetRoot()
	go func() {
		data := quickLookThumbnail(base, path)
		a.win.Update(func() {
			if len(data) > 0 {
				bitmap, err := ui.DecodeBitmap(data)
				if err == nil {
					a.filePreviews[key] = bitmap
				}
			}
		})
	}()
	return nil
}

func quickLookThumbnail(base, path string) []byte {
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(path))
	if err != nil {
		return nil
	}
	f.Close()
	dir, err := os.MkdirTemp(base, ".preview-")
	if err != nil {
		return nil
	}
	defer func() {
		files, _ := os.ReadDir(dir)
		for _, file := range files {
			if !file.IsDir() {
				_ = os.Remove(filepath.Join(dir, file.Name()))
			}
		}
		_ = os.Remove(dir)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if exec.CommandContext(ctx, "/usr/bin/qlmanage", "-t", "-s", "800", "-o", dir, filepath.Join(base, filepath.FromSlash(path))).Run() != nil {
		return nil
	}
	files, _ := os.ReadDir(dir)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".png") {
			b, _ := os.ReadFile(filepath.Join(dir, file.Name()))
			return b
		}
	}
	return nil
}

func (a *app) quickLook(path string) {
	if a.win == nil {
		return
	}
	root, err := os.OpenRoot(a.assetRoot())
	if a.fail(err) {
		return
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(path))
	if a.fail(err) {
		return
	}
	f.Close()
	absolute := filepath.Join(a.assetRoot(), filepath.FromSlash(path))
	go func() { _ = exec.Command("/usr/bin/qlmanage", "-p", absolute).Run() }()
}
