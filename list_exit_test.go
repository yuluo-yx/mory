package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestEmptyListExitPreservesSurroundingItems(t *testing.T) {
	for _, kind := range []string{"ul", "ol"} {
		for _, position := range []string{"first", "middle", "last", "only"} {
			t.Run(kind+"/"+position, func(t *testing.T) {
				items := "<li>Before item</li><li></li><li>After item</li>"
				switch position {
				case "first":
					items = "<li></li><li>After item</li>"
				case "last":
					items = "<li>Before item</li><li></li>"
				case "only":
					items = "<li></li>"
				}
				source := fmt.Sprintf("<p>Leading</p><%s>%s</%s><p>Trailing</p>", kind, items, kind)
				a, tt := xmlApp(t, source)
				var empty *blocks.Node
				a.blockEditor.doc.Root.Walk(func(node *blocks.Node) {
					if node.Kind == "li" && node.Plain() == "" {
						empty = node
					}
				})
				if empty == nil {
					t.Fatal("missing empty list fixture")
				}
				label := "Block text " + empty.ID
				click(t, tt, label)
				tt.Key(0, ui.KeyEnter)
				tt.Type("Prose")
				a.capture()
				node := a.blockEditor.doc.Find(empty.ID)
				parent, _ := a.blockEditor.doc.Parent(empty.ID)
				if node == nil || node.Kind != "p" || node.Plain() != "Prose" || parent != a.blockEditor.doc.Root || !tt.Focused(label) {
					t.Fatalf("empty item did not become focused prose: node=%+v parent=%+v", node, parent)
				}
				for _, text := range []string{"Leading", "Trailing"} {
					if !strings.Contains(a.current.Source, text) {
						t.Fatal("exit removed surrounding content", text)
					}
				}
				for _, text := range []string{"Before item", "After item"} {
					if strings.Contains(source, text) != strings.Contains(a.current.Source, text) {
						t.Fatal("exit changed a neighboring list item", text)
					}
				}
				if kind == "ol" && (position == "first" || position == "middle") {
					start := 2
					if position == "middle" {
						start = 3
					}
					lastList := a.blockEditor.doc.Root.Children[len(a.blockEditor.doc.Root.Children)-2]
					if lastList.Kind != "ol" || lastList.Int("start", 1) != start {
						t.Fatal("exit renumbered following ordered items", lastList)
					}
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				tt.Key(ui.Cmd, ui.KeyZ)
				a.capture()
				if a.current.Source != source {
					t.Fatal("undo did not restore the original list", a.current.Source)
				}
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				if !a.save() || a.blockEditor.doc.Find(empty.ID).Kind != "p" {
					t.Fatal("redo did not restore prose", a.problem)
				}
			})
		}
	}
}
