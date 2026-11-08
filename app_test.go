package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func testApp(t *testing.T) *app {
	t.Helper()
	a, err := newApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func click(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	if err := tt.Click(text); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWritingFlow(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Create note")
	if a.current == nil {
		t.Fatal("new note not opened")
	}
	click(t, tt, "Document title")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Daily thoughts")
	click(t, tt, "Document editor")
	tt.Type("Hello world")
	tt.Key(ui.Cmd, ui.KeyA)
	click(t, tt, "Bold")
	tt.Key(ui.Cmd, ui.KeyS)
	if !strings.Contains(a.current.Source, "**Hello world**") {
		t.Fatal(a.current.Source)
	}
	id := a.current.ID
	other, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if other.Get(id).Title != "Daily thoughts" || other.Get(id).Source != a.current.Source {
		t.Fatal("saved data differs")
	}
	click(t, tt, "Source")
	if a.mode != "Source" {
		t.Fatal(a.mode)
	}
	click(t, tt, "Source editor")
	tt.Key(ui.Cmd, ui.KeyEnd)
	tt.Type("\n\n## Next\n")
	tt.Key(ui.Cmd, ui.KeyS)
	click(t, tt, "Read")
	click(t, tt, "Write")
	if !strings.Contains(a.doc.Text().String(), "Next") {
		t.Fatal("mode switch lost text")
	}
	click(t, tt, "Back to library")
	if !tt.HasText("Daily thoughts") {
		t.Fatal("card missing")
	}
	click(t, tt, "Favorites")
	if len(a.lib.Query("Favorites", "", "", "")) != 0 {
		t.Fatal("unexpected favorite")
	}
}

func TestStateAndLosslessSwitch(t *testing.T) {
	a := testApp(t)
	source := "# Exact heading\r\n\r\nText  \r\n"
	a.createNote(source, "Exact")
	n := a.current
	a.changeMode("Source")
	a.changeMode("Read")
	a.changeMode("Write")
	a.save()
	if n.Source != source {
		t.Fatal("viewing normalized source", n.Source)
	}
	a.doc.InsertText("New ")
	a.capture()
	if !a.dirty {
		t.Fatal("edit not dirty")
	}
	a.doc.Undo()
	a.capture()
	if !a.save() {
		t.Fatal(a.problem)
	}
	a.insert("\n| A | B |\n| --- | --- |\n| a | b |\n")
	if a.mode != "Write" || a.blockEditor == nil {
		t.Fatal("table did not open in the native block editor")
	}
	a.changeMode("Write")
	if a.mode != "Write" || a.problem != "" {
		t.Fatal("structured write mode failed")
	}
	a.problem = ""
	a.home("All notes")
	if a.current != nil {
		t.Fatal("home")
	}
	a.openNote(n)
	if a.mode != "Write" || a.blockEditor == nil {
		t.Fatal("complex note did not open natively")
	}
	a.duplicate(n)
	if len(a.lib.Notes) != 2 {
		t.Fatal("duplicate")
	}
	copy := a.current
	a.trash(copy)
	if a.current != nil || !copy.Deleted {
		t.Fatal("trash")
	}
	a.mutate(n, func() { n.Starred = true })
	if !n.Starred {
		t.Fatal("mutation")
	}
}

func TestImportsAndPlugins(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(t.TempDir(), "note.xml")
	xml := "<title>Imported</title><h1>Heading</h1><table><tr><td>Cell</td></tr></table>"
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	a.importFile(path)
	if a.current.Format != "xml" || a.current.Title != "Imported" || a.current.Source != xml {
		t.Fatal(a.current)
	}
	if !strings.Contains(a.markdownSource(), "Cell") {
		t.Fatal(a.markdownSource())
	}
	a.insert("x")
	if a.problem != "" || !strings.Contains(a.current.Source, "<p>x</p>") {
		t.Fatal("native XML insertion failed", a.problem)
	}
	a.problem = ""
	a.runPlugin(a.plugins[0])
	if a.problem == "" {
		t.Fatal("XML plugin accepted")
	}
	a.problem = ""
	a.createNote("# Title\n\nText \t\n", "Plugin note")
	for _, p := range a.plugins {
		a.runPlugin(p)
		if a.problem != "" {
			t.Fatal(a.problem)
		}
	}
	if !strings.Contains(a.current.Source, "mory:toc") {
		t.Fatal("TOC missing")
	}
	a.importFile(filepath.Join(t.TempDir(), "missing.md"))
	if a.problem == "" {
		t.Fatal("missing import ignored")
	}
}

func TestConflictStopsNavigation(t *testing.T) {
	a := testApp(t)
	a.createNote("First", "A")
	n := a.current
	a.createNote("Second", "B")
	other := a.current
	a.openNote(n)
	l, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	external := l.Get(n.ID)
	external.Source = "external"
	if err = l.Save(external); err != nil {
		t.Fatal(err)
	}
	a.doc.InsertText("local")
	a.openNote(other)
	if a.current != n || !a.dirty || a.problem == "" {
		t.Fatal("conflict discarded edits")
	}
	a.home("All notes")
	if a.current != n {
		t.Fatal("home discarded edits")
	}
}

func TestFindAndParentCycle(t *testing.T) {
	a := testApp(t)
	a.createNote("Alpha 世界 Alpha", "A")
	n := a.current
	a.findText = "Alpha"
	a.doc.SetSelection(0, 0)
	a.findNext()
	start, end := a.doc.Selection()
	if start != 0 || end != 5 {
		t.Fatal(start, end)
	}
	a.findNext()
	start, end = a.doc.Selection()
	if start != 9 || end != 14 {
		t.Fatal(start, end)
	}
	a.findNext()
	start, _ = a.doc.Selection()
	if start != 0 {
		t.Fatal("find did not wrap")
	}
	a.findText = "missing"
	a.findNext()
	if a.status != "No matches" {
		t.Fatal(a.status)
	}
	a.createNote("Child", "B")
	child := a.current
	child.Parent = n.ID
	if a.canParent(n.ID, child.ID) || a.canParent(n.ID, n.ID) || a.canParent(n.ID, "missing") {
		t.Fatal("cycle accepted")
	}
	if !a.canParent(child.ID, n.ID) {
		t.Fatal("valid parent rejected")
	}
}

func TestViewsAndOverlays(t *testing.T) {
	a := testApp(t)
	a.createNote(welcome, "Welcome to Mory")
	a.home("All notes")
	for i, title := range []string{"A gentle morning", "Ideas in motion", "Reading notes", "A small next step", "Project journal"} {
		n, err := a.lib.Create(title, "## A thought worth keeping\n\nA place for everything on your mind.\n", "markdown")
		if err != nil {
			t.Fatal(err)
		}
		n.Starred = i == 0
		n.Pinned = i == 0
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveImage(t, tt, filepath.Join(dir, "home.png"))
	}
	click(t, tt, "Settings")
	click(t, tt, "Larger text")
	click(t, tt, "Smaller text")
	click(t, tt, "Done")
	click(t, tt, "List view")
	click(t, tt, "Grid view")
	click(t, tt, "Plugins")
	click(t, tt, "Reload plugins")
	click(t, tt, "Done")
	a.openNote(a.lib.Notes[0])
	tt.Frame()
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "editor.png"))
	}
	for _, mode := range []string{"Read", "Source", "Split", "Write"} {
		click(t, tt, mode)
	}
	click(t, tt, "Toggle favorite")
	click(t, tt, "Toggle outline")
	click(t, tt, "Toggle outline")
	click(t, tt, "Toggle focus mode")
	click(t, tt, "Toggle focus mode")
	click(t, tt, "Insert block")
	tt.Key(0, ui.KeyEscape)
	a.findOpen = true
	tt.Frame()
	click(t, tt, "Close find")
	a.linkOpen = true
	tt.Frame()
	click(t, tt, "Apply link")
	a.helpOpen = true
	tt.Frame()
	click(t, tt, "Done")
	a.target = a.current
	a.renameTitle = "Renamed"
	a.renameOpen = true
	tt.Frame()
	click(t, tt, "Rename")
	if a.current.Title != "Renamed" {
		t.Fatal("rename")
	}
	a.target = a.current
	a.moveOpen = true
	tt.Frame()
	click(t, tt, "Move")
	a.problem = "An example error"
	tt.Frame()
	click(t, tt, "Dismiss")
	a.lib.Settings.Appearance = "Dark"
	tt.Frame()
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "dark.png"))
	}
	a.dirty = true
	a.dirtySince = time.Now().Add(-time.Second)
	tt.Frame()
	if a.dirty {
		t.Fatal("autosave")
	}
}

