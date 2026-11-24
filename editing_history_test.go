package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestRichReplacementRedoContinuesAtCaret(t *testing.T) {
	for _, route := range []string{"keyboard", "menu", "custom"} {
		t.Run(route, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Before old", "Replacement")
			if route == "custom" {
				a.lib.Settings.Keys["Redo"] = "Ctrl+Shift+Z"
			}
			tt := ui.NewTester(a.view, 1280, 820)
			click(t, tt, "Document editor")
			tt.Key(ui.Ctrl, ui.KeyEnd)
			for range 3 {
				tt.Key(ui.Shift, ui.KeyLeft)
			}
			tt.Type("new")
			tt.Key(ui.Cmd, ui.KeyZ)
			switch route {
			case "menu":
				tt.Command("redo")
			case "custom":
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			default:
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			}
			tt.Type(" continued")
			if !a.save() || a.current.Source != "Before new continued" {
				t.Fatalf("redo left restored text selected: %q (%s)", a.current.Source, a.problem)
			}
		})
	}
}

func TestFormattingRedoKeepsSelection(t *testing.T) {
	a := testApp(t)
	a.createNote("Keep selection", "Formatting")
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyB)
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	anchor, caret := a.doc.Selection()
	if anchor == caret || a.doc.Text().String() != "Keep selection" {
		t.Fatal("formatting redo collapsed its unchanged text selection")
	}
}

func TestReplaceAllIsOneUndoStep(t *testing.T) {
	a := testApp(t)
	a.createNote("猫 **cat** 猫\n\n猫 stays here", "Replace")
	a.findText, a.replaceText = "猫", "小猫🌱"
	a.replaceAll()
	if a.current.Source != "小猫🌱 **cat** 小猫🌱\n\n小猫🌱 stays here" {
		t.Fatal(a.current.Source)
	}
	a.doc.Undo()
	a.capture()
	if a.current.Source != "猫 **cat** 猫\n\n猫 stays here" {
		t.Fatal("replace-all was not restored by one undo", a.current.Source)
	}
	a.doc.Redo()
	a.capture()
	if a.current.Source != "小猫🌱 **cat** 小猫🌱\n\n小猫🌱 stays here" {
		t.Fatal("replace-all was not restored by one redo", a.current.Source)
	}
}

func TestCloseFindReturnsFocusToDocument(t *testing.T) {
	for _, source := range []string{"cat cat", "<p>cat cat</p>"} {
		t.Run(source, func(t *testing.T) {
			a := testApp(t)
			a.createNote(source, "Find focus")
			tt := ui.NewTester(a.view, 1280, 820)
			label := "Document editor"
			if a.structured() {
				label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
			}
			click(t, tt, label)
			tt.Key(ui.Cmd, ui.KeyF)
			click(t, tt, "Find text")
			tt.Type("cat")
			click(t, tt, "Replace text")
			tt.Type("dog")
			click(t, tt, "Replace all")
			click(t, tt, "Close find")
			if !tt.Focused(label) {
				t.Fatal("closing find did not restore body focus")
			}
			tt.Command("undo")
			a.capture()
			if a.current.Source != source {
				t.Fatal("menu undo after closing find did not restore the document", a.current.Source)
			}
		})
	}
}
