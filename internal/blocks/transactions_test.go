package blocks

import (
	"errors"
	"strings"
	"testing"
)

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
	for _, tag := range []string{"p", "latex", "time", "button", "span", "unknown"} {
		_ = IsInline(New(tag))
		_ = IsBlock(New(tag))
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
