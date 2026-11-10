package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestStylesheetEditorDraftAndUndo(t *testing.T) {
	a := testApp(t)
	a.createNote("# Original document\n\nKeep this text.", "Stylesheet editing")
	original := a.current.Source
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Settings")
	click(t, tt, "Themes")
	click(t, tt, "Edit CSS")
	initial := a.cssText
	if a.cssEditor == nil || a.cssEditor.state.Buffer.Language().Name != "CSS" {
		t.Fatal("stylesheet editor has no CSS highlighter")
	}
	const draft = "/* Syntax highlighting */\n:root { --accent: #007aff; }\n.page { color: var(--accent); font-size: 18px; }\nh1 { font-size: 2em; }\n"
	click(t, tt, "Theme CSS")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(draft)
	tt.Frame()
	if a.cssText != draft {
		t.Fatal("stylesheet validation draft did not follow editor input")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if a.cssText != initial || a.current.Source != original {
		t.Fatal("stylesheet undo changed the document or failed to restore the CSS")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	tt.Frame()
	if a.cssText != draft {
		t.Fatal("stylesheet redo did not restore the draft")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		for _, appearance := range []string{"Light", "Dark"} {
			a.lib.Settings.Appearance = appearance
			tt.Frame()
			saveImage(t, tt, filepath.Join(dir, "stylesheet-"+appearance+".png"))
		}
	}
	a.cssName = "Syntax"
	click(t, tt, "Apply stylesheet")
	saved, err := os.ReadFile(filepath.Join(a.lib.Dir, "themes", "syntax.css"))
	if err != nil || string(saved) != draft || a.lib.Settings.Theme != "Syntax" || a.cssEditor != nil {
		t.Fatalf("applied stylesheet does not match the editor: %v", err)
	}
	click(t, tt, "Settings")
	click(t, tt, "Duplicate theme")
	if a.cssText != draft || a.cssEditor == nil {
		t.Fatal("new stylesheet session did not load the current theme")
	}
	click(t, tt, "Theme CSS")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(".page { color: red; }")
	click(t, tt, "Cancel")
	if a.cssEditor != nil || a.current.Source != original {
		t.Fatal("cancel retained the editor or changed the document")
	}
	if _, err := os.Stat(filepath.Join(a.lib.Dir, "themes", "syntax-copy.css")); !os.IsNotExist(err) {
		t.Fatal("cancel saved the draft")
	}
}
