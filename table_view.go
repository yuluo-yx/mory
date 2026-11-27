package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

const tableRowHandleWidth float32 = 24

func (a *app) blockTable(c *ui.Context, n *blocks.Node, readOnly bool, depth int) {
	s := a.blockEditor
	if pending := s.tableInsert; pending != nil && pending.id == n.ID {
		s.tableInsert = nil
		a.insertTableEdge(pending.id, pending.row, pending.at)
		n = s.doc.Find(n.ID)
	}
	table, simple := a.tableLayout(n)
	surface := ui.Column(c.Key("table-surface")).FillWidth().Label(a.tr("Table area"))
	surface.Children(func() {
		if simple && table.Rows >= 200 {
			a.largeTableView(c, surface, n, table, depth)
		} else {
			a.tableGrid(c, surface, n, table, readOnly, depth)
		}
	})
}

func (a *app) tableGrid(c *ui.Context, surface ui.Element, n *blocks.Node, table *blocks.Table, readOnly bool, depth int) {
	s := a.blockEditor
	selection := table.Expand(s.cells)
	if !readOnly {
		a.tableToolbar(c, surface, n, table)
	}
	// A toolbar command may have replaced the table during this frame.
	if fresh := s.doc.Find(n.ID); fresh != nil {
		n = fresh
		table, _ = a.tableLayout(n)
		selection = table.Expand(s.cells)
	} else {
		return
	}
	tracks := make([]ui.Track, 0, table.Columns+1)
	offset, total := 0, float32(0)
	if !readOnly {
		tracks = append(tracks, ui.Fixed(tableRowHandleWidth))
		offset = 1
		total = tableRowHandleWidth
	}
	for col, width := range table.Widths {
		if s.tableResize.active && s.tableResize.id == n.ID && s.tableResize.column == col {
			width = max(64, min(1200, s.tableResize.width+s.tableResize.delta))
		}
		tracks = append(tracks, ui.Fixed(float32(width)))
		total += float32(width)
	}
	scroll := ui.ScrollHorizontal(c).FillWidth().Padding(0, 12, 12, 0).Label(a.tr("Table viewport")).Focusable()
	if !readOnly {
		a.tableSelectionInput(scroll, n, table, selection)
	}
	scroll.Children(func() {
		ui.Grid(c).ColumnTracks(tracks...).Width(total).Gap(0).Children(func() {
			rowOffset := 0
			if !readOnly {
				rowOffset = 1
				a.tableCornerControl(c, n, table).ColumnStart(1).RowStart(1).Width(12).Height(12).Margin(0, 12, 8, 0)
				for col := range table.Widths {
					strip := ui.Box(c.Key(fmt.Sprintf("column-%d", col))).ColumnStart(col + 2).RowStart(1).Height(tableColumnHandleHeight).ZIndex(3)
					strip.Children(func() { a.tableColumnControls(c, n, table, col, selection) })
				}
				for row := 0; row < table.Rows; row++ {
					strip := ui.Box(c.Key(fmt.Sprintf("row-%d", row))).ColumnStart(1).RowStart(row + 2).ZIndex(2)
					strip.Children(func() { a.tableRowControls(c, n, table, row, selection) })
				}
			}
			for _, cell := range table.Cells {
				background := c.Theme().Background
				if cell.Node.Kind == "th" {
					background = a.tableHeaderColor(c.Theme())
				}
				if value := cell.Node.Attr("background-color"); value != "" {
					background = blockColor(value, background)
				}
				chosen := s.tableID == n.ID && cell.Row >= selection.Top && cell.Row <= selection.Bottom && cell.Column >= selection.Left && cell.Column <= selection.Right
				box := ui.Column(c.Key(cell.Node.ID)).ColumnStart(cell.Column+1+offset).RowStart(cell.Row+1+rowOffset).ColumnSpan(cell.Columns).RowSpan(cell.Rows).MinWidth(0).MinHeight(42).Padding(6, 10).Gap(2).Background(background).
					BorderWidth(0, 1, 1, 0).BorderColor(a.tableBorderColor(c.Theme())).Label(a.trf("Table cell %d %d", cell.Row+1, cell.Column+1))
				if cell.Row == 0 {
					box.BorderWidth(1, 1, 1, 0)
				}
				if cell.Column == 0 {
					box.BorderWidth(0, 1, 1, 1)
					if cell.Row == 0 {
						box.BorderWidth(1)
					}
				}
				if chosen && !readOnly {
					box.DrawOver(func(p *ui.Painter, r ui.Rect) {
						if s.tableSelection != "" {
							p.Fill(r, c.Theme().Accent.Alpha(.1), 0)
						}
						p.Stroke(r, c.Theme().Accent, 1, 0)
					})
				}
				if cell.Node.Attr("vertical-align") == "middle" {
					box.Justify(ui.Center)
				} else if cell.Node.Attr("vertical-align") == "bottom" {
					box.Justify(ui.End)
				}
				box.Children(func() {
					if !readOnly && cell.Row == 0 {
						a.tableResizeControl(c, n.ID, cell.Column+cell.Columns-1, table.Widths[cell.Column+cell.Columns-1]).Absolute().Top(12).Bottom(0).Right(-3).Width(6).ZIndex(2)
					}
					// The cell selector lives in the corner instead of taking a text row.
					if !readOnly {
						button := ghostButton(c, "").Label(a.trf("Select cell %d %d", cell.Row+1, cell.Column+1)).Absolute().Top(0).Right(10).Width(8).Height(8).Padding(0).KeepFocus()
						if button.Hovered() {
							button.Background(c.Theme().Accent.Alpha(.18))
						}
						if button.Clicked() {
							anchor := [2]int{cell.Row, cell.Column}
							if button.ClickModifiers()&ui.Shift != 0 && s.tableID == n.ID {
								anchor = s.cellAnchor
							}
							a.selectTable(n.ID, blocks.Rect{Top: min(anchor[0], cell.Row), Bottom: max(anchor[0], cell.Row), Left: min(anchor[1], cell.Column), Right: max(anchor[1], cell.Column)}, "range")
							s.cellAnchor = anchor
						}
					}
					for _, child := range blocks.Visible(cell.Node.Children) {
						a.blockNode(c, child, readOnly, depth+1)
					}
				})
				if !readOnly {
					box.PopoverMenu(func(m *ui.Menu) {
						if s.tableID != n.ID || cell.Row < selection.Top || cell.Row > selection.Bottom || cell.Column < selection.Left || cell.Column > selection.Right {
							a.selectTable(n.ID, blocks.Rect{Top: cell.Row, Bottom: cell.Row, Left: cell.Column, Right: cell.Column}, "cell")
						}
						a.tableMenu(m, n.ID, s.cells)
					})
				}
			}
		})
	})
}

