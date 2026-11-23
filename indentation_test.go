package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestBodyTabIndent(t *testing.T) {
	for _, tc := range []struct{ name, source, format string }{
		{"rich", "Body", "markdown"},
		{"empty rich", "", "markdown"},
		{"blocks", "<p>Body</p>", "markdown"},
		{"XML", "<p>Body</p>", "xml"},
		{"empty XML", "<title>Title</title>", "xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			n, err := a.lib.Create("Title", tc.source, tc.format)
			if err != nil {
				t.Fatal(err)
			}
			a.openNote(n)
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, "Document title")
			tt.Key(0, ui.KeyEnter)
			wantText := "Indented " + a.doc.Text().String()
			anchor, caret := a.doc.Selection()
			tt.Key(0, ui.KeyTab)
			if got := a.doc.Formatting().Paragraph.Indent; got != 1 {
				t.Fatalf("Tab did not indent the body: indent=%d titleFocused=%t", got, tt.Focused("Document title"))
			}
			if x, y := a.doc.Selection(); x != anchor || y != caret || tt.Focused("Document title") {
				t.Fatal("Tab moved the caret or focus")
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if a.doc.Formatting().Paragraph.Indent != 0 {
				t.Fatal("undo did not remove indentation")
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			if a.doc.Formatting().Paragraph.Indent != 1 {
				t.Fatal("redo did not restore indentation")
			}
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			if a.doc.Formatting().Paragraph.Indent != 0 || tt.Focused("Document title") {
				t.Fatal("outdent escaped the body or went below zero")
			}
			tt.Key(0, ui.KeyTab)
			tt.Type("Indented ")
			if a.current.Title != "Title" || a.doc.Text().String() != wantText {
				t.Fatalf("typing missed the body: title=%q text=%q", a.current.Title, a.doc.Text().String())
			}
			if !a.save() {
				t.Fatal(a.problem)
			}
			reopened, err := library.Open(a.lib.Dir)
			if err != nil {
				t.Fatal(err)
			}
			a.current = reopened.Get(n.ID)
			a.loadEditors()
			if a.blockEditor == nil {
				t.Fatalf("saved indentation has no structured representation: %q", a.current.Source)
			}
			var found bool
			a.blockEditor.doc.Root.Walk(func(node *blocks.Node) {
				if blocks.IsText(node) && node.Kind != "title" && node.Plain() == wantText {
					found = blocks.ToRich(node, blocks.RichOptions{}).ParagraphStyleAt(0).Indent == 1
				}
			})
			if !found {
				t.Fatalf("saved indentation was lost: %q", a.current.Source)
			}
		})
	}
}

func TestBodyTabSelection(t *testing.T) {
	a := testApp(t)
	a.createNote("# Heading\n\nFirst paragraph\n\nSecond paragraph", "Selection")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Document editor")
	text := a.doc.Text().String()
	start := strings.Index(text, "First")
	end := strings.Index(text, "Second")
	a.doc.SetSelection(start+3, start+3)
	tt.Key(0, ui.KeyTab)
	if a.doc.Text().ParagraphStyleAt(start).Indent != 1 || a.doc.Text().ParagraphStyleAt(end).Indent != 0 || a.doc.Text().ParagraphStyleAt(0).Indent != 0 {
		t.Fatal("Tab changed a paragraph outside the caret")
	}
	a.doc.SetSelection(start, len([]rune(text)))
	tt.Key(0, ui.KeyTab)
	if a.doc.Text().ParagraphStyleAt(start).Indent != 2 || a.doc.Text().ParagraphStyleAt(end).Indent != 1 {
		t.Fatal("Tab did not indent both selected paragraphs")
	}
	if anchor, caret := a.doc.Selection(); anchor != start || caret != len([]rune(text)) || a.doc.Text().String() != text || !tt.Focused("Document editor") {
		t.Fatal("Tab replaced the selection or lost focus")
	}
	tt.Key(ui.Shift, ui.KeyTab)
	if a.doc.Text().ParagraphStyleAt(start).Indent != 1 || a.doc.Text().ParagraphStyleAt(end).Indent != 0 {
		t.Fatal("Shift+Tab did not outdent the selection")
	}
}

