package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func sameTablePixels(a, b *image.RGBA, bounds ui.Rect) bool {
	for y := int(bounds.Y); y < int(bounds.Y+bounds.H); y++ {
		for x := int(bounds.X); x < int(bounds.X+bounds.W); x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				return false
			}
		}
	}
	return true
}

func TestTableHoverControlsStayQuietAndKeepGeometry(t *testing.T) {
	for _, appearance := range []string{"Light", "Dark"} {
		t.Run(appearance, func(t *testing.T) {
			a, tt, _ := tableFixture(t)
			a.lib.Settings.Appearance = appearance
			tt.Move(4, 4)
			tt.Frame()
			rest := tt.Image()
			bar, _ := tt.Find("Table toolbar")
			cell, _ := tt.Find("Table cell 1 1")
			row, _ := tt.Find("Select row 2")
			col, _ := tt.Find("Select column B")
			tt.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
			hover := tt.Image()
			if sameTablePixels(rest, hover, bar) {
				t.Fatal("toolbar did not appear over the table")
			}
			if !sameTablePixels(rest, hover, row) || !sameTablePixels(rest, hover, col) {
				t.Fatal("hover revealed row numbers or column letters")
			}
			cellAfter, _ := tt.Find("Table cell 1 1")
			if cell != cellAfter {
				t.Fatal("hover moved or resized the table", cell, cellAfter)
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "table-hover-"+appearance+".png"))
			}
			for _, label := range []string{"Insert column at 2", "Insert row at 2"} {
				tt.Move(4, 4)
				before := tt.Image()
				button, ok := tt.Find(label)
				if !ok || button.W < 18 || button.H < 18 {
					t.Fatal("missing usable insertion target", label, button)
				}
				tt.Move(button.X+button.W/2, button.Y+button.H/2)
				if sameTablePixels(before, tt.Image(), button) {
					t.Fatal("boundary plus did not appear", label)
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
					saveImage(t, tt, filepath.Join(dir, strings.ReplaceAll(label, " ", "-")+"-"+appearance+".png"))
				}
			}
			tt.Move(4, 4)
			beforeEdge := tt.Image()
			secondRow, _ := tt.Find("Table cell 2 1")
			rowAdd, _ := tt.Find("Insert row at 2")
			tt.Move(secondRow.X, secondRow.Y+1)
			if sameTablePixels(beforeEdge, tt.Image(), rowAdd) {
				t.Fatal("hovering the actual left border did not reveal the row plus")
			}
			tt.Move(4, 4)
			if !sameTablePixels(rest, tt.Image(), bar) {
				t.Fatal("toolbar stayed visible after pointer left")
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "table-rest-"+appearance+".png"))
			}
		})
	}
}

func TestTableBoundaryInsertionAndImmediateTyping(t *testing.T) {
	for _, row := range []bool{true, false} {
		for _, at := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("row=%t/at=%d", row, at), func(t *testing.T) {
				a, tt, id := tableFixture(t)
				before := a.blockEditor.doc.Find(id).XML()
				kind := "column"
				if row {
					kind = "row"
				}
				click(t, tt, fmt.Sprintf("Insert %s at %d", kind, at+1))
				tt.Frame()
				tt.Type("Inserted here")
				a.capture()
				table := blocks.ReadTable(a.blockEditor.doc.Find(id))
				r, col := 0, at
				if row {
					r, col = at, 0
				}
				if table.At(r, col).Node.Plain() != "Inserted here" || row && table.Rows != 4 || !row && table.Columns != 4 {
					t.Fatal("boundary inserted wrong row/column or lost typing", a.current.Source)
				}
				for _, word := range []string{"Name", "Status", "Notes", "Alpha", "Ready", "First", "Beta", "Draft", "Second"} {
					if !strings.Contains(a.current.Source, word) {
						t.Fatal("insertion lost existing content", word)
					}
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				tt.Key(ui.Cmd, ui.KeyZ)
				if a.blockEditor.doc.Find(id).XML() != before {
					t.Fatal("insert was not one undo step after undoing typing")
				}
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				if !a.save() {
					t.Fatal(a.problem)
				}
				saved := a.current.Source
				note := a.current
				a.home("All notes")
				a.openNote(note)
				tt.Frame()
				if a.current.Source != saved {
					t.Fatal("reopening changed boundary insertion")
				}
			})
		}
	}
}

