package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestInsertEmptyBlocksAcceptOnlyAuthoredText(t *testing.T) {
	for _, name := range []string{"Heading", "Bullet list", "To-do list", "Quote", "Code block", "Callout"} {
		t.Run(name, func(t *testing.T) {
			a := testApp(t)
			a.createNote("", "Empty block")
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, "Document editor")
			click(t, tt, "Insert block")
			click(t, tt, "Insert "+name)
			if a.blockEditor == nil {
				t.Fatal("missing block editor")
			}
			var content strings.Builder
			a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
				if n.Kind == "#text" {
					content.WriteString(n.Text)
				}
			})
			if content.Len() != 0 {
				t.Fatalf("inserted sample text: %q", content.String())
			}
			tt.Type("My own words 世界")
			a.capture()
			if !strings.Contains(a.current.Source, "My own words 世界") {
				t.Fatalf("new block did not receive typing: %q", a.current.Source)
			}
			if !a.save() {
				t.Fatal(a.problem)
			}
			reopened, err := library.Open(a.lib.Dir)
			if err != nil || reopened.Get(a.current.ID).Source != a.current.Source {
				t.Fatal("new block did not persist", err)
			}
			a.loadEditors()
			tt.Frame()
			if !strings.Contains(a.current.Source, "My own words 世界") {
				t.Fatal("reopen lost authored text")
			}
		})
	}
}

func TestEmptyQuoteSurvivesReopening(t *testing.T) {
	a := testApp(t)
	a.createNote("", "Quote")
	a.insertEmptyBlock("Quote")
	if !a.save() {
		t.Fatal(a.problem)
	}
	if strings.TrimSpace(a.current.Source) != ">" {
		t.Fatalf("not an empty quote: %q", a.current.Source)
	}
	a.loadEditors()
	tt := ui.NewTester(a.view, 1280, 900)
	if a.blockEditor == nil {
		click(t, tt, "Document editor")
		if !a.doc.Formatting().Paragraph.Quote {
			t.Fatal("empty quote became a plain paragraph")
		}
	} else {
		var text *blocks.Node
		a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
			if n.Kind == "blockquote" && len(n.Children) > 0 {
				text = n.Children[0]
			}
		})
		if text == nil {
			t.Fatal("empty quote has no editable paragraph")
		}
		click(t, tt, "Block text "+text.ID)
	}
	tt.Type("A real quotation")
	a.capture()
	if !strings.Contains(a.current.Source, "> A real quotation") {
		t.Fatal("reopened quote lost its structure", a.current.Source)
	}
}

func TestInsertEmptyQuoteConsumesSlash(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		a.createNote("Draft", "Slash")
		label := "Document editor"
		if structured {
			if !a.enableBlocks() {
				t.Fatal(a.problem)
			}
			label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
		}
		tt := ui.NewTester(a.view, 1280, 900)
		click(t, tt, label)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("/")
		tt.Frame()
		click(t, tt, "Insert Quote")
		tt.Type("Authored quotation")
		a.capture()
		if strings.Contains(a.current.Source, "/") || !strings.Contains(a.current.Source, "> Authored quotation") {
			t.Fatal("slash remained or text lost", a.current.Source)
		}
	}
}
