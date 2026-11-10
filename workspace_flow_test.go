package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestBatchFolderAndRecoveryUI(t *testing.T) {
	a := testApp(t)
	a.createNote("one", "One")
	one := a.current
	a.createNote("two", "Two")
	a.home("All notes")
	tt := ui.NewTester(a.view, 1440, 1100)
	click(t, tt, "New folder")
	click(t, tt, "Folder title")
	tt.Type("Work")
	click(t, tt, "Save folder")
	if len(a.lib.Folders()) != 1 {
		t.Fatal("new folder")
	}
	click(t, tt, "Select notes")
	click(t, tt, "Select all")
	click(t, tt, "Duplicate selected")
	if len(a.lib.Notes) != 4 {
		t.Fatal("batch duplicate")
	}
	click(t, tt, "Select all")
	click(t, tt, "Move selected…")
	click(t, tt, "Folder name")
	tt.Type("Work")
	click(t, tt, "Move")
	if len(a.lib.Query("All notes", "", "Work", "")) != 4 {
		t.Fatal("batch move")
	}
	click(t, tt, "Select all")
	click(t, tt, "Trash selected")
	if len(a.lib.Query("Trash", "", "", "")) != 4 {
		t.Fatal("batch trash")
	}
	click(t, tt, "Trash")
	click(t, tt, "Select all")
	click(t, tt, "Restore selected")
	if len(a.lib.Query("All notes", "", "", "")) != 4 {
		t.Fatal("batch restore")
	}
	a.home("All notes")
	tt.Frame()
	if err := tt.RightClick("Folder Work"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Edit folder…"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Remove folder, keep notes")
	if len(a.lib.Folders()) != 0 {
		t.Fatal("remove folder")
	}
	a.reorderNote(a.lib.Notes[3].ID, one.ID)
	if a.order != "Manual" {
		t.Fatal("manual ordering")
	}
	a.openNote(one)
	a.current.Source = "changed"
	a.loadEditors()
	a.markDirty()
	if !a.save() {
		t.Fatal(a.problem)
	}
	a.revisions, _ = a.lib.History(one.ID)
	a.historyOpen = true
	tt.Frame()
	click(t, tt, "Restore")
	if a.current.Source != "one" {
		t.Fatal("history restore")
	}
	a.current.Source = "Recover me"
	if err := a.lib.Recover(a.current); err != nil {
		t.Fatal(err)
	}
	b, err := newApp(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	rr := ui.NewTester(b.view, 1280, 900)
	click(t, rr, "Recover as a copy")
	if b.current.Source != "Recover me" || b.recoveryOpen {
		t.Fatal("recovery flow")
	}
	a.home("All notes")
	a.trash(one)
	a.home("Trash")
	tt.Frame()
	click(t, tt, "Empty Trash…")
	click(t, tt, "Cancel")
	click(t, tt, "Empty Trash…")
	click(t, tt, "Empty Trash")
	if a.lib.Get(one.ID) != nil {
		t.Fatal("empty trash")
	}
}

func TestExternalReloadConflictAndFolderImport(t *testing.T) {
	a := testApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "Original.md")
	writeTest(t, path, "# Original\n\nFirst")
	a.openExternal(path)
	canonical, _ := filepath.EvalSymlinks(path)
	if a.current.External != canonical || a.assetRoot() != filepath.Dir(canonical) {
		t.Fatal("external source")
	}
	writeTest(t, path, "# Changed elsewhere")
	a.lastDiskCheck = time.Time{}
	a.refreshDisk()
	if !strings.Contains(a.current.Source, "elsewhere") {
		t.Fatal("external reload")
	}
	a.doc.InsertText("Local ")
	a.capture()
	writeTest(t, path, "Competing version")
	a.lastDiskCheck = time.Time{}
	a.refreshDisk()
	if !a.dirty {
		t.Fatal("unsaved edits discarded")
	}
	if a.save() {
		t.Fatal("conflicting save succeeded")
	}
	if recovery, err := a.lib.Recoveries(); err != nil || len(recovery) == 0 {
		t.Fatal("no recovery", err)
	}
	a.dirty = false
	a.problem = ""
	a.lib.Reload(a.current.ID)
	a.loadEditors()
	a.home("All notes")
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTest(t, filepath.Join(dir, "nested", "Local.xml"), "<title>Local</title><p>Imported</p>")
	a.importFolder(dir)
	if a.problem != "" || a.current.Folder != "nested" {
		t.Fatal(a.problem)
	}
	indexed := t.TempDir()
	if err := os.Mkdir(filepath.Join(indexed, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTest(t, filepath.Join(indexed, "docs", "p.xml"), "<title>Parent</title><p>Parent body</p>")
	writeTest(t, filepath.Join(indexed, "docs", "c.md"), "Child body")
	writeTest(t, filepath.Join(indexed, "index.json"), `{"p":{"pinned":true,"created":1234567890000,"modified":1234567890000,"starred":1,"width":"wide"},"c":{"name":"Child","parent":"p","folder":"Imported"}}`)
	a.importFolder(indexed)
	if a.problem != "" {
		t.Fatal(a.problem)
	}
	var parent, child *library.Note
	for _, n := range a.lib.Notes {
		if n.Title == "Parent" {
			parent = n
		}
		if n.Title == "Child" {
			child = n
		}
	}
	if parent == nil || child == nil || child.Parent != parent.ID || !parent.Pinned || !parent.Starred || parent.Width != "Wide" {
		t.Fatal("indexed metadata not mapped")
	}
	a.importFolder(filepath.Join(dir, "missing"))
	if a.problem == "" {
		t.Fatal("missing folder")
	}
	writeTest(t, filepath.Join(indexed, "index.json"), "bad")
	a.problem = ""
	a.importFolder(indexed)
	if a.problem == "" {
		t.Fatal("invalid index")
	}
}

func writeTest(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestThemesShortcutsAndDocumentControls(t *testing.T) {
	a, tt := xmlApp(t, `<title icon="🌱">Document</title><p>Theme preview</p>`)
	click(t, tt, "Settings")
	click(t, tt, "Themes")
	click(t, tt, "Edit CSS")
	a.cssName = "Personal"
	a.cssText = `:root{--theme-name:"Personal"}.page{font-size:18px;color:#345678;background-color:#ffffff} h1{font-size:2em}.unsupported{color:red}`
	tt.Frame()
	click(t, tt, "Apply stylesheet")
	if a.lib.Settings.Theme != "Personal" || len(a.docTheme.Warnings) == 0 {
		t.Fatal("custom theme")
	}
	click(t, tt, "Settings")
	click(t, tt, "Duplicate theme")
	click(t, tt, "Cancel")
	a.settingsOpen = true
	tt.Frame()
	click(t, tt, "Reset theme")
	click(t, tt, "Done")
	a.lib.Settings.Theme = "Craft"
	a.lib.Settings.Appearance = "Dark"
	tt.Frame()
	if !a.pageTheme(ui.DarkTheme()).Dark {
		t.Fatal("dark theme")
	}
	a.lib.Settings.Appearance = "Light"
	tt.Frame()
	a.helpOpen = true
	tt.Frame()
	click(t, tt, "Shortcut New note")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Cmd+Alt+J")
	click(t, tt, "Save shortcuts")
	if a.lib.Settings.Keys["New note"] != "Cmd+Alt+J" {
		t.Fatal("shortcut save")
	}
	before := len(a.lib.Notes)
	tt.Key(ui.Cmd|ui.Alt, ui.KeyJ)
	if len(a.lib.Notes) != before+1 {
		t.Fatal("rebound shortcut inactive")
	}
	a.helpOpen = true
	tt.Frame()
	click(t, tt, "Restore defaults")
	click(t, tt, "Save shortcuts")
	a.helpOpen = true
	a.lib.Settings.Keys["New note"] = "Cmd+S"
	tt.Frame()
	click(t, tt, "Save shortcuts")
	if !strings.Contains(a.problem, "conflict") {
		t.Fatal("duplicate binding accepted")
	}
	a.problem = ""
	a.helpOpen = false
	a.lib.Settings.Keys = map[string]string{}
	tt.Frame()
	for _, key := range []string{"Cmd+A", "Ctrl+Alt+Shift+9", "Cmd+,", "Alt+Space", "Cmd+Left", "Alt+Enter"} {
		if _, _, err := parseKey(key); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"Bad+A", "Cmd+NoSuchKey", ""} {
		if _, _, err := parseKey(key); err == nil {
			t.Fatal("invalid key", key)
		}
	}
	click(t, tt, "Document actions")
	click(t, tt, "Document icon")
	tt.Type("📘")
	click(t, tt, "Document information")
	click(t, tt, "Created date")
	click(t, tt, "History…")
	click(t, tt, "Close")
}

func TestClipboardFormattingAndObjectInsertion(t *testing.T) {
	a := testApp(t)
	a.createNote("Initial", "Clipboard")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.SetClipboard("Plain pasted text")
	tt.Key(ui.Cmd, ui.KeyV)
	if !strings.Contains(a.current.Source, "Plain pasted") {
		t.Fatal("plain paste", a.current.Source)
	}
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte(`<table><tr><td>Cell A</td><td><b>Cell B</b></td></tr></table>`)))))
	a.pasteStructured()
	tt.Frame()
	if a.blockEditor == nil || !strings.Contains(a.current.Source, "Cell A") {
		t.Fatal("HTML paste")
	}
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, pngBytes(t)))))
	a.pasteStructured()
	tt.Frame()
	if firstBlock(a, "img") == nil {
		t.Fatal("PNG paste")
	}
	path := filepath.Join(a.lib.Dir, "file.txt")
	writeTest(t, path, "Attachment")
	files, err := transfer.FileData(path)
	if err != nil {
		t.Fatal(err)
	}
	tt.SetClipboardData(files)
	a.pasteStructured()
	tt.Frame()
	if firstBlock(a, "source") == nil {
		t.Fatal("file paste")
	}
	for _, kind := range []string{"time", "button", "cite", "doc-link"} {
		n := blocks.New(kind)
		n.Set("title", "Example")
		a.insertNodes(blocks.New("p", n))
		tt.Frame()
	}
}

