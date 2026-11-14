package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCodeBlockUndoRedoRoutes(t *testing.T) {
	for _, route := range []string{"keyboard", "menu", "custom-control"} {
		t.Run(route, func(t *testing.T) {
			a := testApp(t)
			a.createNote("## Title 1\n\n```go\noriginal\n```\n", "Code undo")
			if route == "custom-control" {
				a.lib.Settings.Keys["Undo"] = "Ctrl+Z"
				a.lib.Settings.Keys["Redo"] = "Ctrl+Shift+Z"
			}
			tt := ui.NewTester(a.view, 1280, 900)
			undo := func(redo bool) {
				if route == "menu" {
					if redo {
						tt.Command("redo")
					} else {
						tt.Command("undo")
					}
					return
				}
				mods := ui.Cmd
				if route == "custom-control" {
					mods = ui.Ctrl
				}
				if redo {
					mods |= ui.Shift
				}
				tt.Key(mods, ui.KeyZ)
			}
			check := func(want string) {
				t.Helper()
				tt.Frame()
				a.capture()
				if got := firstCodeBlock(t, a).Plain(); got != want {
					t.Fatalf("code = %q, want %q", got, want)
				}
				if !strings.Contains(a.current.Source, "## Title 1") {
					t.Fatal("undo changed the heading")
				}
			}
			click(t, tt, "Code editor")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("replacement")
			check("replacement")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyBackspace)
			check("")
			undo(false)
			check("replacement")
			undo(false)
			check("original\n")
			undo(true)
			check("replacement")
			undo(true)
			check("")
			if !tt.Focused("Code editor") {
				t.Fatal("undo/redo lost code focus")
			}
			tt.Type("next")
			check("next")
			if !a.save() {
				t.Fatal(a.problem)
			}
		})
	}
}

func TestCodeUndoCanMixKeyboardAndMenu(t *testing.T) {
	a := testApp(t)
	a.createNote("```go\nkeep\n```\n", "Mixed history")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Code editor")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("replace")
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Command("redo")
	a.capture()
	if got := firstCodeBlock(t, a).Plain(); got != "replace" {
		t.Fatalf("menu could not redo keyboard undo: %q", got)
	}
	tt.Command("undo")
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	a.capture()
	if got := firstCodeBlock(t, a).Plain(); got != "replace" {
		t.Fatalf("keyboard could not redo menu undo: %q", got)
	}
}

func TestUndoCodeInsertionFromFocusedEmptyCode(t *testing.T) {
	for _, menu := range []bool{false, true} {
		a := testApp(t)
		a.createNote("## Title 1\n", "Insert undo")
		tt := ui.NewTester(a.view, 1280, 900)
		a.insertEmptyBlock("Code block")
		tt.Frame()
		click(t, tt, "Code editor")
		if menu {
			tt.Command("undo")
		} else {
			tt.Key(ui.Cmd, ui.KeyZ)
		}
		tt.Frame()
		a.capture()
		if strings.Contains(a.current.Source, "```") {
			t.Fatalf("focused empty code swallowed insertion undo (menu=%t): %q", menu, a.current.Source)
		}
	}
}

func TestCodeUndoPreservesCaretAndReadOnlyContent(t *testing.T) {
	a := testApp(t)
	a.createNote("```go\noriginal\n```\n", "Caret")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Code editor")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("changed")
	code := firstCodeBlock(t, a)
	a.blockEditor.fields[code.ID].code.Select(3, 3)
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if caret := a.blockEditor.fields[code.ID].code.Selection().Head; caret != 3 {
		t.Fatalf("undo reset caret to %d", caret)
	}
	a.changeMode("Read")
	tt.Frame()
	before := a.current.Source
	click(t, tt, "Code editor")
	tt.Command("undo")
	tt.Command("redo")
	tt.Key(ui.Cmd, ui.KeyZ)
	a.capture()
	if a.current.Source != before {
		t.Fatal("read-only undo changed content")
	}
}
