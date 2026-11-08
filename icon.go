package main

import (
	_ "embed"
	"github.com/egoist/mygo/ui"
	"sync"
)

//go:embed resources/icon.png
var iconPNG []byte
var appIcon = sync.OnceValue(func() *ui.Bitmap { bitmap, _ := ui.DecodeBitmap(iconPNG); return bitmap })