func TestSafeWindowRefreshKeepsNewLibraryEntries(t *testing.T) {
	a := testApp(t)
	a.createNote("Body", "One")
	id := a.current.ID
	a.home("All notes")
	other, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = other.Create("Two", "Other", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	a.lastDiskCheck = time.Time{}
	a.refreshDisk()
	if len(a.lib.Notes) != 2 {
		t.Fatal("new note not visible")
	}
	n := a.lib.Get(id)
	a.openNote(n)
	n.Source = "Local"
	a.loadEditors()
	a.markDirty()
	if err := other.Reload(id); err != nil {
		t.Fatal(err)
	}
	copy := other.Get(id)
	copy.Source = "Other window"
	if err = other.Save(copy); err != nil {
		t.Fatal(err)
	}
	a.lastDiskCheck = time.Time{}
	a.refreshDisk()
	if a.current.Source != "Local" {
		t.Fatal("dirty buffer overwritten")
	}
	if a.save() {
		t.Fatal("stale writer saved")
	}
	recovery, _ := a.lib.Recoveries()
	if len(recovery) == 0 {
		t.Fatal("missing recovery")
	}
	data, _ := json.Marshal(map[string]string{"invalid": "data"})
	if err = os.WriteFile(filepath.Join(a.lib.Dir, "notes", id+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
