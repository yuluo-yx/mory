package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/transfer"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

// macOS supplies a responder-chain fallback for ordinary text fields. Keeping
// it separate lets the app own attachment paste without replacing text editing.
var nativePaste func()

func (a *app) pasteFromMenu() {
	if a.current != nil && a.mode == "Write" && a.textFocused && !a.settingsOpen && !a.renameOpen && !a.linkOpen {
		// Code blocks retain their text editor's paste behavior.
		code := false
		if a.blockEditor != nil {
			n := a.blockEditor.doc.Find(a.blockEditor.active)
			code = n != nil && n.Kind == "pre"
		}
		if !code {
			if a.pasteObjects(a.services.ReadClipboardData()) {
				return
			}
		}
	}
	if nativePaste != nil {
		nativePaste()
	}
}

func (a *app) attachFile(path string) {
	data, err := os.ReadFile(path)
	if a.fail(err) {
		return
	}
	a.insertAsset(data, filepath.Base(path))
}

func (a *app) insertAsset(data []byte, name string) {
	if a.current == nil {
		return
	}
	if len(data) > 32<<20 {
		a.fail(fmt.Errorf("attachment exceeds 32 MiB"))
		return
	}
	sum := sha256.Sum256(data)
	ext := strings.ToLower(filepath.Ext(name))
	dest := fmt.Sprintf("%x%s", sum[:12], ext)
	dir := filepath.Join(a.assetRoot(), "assets")
	if a.fail(os.MkdirAll(dir, 0o755)) || a.fail(library.AtomicWrite(filepath.Join(dir, dest), data)) {
		return
	}
	n := blocks.New("source")
	n.Set("name", name)
	n.Set("href", "assets/"+dest)
	switch ext {
	case ".png", ".jpeg", ".jpg", ".gif", ".webp":
		n.Kind = "img"
		n.Set("caption", name)
	case ".mp4", ".mov", ".webm":
		n.Kind = "video"
	}
	a.insertNodes(n)
}

func (a *app) pasteStructured() {
	if a.current == nil || a.mode != "Write" {
		return
	}
	data := a.services.ReadClipboardData()
	if a.pasteObjects(data) {
		return
	}
	if source, err := data.Read(transfer.HTML); err == nil && len(source) > 0 {
		a.pasteHTML(string(source))
		return
	}
	if text := a.services.ReadClipboard(); text != "" && a.doc != nil {
		a.doc.InsertText(text)
		a.capture()
	}
}

// Prefer original files over thumbnail representations, and image bytes over
// HTML wrappers, so copied images become portable local assets.
func (a *app) pasteObjects(data transfer.Data) bool {
	if source, err := data.Read(transfer.Format("application/vnd.mory.docxml")); err == nil && len(source) > 0 {
		d, err := blocks.ParseXML(string(source))
		if !a.fail(err) {
			a.insertNodes(d.Children...)
		}
		return true
	}
	if files, err := data.Files(); err == nil && len(files) > 0 {
		for _, file := range files {
			a.attachFile(file)
		}
		return true
	}
	if png, err := data.Read(transfer.PNG); err == nil && len(png) > 0 {
		a.insertAsset(png, "Clipboard.png")
		return true
	}
	return false
}

func (a *app) pasteHTML(source string) {
	nodes, err := blocks.ParseHTML(source)
	if a.fail(err) {
		return
	}
	a.insertNodes(nodes.Children...)
}