func saveImage(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestContextMenus(t *testing.T) {
	for _, action := range []string{"Open", "Rename…", "Move…", "Duplicate", "Favorite", "Pin to top", "Move to Trash"} {
		t.Run(action, func(t *testing.T) {
			a := testApp(t)
			a.createNote("# Heading\n\nText", "Menu note")
			a.home("All notes")
			tt := ui.NewTester(a.view, 1280, 820)
			if err := tt.RightClick("Open Menu note"); err != nil {
				t.Fatal(err)
			}
			if err := tt.ChooseMenuItem(action); err != nil {
				t.Fatal(err)
			}
			if a.problem != "" {
				t.Fatal(a.problem)
			}
		})
	}
	for _, action := range []string{"Restore", "Delete permanently…"} {
		t.Run(action, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Text", "Deleted")
			a.trash(a.current)
			a.home("Trash")
			tt := ui.NewTester(a.view, 1280, 820)
			if err := tt.RightClick("Note Deleted"); err != nil {
				t.Fatal(err)
			}
			if err := tt.ChooseMenuItem(action); err != nil {
				t.Fatal(err)
			}
			if a.deleteOpen {
				click(t, tt, "Delete permanently")
			}
			if action == "Restore" && a.lib.Notes[0].Deleted {
				t.Fatal("restore")
			}
		})
	}
}

