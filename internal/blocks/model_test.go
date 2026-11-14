package blocks

import (
	"errors"
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

func TestMarkdownTableRepresentation(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows int
	}{
		{"small", 2}, {"below threshold", 199}, {"at threshold", 200}, {"large", 1201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "| Left | Center | Right |\r\n| :--- | :---: | ---: |\r\n" + strings.Repeat("| 中文 **bold** | escaped \\| pipe | `code` [link](https://example.com) |\r\n", tc.rows-1)
			doc, err := Parse(source, "markdown")
			if err != nil {
				t.Fatal(err)
			}
			n := doc.Root.Children[0]
			table := ReadTable(n)
			if table.Rows != tc.rows || table.Columns != 3 || doc.Source() != source || n.Clone().Markdown() != n.Markdown() {
				t.Fatal("table parsing changed geometry or original source")
			}
			if tc.rows < 200 {
				if n.Compact != nil || len(table.Cells) != tc.rows*3 {
					t.Fatal("ordinary table no longer exposes editable cells")
				}
				return
			}
			c := table.Compact
			if c == nil || len(c.Cells) != tc.rows*3 || len(n.Children) != 0 || c.Cells[0] != "Left" || c.Align[1] != "center" || c.Align[2] != "right" {
				t.Fatal("large table lost its header, alignment or compact representation")
			}
			for row := 1; row < tc.rows; row++ {
				if c.Cells[row*3] != "中文 **bold**" || c.Cells[row*3+1] != "escaped \\| pipe" || c.Cells[row*3+2] != "`code` [link](https://example.com)" {
					t.Fatalf("row %d lost content or escaped cell separators", row)
				}
			}
		})
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

func TestAtomicBlockTransactions(t *testing.T) {
	d, _ := Parse("<p>A</p><p>B</p><p>C</p>", "xml")
	original := d.Source()
	a, b := d.Root.Children[0], d.Root.Children[1]
	if d.Undo() || d.Redo() {
		t.Fatal("empty history")
	}
	d.Coalesce()
	if err := d.Move(a.ID, d.Root.ID, 3); err != nil {
		t.Fatal(err)
	}
	if d.Root.Elements()[2].Plain() != "A" {
		t.Fatal("move order")
	}
	d.Undo()
	err := d.Transaction(func() error {
		if err := d.Remove(a.ID); err != nil {
			return err
		}
		return d.Transaction(func() error { return errors.New("abort") })
	})
	if err == nil || d.Source() != original {
		t.Fatal("transaction did not rollback")
	}
	if err := d.Transform(a.ID, func(n *Node) { n.Children[0].Text = "First" }); err != nil {
		t.Fatal(err)
	}
	if err := d.Transform(b.ID, func(n *Node) { n.Children[0].Text = "Second" }); err != nil {
		t.Fatal(err)
	}
	d.Coalesce()
	d.Undo()
	if d.Source() != original {
		t.Fatal("coalesced undo")
	}
	d.Redo()
	for _, fn := range []func() error{func() error { return d.Splice(d.Root.ID, -1, 0) }, func() error { return d.Transform("absent", func(*Node) {}) }, func() error { return d.Transform(d.Root.ID, func(*Node) {}) }, func() error { return d.Remove("absent") }, func() error { return d.Move(a.ID, a.ID, 0) }, func() error { return d.Move(d.Root.ID, b.ID, 0) }, func() error { return d.Move(a.ID, d.Root.ID, 999) }} {
		if fn() == nil {
			t.Fatal("invalid edit accepted")
		}
	}
	if err := d.Transform(a.ID, func(*Node) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseXML(strings.Repeat("<p>", 260) + strings.Repeat("</p>", 260)); err == nil {
		t.Fatal("unbounded nesting")
	}
	if _, err := ParseXML("<p>"); err == nil {
		t.Fatal("invalid XML")
	}
	if _, err := ParseXML(`<!--comment--><?test value?><p x:a="v">A</p>`); err != nil {
		t.Fatal(err)
	}
	n := NewColumns(3)
	old := n.ID
	FreshIDs(n)
	if n.ID == old {
		t.Fatal("duplicate identities")
	}
}

func TestTableInvalidOperationsAndSpanMoves(t *testing.T) {
	table := ReadTable(NewTable(3, 3))
	_ = table.Merge(Rect{0, 0, 1, 1})
	if err := table.MoveRows(0, 1, 3); err == nil {
		t.Fatal("split span allowed")
	}
	if err := table.MoveColumns(0, 1, 3); err == nil {
		t.Fatal("split span allowed")
	}
	for _, fn := range []func() error{func() error { return table.Merge(Rect{-1, 0, 1, 1}) }, func() error { return table.Split(90, 90) }, func() error { return table.InsertRow(-1) }, func() error { return table.InsertColumn(-1) }, func() error { return table.DeleteRows(2, 1) }, func() error { return table.DeleteColumns(2, 1) }, func() error { return table.Resize(99, 200) }, func() error { return table.MoveRows(-1, 1, 3) }} {
		if fn() == nil {
			t.Fatal("invalid table operation")
		}
	}
	_ = table.DeleteRows(0, table.Rows)
	_ = table.DeleteColumns(0, table.Columns)
	if table.Rows != 1 || table.Columns != 1 || table.At(0, 0) == nil {
		t.Fatal("empty table not repaired")
	}
	_ = table.Merge(Rect{0, 0, 0, 0})
	_ = table.Resize(0, -100)
	if table.Widths[0] != 40 {
		t.Fatal("minimum width")
	}
	n := NewTable(2, 2)
	n.Children = append(n.Children, New("caption", Text("Caption")))
	nested := NewTable(1, 1)
	n.Children[1].Children[0].Children[0].Children = append(n.Children[1].Children[0].Children[0].Children, nested)
	tt := ReadTable(n)
	tt.Write(n)
	if !strings.Contains(n.XML(), "Caption") || tt.Rows != 2 {
		t.Fatal("nested table or caption corrupted")
	}
}
