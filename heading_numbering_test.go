package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNextHeadingNumber(t *testing.T) {
	for _, tc := range []struct {
		name     string
		previous []numberedHeading
		level    int
		want     string
	}{
		{"no seed", []numberedHeading{{2, "Overview"}}, 3, ""},
		{"child", []numberedHeading{{2, "1. Overview"}}, 3, "1.1 "},
		{"sibling", []numberedHeading{{2, "1. Overview"}, {3, "1.1 First"}}, 3, "1.2 "},
		{"parent sibling", []numberedHeading{{2, "1. Overview"}, {3, "1.1 First"}}, 2, "2. "},
		{"reset children", []numberedHeading{{2, "1. Old"}, {3, "1.4 Old"}, {2, "2. New"}}, 3, "2.1 "},
		{"unnumbered section", []numberedHeading{{2, "1. Old"}, {2, "New"}}, 3, ""},
		{"skip level", []numberedHeading{{2, "1. Overview"}}, 4, "1.1.1 "},
		{"unicode space", []numberedHeading{{2, "9.\u00a0Overview"}}, 2, "10. "},
		{"no preceding headings", nil, 2, ""},
		{"higher than seed", []numberedHeading{{2, "1. Overview"}}, 1, ""},
		{"invalid number", []numberedHeading{{2, "1.x Overview"}}, 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextHeadingNumber(tc.previous, tc.level); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAutomaticHeadingInput(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, toolbar := range []bool{false, true} {
			a := testApp(t)
			a.createNote("## 1. Overview\n\n### 1.1 First\n\nDraft", "Numbered headings")
			label := "Document editor"
			if structured {
				if !a.enableBlocks() {
					t.Fatal(a.problem)
				}
				root := a.blockEditor.doc.Root
				label = "Block text " + root.Children[len(root.Children)-1].ID
			}
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, label)
			tt.Key(ui.Cmd, ui.KeyEnd)
			for range 5 {
				tt.Key(0, ui.KeyBackspace)
			}
			if toolbar {
				a.setHeading(3)
				tt.Frame()
			} else {
				for _, r := range "### " {
					tt.Type(string(r))
				}
			}
			_, caret := a.doc.Selection()
			start, end := a.doc.Text().ParagraphRange(caret)
			if got := a.doc.Text().Slice(start, end).String(); got != "1.2 " {
				t.Fatalf("structured=%t toolbar=%t: %q", structured, toolbar, got)
			}
			if !structured && !toolbar {
				tt.Key(ui.Cmd, ui.KeyZ)
				if a.doc.Text().Slice(start, a.doc.Text().Len()).String() != "" {
					t.Fatal("undo did not remove the generated prefix")
				}
				tt.Frame()
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				if a.doc.Text().Slice(start, a.doc.Text().Len()).String() != "1.2 " {
					t.Fatal("redo did not restore the prefix")
				}
			}
			tt.Type("Second")
			a.capture()
			if !a.save() {
				t.Fatal(a.problem)
			}
			parsed := a.richText(a.current.Source)
			found := false
			for _, p := range parsed.Paragraphs() {
				if p.Style.Heading == 3 && pText(p) == "1.2 Second" {
					found = true
				}
			}
			if !found {
				t.Fatal("numbered heading did not persist", a.current.Source)
			}
		}
	}
}

func TestInsertedHeadingContinuesNumbers(t *testing.T) {
	a := testApp(t)
	a.createNote("## 1. First", "Insert numbered heading")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyEnd)
	click(t, tt, "Insert block")
	click(t, tt, "Insert Heading")
	if a.doc.Text().String() != "2. " {
		t.Fatalf("inserted heading: %q", a.doc.Text().String())
	}
	tt.Type("Next")
	a.capture()
	if a.doc.Text().String() != "2. Next" {
		t.Fatal("new heading did not retain typing focus", a.doc.Text().String())
	}
}
