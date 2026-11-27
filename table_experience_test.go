package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestTableTextSelectionAndMixedShortcutFormatting(t *testing.T) {
	a, tt, id := tableFixture(t)
	a.showToolbar = false
	click(t, tt, tableCellField(t, a, id, 1, 0))
	tt.Key(0, ui.KeyHome)
	tt.Key(ui.Shift, ui.KeyRight)
	click(t, tt, "Underline")
	a.capture()
	if got := blocks.ReadTable(a.blockEditor.doc.Find(id)).At(1, 0).Node.XML(); !strings.Contains(got, "<u>A</u>lpha") {
		t.Fatal("text formatting escaped selection", got)
	}
	click(t, tt, "Select row 2")
	tt.Key(ui.Cmd, ui.KeyB)
	tt.Key(0, ui.KeyEscape)
	click(t, tt, tableCellField(t, a, id, 1, 1))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyB)
	click(t, tt, "Select row 2")
	tt.Key(ui.Cmd, ui.KeyB)
	for _, cell := range blocks.ReadTable(a.blockEditor.doc.Find(id)).Cells {
		if cell.Row == 1 && !strings.Contains(cell.Node.XML(), "<b>") {
			t.Fatal("mixed selection shortcut inverted each cell instead of making all bold", cell.Node.XML())
		}
	}
}

func TestTableRangeLinkHeadingAndTitleIsolation(t *testing.T) {
	a, tt, id := tableFixture(t)
	click(t, tt, "Select column B")
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("https://example.com/column")
	tt.Key(0, ui.KeyEnter)
	for _, cell := range blocks.ReadTable(a.blockEditor.doc.Find(id)).Cells {
		if strings.Contains(cell.Node.XML(), "https://example.com/column") != (cell.Column == 1) {
			t.Fatal("link did not match column selection", cell.Node.XML())
		}
	}
	click(t, tt, "Select row 2")
	click(t, tt, "Paragraph style")
	if err := tt.ChooseMenuItem("Heading 4"); err != nil {
		t.Fatal(err)
	}
	for _, cell := range blocks.ReadTable(a.blockEditor.doc.Find(id)).Cells {
		if (firstCellText(cell.Node).Kind == "h4") != (cell.Row == 1) {
			t.Fatal("heading escaped row selection")
		}
	}
	for _, action := range a.actions() {
		if action.Name == "Heading 2" {
			action.Run()
		}
	}
	if firstCellText(blocks.ReadTable(a.blockEditor.doc.Find(id)).At(1, 2).Node).Kind != "h2" {
		t.Fatal("heading shortcut missed part of selected row")
	}
	click(t, tt, "Document title")
	if a.blockEditor.tableSelection != "" {
		t.Fatal("title retained table selection")
	}
}

func TestTableInlineAndEmptyXMLCells(t *testing.T) {
	source := `<table><tr><td/><td><b>Inline</b> text</td></tr></table>`
	a, tt := xmlApp(t, source)
	a.save()
	if a.current.Source != source {
		t.Fatal("viewing inline cells changed original XML")
	}
	n := firstBlock(a, "table")
	click(t, tt, tableCellField(t, a, n.ID, 0, 0))
	tt.Type("First cell")
	tt.Key(0, ui.KeyTab)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Second cell")
	a.capture()
	if !strings.Contains(a.current.Source, "First cell") || !strings.Contains(a.current.Source, "Second cell") {
		t.Fatal("inline or empty cell was not editable", a.current.Source)
	}
	click(t, tt, "Select table")
	click(t, tt, "Clear formatting")
	click(t, tt, "Table actions")
	if err := tt.ChooseMenuItem("Header row"); err != nil {
		t.Fatal(err)
	}
	if blocks.ReadTable(a.blockEditor.doc.Find(n.ID)).At(0, 0).Node.Kind != "th" {
		t.Fatal("header toggle failed")
	}
	click(t, tt, "Table actions")
	if err := tt.ChooseMenuItem("Delete table"); err != nil {
		t.Fatal(err)
	}
	if a.blockEditor.doc.Find(n.ID) != nil {
		t.Fatal("table deletion failed")
	}
	a.blockUndo(false)
	if a.blockEditor.doc.Find(n.ID) == nil {
		t.Fatal("table deletion was not undoable")
	}
}