func TestTabOnNewLineBelowHeading(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		a.createNote("# Heading", "Title")
		label := "Document editor"
		if structured {
			if !a.enableBlocks() {
				t.Fatal(a.problem)
			}
			label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
		}
		tt := ui.NewTester(a.view, 1280, 900)
		click(t, tt, label)
		a.doc.SetSelection(a.doc.Text().Len(), a.doc.Text().Len())
		tt.Key(0, ui.KeyEnter)
		tt.Key(0, ui.KeyTab)
		tt.Type("Below heading")
		if a.current.Title != "Title" || a.doc.Formatting().Paragraph.Indent != 1 || a.doc.Formatting().Paragraph.Heading != 0 {
			t.Fatal("Tab below a heading escaped the new body paragraph")
		}
		if !a.save() || !strings.Contains(a.current.Source, "Below heading") || !strings.Contains(a.current.Source, "# Heading") {
			t.Fatalf("editing below a heading changed its title or lost text: %q", a.current.Source)
		}
	}
}

func TestBodyTabLists(t *testing.T) {
	for _, source := range []string{"- Parent\n- Child", "1. Parent\n2. Child", "- [ ] Parent\n- [x] Child"} {
		for _, structured := range []bool{false, true} {
			t.Run(source+"/"+map[bool]string{false: "rich", true: "blocks"}[structured], func(t *testing.T) {
				a := testApp(t)
				a.createNote(source, "List")
				label := "Document editor"
				if structured {
					if !a.enableBlocks() {
						t.Fatal(a.problem)
					}
					a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
						if blocks.IsText(n) && n.Plain() == "Child" {
							label = "Block text " + n.ID
						}
					})
				}
				tt := ui.NewTester(a.view, 1280, 900)
				click(t, tt, label)
				a.doc.SetSelection(a.doc.Text().Len(), a.doc.Text().Len())
				before := a.doc.Formatting().Paragraph
				tt.Key(0, ui.KeyTab)
				after := a.doc.Formatting().Paragraph
				if after.Indent != before.Indent+1 || after.List != before.List || after.Checked != before.Checked || !tt.Focused(label) {
					t.Fatal("Tab lost the list style or focus")
				}
				tt.Key(ui.Shift, ui.KeyTab)
				if a.doc.Formatting().Paragraph.Indent != before.Indent {
					t.Fatal("Shift+Tab did not restore the list indent")
				}
			})
		}
	}
}

func TestBodyTabScope(t *testing.T) {
	for _, mode := range []string{"Write", "Source", "Read"} {
		a := testApp(t)
		a.createNote("<p>Body</p>", "Title")
		a.changeMode(mode)
		tt := ui.NewTester(a.view, 1280, 900)
		click(t, tt, "Document title")
		tt.Key(0, ui.KeyTab)
		if tt.Focused("Document title") || a.current.Source != "<p>Body</p>" || a.current.Title != "Title" {
			t.Fatalf("body shortcut intercepted title navigation in %s", mode)
		}
		if mode == "Read" {
			click(t, tt, "Block text "+a.blockEditor.doc.Root.Children[0].ID)
			tt.Key(0, ui.KeyTab)
			if a.current.Source != "<p>Body</p>" {
				t.Fatal("Tab edited a read-only block")
			}
		}
	}
	for _, mode := range []string{"Write", "Source"} {
		a := testApp(t)
		a.createNote("```go\nvalue\n```", "Code")
		a.changeMode(mode)
		tt := ui.NewTester(a.view, 1280, 900)
		label := "Code editor"
		if mode == "Source" {
			label = "Source editor"
		}
		click(t, tt, label)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("value")
		tt.Key(ui.Cmd, ui.KeyLeft)
		tt.Key(0, ui.KeyTab)
		a.capture()
		got := a.current.Source
		if mode == "Write" {
			got = firstCodeBlock(t, a).Plain()
		}
		if !tt.Focused(label) || strings.TrimLeft(got, " \t") != "value" || got == "value" {
			t.Fatalf("code Tab was intercepted in %s: %q", mode, got)
		}
	}
}
