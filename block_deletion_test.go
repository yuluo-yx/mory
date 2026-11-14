package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func editableBody(t *testing.T, a *app, tt *ui.Tester) string {
	t.Helper()
	for id, field := range a.blockEditor.fields {
		label := "Block text " + id
		if field.doc != nil {
			if _, ok := tt.AccessByRole(ui.RoleTextField, label); ok {
				return label
			}
		}
	}
	t.Fatal("document has no editable body field")
	return ""
}

func TestDeleteLastCalloutKeepsEditableBody(t *testing.T) {
	a := testApp(t)
	a.createNote("", "Callout")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Document editor")
	tt.Type("/")
	tt.Frame()
	click(t, tt, "Insert Callout")
	if err := tt.RightClick("Block actions callout"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Delete block"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	label := editableBody(t, a, tt)
	click(t, tt, label)
	tt.Type("Writing")
	tt.Type(" after deletion")
	a.capture()
	if !strings.Contains(a.current.Source, "Writing after deletion") || strings.Contains(a.current.Source, "[!NOTE]") {
		t.Fatal(a.current.Source)
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
}

func TestSelectedBlocksRespondToDeleteKeys(t *testing.T) {
	for _, command := range []string{"backspace", "delete", "menu"} {
		t.Run(command, func(t *testing.T) {
			a, tt := xmlApp(t, `<callout><p>Note</p></callout><p>Second</p>`)
			click(t, tt, "Block actions callout")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Frame()
			if a.blockEditor.chosen.Len() != 2 || !tt.HasText("Delete selected blocks") {
				t.Fatal("block selection was not established")
			}
			switch command {
			case "backspace":
				tt.Key(0, ui.KeyBackspace)
			case "delete":
				tt.Key(0, ui.KeyDelete)
			case "menu":
				tt.Command("delete")
			}
			tt.Frame()
			if strings.Contains(a.current.Source, "Note") || strings.Contains(a.current.Source, "Second") {
				t.Fatal("selected blocks survived deletion", a.current.Source)
			}
			if a.blockEditor.chosen.Len() != 0 {
				t.Fatal("selection survived deletion")
			}
			label := editableBody(t, a, tt)
			if !tt.Focused(label) {
				t.Fatal("empty body did not receive focus")
			}
			a.blockUndo(false)
			tt.Frame()
			if !strings.Contains(a.current.Source, "Note") || !strings.Contains(a.current.Source, "Second") {
				t.Fatal("one undo did not restore the complete selection")
			}
			a.blockUndo(true)
			tt.Frame()
			label = editableBody(t, a, tt)
			click(t, tt, label)
			tt.Type("New body")
			a.capture()
			if !strings.Contains(a.current.Source, "New body") {
				t.Fatal("redo left the body uneditable")
			}
		})
	}
}

func TestTextDeletionDoesNotDeleteBlocks(t *testing.T) {
	a, tt := xmlApp(t, `<callout><p>Inside</p></callout><p>Outside</p>`)
	callout := firstBlock(a, "callout")
	text := callout.Children[0]
	click(t, tt, "Block actions callout")
	tt.Key(ui.Cmd, ui.KeyA)
	click(t, tt, "Block text "+text.ID)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	a.capture()
	if a.blockEditor.doc.Find(callout.ID) == nil || !strings.Contains(a.current.Source, "Outside") || strings.Contains(a.current.Source, "Inside") {
		t.Fatal("text deletion removed the container or other blocks", a.current.Source)
	}
	tt.Type("Replacement")
	a.capture()
	if !strings.Contains(a.current.Source, "Replacement") {
		t.Fatal("empty callout is no longer editable")
	}
}

func TestEmptyStructuredBodyDoesNotRewriteOnView(t *testing.T) {
	for _, source := range []string{"", "<title>Title</title>"} {
		a, tt := xmlApp(t, source)
		label := editableBody(t, a, tt)
		click(t, tt, label)
		if a.current.Source != source || a.dirty {
			t.Fatal("viewing empty body rewrote source")
		}
		tt.Type("First")
		tt.Type(" words")
		a.capture()
		if !strings.Contains(a.current.Source, "First words") {
			t.Fatal("empty body did not accept text")
		}
		a.blockUndo(false)
		tt.Frame()
		if a.current.Source != source {
			t.Fatal("undo failed to recover exact empty source", a.current.Source)
		}
	}
}

func TestBlockDeletionToolbarAndSingleSelection(t *testing.T) {
	for _, action := range []string{"Cut blocks", "Delete selected blocks", "Copy then delete", "Single block"} {
		t.Run(action, func(t *testing.T) {
			a, tt := xmlApp(t, `<callout><p>Note</p></callout><p>Keep</p>`)
			click(t, tt, "Block actions callout")
			if action == "Single block" {
				a.blockEditor.chosen.Clear()
				a.blockEditor.chosen.Add(firstBlock(a, "callout").ID)
				tt.Frame()
				tt.Key(0, ui.KeyDelete)
			} else {
				tt.Key(ui.Cmd, ui.KeyA)
				if action == "Copy then delete" {
					click(t, tt, "Copy blocks")
					tt.Key(0, ui.KeyDelete)
				} else {
					click(t, tt, action)
				}
			}
			tt.Frame()
			if strings.Contains(a.current.Source, "Note") {
				t.Fatal("selected callout survived deletion")
			}
			if action == "Single block" && !strings.Contains(a.current.Source, "Keep") {
				t.Fatal("unselected paragraph was deleted")
			}
			tt.Type("Continue")
			a.capture()
			if !strings.Contains(a.current.Source, "Continue") {
				t.Fatal("deletion lost typing focus")
			}
		})
	}
}

func TestReadOnlyBlockSelectionCannotDelete(t *testing.T) {
	const source = `<callout><p>Protected</p></callout><p>Keep</p>`
	a, tt := xmlApp(t, source)
	a.changeMode("Read")
	tt.Frame()
	for _, n := range a.blockEditor.doc.Root.Children {
		a.blockEditor.chosen.Add(n.ID)
	}
	click(t, tt, "Document blocks")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyDelete)
	tt.Key(0, ui.KeyBackspace)
	tt.Command("delete")
	a.capture()
	if a.current.Source != source {
		t.Fatal("read-only selection changed the document")
	}
}
