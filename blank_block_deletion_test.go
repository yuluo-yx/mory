package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestBackspaceRemovesBlankBlocksBeforeList(t *testing.T) {
	for _, format := range []string{"markdown", "xml"} {
		t.Run(format, func(t *testing.T) {
			a := testApp(t)
			source := "<p/>\n\n##\n\n<p/>\n\n##\n\n1. Open source\n2. Keep item\n\n## Next\n\nBody"
			if format == "xml" {
				d, err := blocks.ParseMarkdown(source)
				if err != nil {
					t.Fatal(err)
				}
				source = d.XML()
			}
			note, err := a.lib.Create("Blank deletion", source, format)
			if err != nil {
				t.Fatal(err)
			}
			a.openNote(note)
			tt := ui.NewTester(a.view, 1280, 1000)
			var item *blocks.Node
			a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
				if n.Kind == "li" && n.Plain() == "Open source" {
					item = n
				}
			})
			if item == nil {
				t.Fatal("missing list item")
			}
			label := "Block text " + item.ID
			click(t, tt, label)
			a.doc.SetSelection(0, 0)
			tt.Frame()
			before, _ := tt.Find(label)
			for remaining := 3; remaining >= 0; remaining-- {
				tt.Key(0, ui.KeyBackspace)
				for range 4 {
					tt.Frame()
				}
				count := 0
				a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
					if (n.Kind == "p" || headingLevel(n) > 0) && n.Plain() == "" {
						count++
					}
				})
				if count != remaining {
					t.Fatalf("blank blocks = %d, want %d; source: %s", count, remaining, a.current.Source)
				}
				if !tt.Focused(label) || a.blockEditor.doc.Find(item.ID).Kind != "li" {
					t.Fatal("deletion changed list identity or focus")
				}
				after, _ := tt.Find(label)
				if after.Y >= before.Y {
					t.Fatalf("blank deletion did not move content up: %v >= %v", after.Y, before.Y)
				}
				before = after
			}
			clean := a.current.Source
			for range 4 {
				tt.Key(ui.Cmd, ui.KeyZ)
			}
			if a.current.Source != source {
				t.Fatalf("undo did not restore exact source: %s", a.current.Source)
			}
			for range 4 {
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			}
			if a.current.Source != clean {
				t.Fatal("redo failed")
			}
			if !a.save() {
				t.Fatal(a.problem)
			}
			a.loadEditors()
			tt.Frame()
			if !strings.Contains(a.current.Source, "Keep item") || !strings.Contains(a.current.Source, "Next") {
				t.Fatal("lost surrounding content")
			}
		})
	}
}

func TestBlankDeletionPreservesHeading(t *testing.T) {
	for _, blank := range []string{"<p/>", "<h2/>"} {
		for _, forward := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/forward=%t", blank, forward), func(t *testing.T) {
				a, tt := xmlApp(t, blank+`<h2>Section</h2><p>Body</p>`)
				heading := firstBlock(a, "h2")
				if blank == "<h2/>" {
					heading = a.blockEditor.doc.Root.Children[1]
				}
				id := heading.ID
				key := ui.KeyBackspace
				label := "Block text " + id
				if forward {
					label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
					key = ui.KeyDelete
				}
				click(t, tt, label)
				end := a.doc.Text().Len()
				a.doc.SetSelection(end, end)
				tt.Frame()
				// Navigation can update the native selection before another frame
				// registers shortcuts. The key must inspect the live caret.
				a.doc.SetSelection(0, 0)
				tt.Key(0, key)
				for range 4 {
					tt.Frame()
				}
				n := a.blockEditor.doc.Find(id)
				if n == nil || n.Kind != "h2" || n.Plain() != "Section" || len(a.blockEditor.doc.Root.Children) != 2 {
					t.Fatal("blank deletion changed heading", a.current.Source)
				}
			})
		}
	}
}

func TestBlankDeletionAcrossListBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, source, target string
		key                  ui.Key
	}{
		{"forward into list", `<p/><ol><li>Keep</li><li>Second</li></ol>`, "", ui.KeyDelete},
		{"forward out of list", `<ul><li>Keep</li></ul><h2/>`, "Keep", ui.KeyDelete},
		{"backspace into list", `<ul><li>Keep</li></ul><p/>`, "", ui.KeyBackspace},
		{"nested list", `<p/><ul><li>Keep<ul><li>Nested</li></ul></li></ul>`, "KeepNested", ui.KeyBackspace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, tt := xmlApp(t, tc.source)
			var target *blocks.Node
			a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
				if blocks.IsText(n) && n.Plain() == tc.target {
					target = n
				}
			})
			if target == nil {
				t.Fatal("missing target")
			}
			click(t, tt, "Block text "+target.ID)
			at := 0
			if tc.key == ui.KeyDelete {
				at = a.doc.Text().Len()
			}
			a.doc.SetSelection(at, at)
			tt.Frame()
			tt.Key(0, tc.key)
			for range 4 {
				tt.Frame()
			}
			if strings.Contains(a.current.Source, "<p/>") || strings.Contains(a.current.Source, "<h2/>") || !strings.Contains(a.current.Source, "Keep") {
				t.Fatal(a.current.Source)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if a.current.Source != tc.source {
				t.Fatal("undo changed list contents", a.current.Source)
			}
		})
	}
}

func TestBlankDeletionRespectsOpaqueAndContainerBoundaries(t *testing.T) {
	for _, source := range []string{
		`<p/><callout><p>Keep</p></callout>`,
		`<p/><table><tr><td><p>Keep</p></td></tr></table>`,
		"<p/>\n\nimport Example from './example';\n\n1. Keep",
	} {
		a := testApp(t)
		a.createNote(source, "Protected boundaries")
		tt := ui.NewTester(a.view, 1280, 1000)
		var target *blocks.Node
		a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
			if blocks.IsText(n) && n.Plain() == "Keep" {
				target = n
			}
		})
		if target == nil {
			t.Fatal("missing target")
		}
		click(t, tt, "Block text "+target.ID)
		a.doc.SetSelection(0, 0)
		tt.Frame()
		tt.Key(0, ui.KeyBackspace)
		if a.current.Source != source {
			t.Fatal("crossed protected boundary", a.current.Source)
		}
	}
}