func (a *app) tableMenu(m *ui.Menu, id string, rect blocks.Rect) {
	s := a.blockEditor
	n := s.doc.Find(id)
	if n == nil {
		return
	}
	table, _ := a.tableLayout(n)
	rect = table.Expand(rect)
	for _, action := range []struct {
		label string
		fn    func(*blocks.Table) error
	}{
		{"Insert row above", func(t *blocks.Table) error { return t.InsertRow(rect.Top) }},
		{"Insert row below", func(t *blocks.Table) error { return t.InsertRow(rect.Bottom + 1) }},
		{"Insert column left", func(t *blocks.Table) error { return t.InsertColumn(rect.Left) }},
		{"Insert column right", func(t *blocks.Table) error { return t.InsertColumn(rect.Right + 1) }},
		{"Merge selected cells", func(t *blocks.Table) error { return t.Merge(rect) }},
		{"Split cell", func(t *blocks.Table) error { return t.Split(rect.Top, rect.Left) }},
	} {
		if m.Item(a.tr(action.label)).Chosen() {
			s.tableID = id
			a.tableChange(id, action.fn)
		}
	}
	m.Submenu(a.tr("Cell background"), func(m *ui.Menu) {
		for _, name := range []string{"Default", "light-gray", "light-red", "light-orange", "light-yellow", "light-green", "light-blue", "light-purple"} {
			if m.Item(a.tr(name)).Chosen() {
				a.selectTable(id, rect, "range")
				value := name
				if name == "Default" {
					value = ""
				}
				a.tableCellAttribute("background-color", value)
			}
		}
	})
	m.Separator()
	for _, action := range []struct {
		label    string
		disabled bool
		fn       func(*blocks.Table) error
	}{
		{"Move row up", rect.Top == 0, func(t *blocks.Table) error { return t.MoveRows(rect.Top, rect.Bottom+1, rect.Top-1) }},
		{"Move row down", rect.Bottom == table.Rows-1, func(t *blocks.Table) error { return t.MoveRows(rect.Top, rect.Bottom+1, rect.Bottom+2) }},
		{"Move column left", rect.Left == 0, func(t *blocks.Table) error { return t.MoveColumns(rect.Left, rect.Right+1, rect.Left-1) }},
		{"Move column right", rect.Right == table.Columns-1, func(t *blocks.Table) error { return t.MoveColumns(rect.Left, rect.Right+1, rect.Right+2) }},
	} {
		if m.Item(a.tr(action.label)).Disabled(action.disabled).Chosen() {
			a.tableChange(id, action.fn)
		}
	}
	m.Separator()
	if m.Item(a.tr("Header row")).Checked(table.At(0, 0) != nil && table.At(0, 0).Node.Kind == "th").Chosen() {
		a.tableChange(id, func(t *blocks.Table) error {
			header := t.At(0, 0).Node.Kind != "th"
			for _, cell := range t.Cells {
				if cell.Row == 0 {
					cell.Node.Kind = "td"
					if header {
						cell.Node.Kind = "th"
					}
				}
			}
			return nil
		})
	}
	if m.Item(a.tr("Table options")).Chosen() {
		a.editBlockProperties(n)
	}
	m.Separator()
	if m.Item(a.tr("Delete row")).Chosen() {
		a.tableChange(id, func(t *blocks.Table) error { return t.DeleteRows(rect.Top, rect.Bottom+1) })
	}
	if m.Item(a.tr("Delete column")).Chosen() {
		a.tableChange(id, func(t *blocks.Table) error { return t.DeleteColumns(rect.Left, rect.Right+1) })
	}
	if m.Item(a.tr("Delete table")).Chosen() {
		a.removeBlocks([]string{id})
		s.tableID, s.tableSelection = "", ""
	}
}