func TestFormattingAndFindControls(t *testing.T) {
	a := testApp(t)
	a.createNote("Hello world\n\nSecond paragraph", "Formatting")
	tt := ui.NewTester(a.view, 1280, 820)
	for _, label := range []string{"Bold", "Italic", "Underline", "Strikethrough", "Inline code", "Bullet list", "Numbered list", "Task list", "Quote", "Code block"} {
		a.doc.SetSelection(0, 5)
		tt.Frame()
		click(t, tt, label)
		click(t, tt, label)
	}
	click(t, tt, "Paragraph style")
	click(t, tt, "Heading 2")
	a.capture()
	tt.Frame()
	if a.doc.Formatting().Paragraph.Heading != 2 {
		t.Fatal("heading")
	}
	click(t, tt, "Link")
	a.linkURL = "https://example.com"
	tt.Frame()
	click(t, tt, "Apply link")
	click(t, tt, "Link")
	click(t, tt, "Remove link")
	a.findOpen = true
	a.findText = "world"
	a.replaceText = "earth"
	tt.Frame()
	click(t, tt, "Next")
	click(t, tt, "Replace")
	a.capture()
	if !strings.Contains(a.current.Source, "earth") {
		t.Fatal(a.current.Source)
	}
	click(t, tt, "Close find")
	a.mode = "Write"
	a.doc.SetSelection(0, 0)
	tt.Frame()
	click(t, tt, "Insert block")
	click(t, tt, "Insert Bullet list")
	a.capture()
	if !strings.Contains(a.current.Source, "List item") {
		t.Fatal("insert")
	}
	for _, theme := range []string{"Craft", "Feishu", "Notion", "Obsidian"} {
		a.lib.Settings.Theme = theme
		a.lib.Settings.Appearance = "Light"
		tt.Frame()
		a.lib.Settings.Appearance = "Dark"
		tt.Frame()
	}
}

func TestDocumentActions(t *testing.T) {
	for _, action := range []string{"Favorite", "Pin", "Duplicate", "Move to Trash", "Move…"} {
		t.Run(action, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Content", "Actions")
			a.target = a.current
			a.renameTitle = "Actions"
			a.renameOpen = true
			tt := ui.NewTester(a.view, 1280, 820)
			click(t, tt, action)
			if a.moveOpen {
				a.moveFolder = "Work"
				tt.Frame()
				click(t, tt, "Move")
			}
			if a.problem != "" {
				t.Fatal(a.problem)
			}
		})
	}
	a := testApp(t)
	a.createNote("# Heading\n\nText", "Outline")
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Heading Heading")
	a.changeMode("Read")
	tt.Frame()
	click(t, tt, "Heading Heading")
	a.changeMode("Source")
	tt.Frame()
	click(t, tt, "Heading Heading")
	a.exportMenuOpen()
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	a.home("Recent")
	tt.Frame()
	a.query = "Heading"
	tt.Frame()
	a.folder = "No notes"
	tt.Frame()
	click(t, tt, "Create a blank note")
}