func TestTableBoundaryControlsAbsentInReadOnlyModes(t *testing.T) {
	a, tt, _ := tableFixture(t)
	a.save()
	before := a.current.Source
	for _, mode := range []string{"Read", "Split"} {
		a.changeMode(mode)
		tt.Frame()
		for _, label := range []string{"Table toolbar", "Select row 1", "Select column A", "Insert row at 1", "Insert column at 1"} {
			if _, ok := tt.Find(label); ok {
				t.Fatal("read-only mode exposes editing control", mode, label)
			}
		}
	}
	if a.current.Source != before {
		t.Fatal("viewing controls changed source")
	}
}

func TestTableBoundaryInsertionWithMergedCells(t *testing.T) {
	for _, row := range []bool{true, false} {
		a, tt, id := tableFixture(t)
		a.tableChange(id, func(tab *blocks.Table) error { return tab.Merge(blocks.Rect{Top: 0, Bottom: 1, Left: 0, Right: 1}) })
		tt.Frame()
		before := a.blockEditor.doc.Find(id).XML()
		label := "Insert column at 2"
		if row {
			label = "Insert row at 2"
		}
		click(t, tt, label)
		tt.Frame()
		tab := blocks.ReadTable(a.blockEditor.doc.Find(id))
		cell := tab.At(0, 0)
		if row && (tab.Rows != 4 || cell.Rows != 3) || !row && (tab.Columns != 4 || cell.Columns != 3) {
			t.Fatal("insertion broke a spanning cell", tab.Rows, tab.Columns, cell.Rows, cell.Columns)
		}
		tt.Key(ui.Cmd, ui.KeyZ)
		if a.blockEditor.doc.Find(id).XML() != before {
			t.Fatal("merged insertion did not undo")
		}
	}
}

func TestLargeTableBoundaryControls(t *testing.T) {
	a := testApp(t)
	a.outline = false
	var source strings.Builder
	source.WriteString("| Name | Value |\n| --- | --- |\n")
	for row := 1; row <= 220; row++ {
		fmt.Fprintf(&source, "| Row %d | Value %d |\n", row, row)
	}
	a.createNote(source.String(), "Large table controls")
	tt := ui.NewTester(a.view, 1280, 820)
	n := firstBlock(a, "table")
	click(t, tt, "Insert column at 2")
	tt.Frame()
	tt.Type("New column")
	a.capture()
	if tab := blocks.ReadTable(a.blockEditor.doc.Find(n.ID)); tab.Columns != 3 || tab.At(0, 1) == nil || tab.At(0, 1).Node.Plain() != "New column" {
		t.Fatal("lazy table column insertion lost focus", tab.Columns, len(tab.Cells), a.blockEditor.tableInsert, a.blockEditor.active, a.blockEditor.focus, a.problem)
	}
	state := a.blockEditor.tables[n.ID].rows
	state.ScrollToEnd()
	tt.Frame()
	click(t, tt, "Insert row at 222")
	tt.Frame()
	tt.Type("Last row")
	a.capture()
	if tab := blocks.ReadTable(a.blockEditor.doc.Find(n.ID)); tab.Rows != 222 || tab.At(221, 0).Node.Plain() != "Last row" {
		t.Fatal("virtualized table tail insertion failed")
	}
	first, last := state.Visible()
	if last-first > 40 {
		t.Fatal("edge controls disabled row virtualization")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "large-table-edge-controls.png"))
	}
}
