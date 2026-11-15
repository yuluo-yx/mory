package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestProgressiveBlockSelectAll(t *testing.T) {
	for _, route := range []string{"command", "control", "menu"} {
		for _, code := range []bool{false, true} {
			t.Run(route+"/"+map[bool]string{false: "callout", true: "code"}[code], func(t *testing.T) {
				source := `<title>Keep title</title><callout><p>First</p><p>Current</p><p>Last</p></callout><p>Outside</p>`
				if code {
					source = `<title>Keep title</title><pre><code>Current</code></pre><p>Outside</p>`
				}
				a, tt := xmlApp(t, source)
				label := "Code editor"
				if !code {
					label = "Block text " + firstBlock(a, "callout").Children[1].ID
				}
				click(t, tt, label)
				selectAll := func() {
					if route == "menu" {
						tt.Command("selectAll")
					} else if route == "control" {
						tt.CodeDown(ui.Ctrl, ui.CodeControlLeft)
						tt.Key(ui.Ctrl, ui.KeyA)
						tt.CodeUp(0, ui.CodeControlLeft)
					} else {
						tt.CodeDown(ui.Cmd, ui.CodeSuperLeft)
						tt.Key(ui.Cmd, ui.KeyA)
						tt.CodeUp(0, ui.CodeSuperLeft)
					}
					tt.Frame()
				}
				selectAll()
				if a.blockEditor.chosen.Len() != 0 || !tt.HasText(selectAllHint) {
					t.Fatal("first select all must stay local and display a hint", a.status)
				}
				// Copy checks the actual editor selection, not just application state.
				tt.Command("copy")
				if got := tt.Clipboard(); got != "Current" {
					t.Fatalf("local selection copied %q", got)
				}
				selectAll() // Copy resets the sequence.
				selectAll()
				if a.blockEditor.chosen.Len() != 2 || !tt.Focused("Document blocks") {
					t.Fatalf("second select all did not select the body: %d", a.blockEditor.chosen.Len())
				}
				tt.Command("copy")
				if got := tt.Clipboard(); !strings.Contains(got, "Current") || !strings.Contains(got, "Outside") || strings.Contains(got, "Keep title") {
					t.Fatalf("document selection copied %q", got)
				}
				tt.Key(0, ui.KeyBackspace)
				a.capture()
				if strings.Contains(a.current.Source, "Current") || strings.Contains(a.current.Source, "Outside") || !strings.Contains(a.current.Source, "Keep title") {
					t.Fatal("body deletion did not respect the selection", a.current.Source)
				}
				a.blockUndo(false)
				tt.Frame()
				if a.current.Source != source {
					t.Fatal("one undo did not restore the body", a.current.Source)
				}
			})
		}
	}
}

func TestProgressiveSelectAllResetsAfterInput(t *testing.T) {
	for _, action := range []string{"caret", "type", "click", "outside", "repeat"} {
		t.Run(action, func(t *testing.T) {
			a, tt := xmlApp(t, `<callout><p>Current</p></callout><p>Outside</p>`)
			id := firstBlock(a, "callout").Children[0].ID
			label := "Block text " + id
			click(t, tt, label)
			tt.Key(ui.Cmd, ui.KeyA)
			switch action {
			case "caret":
				tt.Key(0, ui.KeyLeft)
			case "type":
				tt.Type("Replacement")
			case "click":
				click(t, tt, label)
			case "outside":
				click(t, tt, "Document title")
				if a.blockEditor.selectAllID != "" {
					t.Fatal("clicking outside retained the select-all sequence")
				}
				click(t, tt, label)
			case "repeat":
				tt.KeyRepeat(ui.Cmd, ui.KeyA)
				tt.Frame()
				if a.blockEditor.chosen.Len() != 0 {
					t.Fatal("holding select all promoted the selection")
				}
				tt.Key(0, ui.KeyLeft)
			}
			tt.Key(ui.Cmd, ui.KeyA)
			if a.blockEditor.chosen.Len() != 0 || a.blockEditor.selectAllID != id {
				t.Fatal("input did not reset the select-all sequence")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			if a.blockEditor.chosen.Len() != 2 {
				t.Fatal("new consecutive pair did not select the body")
			}
		})
	}
}

func TestProgressiveSelectAllIncludesFoldedAndOffscreenBlocks(t *testing.T) {
	a, tt := xmlApp(t, `<p>Start</p><h1 folded="true">Folded</h1><p>Hidden</p><h1>Visible</h1>`+strings.Repeat(`<p>Offscreen</p>`, 400))
	id := firstBlock(a, "p").ID
	click(t, tt, "Block text "+id)
	for range 3 {
		tt.Key(ui.Cmd, ui.KeyA)
	}
	if a.blockEditor.chosen.Len() != 404 || len(a.blockEditor.fields) > maxCachedBlockFields {
		t.Fatalf("select all missed hidden blocks or created unbounded fields: chosen=%d fields=%d", a.blockEditor.chosen.Len(), len(a.blockEditor.fields))
	}
	tt.Key(ui.Ctrl, ui.KeyC)
	if !strings.Contains(tt.Clipboard(), "Hidden") || strings.Count(tt.Clipboard(), "Offscreen") != 400 {
		t.Fatal("copy omitted folded or offscreen content")
	}
	tt.Key(ui.Ctrl, ui.KeyX)
	if len(a.blockEditor.visibleBlocks()) != 0 {
		t.Fatal("cut left selected blocks behind")
	}
}

func TestEmptyBodySelectAllDoesNotSelectLaterInput(t *testing.T) {
	a, tt := xmlApp(t, "")
	click(t, tt, editableBody(t, a, tt))
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("First content")
	if a.blockEditor.selectAllPending || a.blockEditor.chosen.Len() != 0 || !strings.Contains(a.current.Source, "First content") {
		t.Fatal("empty select all affected later typing", a.current.Source)
	}
}