func (a *app) tableSelectionInput(scroll ui.Element, n *blocks.Node, table *blocks.Table, selection blocks.Rect) {
	s := a.blockEditor
	if s.tableID == n.ID && s.tableSelection != "" && a.mode == "Write" {
		scroll.HandleInput(func(event ui.InputEvent) bool {
			if event.Kind == ui.InputCommand {
				switch event.Text {
				case "copy", "cut":
					a.copyTableCells()
					if event.Text == "cut" {
						a.clearTableCells()
					}
					return true
				case "paste":
					return a.pasteTableCells(s.active, a.services.ReadClipboard())
				case "undo", "redo":
					a.blockUndo(event.Text == "redo")
					return true
				}
			}
			return false
		})
		a.textFocused = true
		if s.tableSelectionFocus {
			scroll.Focus()
			s.tableSelectionFocus = false
		}
		for _, key := range []ui.Key{ui.KeyBackspace, ui.KeyDelete, ui.KeyEscape} {
			if scroll.Shortcut(0, key) {
				a.tableSelectionKey(key, 0)
			}
		}
		if scroll.Shortcut(ui.Cmd, ui.KeyC) || scroll.Shortcut(ui.Ctrl, ui.KeyC) {
			a.copyTableCells()
		}
		if scroll.Shortcut(ui.Cmd, ui.KeyX) || scroll.Shortcut(ui.Ctrl, ui.KeyX) {
			a.copyTableCells()
			a.clearTableCells()
		}
		if scroll.Shortcut(ui.Cmd, ui.KeyV) || scroll.Shortcut(ui.Ctrl, ui.KeyV) {
			if cell := table.At(selection.Top, selection.Left); cell != nil {
				if text := firstCellText(cell.Node); text != nil {
					a.pasteTableCells(text.ID, a.services.ReadClipboard())
				}
			}
		}
	}
}
