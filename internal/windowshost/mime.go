package windowshost

import (
	"mime"
	"path/filepath"
	"strings"
)

var portableMIMETypes = map[string]string{
	".bmp":   "image/bmp",
	".gif":   "image/gif",
	".jpeg":  "image/jpeg",
	".jpg":   "image/jpeg",
	".otf":   "font/otf",
	".png":   "image/png",
	".svg":   "image/svg+xml",
	".ttf":   "font/ttf",
	".webp":  "image/webp",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

// mimeTypeForPath 优先使用跨平台固定映射，避免 Windows 注册表改变导出结果。
func mimeTypeForPath(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	if mimeType := portableMIMETypes[extension]; mimeType != "" {
		return mimeType
	}
	if mimeType := mime.TypeByExtension(extension); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}
