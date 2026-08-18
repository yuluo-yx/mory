package mory

import (
	"embed"
	"io/fs"
)

// webAssets contains the editor frontend shared by macOS and Windows.
//
//go:embed Sources/Mory/Web
var webAssets embed.FS

// WebAssets returns a read-only filesystem rooted at index.html.
func WebAssets() fs.FS {
	assets, err := fs.Sub(webAssets, "Sources/Mory/Web")
	if err != nil {
		panic(err)
	}
	return assets
}
