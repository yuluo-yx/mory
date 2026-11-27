package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestFolderEnterCreatesRenamesAndValidates(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		t.Run(language, func(t *testing.T) {
			a := testApp(t)
			a.lib.Settings.Language = language
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, a.tr("New folder"))
			tt.Type("Nvidia 网络")
			tt.Key(0, ui.KeyEnter)
			if a.folderOpen || !slices.Contains(a.lib.Folders(), "Nvidia 网络") {
				t.Fatal("Enter did not create the focused folder", a.problem)
			}
			tt.KeyRepeat(0, ui.KeyEnter)
			if len(a.lib.Folders()) != 1 || a.problem != "" {
				t.Fatal("repeated Enter created a duplicate or error")
			}
			a.folderFrom, a.folderName, a.folder, a.folderOpen = "Nvidia 网络", "Nvidia 网络", "Nvidia 网络", true
			tt.Frame()
			click(t, tt, a.tr("Folder title"))
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("GPU 网络")
			tt.Key(0, ui.KeyEnter)
			if a.folderOpen || a.folder != "GPU 网络" || slices.Contains(a.lib.Folders(), "Nvidia 网络") {
				t.Fatal("Enter failed to rename folder or follow its scope")
			}
			stored, err := library.Open(a.lib.Dir)
			if err != nil || !slices.Contains(stored.Folders(), "GPU 网络") {
				t.Fatal("folder was not persisted", err)
			}
			a.folderFrom, a.folderName, a.folderOpen = "", "", true
			tt.Frame()
			click(t, tt, a.tr("Folder title"))
			tt.Type("   ")
			tt.Key(0, ui.KeyEnter)
			if !a.folderOpen || a.problem != "folder name is empty" || len(a.lib.Folders()) != 1 {
				t.Fatal("Enter bypassed folder validation", a.problem)
			}
		})
	}
}

func TestFormEnterRespectsIMEAndModifiers(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "New folder")
	tt.Type("Nvidia ")
	tt.Compose("网络", 2)
	tt.Key(0, ui.KeyEnter)
	if !a.folderOpen || len(a.lib.Folders()) != 0 {
		t.Fatal("IME confirmation submitted the form")
	}
	tt.Type("网络")
	for _, mods := range []ui.Modifiers{ui.Shift, ui.Cmd, ui.Ctrl, ui.Alt} {
		tt.Key(mods, ui.KeyEnter)
		if !a.folderOpen || len(a.lib.Folders()) != 0 {
			t.Fatal("modified Enter submitted the form", mods)
		}
	}
	tt.Key(0, ui.KeyEnter)
	if a.folderOpen || !slices.Contains(a.lib.Folders(), "Nvidia 网络") {
		t.Fatal("committed Chinese input did not submit", a.folderName)
	}
}

