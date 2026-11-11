package blocks

import (
	"strings"
	"testing"
)

func TestLargeMarkdownTableUsesCompactRepresentation(t *testing.T) {
	source := "| A | B |\n| --- | --- |\n" + strings.Repeat("| **value** | [link](https://example.com) |\n", 240)
	doc, err := Parse(source, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	var tableNode *Node
	for _, n := range doc.Root.Children {
		if n.Kind == "table" {
			tableNode = n
			break
		}
	}
	if tableNode == nil || tableNode.Compact == nil {
		t.Fatal("large Markdown table was expanded instead of compacted")
	}
	if got := doc.Source(); got != source {
		t.Fatalf("compact table changed untouched source: got %d bytes, want %d", len(got), len(source))
	}
	table := ReadTable(tableNode)
	if table.Rows != 241 || table.Columns != 2 {
		t.Fatalf("compact table geometry = %dx%d, want 241x2", table.Rows, table.Columns)
	}
	if table.Preview[[2]int{0, 0}] != "A" || table.Preview[[2]int{240, 1}] == "" {
		t.Fatal("compact table preview did not retain boundary cells")
	}
}

func TestSmallMarkdownTableStillExpandsCells(t *testing.T) {
	doc, err := Parse("| A | B |\n| --- | --- |\n| 1 | 2 |\n", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	n := doc.Root.Children[0]
	if n.Compact != nil || len(ReadTable(n).Cells) != 4 {
		t.Fatal("small Markdown table was unexpectedly compacted")
	}
}
