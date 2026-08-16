package mory

import (
	"embed"
	"io/fs"
)

// webAssets 保存 macOS 与 Windows 共用的编辑器前端。
//
//go:embed Sources/Mory/Web
var webAssets embed.FS

// WebAssets 返回以 index.html 为根的只读资源文件系统。
func WebAssets() fs.FS {
	assets, err := fs.Sub(webAssets, "Sources/Mory/Web")
	if err != nil {
		panic(err)
	}
	return assets
}
