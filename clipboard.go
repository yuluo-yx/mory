package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/egoist/mygo/transfer"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
)

// macOS supplies a responder-chain fallback for ordinary text fields. Keeping
// it separate lets the app own attachment paste without replacing text editing.
var nativePaste func()

func (a *app) pasteFromMenu() {
	if a.blockEditor != nil && a.mode == "Write" && (a.textFocused || a.blockEditor.tableSelection != "") && !a.settingsOpen && !a.renameOpen && !a.linkOpen {
		if a.pasteTableCells(a.blockEditor.active, a.services.ReadClipboard()) {
			return
		}
	}
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
	data, err := document.ReadAsset(path)
	if a.fail(err) {
		return
	}
	a.insertAsset(data, filepath.Base(path))
}

func (a *app) insertAsset(data []byte, name string) {
	if n := a.storeAsset(data, name); n != nil {
		a.insertNodes(n)
		a.focusAttachments(n)
	}
}

// A native file dialog releases the old text field. Select the inserted asset
// in the document so immediate Undo/Delete works without an extra mouse click.
func (a *app) focusAttachments(nodes ...*blocks.Node) {
	if s := a.blockEditor; s != nil {
		s.chosen.Clear()
		for _, n := range nodes {
			s.chosen.Add(n.ID)
		}
		s.attachmentFocus = true
	}
}

// Store attachments before editing the document, so a multi-file paste can
// insert all its blocks together and remain a single undoable operation.
func (a *app) storeAsset(data []byte, name string) *blocks.Node {
	if a.current == nil {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(name))
	dest, err := document.StoreAsset(data, name, a.assetRoot())
	if a.fail(err) {
		return nil
	}
	n := blocks.New("source")
	n.Set("name", name)
	n.Set("href", dest)
	switch ext {
	case ".png", ".jpeg", ".jpg", ".gif", ".webp":
		n.Kind = "img"
		n.Set("caption", name)
	case ".mp4", ".mov", ".webm":
		n.Kind = "video"
	}
	return n
}

func (a *app) pasteStructured() {
	if a.current == nil || a.mode != "Write" {
		return
	}
	if a.blockEditor != nil && a.pasteTableCells(a.blockEditor.active, a.services.ReadClipboard()) {
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
		var nodes []*blocks.Node
		for _, file := range files {
			data, err := document.ReadAsset(file)
			if a.fail(err) {
				return true
			}
			n := a.storeAsset(data, filepath.Base(file))
			if n == nil {
				return true
			}
			nodes = append(nodes, n)
		}
		a.insertNodes(nodes...)
		a.focusAttachments(nodes...)
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

// Match the active diagnostic so a dismissed attachment error cannot change
// the presentation of an unrelated error assigned by another editor action.
func (a *app) attachmentProblemText() string {
	e := a.attachmentProblem
	if e == nil || a.problem != e.Error() {
		return ""
	}
	switch e.Reason {
	case "too-large":
		return a.trf("“%s” is %.1f MiB. The limit per attachment is 128 MiB. Resize or compress the file, then try again. The original file has not been changed.", e.Name, float64(e.Size)/(1<<20))
	case "not-file":
		return a.trf("“%s” is not a regular file. Choose an image or attachment file instead of a folder.", e.Name)
	case "write":
		if errors.Is(e, syscall.ENOSPC) {
			return a.trf("There is not enough disk space to save “%s”. Free up space on the document's disk, then try again.", e.Name)
		}
		return a.trf("Could not save “%s” to the document's assets folder. Check available disk space and folder write permissions, then try again.", e.Name)
	default:
		if errors.Is(e, os.ErrNotExist) {
			return a.trf("“%s” could not be found. It may have been moved or deleted. Select the file again from its current location.", e.Name)
		}
		if errors.Is(e, os.ErrPermission) {
			return a.trf("Mory cannot read “%s”. Check the file's access permissions in Finder, then select it again.", e.Name)
		}
		return a.trf("Could not read “%s”. Make sure the file is available on this Mac and can be opened in Finder, then select it again.", e.Name)
	}
}
