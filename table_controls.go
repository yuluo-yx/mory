package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

const tableColumnHandleHeight float32 = 20

func (a *app) tableHeaderColor(theme *ui.Theme) ui.Color {
	if theme.Dark {
		return theme.Surface
	}
	return a.docTheme.Color("th", "background-color", theme.Surface)
}

func (a *app) tableBorderColor(theme *ui.Theme) ui.Color {
	if theme.Dark {
		return theme.Border
	}
	return a.docTheme.Color("table", "border-color", theme.Border)
}

type tableInsertRequest struct {
	id  string
	row bool
	at  int
}

func (a *app) queueTableEdge(c *ui.Context, id string, row bool, at int) {
	// List rows may build during layout. Apply insertion on the next frame,
	// before traversing cells, so stale editors cannot rebuild their old fields.
	a.blockEditor.tableInsert = &tableInsertRequest{id: id, row: row, at: at}
	c.AnimationFrame()
}

// Keep the toolbar's geometry stable: revealing it must never move cells under
// the pointer. Keyboard users can reveal the controls by tabbing into them.
func (a *app) tableToolbar(c *ui.Context, surface ui.Element, n *blocks.Node, table *blocks.Table) {
	bar := ui.Row(c.Key("table-toolbar")).Height(30).Gap(5).AlignItems(ui.Center).Label(a.tr("Table toolbar"))
	show := surface.Hovered() || bar.FocusWithin()
	if !show {
		bar.Opacity(0)
	}
	bar.Children(func() {
		if ghostButton(c, "+").Label(a.tr("+ Row")).Tooltip(a.tr("+ Row")).KeepFocus().Clicked() {
			at := table.Rows
			if a.blockEditor.tableID == n.ID {
				at = table.Expand(a.blockEditor.cells).Bottom + 1
			}
			a.queueTableEdge(c, n.ID, true, at)
		}
		if ghostButton(c, "+ │").Label(a.tr("+ Column")).Tooltip(a.tr("+ Column")).KeepFocus().Clicked() {
			at := table.Columns
			if a.blockEditor.tableID == n.ID {
				at = table.Expand(a.blockEditor.cells).Right + 1
			}
			a.queueTableEdge(c, n.ID, false, at)
		}
		ghostButton(c, "•••").Label(a.tr("Table actions")).Tooltip(a.tr("Table actions")).KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
			if n.Compact != nil && !a.expandTable(n.ID) {
				return
			}
			a.tableMenu(m, n.ID, a.blockEditor.cells)
		})
		ui.Text(c, fmt.Sprintf("%d × %d", table.Rows, table.Columns)).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}

func (a *app) tableCornerControl(c *ui.Context, n *blocks.Node, table *blocks.Table) ui.Element {
	button := ghostButton(c, "").Label(a.tr("Select table")).Tooltip(a.tr("Select table")).Padding(0).KeepFocus()
	if button.Hovered() || button.FocusVisible() {
		button.Background(c.Theme().Accent.Alpha(.12))
		button.DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Stroke(ui.Rect{X: r.X + r.W/2 - 3, Y: r.Y + r.H/2 - 3, W: 6, H: 6}, c.Theme().Accent, 1, 1)
		})
	}
	if button.Clicked() {
		a.selectTable(n.ID, blocks.Rect{Top: 0, Bottom: table.Rows - 1, Left: 0, Right: table.Columns - 1}, "table")
	}
	return button
}

// Selection strips have accessible names but no permanent spreadsheet labels.
// A short grip appears only over the strip, independently of its edge add button.
func tableGrip(c *ui.Context, button ui.Element, horizontal, selected bool) {
	if !button.Hovered() && !button.FocusVisible() && !selected {
		return
	}
	button.Background(c.Theme().Accent.Alpha(.08))
	button.DrawOver(func(p *ui.Painter, r ui.Rect) {
		w, h := float32(3), float32(14)
		if horizontal {
			w, h = h, w
		}
		p.Fill(ui.Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}, c.Theme().Accent.Alpha(.65), 2)
	})
}

func (a *app) tableColumnControls(c *ui.Context, n *blocks.Node, table *blocks.Table, col int, selection blocks.Rect) {
	s := a.blockEditor
	button := ghostButton(c, "").Label(a.tr("Select column ") + tableColumnName(col)).Tooltip(a.tr("Select column")).Absolute().Top(0).Bottom(0).Left(12).Right(12).Padding(0).KeepFocus()
	tableGrip(c, button, true, s.tableID == n.ID && s.tableSelection == "column" && col >= selection.Left && col <= selection.Right)
	if button.Clicked() {
		start := col
		if button.ClickModifiers()&ui.Shift != 0 && s.tableID == n.ID && s.tableSelection == "column" {
			start = s.cellAnchor[1]
		}
		a.selectTable(n.ID, blocks.Rect{Top: 0, Bottom: table.Rows - 1, Left: min(start, col), Right: max(start, col)}, "column")
	}
	a.tableInsertControl(c.Key("after"), n.ID, false, col+1).Absolute().Top(9).Right(-9)
	if col == 0 {
		a.tableInsertControl(c.Key("before"), n.ID, false, 0).Absolute().Top(9).Left(-9)
	}
}