func TestTableRangeNativeCommandsAndQuotedPaste(t *testing.T) {
	a, tt, id := tableFixture(t)
	click(t, tt, tableCellField(t, a, id, 1, 0))
	tt.SetClipboard("\"Line one\nLine two\"\t\"A\tB\"\t\"Say \"\"yes\"\"\"")
	tt.Command("paste")
	a.capture()
	tab := blocks.ReadTable(a.blockEditor.doc.Find(id))
	if tab.At(1, 0).Node.Plain() != "Line one\nLine two" || tab.At(1, 1).Node.Plain() != "A\tB" {
		t.Fatal("quoted TSV lost multiline content")
	}
	click(t, tt, "Select row 2")
	tt.Command("copy")
	click(t, tt, "Select row 3")
	tt.Command("paste")
	a.capture()
	tab = blocks.ReadTable(a.blockEditor.doc.Find(id))
	for col := 0; col < 3; col++ {
		if tab.At(1, col).Node.Plain() != tab.At(2, col).Node.Plain() {
			t.Fatal("native range copy/paste changed values")
		}
	}
	click(t, tt, "Select row 3")
	tt.Command("cut")
	if blocks.ReadTable(a.blockEditor.doc.Find(id)).At(2, 0).Node.Plain() != "" {
		t.Fatal("native Cut did not clear cells")
	}
	tt.Command("undo")
	if blocks.ReadTable(a.blockEditor.doc.Find(id)).At(2, 0).Node.Plain() != "Line one\nLine two" {
		t.Fatal("native undo lost multiline value")
	}
}