func TestEnterSubmitsNoteRenameMoveAndLink(t *testing.T) {
	a := testApp(t)
	a.createNote("Link body", "Before")
	note := a.current
	tt := ui.NewTester(a.view, 1280, 900)
	a.target, a.renameTitle, a.renameOpen = note, note.Title, true
	tt.Frame()
	click(t, tt, "Note name")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Renamed 世界")
	tt.Key(0, ui.KeyEnter)
	if a.renameOpen || note.Title != "Renamed 世界" {
		t.Fatal("Enter failed to rename note")
	}
	a.moveOpen = true
	tt.Frame()
	tt.Type("Destination")
	tt.Key(0, ui.KeyEnter)
	if a.moveOpen || note.Folder != "Destination" {
		t.Fatal("Enter failed to move note", note.Folder, a.problem)
	}
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("https://example.com/path")
	tt.Key(0, ui.KeyEnter)
	a.capture()
	if a.linkOpen || !strings.Contains(note.Source, "[Link body](https://example.com/path)") {
		t.Fatal("Enter failed to apply link", note.Source)
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
	stored, err := library.Open(a.lib.Dir)
	if err != nil || stored.Get(note.ID).Source != note.Source || stored.Get(note.ID).Title != note.Title || stored.Get(note.ID).Folder != note.Folder {
		t.Fatal("form changes were not persisted", err)
	}
}

func TestEnterValidationAndCancellationKeepDrafts(t *testing.T) {
	a := testApp(t)
	a.createNote("Original body", "Original title")
	note := a.current
	a.target, a.renameTitle, a.renameOpen = note, "Original title", true
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Note name")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("   ")
	tt.Key(0, ui.KeyEnter)
	if !a.renameOpen || a.problem == "" || note.Title != "Original title" {
		t.Fatal("invalid rename bypassed validation")
	}
	click(t, tt, "Dismiss")
	click(t, tt, "Note name")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Canceled draft")
	tt.Key(0, ui.KeyEscape)
	if a.renameOpen || note.Title != "Original title" || note.Source != "Original body" {
		t.Fatal("Escape applied the rename draft")
	}
	a.helpOpen = true
	tt.Frame()
	click(t, tt, "Shortcut New note")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Cmd+S")
	tt.Key(0, ui.KeyEnter)
	if !a.helpOpen || !strings.Contains(a.problem, "conflict") || a.lib.Settings.Keys["New note"] == "Cmd+S" {
		t.Fatal("Enter saved conflicting shortcuts", a.problem)
	}
}

func TestEnterSubmitsBatchMove(t *testing.T) {
	a := testApp(t)
	a.createNote("One body", "One")
	one := a.current
	a.createNote("Two body", "Two")
	two := a.current
	a.home("All notes")
	a.selected = map[string]bool{one.ID: true, two.ID: true}
	a.batchMove, a.moveOpen = true, true
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Folder name")
	tt.Type("Batch destination")
	tt.Key(0, ui.KeyEnter)
	if a.moveOpen || a.batchMove || one.Folder != "Batch destination" || two.Folder != one.Folder || one.Source != "One body" || two.Source != "Two body" {
		t.Fatal("Enter did not move all selected documents intact")
	}
}

func TestEnterAppliesPropertiesAndRejectsInvalidInput(t *testing.T) {
	a, tt := xmlApp(t, `<countdown duration="60"/>`)
	n := firstBlock(a, "countdown")
	a.editBlockProperties(n)
	tt.Frame()
	click(t, tt, "Property duration")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("0")
	tt.Key(0, ui.KeyEnter)
	if !a.blockEditor.properties || a.problem != "Enter a positive number for duration" || a.blockEditor.doc.Find(n.ID).Attr("duration") != "60" {
		t.Fatal("keyboard submission bypassed property validation", a.problem)
	}
	click(t, tt, "Dismiss")
	click(t, tt, "Property duration")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("120")
	tt.Key(0, ui.KeyEnter)
	if a.blockEditor.properties || a.blockEditor.doc.Find(n.ID).Attr("duration") != "120" {
		t.Fatal("Enter did not apply latest property value")
	}
	t.Run("code caption survives Markdown reopen", func(t *testing.T) {
		a := testApp(t)
		a.createNote("```go\nKeep code\n```\n", "Code")
		tt := ui.NewTester(a.view, 1280, 900)
		n := firstBlock(a, "pre")
		a.editBlockProperties(n)
		tt.Frame()
		click(t, tt, "Property caption")
		tt.Type("Saved caption 世界")
		tt.Key(0, ui.KeyEnter)
		if a.blockEditor.properties || !a.save() {
			t.Fatal("property form did not submit", a.problem)
		}
		a.loadEditors()
		tt.Frame()
		if n := firstBlock(a, "pre"); n == nil || n.Attr("caption") != "Saved caption 世界" || !strings.Contains(n.Plain(), "Keep code") {
			t.Fatal("saved caption was lost", a.current.Source)
		}
	})
}

func TestEnterSubmitsThemeAndShortcutFields(t *testing.T) {
	a := testApp(t)
	a.cssName, a.cssText, a.cssOpen = "draft", ".page{color:#123456}", true
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Theme CSS")
	tt.Key(ui.Cmd, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	if !a.cssOpen || !strings.HasSuffix(a.cssText, "\n") {
		t.Fatal("Enter submitted the multiline stylesheet")
	}
	click(t, tt, "Theme name")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Keyboard theme")
	tt.Key(0, ui.KeyEnter)
	if a.cssOpen || a.lib.Settings.Theme != "Keyboard theme" {
		t.Fatal("Enter did not save theme name", a.problem)
	}
	if data, err := os.ReadFile(filepath.Join(a.lib.Dir, "themes", "keyboard theme.css")); err != nil || string(data) != a.cssText {
		t.Fatal("stylesheet did not persist", err)
	}
	a.helpOpen = true
	tt.Frame()
	click(t, tt, "Shortcut New note")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Ctrl+Alt+N")
	tt.Key(0, ui.KeyEnter)
	if a.helpOpen || a.lib.Settings.Keys["New note"] != "Ctrl+Alt+N" {
		t.Fatal("Enter did not save shortcut draft", a.problem)
	}
	stored, err := library.Open(a.lib.Dir)
	if err != nil || stored.Settings.Keys["New note"] != "Ctrl+Alt+N" {
		t.Fatal("shortcut did not persist", err)
	}
}

func TestFindReplaceEnterAndMultilineEquation(t *testing.T) {
	a := testApp(t)
	a.createNote("one one", "Find")
	a.findOpen = true
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Find text")
	tt.Type("one")
	tt.Key(0, ui.KeyEnter)
	start, end := a.doc.Selection()
	if start != 0 || end != 3 {
		t.Fatal("Enter did not find next match", start, end)
	}
	click(t, tt, "Replace text")
	tt.Type("two")
	tt.Key(0, ui.KeyEnter)
	if a.doc.Text().String() != "two one" {
		t.Fatal("Enter did not replace selected match", a.doc.Text().String())
	}
	a, tt = xmlApp(t, `<latex display="true">x</latex>`)
	click(t, tt, "Edit equation")
	click(t, tt, "LaTeX source")
	tt.Key(ui.Cmd, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	if !a.blockEditor.formulaOpen || !strings.Contains(a.blockEditor.formulaText, "\n") || a.current.Source != `<latex display="true">x</latex>` {
		t.Fatal("Enter submitted or changed the saved multiline equation")
	}
}
