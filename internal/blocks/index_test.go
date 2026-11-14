package blocks

import (
	"fmt"
	"testing"
)

// Compare indexed lookups with the authoritative tree after every mutation,
// including sibling shifts, nested replacements and reverse transactions.
func assertLocations(t *testing.T, d *Document) {
	t.Helper()
	seen := map[string]bool{}
	var visit func(*Node, *Node, int)
	visit = func(n, parent *Node, at int) {
		seen[n.ID] = true
		if d.Find(n.ID) != n {
			t.Fatalf("stale node %s", n.ID)
		}
		p, i := d.Parent(n.ID)
		if p != parent || i != at {
			t.Fatalf("stale location %s: %d, want %d", n.ID, i, at)
		}
		for j, c := range n.Children {
			visit(c, n, j)
		}
	}
	visit(d.Root, nil, -1)
	if len(seen) != len(d.locations) {
		t.Fatal("detached nodes remain indexed")
	}
}

func TestLocationsAcrossTransactions(t *testing.T) {
	d, err := Parse("<p>A</p><blockquote><p>B</p><p>C</p></blockquote><p>D</p>", "xml")
	if err != nil {
		t.Fatal(err)
	}
	assertLocations(t, d)
	container := d.Root.Children[1]
	b := container.Children[0]
	for i := range 40 {
		if err := d.Transform(b.ID, func(n *Node) { n.Children = []*Node{New("b", Text(fmt.Sprint(i)))} }); err != nil {
			t.Fatal(err)
		}
		assertLocations(t, d)
	}
	if err := d.Splice(d.Root.ID, 0, 0, Paragraph("Before"), Paragraph("More")); err != nil {
		t.Fatal(err)
	}
	assertLocations(t, d)
	if err := d.Move(b.ID, d.Root.ID, len(d.Root.Children)); err != nil {
		t.Fatal(err)
	}
	assertLocations(t, d)
	before := d.Source()
	err = d.Transaction(func() error {
		if err := d.Remove(container.ID); err != nil {
			return err
		}
		assertLocations(t, d)
		return d.Splice(d.Root.ID, -1, 0, Paragraph("Invalid"))
	})
	if err == nil || d.Source() != before {
		t.Fatal("failed transaction was not rolled back")
	}
	assertLocations(t, d)
	for d.Undo() {
		assertLocations(t, d)
	}
	for d.Redo() {
		assertLocations(t, d)
	}
	if d.Source() != before {
		t.Fatal("redo changed committed content")
	}
	if d.Find("missing") != nil {
		t.Fatal("missing ID resolved")
	}
	if p, i := d.Parent("missing"); p != nil || i != -1 {
		t.Fatal("missing parent resolved")
	}
}

func TestTypingCoalesceRetainsOnlyEndpoints(t *testing.T) {
	d, err := Parse("<p>Original</p>", "xml")
	if err != nil {
		t.Fatal(err)
	}
	id := d.Root.Children[0].ID
	for i := range 100 {
		if err := d.Transform(id, func(n *Node) { n.Children = []*Node{Text(fmt.Sprintf("Edit %d", i))} }); err != nil {
			t.Fatal(err)
		}
		d.Coalesce()
	}
	if len(d.undo) != 1 || len(d.undo[0].Edits) != 1 {
		t.Fatal("typing retained intermediate snapshots")
	}
	if !d.Undo() || d.Source() != "<p>Original</p>" {
		t.Fatal("undo did not restore exact source")
	}
	assertLocations(t, d)
	if !d.Redo() || d.Root.Children[0].Plain() != "Edit 99" {
		t.Fatal("redo did not restore final text")
	}
	assertLocations(t, d)
}
