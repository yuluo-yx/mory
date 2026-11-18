package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestTitleEnterFocusesBody(t *testing.T) {
	for _, tc := range []struct {
		name, source, format, mode string
	}{
		{"empty", "", "markdown", "Write"},
		{"text", "Existing body", "markdown", "Write"},
		{"empty XML", "<title>Original</title>", "xml", "Write"},
		{"XML body", "<title>Original</title><p>Existing body</p>", "xml", "Write"},
		{"code", "```go\nExisting body\n```", "markdown", "Write"},
		{"callout", "> [!NOTE]\n> Existing body", "markdown", "Write"},
		{"non-text", "<hr/>", "xml", "Write"},
		{"source", "Existing body", "markdown", "Source"},
		{"split", "Existing body", "markdown", "Split"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			a.createNote(tc.source, "Original")
			a.current.Format = tc.format
			a.loadEditors()
			a.changeMode(tc.mode)
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, "Document title")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("Authored title")
			a.capture()
			a.flushSource()
			before := a.current.Source
			tt.Key(0, ui.KeyEnter)
			if tt.Focused("Document title") {
				t.Fatal("Enter left focus in the title")
			}
			a.capture()
			a.flushSource()
			if tc.name != "non-text" && a.current.Source != before {
				t.Fatalf("moving into the body changed content: %q -> %q", before, a.current.Source)
			}
			// Do not click or repair focus after Enter, including after the first edit.
			tt.Type("Body probe ")
			tt.Type("continued ")
			if !a.save() {
				t.Fatal(a.problem)
			}
			if a.current.Title != "Authored title" || !strings.Contains(a.current.Source, "Body probe continued ") {
				t.Fatalf("typing did not reach the body: title=%q source=%q", a.current.Title, a.current.Source)
			}
			if strings.Contains(tc.source, "Existing body") && !strings.Contains(a.current.Source, "Body probe continued Existing body") {
				t.Fatalf("existing text was replaced or the caret missed the body start: %q", a.current.Source)
			}
			reopened, err := library.Open(a.lib.Dir)
			if err != nil {
				t.Fatal(err)
			}
			saved := reopened.Get(a.current.ID)
			if saved.Title != a.current.Title || saved.Source != a.current.Source {
				t.Fatal("title-to-body edits did not persist")
			}
		})
	}
}

func TestTitleEnterInReadMode(t *testing.T) {
	a := testApp(t)
	a.createNote("Existing body", "Title")
	a.changeMode("Read")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Document title")
	tt.Key(0, ui.KeyEnter)
	if !tt.Focused("Document title") || a.bodyFocus || a.current.Source != "Existing body" {
		t.Fatal("Read mode requested focus in a hidden editor or changed content")
	}
}
