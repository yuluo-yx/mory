package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLargeTableReadsMiddleAndEndWithoutExpandingAllRows(t *testing.T) {
	for _, mode := range []string{"Write", "Read", "Split"} {
		t.Run(mode, func(t *testing.T) {
			a := testApp(t)
			var source strings.Builder
			source.WriteString("| Name | Description |\n| --- | --- |\n")
			for row := 1; row <= 1200; row++ {
				fmt.Fprintf(&source, "| Row %04d | **中文** and `code` %s END-%04d |\n", row, strings.Repeat("wrapped text ", 12), row)
			}
			a.createNote(source.String(), "Table")
			a.changeMode(mode)
			tt := ui.NewTester(a.view, 1280, 820)
			n := a.blockEditor.doc.Root.Children[0]
			state := a.blockEditor.tables[n.ID].rows
			if !tt.HasText("Name") || tt.HasText("Row 0600") || tt.HasText("Row 1200") {
				t.Fatal("initial viewport omitted the header or eagerly rendered distant rows")
			}
			for _, row := range []int{600, 1200, 1} {
				state.ScrollTo(row, ui.Start)
				tt.Frame()
				if !tt.HasText(fmt.Sprintf("Row %04d", row)) || !tt.HasText(fmt.Sprintf("END-%04d", row)) || !tt.HasText("Name") {
					t.Fatalf("row %d or sticky header missing; long cell may have been truncated", row)
				}
				first, last := state.Visible()
				if last-first > 40 {
					t.Fatalf("viewport expanded too many rows: %d:%d", first, last)
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && row == 600 {
					saveImage(t, tt, filepath.Join(dir, "large-table-"+mode+".png"))
				}
			}
			if !a.save() || a.current.Source != source.String() || a.dirty {
				t.Fatal("reading or scrolling changed the source")
			}
		})
	}
}

func TestLargeXMLTableUsesVirtualRows(t *testing.T) {
	a := testApp(t)
	var source strings.Builder
	source.WriteString("<table><tr><th><p>Header</p></th></tr>")
	for row := range 500 {
		fmt.Fprintf(&source, "<tr><td><p>XML row %03d</p></td></tr>", row)
	}
	source.WriteString("</table>")
	note, err := a.lib.Create("XML table", source.String(), "xml")
	if err != nil {
		t.Fatal(err)
	}
	a.openNote(note)
	tt := ui.NewTester(a.view, 1280, 820)
	n := a.blockEditor.doc.Root.Children[0]
	state := a.blockEditor.tables[n.ID].rows
	state.ScrollToEnd()
	tt.Frame()
	table := a.blockEditor.tables[n.ID].table
	for _, row := range []int{0, 500} {
		cell := table.Cells[row].Node.Children[0]
		field, ok := tt.Access("Block text " + cell.ID)
		if !ok || field.Offscreen || a.blockEditor.fields[cell.ID].doc.Text().String() != cell.Plain() {
			t.Fatalf("row %d not accessible in the viewport: %+v", row, field)
		}
	}
	if _, ok := tt.Access("Block text " + table.Cells[250].Node.Children[0].ID); ok {
		t.Fatal("offscreen row was rendered")
	}
	if a.current.Source != source.String() {
		t.Fatal("table reading changed XML")
	}
}