func (a *app) tableRowControls(c *ui.Context, n *blocks.Node, table *blocks.Table, row int, selection blocks.Rect) {
	s := a.blockEditor
	button := ghostButton(c, "").Label(a.trf("Select row %d", row+1)).Tooltip(a.tr("Select row")).Absolute().Left(0).Right(0).Top(10).Bottom(10).Padding(0).KeepFocus()
	tableGrip(c, button, false, s.tableID == n.ID && s.tableSelection == "row" && row >= selection.Top && row <= selection.Bottom)
	if button.Clicked() {
		start := row
		if button.ClickModifiers()&ui.Shift != 0 && s.tableID == n.ID && s.tableSelection == "row" {
			start = s.cellAnchor[0]
		}
		a.selectTable(n.ID, blocks.Rect{Top: min(start, row), Bottom: max(start, row), Left: 0, Right: table.Columns - 1}, "row")
	}
	// Own each shared boundary from the row below it. The one-pixel inset
	// keeps its center inside that row's hit-test area, not its older sibling.
	before := a.tableInsertControl(c.Key("before"), n.ID, true, row).Absolute().Right(-4).Top(-9)
	if row == 0 {
		// Separate the top-left row and column insertion targets.
		before.Right(3)
	}
	if row == table.Rows-1 {
		a.tableInsertControl(c.Key("after"), n.ID, true, row+1).Absolute().Right(-4).Bottom(-9)
	}
}

func (a *app) tableInsertControl(c *ui.Context, id string, row bool, at int) ui.Element {
	label := a.trf("Insert column at %d", at+1)
	if row {
		label = a.trf("Insert row at %d", at+1)
	}
	button := ghostButton(c, "").Label(label).Tooltip(label).Width(20).Height(20).Padding(0).Radius(10).KeepFocus().ZIndex(4)
	if button.Hovered() || button.FocusVisible() || button.Pressed() {
		button.Background(c.Theme().Accent).DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X + r.W/2 - 4, Y: r.Y + r.H/2 - .75, W: 8, H: 1.5}, c.Theme().AccentText, 0)
			p.Fill(ui.Rect{X: r.X + r.W/2 - .75, Y: r.Y + r.H/2 - 4, W: 1.5, H: 8}, c.Theme().AccentText, 0)
		})
	}
	button.OnClick(func() { a.queueTableEdge(c, id, row, at) })
	return button
}

func (a *app) insertTableEdge(id string, row bool, at int) {
	if a.mode != "Write" {
		return
	}
	n := a.blockEditor.doc.Find(id)
	if n == nil || n.Compact != nil && !a.expandTable(id) {
		return
	}
	if row {
		if a.tableChange(id, func(t *blocks.Table) error { return t.InsertRow(at) }) {
			a.focusTableCell(id, at, 0, false)
		}
	} else if a.tableChange(id, func(t *blocks.Table) error { return t.InsertColumn(at) }) {
		a.focusTableCell(id, 0, at, false)
	}
}

func (a *app) tableResizeControl(c *ui.Context, id string, col int, width float64) ui.Element {
	s := a.blockEditor
	resize := ui.Box(c.Key("resize")).Label(a.tr("Resize column ") + tableColumnName(col)).Cursor(ui.CursorResizeEW).Focusable()
	if resize.Hovered() || resize.Pressed() {
		resize.Background(c.Theme().Accent.Alpha(.35))
	}
	if resize.Focused() {
		a.textFocused = true
	}
	dx, _, held := resize.Dragged()
	if resize.PressCanceled() {
		s.tableResize.active = false
	} else if held {
		if !s.tableResize.active {
			s.tableResize = tableResizeState{id: id, column: col, width: width, active: true}
		}
		s.tableResize.delta += float64(dx)
		resize.Focus(ui.FocusOptions{PreventScroll: true})
	}
	if s.tableResize.active && s.tableResize.id == id && s.tableResize.column == col && !held {
		value := s.tableResize.width + s.tableResize.delta
		s.tableResize.active = false
		a.tableChange(id, func(t *blocks.Table) error { return t.Resize(col, max(64, value)) })
	}
	return resize
}