func TestUndoAcrossModesAndSlash(t *testing.T) {
	a := testApp(t)
	a.createNote("Original", "Undo")
	a.doc.SetSelection(0, 0)
	a.doc.InsertText("New ")
	a.capture()
	a.changeMode("Read")
	a.changeMode("Write")
	a.doc.Undo()
	a.capture()
	if strings.Contains(a.current.Source, "New") {
		t.Fatal("mode change discarded undo history")
	}
	a.createNote("", "Slash")
	a.doc.InsertText("/")
	a.capture()
	if !a.insertOpen {
		t.Fatal("slash menu not opened")
	}
	a.insert("## Heading\n")
	if strings.Contains(a.current.Source, "/") {
		t.Fatal("slash retained")
	}
}

func TestShortcutsAndImportFailures(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Key(ui.Cmd, ui.KeyN)
	if a.current == nil {
		t.Fatal("new shortcut")
	}
	tt.Key(ui.Cmd, ui.KeyS)
	tt.Key(ui.Cmd, ui.KeyF)
	if !a.findOpen {
		t.Fatal("find shortcut")
	}
	a.findOpen = false
	tt.Key(ui.Cmd|ui.Shift, ui.KeyL)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyL)
	tt.Key(ui.Cmd, ui.KeyP)
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Cmd, ui.KeyO)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	if a.current != nil {
		t.Fatal("library shortcut")
	}
	for _, fixture := range []struct{ name, text string }{{"invalid.xml", "<broken>"}, {"invalid.md", string([]byte{0xff})}, {"missing-image.md", "![image](missing.png)"}} {
		path := filepath.Join(t.TempDir(), fixture.name)
		if err := os.WriteFile(path, []byte(fixture.text), 0o644); err != nil {
			t.Fatal(err)
		}
		a.importFile(path)
		if a.problem == "" {
			t.Fatal("invalid import accepted", fixture.name)
		}
		a.problem = ""
	}
	large := filepath.Join(t.TempDir(), "large.md")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate((32 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	a.importFile(large)
	if a.problem == "" {
		t.Fatal("oversized file accepted")
	}
	a.loadEditors()
	a.insert("ignored")
	a.runPlugin(a.plugins[0])
	if a.markdownSource() != "" {
		t.Fatal("no document source")
	}
	a.chooseExport("PDF")
	a.chooseAttachment()
	a.reveal(a.lib.Dir)
}

func TestRichImages(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(a.lib.Dir, "assets", "test.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	text := a.richText("![image](assets/test.png)")
	found := false
	for _, run := range text.Runs(0, text.Len()) {
		if run.Attributes.Attachment != nil {
			found = true
			if run.Attributes.Attachment.Image == nil {
				t.Fatal("local image was not decoded")
			}
		}
	}
	if !found {
		t.Fatal("image not parsed")
	}
	a.richText("![image](assets/missing.png)")
	if err = os.WriteFile(filepath.Join(a.lib.Dir, "assets", "bad.png"), []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.richText("![image](assets/bad.png)")
	a.createNote("Text", "Parent")
	parent := a.current
	parent.Folder = "Work"
	a.createNote("Child", "Child")
	a.current.Parent = parent.ID
	a.home("All notes")
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Folder Work")
	click(t, tt, "Open Child")
}

func BenchmarkNativeMillionCharacterEdit(b *testing.B) {
	a, err := newApp(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	a.createNote(strings.Repeat("A paragraph of local writing.\n\n", 33000), "Large document")
	tt := ui.NewTester(a.view, 1280, 820)
	a.doc.SetSelection(0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.doc.InsertText("x")
		a.capture()
		tt.Frame()
	}
}

func TestLargeDocumentSaveBoundary(t *testing.T) {
	a := testApp(t)
	original := strings.Repeat("A calm paragraph.\n\n", 13000)
	a.createNote(original, "Large")
	a.doc.SetSelection(0, 0)
	a.doc.InsertText("Rich edit ")
	a.capture()
	if !a.richPending || !a.dirty {
		t.Fatal("large edit not pending")
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
	if !strings.HasPrefix(a.current.Source, "Rich edit ") {
		t.Fatal("save lost pending rich edit")
	}
	a.changeMode("Source")
	a.code.Select(0, 0)
	a.code.Insert("Source edit ")
	a.capture()
	if !a.codePending {
		t.Fatal("large source edit not pending")
	}
	a.changeMode("Read")
	if !strings.HasPrefix(a.current.Source, "Source edit ") {
		t.Fatal("mode change lost pending source edit")
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
	l, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if l.Get(a.current.ID).Source != a.current.Source {
		t.Fatal("disk does not match editor")
	}
}
