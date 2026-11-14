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
	if table.Compact.Cells[0] != "A" || table.Compact.Cells[240*2+1] == "" {
		t.Fatal("compact table index did not retain boundary cells")
	}
}

func TestCompactTableRetainsEveryCellAndAlignment(t *testing.T) {
	source := "| Left | Center | Right |\r\n| :--- | :---: | ---: |\r\n" + strings.Repeat("| 中文 **bold** | escaped \\| pipe | `code` [link](https://example.com) |\r\n", 1200)
	doc, err := Parse(source, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	n := doc.Root.Children[0]
	c := n.Compact
	if c == nil || c.Rows != 1201 || len(c.Cells) != c.Rows*c.Columns || len(n.Children) != 0 {
		t.Fatal("large table was not indexed without a rich cell tree")
	}
	for row := 1; row < c.Rows; row++ {
		if c.Cells[row*3] != "中文 **bold**" || c.Cells[row*3+1] != "escaped \\| pipe" || c.Cells[row*3+2] != "`code` [link](https://example.com)" {
			t.Fatalf("row %d lost source or escaped cell separators", row)
		}
	}
	if c.Align[1] != "center" || c.Align[2] != "right" || doc.Source() != source || n.Clone().Markdown() != n.Markdown() {
		t.Fatal("index changed alignment or original source")
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