func TestLargeTableEditingPreservesSourceUntilTyping(t *testing.T) {
	a := testApp(t)
	var source strings.Builder
	source.WriteString("| Name | Value |\n| --- | ---: |\n")
	for row := 1; row <= 220; row++ {
		fmt.Fprintf(&source, "| Row %d | Value %d |\n", row, row)
	}
	a.createNote(source.String(), "Large editing")
	a.save()
	tt := ui.NewTester(a.view, 1280, 820)
	n := firstBlock(a, "table")
	state := a.blockEditor.tables[n.ID].rows
	state.ScrollTo(220, ui.End)
	tt.Frame()
	click(t, tt, "Edit cell 221 2")
	if a.dirty || a.current.Source != source.String() {
		t.Fatal("entering large cell changed source")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Edited last cell")
	a.capture()
	if !a.dirty || !strings.Contains(a.current.Source, "Edited last cell") {
		t.Fatal("last cell did not accept typing")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	a.capture()
	if a.current.Source != source.String() {
		t.Fatal("large cell undo did not restore source")
	}
	click(t, tt, "Select column B")
	click(t, tt, "Italic")
	if !strings.Contains(blocks.ReadTable(a.blockEditor.doc.Find(n.ID)).At(220, 1).Node.XML(), "<em>") {
		t.Fatal("large column formatting missed offscreen cells")
	}
	first, last := state.Visible()
	if last-first > 40 || len(a.blockEditor.fields) > maxCachedBlockFields+80 {
		t.Fatal("editing retained unbounded visible rows", first, last, len(a.blockEditor.fields))
	}
}

func TestMarkdownTableAlignmentAndPickerKeyboard(t *testing.T) {
	a := testApp(t)
	a.createNote("| Name | Value |\n| --- | ---: |\n| A | B |\n", "Aligned table")
	tt := ui.NewTester(a.view, 1280, 820)
	n := firstBlock(a, "table")
	field := a.blockEditor.fields[firstCellText(blocks.ReadTable(n).At(1, 1).Node).ID]
	if field.doc.Formatting().Paragraph.Align != ui.End {
		t.Fatal("Markdown column alignment not inherited")
	}
	click(t, tt, "Select column B")
	click(t, tt, "Align")
	if err := tt.ChooseMenuItem("Left"); err != nil {
		t.Fatal(err)
	}
	if blocks.ReadTable(a.blockEditor.doc.Find(n.ID)).At(1, 1).Node.Attr("align") != "" {
		t.Fatal("Left failed to override inherited alignment")
	}
	a.createNote("", "Picker keyboard")
	tt.Frame()
	click(t, tt, "Document editor")
	tt.Type("/")
	tt.Type("table")
	tt.Key(0, ui.KeyEnter)
	cell, _ := tt.Find("Table size 2 2")
	tt.Move(cell.X+5, cell.Y+5)
	tt.Key(0, ui.KeyRight)
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.tableInsertRows != 3 || a.tableInsertColumns != 3 {
		t.Fatal("stationary pointer reset keyboard dimensions")
	}
	tt.Key(0, ui.KeyEnter)
	if blocks.ReadTable(firstBlock(a, "table")).Rows != 3 {
		t.Fatal("Enter did not confirm picker")
	}
}

func tableFixture(t *testing.T) (*app, *ui.Tester, string) {
	t.Helper()
	a := testApp(t)
	a.outline = false
	a.createNote("| Name | Status | Notes |\n| --- | --- | --- |\n| Alpha | Ready | First |\n| Beta | Draft | Second |\n", "Table workflow")
	tt := ui.NewTester(a.view, 1280, 820)
	return a, tt, firstBlock(a, "table").ID
}

func tableCellField(t *testing.T, a *app, id string, row, col int) string {
	t.Helper()
	n := a.blockEditor.doc.Find(id)
	table := blocks.ReadTable(n)
	cell := table.At(row, col)
	if cell == nil {
		t.Fatal("missing cell", row, col)
	}
	text := firstCellText(cell.Node)
	if text == nil {
		t.Fatal("missing cell paragraph")
	}
	return "Block text " + text.ID
}

func TestTableInsertionAndKeyboardFlow(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		a.createNote("", "Insert table")
		if structured {
			a.enableBlocks()
		}
		tt := ui.NewTester(a.view, 1280, 820)
		if structured {
			click(t, tt, "Block text "+a.blockEditor.emptyBody.ID)
		} else {
			click(t, tt, "Document editor")
		}
		tt.Type("/")
		tt.Type("table")
		tt.Key(0, ui.KeyEnter)
		if !a.tablePickerOpen {
			t.Fatal("table picker did not open")
		}
		click(t, tt, "Table size 2 3")
		n := firstBlock(a, "table")
		if n == nil {
			t.Fatal("table was not inserted")
		}
		id := n.ID
		tt.Type("Name")
		tt.Key(0, ui.KeyTab)
		tt.Type("Status")
		tt.Key(0, ui.KeyTab)
		tt.Type("Notes")
		tt.Key(0, ui.KeyTab)
		tt.Type("Alpha")
		tt.Key(0, ui.KeyTab)
		tt.Type("Ready")
		tt.Key(0, ui.KeyTab)
		tt.Type("Last")
		tt.Key(0, ui.KeyTab)
		tt.Type("New row")
		a.capture()
		table := blocks.ReadTable(a.blockEditor.doc.Find(id))
		if table.Rows != 3 || table.Columns != 3 || table.At(0, 1).Node.Plain() != "Status" || table.At(2, 0).Node.Plain() != "New row" {
			t.Fatal("Tab lost focus or data", a.current.Source)
		}
		tt.Key(ui.Shift, ui.KeyTab)
		tt.Type("!")
		a.capture()
		if blocks.ReadTable(a.blockEditor.doc.Find(id)).At(1, 2).Node.Plain() != "Last!" {
			t.Fatal("Shift Tab did not go back")
		}
		tt.Key(ui.Cmd, ui.KeyEnter)
		tt.Type("After table")
		a.capture()
		if !strings.Contains(a.current.Source, "After table") || strings.Contains(a.current.Source, "Column A") || strings.Contains(a.current.Source, "Value") {
			t.Fatal("table exit or placeholder content", a.current.Source)
		}
		if !a.save() {
			t.Fatal(a.problem)
		}
		fresh, err := library.Open(a.lib.Dir)
		if err != nil || fresh.Get(a.current.ID).Source != a.current.Source {
			t.Fatal("table save failed", err)
		}
	}
}

func TestTableRowAndColumnFormatting(t *testing.T) {
	a, tt, id := tableFixture(t)
	click(t, tt, tableCellField(t, a, id, 1, 1))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Updated")
	click(t, tt, "Select row 2")
	click(t, tt, "Bold")
	table := blocks.ReadTable(a.blockEditor.doc.Find(id))
	for _, cell := range table.Cells {
		bold := strings.Contains(cell.Node.XML(), "<b>")
		if bold != (cell.Row == 1) {
			t.Fatal("format affected wrong row", cell.Row, cell.Column, cell.Node.XML())
		}
	}
	if table.At(1, 1).Node.Plain() != "Updated" {
		t.Fatal("pending text lost")
	}
	click(t, tt, "Bold")
	if strings.Contains(a.blockEditor.doc.Find(id).XML(), "<b>") {
		t.Fatal("bold did not toggle off")
	}
	click(t, tt, "Select column B")
	click(t, tt, "Italic")
	click(t, tt, "Cell background")
	if err := tt.ChooseMenuItem("light-blue"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Align")
	if err := tt.ChooseMenuItem("Center"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Align")
	if err := tt.ChooseMenuItem("Align middle"); err != nil {
		t.Fatal(err)
	}
	table = blocks.ReadTable(a.blockEditor.doc.Find(id))
	for _, cell := range table.Cells {
		if cell.Column == 1 {
			if !strings.Contains(cell.Node.XML(), "<em>") || cell.Node.Attr("background-color") != "light-blue" || cell.Node.Attr("vertical-align") != "middle" || firstCellText(cell.Node).Attr("align") != "center" {
				t.Fatal("column style missing", cell.Node.XML())
			}
		} else if cell.Node.Attr("background-color") != "" {
			t.Fatal("outside column changed")
		}
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "table-column-formatting.png"))
	}
	before := a.blockEditor.doc.Find(id).XML()
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	if a.blockEditor.doc.Find(id).XML() != before {
		t.Fatal("undo redo lost table styles")
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
	note := a.current
	a.home("All notes")
	a.openNote(note)
	tt.Frame()
	table = blocks.ReadTable(firstBlock(a, "table"))
	if table.At(1, 1).Node.Attr("vertical-align") != "middle" {
		t.Fatal("alignment lost on reopen")
	}
}

func TestTablePasteMergeDeleteAndResize(t *testing.T) {
	a, tt, id := tableFixture(t)
	click(t, tt, tableCellField(t, a, id, 1, 1))
	tt.SetClipboard("One\tTwo\nThree\tFour")
	tt.Key(ui.Cmd, ui.KeyV)
	a.capture()
	table := blocks.ReadTable(a.blockEditor.doc.Find(id))
	if table.At(1, 1).Node.Plain() != "One" || table.At(2, 2).Node.Plain() != "Four" {
		t.Fatal("TSV paste did not fill grid", a.current.Source)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	table = blocks.ReadTable(a.blockEditor.doc.Find(id))
	if table.At(1, 1).Node.Plain() != "Ready" {
		t.Fatal("paste not one undo")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	click(t, tt, "Select cell 2 2")
	if err := tt.ClickWith(ui.Shift, "Select cell 3 3"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Merge cells")
	table = blocks.ReadTable(a.blockEditor.doc.Find(id))
	if table.At(1, 1).Rows != 2 || !strings.Contains(table.At(1, 1).Node.Plain(), "Four") {
		t.Fatal("merge lost text")
	}
	click(t, tt, "Split cell")
	click(t, tt, "Select row 3")
	tt.Key(0, ui.KeyBackspace)
	table = blocks.ReadTable(a.blockEditor.doc.Find(id))
	if table.Rows != 3 || table.At(2, 0).Node.Plain() != "" {
		t.Fatal("range delete removed structure or missed cells")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	click(t, tt, "Select column C")
	click(t, tt, "Table actions")
	if err := tt.ChooseMenuItem("Delete column"); err != nil {
		t.Fatal(err)
	}
	if blocks.ReadTable(a.blockEditor.doc.Find(id)).Columns != 2 {
		t.Fatal("column deletion missed selection")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	resize, ok := tt.Find("Resize column A")
	if !ok {
		t.Fatal("resize handle missing")
	}
	before := blocks.ReadTable(a.blockEditor.doc.Find(id)).Widths[0]
	tt.Move(resize.X+3, resize.Y+10)
	tt.Press(resize.X+3, resize.Y+10)
	tt.Move(resize.X+63, resize.Y+10)
	if blocks.ReadTable(a.blockEditor.doc.Find(id)).Widths[0] != before {
		t.Fatal("resize wrote before release")
	}
	tt.Release(resize.X+63, resize.Y+10)
	if blocks.ReadTable(a.blockEditor.doc.Find(id)).Widths[0] <= before {
		t.Fatal("resize not committed")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if blocks.ReadTable(a.blockEditor.doc.Find(id)).Widths[0] != before {
		t.Fatal("resize not single undo")
	}
}

func TestTableBoundaryAndCurrentCell(t *testing.T) {
	a, tt, id := tableFixture(t)
	click(t, tt, tableCellField(t, a, id, 2, 1))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	before := a.blockEditor.doc.Find(id).XML()
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyDelete)
	if a.blockEditor.doc.Find(id).XML() != before {
		t.Fatal("empty-cell delete changed structure")
	}
	click(t, tt, "+ Row")
	table := blocks.ReadTable(a.blockEditor.doc.Find(id))
	if table.Rows != 4 || table.At(2, 0).Node.Plain() != "Beta" {
		t.Fatal("row added relative to stale selection")
	}
}

func TestSlashTableBetweenParagraphsKeepsDocumentLayout(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		a.outline = false
		a.createNote("Before paragraph\n\nInsert here\n\nAfter paragraph", "Stable document")
		if structured {
			a.enableBlocks()
		}
		tt := ui.NewTester(a.view, 1280, 820)
		titleBefore, _ := tt.Find("Document title")
		if structured {
			var target *blocks.Node
			a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
				if blocks.IsText(n) && n.Plain() == "Insert here" {
					target = n
				}
			})
			click(t, tt, "Block text "+target.ID)
			tt.Key(ui.Cmd, ui.KeyA)
		} else {
			click(t, tt, "Document editor")
			a.doc.SetSelection(len([]rune("Before paragraph\n")), len([]rune("Before paragraph\nInsert here")))
			tt.Frame()
		}
		tt.Type("/")
		tt.Type("table")
		tt.Key(0, ui.KeyEnter)
		if !a.slashPending || !a.tablePickerOpen {
			t.Fatal("slash insertion state lost")
		}
		click(t, tt, "Table size 3 9")
		tt.Type("Cell text")
		titleAfter, _ := tt.Find("Document title")
		if math.Abs(float64(titleBefore.X-titleAfter.X)) > 1 || math.Abs(float64(titleBefore.W-titleAfter.W)) > 1 {
			t.Fatal("table changed page geometry", titleBefore, titleAfter)
		}
		var body []*blocks.Node
		for _, n := range blocks.Visible(a.blockEditor.doc.Root.Children) {
			if n.Kind != "#raw" {
				body = append(body, n)
			}
		}
		if len(body) != 3 || body[0].Plain() != "Before paragraph" || body[1].Kind != "table" || body[2].Plain() != "After paragraph" {
			t.Fatal("insertion changed neighboring paragraphs", a.current.Source)
		}
		for _, n := range body {
			if n.Kind != "table" && strings.Contains(n.Plain(), "/") {
				t.Fatal("slash remained outside table")
			}
		}
		first := firstCellText(blocks.ReadTable(body[1]).At(0, 0).Node)
		if first.Plain() != "Cell text" {
			t.Fatal("inserted table did not receive typing", first.Plain())
		}
		if !a.save() {
			t.Fatal(a.problem)
		}
		note := a.current
		a.home("All notes")
		a.openNote(note)
		tt.Frame()
		if strings.Contains(a.current.Source, "Insert here") || strings.Contains(a.current.Source, "<p>/</p>") {
			t.Fatal("slash reappeared after reopen", a.current.Source)
		}
		if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
			saveImage(t, tt, filepath.Join(dir, "table-in-document.png"))
		}
	}
}
