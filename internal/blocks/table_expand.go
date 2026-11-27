package blocks

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// ExpandTable materializes a compact Markdown table only when editing begins.
// This is a representation change, not a document edit: preserve source spelling,
// undo history, and unchanged ancestor baselines until a cell actually changes.
func (d *Document) ExpandTable(id string) error {
	n := d.Find(id)
	if n == nil || n.Kind != "table" {
		return fmt.Errorf("table no longer exists")
	}
	if n.Compact == nil {
		return nil
	}
	unchanged := d.Root.signature() == d.baseline
	var ancestors []*Node
	for p, _ := d.Parent(id); p != nil; p, _ = d.Parent(p.ID) {
		if p.markdown != "" && p.signature() == p.mdBaseline {
			ancestors = append(ancestors, p)
		}
	}
	c := n.Compact
	var source strings.Builder
	for row := 0; row < c.Rows; row++ {
		source.WriteString("| " + strings.Join(c.Cells[row*c.Columns:(row+1)*c.Columns], " | ") + " |\n")
		if row == 0 {
			source.WriteString("|")
			for _, align := range c.Align {
				marker := " --- "
				if align == "center" {
					marker = " :---: "
				} else if align == "right" {
					marker = " ---: "
				}
				source.WriteString(marker + "|")
			}
			source.WriteByte('\n')
		}
	}
	p := parser.New(parser.Default)
	p.Set(source.String())
	var expanded *Node
	for index := range p.Children(0) {
		if p.Block(index).Kind == parser.Table {
			expanded = markdownBlock(p, index)
			break
		}
	}
	if expanded == nil {
		return fmt.Errorf("cannot expand table")
	}
	t := ReadTable(expanded)
	t.Widths = c.Widths
	t.Write(expanded)
	n.Children, n.Compact = expanded.Children, nil
	n.mdBaseline = n.signature()
	for _, p := range ancestors {
		p.mdBaseline = p.signature()
	}
	if unchanged {
		d.baseline = d.Root.signature()
	}
	d.locations = nil
	d.Version++
	return nil
}
