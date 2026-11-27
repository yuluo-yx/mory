package blocks

import (
	"fmt"
	"strings"
	"testing"
)

func TestTableEditDoesNotLeaveDelimiterOutsideTable(t *testing.T) {
	for _, ending := range []string{"", "\n", "\n\nAfter"} {
		source := "Before\n\n| A | B |\n| --- | --- |\n| One | Two |" + ending
		d, err := Parse(source, "markdown")
		if err != nil {
			t.Fatal(err)
		}
		var table *Node
		d.Root.Walk(func(n *Node) {
			if n.Kind == "table" {
				table = n
			}
		})
		if table == nil {
			t.Fatal("missing table")
		}
		for _, n := range d.Root.Children {
			if n.Kind == "#raw" && strings.Contains(n.Text, "|") {
				t.Fatalf("table delimiter escaped parser range: %q", n.Text)
			}
		}
		cell := ReadTable(table).At(1, 1).Node.Children[0]
		if err := d.Transform(cell.ID, func(n *Node) { n.Children = []*Node{Text("Changed")} }); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(d.Source(), "Before") || !strings.Contains(d.Source(), "Changed") {
			t.Fatal(d.Source())
		}
	}
}

func TestCompactTableEditExpansion(t *testing.T) {
	var b strings.Builder
	b.WriteString("Intro\n\n| Name | Value |\n| --- | ---: |\n")
	for row := 0; row < 220; row++ {
		fmt.Fprintf(&b, "| **Row %d** | value \\| %d |\n", row, row)
	}
	b.WriteString("\nAfter\n")
	d, err := Parse(b.String(), "markdown")
	if err != nil {
		t.Fatal(err)
	}
	var table *Node
	d.Root.Walk(func(n *Node) {
		if n.Kind == "table" {
			table = n
		}
	})
	if table == nil || table.Compact == nil {
		t.Fatal("fixture not compact")
	}
	if err := d.ExpandTable(table.ID); err != nil {
		t.Fatal(err)
	}
	if d.Source() != b.String() {
		t.Fatal("entering a large cell changed source")
	}
	tab := ReadTable(table)
	if tab.Rows != 221 || tab.At(220, 1).Node.Plain() != "value | 219" {
		t.Fatal("expansion lost escaped pipes", tab.At(220, 1).Node.Plain())
	}
	id := tab.At(220, 1).Node.Children[0].ID
	if err := d.Transform(id, func(n *Node) { n.Children = []*Node{Text("Edited last cell")} }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Source(), "Edited last cell") {
		t.Fatal("last cell change not serialized")
	}
	d.Undo()
	if d.Source() != b.String() {
		t.Fatal("undo did not restore original compact spelling")
	}
}
