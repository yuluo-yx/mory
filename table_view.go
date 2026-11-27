package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

const tableRowHandleWidth float32 = 24

func (a *app) blockTable(c *ui.Context, n *blocks.Node, readOnly bool, depth int) {
	s := a.blockEditor
	table, simple := a.tableLayout(n)
	if simple && table.Rows >= 200 {
		a.largeTableView(c, n, table, depth)
		return
	}
	selection := table.Expand(s.cells)
	if !readOnly {
		ui.Row(c).Gap(5).Padding(3, 0).AlignItems(ui.Center).Children(func() {
			if ghostButton(c, "+").Label(a.tr("+ Row")).Tooltip(a.tr("+ Row")).KeepFocus().Clicked() {
				at := table.Rows
				if s.tableID == n.ID {
					at = selection.Bottom + 1
				}
				if a.tableChange(n.ID, func(t *blocks.Table) error { return t.InsertRow(at) }) {
					a.focusTableCell(n.ID, at, 0, false)
				}
			}
			if ghostButton(c, "+ │").Label(a.tr("+ Column")).Tooltip(a.tr("+ Column")).KeepFocus().Clicked() {
				at := table.Columns
				if s.tableID == n.ID {
					at = selection.Right + 1
				}
				if a.tableChange(n.ID, func(t *blocks.Table) error { return t.InsertColumn(at) }) {
					a.focusTableCell(n.ID, 0, at, false)
				}
			}
			ghostButton(c, "•••").Label(a.tr("Table actions")).Tooltip(a.tr("Table actions")).KeepFocus().PopoverMenuButton(func(m *ui.Menu) { a.tableMenu(m, n.ID, selection) })
			ui.Text(c, fmt.Sprintf("%d × %d", table.Rows, table.Columns)).FontSize(11).TextColor(c.Theme().TextMuted)
		})
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
	scroll := ui.ScrollHorizontal(c).FillWidth().Padding(0, 0, 12).Label(a.tr("Table viewport")).Focusable()
	if !readOnly {
		a.tableSelectionInput(scroll, n, table, selection)
	}
	scroll.Children(func() {
		ui.Grid(c).ColumnTracks(tracks...).Width(total).Gap(0).Children(func() {
			rowOffset := 0
			if !readOnly {
				rowOffset = 1
				if ghostButton(c, "⋮").Label(a.tr("Select table")).ColumnStart(1).RowStart(1).Height(22).KeepFocus().Clicked() {
					a.selectTable(n.ID, blocks.Rect{Top: 0, Bottom: table.Rows - 1, Left: 0, Right: table.Columns - 1}, "table")
				}
				for col, width := range table.Widths {
					ui.Row(c.Key(col)).ColumnStart(col + 2).RowStart(1).Height(22).AlignItems(ui.Stretch).Children(func() {
						button := ghostButton(c, tableColumnName(col)).Label(a.tr("Select column ") + tableColumnName(col)).FontSize(10).Grow(1).Padding(0).KeepFocus()
						if s.tableID == n.ID && s.tableSelection == "column" && col >= selection.Left && col <= selection.Right {
							button.Background(c.Theme().Accent.Alpha(.18))
						}
						if button.Clicked() {
							start := col
							if button.ClickModifiers()&ui.Shift != 0 && s.tableID == n.ID && s.tableSelection == "column" {
								start = s.cellAnchor[1]
							}
							a.selectTable(n.ID, blocks.Rect{Top: 0, Bottom: table.Rows - 1, Left: min(start, col), Right: max(start, col)}, "column")
						}
						resize := ui.Box(c.Key("resize")).Width(6).Height(22).Label(a.tr("Resize column ") + tableColumnName(col)).Cursor(ui.CursorResizeEW).Focusable()
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
								s.tableResize = tableResizeState{id: n.ID, column: col, width: width, active: true}
							}
							s.tableResize.delta += float64(dx)
							resize.Focus(ui.FocusOptions{PreventScroll: true})
						}
						if s.tableResize.active && s.tableResize.id == n.ID && s.tableResize.column == col && !held {
							value := s.tableResize.width + s.tableResize.delta
							s.tableResize.active = false
							a.tableChange(n.ID, func(t *blocks.Table) error { return t.Resize(col, max(64, value)) })
						}
					})
				}
				for row := 0; row < table.Rows; row++ {
					button := ghostButton(c, fmt.Sprint(row+1)).Label(a.trf("Select row %d", row+1)).ColumnStart(1).RowStart(row + 2).FontSize(10).Padding(0).KeepFocus()
					if s.tableID == n.ID && s.tableSelection == "row" && row >= selection.Top && row <= selection.Bottom {
						button.Background(c.Theme().Accent.Alpha(.18))
					}
					if button.Clicked() {
						start := row
						if button.ClickModifiers()&ui.Shift != 0 && s.tableID == n.ID && s.tableSelection == "row" {
							start = s.cellAnchor[0]
						}
						a.selectTable(n.ID, blocks.Rect{Top: min(start, row), Bottom: max(start, row), Left: 0, Right: table.Columns - 1}, "row")
					}
				}
			}
			for _, cell := range table.Cells {
				background := c.Theme().Background
				if cell.Node.Kind == "th" {
					background = a.docTheme.Color("th", "background-color", c.Theme().Surface)
				}
				if value := cell.Node.Attr("background-color"); value != "" {
					background = blockColor(value, background)
				}
				chosen := s.tableID == n.ID && cell.Row >= selection.Top && cell.Row <= selection.Bottom && cell.Column >= selection.Left && cell.Column <= selection.Right
				box := ui.Column(c.Key(cell.Node.ID)).ColumnStart(cell.Column+1+offset).RowStart(cell.Row+1+rowOffset).ColumnSpan(cell.Columns).RowSpan(cell.Rows).MinWidth(0).MinHeight(42).Padding(6, 10).Gap(2).Background(background).
					BorderWidth(0, 1, 1, 0).BorderColor(a.docTheme.Color("table", "border-color", c.Theme().Border)).Label(a.trf("Table cell %d %d", cell.Row+1, cell.Column+1))
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
					// The cell selector lives in the corner instead of taking a text row.
					if !readOnly {
						button := ghostButton(c, "").Label(a.trf("Select cell %d %d", cell.Row+1, cell.Column+1)).Absolute().Top(0).Right(0).Width(8).Height(8).Padding(0).KeepFocus()
						if box.Hovered() {
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
