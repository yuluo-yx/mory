package blocks

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/richtext"
)

func TestLosslessAndUndo(t *testing.T) {
	source := `<title icon="x">Title</title>
<p custom="unknown">Hello <b>world</b> &amp; <widget token="abc"/></p>
<grid><column width-ratio="0.3"><p>Left</p></column><column><p>Right</p></column></grid>`
	d, err := Parse(source, "xml")
	if err != nil {
		t.Fatal(err)
	}
	if d.Source() != source {
		t.Fatal("original XML changed")
	}
	p := d.Root.Elements()[1]
	if err = d.Transform(p.ID, func(n *Node) { n.Children[0].Text = "Changed " }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Source(), `<widget token="abc"/>`) {
		t.Fatal("unknown node lost")
	}
	if !d.Undo() || d.Source() != source {
		t.Fatal("undo lost original bytes")
	}
	if !d.Redo() || !strings.Contains(d.Source(), "Changed") {
		t.Fatal("redo")
	}
}

func TestTableOperations(t *testing.T) {
	node := NewTable(3, 3)
	table := ReadTable(node)
	table.At(0, 0).Node.Children = []*Node{Paragraph("A")}
	table.At(1, 1).Node.Children = []*Node{Paragraph("B")}
	if err := table.Merge(Rect{0, 0, 1, 1}); err != nil {
		t.Fatal(err)
	}
	if table.At(1, 1).Rows != 2 || len(table.Cells) != 6 {
		t.Fatal(table.Cells)
	}
	if err := table.InsertRow(1); err != nil {
		t.Fatal(err)
	}
	if table.At(0, 0).Rows != 3 {
		t.Fatal("span did not grow")
	}
	if err := table.InsertColumn(1); err != nil {
		t.Fatal(err)
	}
	table.Write(node)
	again := ReadTable(node)
	if again.Rows != 4 || again.Columns != 4 || again.At(2, 2).Columns != 3 {
		t.Fatal(again)
	}
	if err := again.Split(1, 1); err != nil {
		t.Fatal(err)
	}
	if len(again.Cells) != 16 {
		t.Fatal("split left holes")
	}
	if err := again.DeleteRows(1, 3); err != nil {
		t.Fatal(err)
	}
	if err := again.DeleteColumns(1, 3); err != nil {
		t.Fatal(err)
	}
	if again.Rows != 2 || again.Columns != 2 {
		t.Fatal(again)
	}
	if err := again.MoveRows(0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := again.MoveColumns(0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := again.Resize(0, 220); err != nil {
		t.Fatal(err)
	}
	again.Write(node)
	if ReadTable(node).Widths[0] != 220 {
		t.Fatal("width not persisted")
	}
}

func TestMarkdownAndRichObjects(t *testing.T) {
	source := "# Title\n\n| A | B |\n| --- | --- |\n| a | b |\n\n$$\n\\frac{1}{2}\n$$\n"
	d, err := Parse(source, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if d.Source() != source {
		t.Fatal("Markdown normalized on open")
	}
	var table *Node
	d.Root.Walk(func(n *Node) {
		if n.Kind == "table" {
			table = n
		}
	})
	if table == nil {
		t.Fatal("table missing")
	}
	if err = d.Transform(table.ID, func(n *Node) { t := ReadTable(n); _ = t.Merge(Rect{0, 0, 1, 1}); t.Write(n) }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Source(), `colspan="2"`) {
		t.Fatal(d.Source())
	}
	n := Paragraph("")
	math := New("latex", Text(`\frac{1}{2}`))
	n.Children = []*Node{Text("Formula "), math}
	r := ToRich(n, RichOptions{})
	doc := richtext.NewDocument(r)
	doc.SetSelection(0, 7)
	doc.ToggleBold()
	out := FromRich(doc.Text(), n)
	if !strings.Contains(out[0].XML(), `<latex>\frac{1}{2}</latex>`) {
		t.Fatal(out[0].XML())
	}
}
