package main

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// Text selection uses the ordinary formatting tools, even when the persistent
// toolbar is hidden. Range selection has its own tools and never edits one cell
// through the stale rich-text selection left behind it.
func (a *app) tableTextSelected() bool {
	s := a.blockEditor
	if s == nil || a.doc == nil || s.tableSelection != "" || !s.insideTable(s.active) {
		return false
	}
	anchor, caret := a.doc.Selection()
	return anchor != caret
}

func (a *app) applyLink(url string) {
	if n, _ := a.tableSelectedCells(); n != nil {
		a.formatTableSelection(func(d *richtext.Document) { d.SetLink(url) })
	} else if a.doc != nil {
		a.doc.SetLink(url)
	}
}

func (a *app) expandTable(id string) bool {
	a.captureBlocks()
	s := a.blockEditor
	if a.fail(s.doc.ExpandTable(id)) {
		return false
	}
	// Materializing lazy cells changes their representation, not their source.
	s.version = s.doc.Version
	return true
}

type tableResizeState struct {
	id           string
	column       int
	width, delta float64
	active       bool
}

func tableColumnName(column int) string {
	name := ""
	for column >= 0 {
		name = string(rune('A'+column%26)) + name
		column = column/26 - 1
	}
	return name
}

// Validate against a detached copy before committing a single undo transaction.
// Pending cell typing is flushed first, so structural commands never use stale
// text and failed operations never leave a partially modified table behind.
func (a *app) tableChange(id string, fn func(*blocks.Table) error) bool {
	a.captureBlocks()
	s := a.blockEditor
	n := s.doc.Find(id)
	if n == nil || a.mode != "Write" {
		return false
	}
	copy := n.Clone()
	table := blocks.ReadTable(copy)
	if a.fail(fn(table)) {
		return false
	}
	table.Write(copy)
	s.lastTextID = ""
	if a.fail(s.doc.Transform(id, func(n *blocks.Node) { *n = *copy })) {
		return false
	}
	// Formatting and merge operations retain node IDs. Cached rich documents
	// must be rebuilt, otherwise their previous content can overwrite the edit.
	n.Walk(func(n *blocks.Node) { delete(s.fields, n.ID) })
	s.cells = table.Expand(s.cells)
	a.captureBlocks()
	return true
}

func (a *app) tableCell(id string) (*blocks.Node, *blocks.Cell) {
	s := a.blockEditor
	if s == nil {
		return nil, nil
	}
	var cellID string
	for n := s.doc.Find(id); n != nil; {
		if n.Kind == "td" || n.Kind == "th" {
			cellID = n.ID
		}
		if n.Kind == "table" && cellID != "" {
			t, _ := a.tableLayout(n)
			if i, ok := s.tables[n.ID].cells[cellID]; ok {
				return n, &t.Cells[i]
			}
			return nil, nil
		}
		n, _ = s.doc.Parent(n.ID)
	}
	return nil, nil
}

func (a *app) trackTableCaret(id string) {
	n, cell := a.tableCell(id)
	if cell == nil {
		return
	}
	s := a.blockEditor
	s.tableID = n.ID
	s.cells = blocks.Rect{Top: cell.Row, Bottom: cell.Row + cell.Rows - 1, Left: cell.Column, Right: cell.Column + cell.Columns - 1}
	s.cellAnchor = [2]int{cell.Row, cell.Column}
}

func (a *app) selectTable(id string, rect blocks.Rect, kind string) {
	a.captureBlocks()
	s := a.blockEditor
	n := s.doc.Find(id)
	if n == nil {
		return
	}
	if n.Compact != nil && !a.expandTable(id) {
		return
	}
	table, _ := a.tableLayout(n)
	s.tableID, s.cells, s.tableSelection = id, table.Expand(rect), kind
	s.cellAnchor = [2]int{rect.Top, rect.Left}
	s.focus = ""
	s.tableSelectionFocus = true
	s.chosen.Clear()
	if cell := table.At(rect.Top, rect.Left); cell != nil {
		if text := firstCellText(cell.Node); text != nil {
			s.active = text.ID
		}
	}
}

func firstCellText(cell *blocks.Node) *blocks.Node {
	var text *blocks.Node
	cell.Walk(func(n *blocks.Node) {
		if text == nil && blocks.IsText(n) {
			text = n
		}
	})
	return text
}

func (a *app) focusTableCell(id string, row, column int, end bool) {
	s := a.blockEditor
	n := s.doc.Find(id)
	if n == nil {
		return
	}
	table, _ := a.tableLayout(n)
	cell := table.At(row, column)
	if cell == nil {
		return
	}
	text := firstCellText(cell.Node)
	if text == nil {
		return
	}
	s.tableID, s.tableSelection = id, ""
	s.tableSelectionFocus = false
	s.cells = blocks.Rect{Top: cell.Row, Bottom: cell.Row + cell.Rows - 1, Left: cell.Column, Right: cell.Column + cell.Columns - 1}
	s.cellAnchor = [2]int{cell.Row, cell.Column}
	offset := 0
	if end {
		offset = len([]rune(text.Plain()))
	}
	a.focusBlock(text.ID, offset)
	if cached := s.tables[id]; cached.rows != nil {
		cached.rows.ScrollTo(row, ui.Center)
	}
}

func (a *app) tableCellShortcuts(ed ui.Element, n *blocks.Node, field *blockField) {
	forward, back := ed.Shortcut(0, ui.KeyTab), ed.Shortcut(ui.Shift, ui.KeyTab)
	exit := ed.Shortcut(ui.Cmd, ui.KeyEnter) || ed.Shortcut(ui.Ctrl, ui.KeyEnter)
	if !forward && !back && !exit {
		return
	}
	a.captureBlocks()
	tableNode, cell := a.tableCell(n.ID)
	if cell == nil {
		return
	}
	if exit {
		a.exitTable(tableNode.ID)
		return
	}
	table, _ := a.tableLayout(tableNode)
	index := 0
	for i, item := range table.Cells {
		if item.Node.ID == cell.Node.ID {
			index = i
			break
		}
	}
	if back {
		if index > 0 {
			next := table.Cells[index-1]
			a.focusTableCell(tableNode.ID, next.Row, next.Column, true)
		}
		return
	}
	if index+1 < len(table.Cells) {
		next := table.Cells[index+1]
		a.focusTableCell(tableNode.ID, next.Row, next.Column, false)
	} else if a.tableChange(tableNode.ID, func(t *blocks.Table) error { return t.InsertRow(t.Rows) }) {
		a.focusTableCell(tableNode.ID, table.Rows, 0, false)
	}
}

func (a *app) exitTable(id string) {
	s := a.blockEditor
	parent, at := s.doc.Parent(id)
	if parent == nil {
		return
	}
	var next *blocks.Node
	for _, node := range parent.Children[at+1:] {
		if node.Kind == "#raw" {
			continue
		}
		if blocks.IsText(node) {
			next = node
		}
		break
	}
	if next == nil {
		next = blocks.Paragraph("")
		a.fail(s.doc.Splice(parent.ID, at+1, 0, next))
		a.captureBlocks()
	}
	s.tableSelection, s.tableID = "", ""
	a.focusBlock(next.ID, 0)
}

// Cell boundaries belong to the table. Never let a generic block merge or
// adjacency action jump through an unrelated cell or destroy its structure.
func (a *app) tableBoundaryInput(n *blocks.Node, field *blockField, event ui.InputEvent) bool {
	if event.Mods != 0 {
		return false
	}
	_, caret := field.doc.Selection()
	if event.Key == ui.KeyBackspace && caret == 0 || event.Key == ui.KeyDelete && caret == field.doc.Text().Len() {
		return true
	}
	if event.Key != ui.KeyLeft && event.Key != ui.KeyRight && event.Key != ui.KeyUp && event.Key != ui.KeyDown {
		return false
	}
	if caret != 0 && caret != field.doc.Text().Len() {
		return false
	}
	tableNode, cell := a.tableCell(n.ID)
	if cell == nil {
		return false
	}
	row, col := cell.Row, cell.Column
	switch event.Key {
	case ui.KeyUp:
		if caret != 0 {
			return true
		}
		row--
	case ui.KeyDown:
		if caret != field.doc.Text().Len() {
			return true
		}
		row += cell.Rows
	case ui.KeyLeft:
		if caret != 0 {
			return true
		}
		col--
	case ui.KeyRight:
		if caret != field.doc.Text().Len() {
			return true
		}
		col += cell.Columns
	}
	a.focusTableCell(tableNode.ID, row, col, event.Key == ui.KeyLeft || event.Key == ui.KeyUp)
	return true
}

func (a *app) tableSelectedCells() (*blocks.Node, blocks.Rect) {
	s := a.blockEditor
	if s == nil || s.tableSelection == "" || a.mode != "Write" {
		return nil, blocks.Rect{}
	}
	n := s.doc.Find(s.tableID)
	if n == nil {
		return nil, blocks.Rect{}
	}
	t, _ := a.tableLayout(n)
	return n, t.Expand(s.cells)
}

func (a *app) formatTableSelection(fn func(*richtext.Document)) {
	n, rect := a.tableSelectedCells()
	if n == nil {
		return
	}
	a.tableChange(n.ID, func(t *blocks.Table) error {
		for _, cell := range t.Cells {
			if cell.Row < rect.Top || cell.Row > rect.Bottom || cell.Column < rect.Left || cell.Column > rect.Right {
				continue
			}
			var format func(*blocks.Node)
			format = func(parent *blocks.Node) {
				for i := 0; i < len(parent.Children); i++ {
					child := parent.Children[i]
					if blocks.IsText(child) {
						doc := a.blockField(child, a.documentColor).doc
						copy := richtext.NewDocument(doc.Text())
						copy.SetSelection(0, copy.Text().Len())
						fn(copy)
						replacement := blocks.FromRich(copy.Text(), child)
						if len(replacement) == 1 {
							parent.Children[i] = replacement[0]
						}
					} else {
						format(child)
					}
				}
			}
			format(cell.Node)
			// Inherited cell alignment is now represented on each paragraph.
			// Clearing it also lets an explicit Left command survive reopening.
			cell.Node.Set("align", "")
		}
		return nil
	})
}

func (a *app) tableCellAttribute(key, value string) {
	n, rect := a.tableSelectedCells()
	if n == nil {
		return
	}
	a.tableChange(n.ID, func(t *blocks.Table) error {
		for _, cell := range t.Cells {
			if cell.Row >= rect.Top && cell.Row <= rect.Bottom && cell.Column >= rect.Left && cell.Column <= rect.Right {
				cell.Node.Set(key, value)
			}
		}
		return nil
	})
}

func (a *app) tableSelectionToolbar(c *ui.Context) {
	n, rect := a.tableSelectedCells()
	if n == nil {
		return
	}
	ui.Row(c).Padding(6, 28).Gap(4).Wrap().AlignItems(ui.Center).BorderWidth(0, 0, 1).BorderColor(c.Theme().Border).Children(func() {
		ui.Text(c, fmt.Sprintf("%s%d – %s%d", tableColumnName(rect.Left), rect.Top+1, tableColumnName(rect.Right), rect.Bottom+1)).FontSize(12).TextColor(c.Theme().TextMuted).Margin(0, 8, 0, 0)
		for _, item := range []struct {
			label, text string
			fn          func(*richtext.Document)
		}{
			{"Bold", "B", func(d *richtext.Document) { d.Format(func(a *richtext.Attributes) { a.Weight = 700 }) }},
			{"Italic", "I", func(d *richtext.Document) { d.Format(func(a *richtext.Attributes) { a.Italic = true }) }},
			{"Underline", "U", func(d *richtext.Document) { d.Format(func(a *richtext.Attributes) { a.Underline = true }) }},
			{"Strikethrough", "S", func(d *richtext.Document) { d.Format(func(a *richtext.Attributes) { a.Strikethrough = true }) }},
			{"Inline code", "‹›", func(d *richtext.Document) { d.Format(func(a *richtext.Attributes) { a.Code = true }) }},
			{"Clear formatting", "Tx", func(d *richtext.Document) {
				d.Format(func(a *richtext.Attributes) {
					a.Weight = 0
					a.Italic = false
					a.Underline = false
					a.Strikethrough = false
					a.Code = false
					a.Color = ui.Color{}
					a.Background = ui.Color{}
				})
			}},
		} {
			if ghostButton(c, item.text).Label(a.tr(item.label)).Tooltip(a.tr(item.label)).KeepFocus().Clicked() {
				if item.label == "Clear formatting" {
					a.formatTableSelection(item.fn)
				} else {
					a.toggleTableFormat(item.label)
				}
			}
		}
		ghostButton(c, a.tr("Paragraph style")).KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
			for _, item := range []string{"Body", "Heading 1", "Heading 2", "Heading 3", "Heading 4", "Heading 5", "Heading 6", "Bullet list", "Numbered list", "Task list"} {
				if m.Item(a.tr(item)).Chosen() {
					a.formatTableSelection(func(d *richtext.Document) {
						if strings.HasPrefix(item, "Heading ") {
							var level int
							fmt.Sscanf(item, "Heading %d", &level)
							d.SetHeading(level)
						} else if item == "Body" {
							d.SetHeading(0)
							d.FormatParagraphs(func(p *richtext.ParagraphStyle) { p.List = richtext.NoList })
						} else {
							d.FormatParagraphs(func(p *richtext.ParagraphStyle) {
								p.List = map[string]richtext.ListKind{"Bullet list": richtext.Bullets, "Numbered list": richtext.Numbers, "Task list": richtext.Tasks}[item]
							})
						}
					})
				}
			}
		})
		if ghostButton(c, a.tr("Link")).KeepFocus().Clicked() {
			a.linkURL, a.linkOpen = "", true
		}
		ghostButton(c, a.tr("Cell background")).KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
			for _, name := range []string{"Default", "light-gray", "light-red", "light-orange", "light-yellow", "light-green", "light-blue", "light-purple"} {
				if m.Item(a.tr(name)).Chosen() {
					value := name
					if name == "Default" {
						value = ""
					}
					a.tableCellAttribute("background-color", value)
				}
			}
		})
		for _, kind := range []string{"Color", "Highlight"} {
			ghostButton(c, a.tr(kind)).KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
				for _, name := range []string{"Default", "gray", "red", "orange", "yellow", "green", "blue", "purple"} {
					if m.Item(a.tr(name)).Chosen() {
						a.formatTableSelection(func(d *richtext.Document) {
							d.Format(func(attr *richtext.Attributes) {
								color := blockColor(name, c.Theme().Text)
								if kind == "Highlight" {
									color = blockColor("light-"+name, ui.Transparent)
								}
								if name == "Default" {
									color = ui.Color{}
								}
								if kind == "Color" {
									attr.Color = color
								} else {
									attr.Background = color
								}
							})
						})
					}
				}
			})
		}
		ghostButton(c, a.tr("Align")).KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
			for _, item := range []struct {
				name  string
				align ui.Align
			}{{"Left", ui.Start}, {"Center", ui.Center}, {"Right", ui.End}} {
				if m.Item(a.tr(item.name)).Chosen() {
					a.formatTableSelection(func(d *richtext.Document) { d.SetAlign(item.align) })
				}
			}
			m.Separator()
			for _, value := range []string{"top", "middle", "bottom"} {
				if m.Item(a.tr("Align " + value)).Chosen() {
					a.tableCellAttribute("vertical-align", value)
				}
			}
		})
		if ghostButton(c, a.tr("Merge cells")).KeepFocus().Clicked() {
			a.tableChange(n.ID, func(t *blocks.Table) error { return t.Merge(rect) })
		}
		if ghostButton(c, a.tr("Split cell")).KeepFocus().Clicked() {
			a.tableChange(n.ID, func(t *blocks.Table) error { return t.Split(rect.Top, rect.Left) })
		}
		ghostButton(c, a.tr("Table actions")).KeepFocus().PopoverMenuButton(func(m *ui.Menu) { a.tableMenu(m, n.ID, rect) })
		if ghostButton(c, a.tr("Clear cells")).KeepFocus().Clicked() {
			a.clearTableCells()
		}
	})
}

func (a *app) toggleTableFormat(kind string) {
	n, rect := a.tableSelectedCells()
	if n == nil {
		return
	}
	all, any := true, false
	t, _ := a.tableLayout(n)
	for _, cell := range t.Cells {
		if cell.Row < rect.Top || cell.Row > rect.Bottom || cell.Column < rect.Left || cell.Column > rect.Right {
			continue
		}
		cell.Node.Walk(func(n *blocks.Node) {
			if !blocks.IsText(n) {
				return
			}
			for _, p := range a.blockField(n, a.documentColor).doc.Text().Paragraphs() {
				for _, r := range p.Runs {
					if r.Text == "" {
						continue
					}
					any = true
					v := r.Attributes
					all = all && map[string]bool{"Bold": v.Weight >= 600, "Italic": v.Italic, "Underline": v.Underline, "Strikethrough": v.Strikethrough, "Inline code": v.Code}[kind]
				}
			}
		})
	}
	on := !any || !all
	a.formatTableSelection(func(d *richtext.Document) {
		d.Format(func(v *richtext.Attributes) {
			switch kind {
			case "Bold":
				v.Weight = 0
				if on {
					v.Weight = 700
				}
			case "Italic":
				v.Italic = on
			case "Underline":
				v.Underline = on
			case "Strikethrough":
				v.Strikethrough = on
			case "Inline code":
				v.Code = on
			}
		})
	})
}

func (a *app) copyTableCells() {
	n, r := a.tableSelectedCells()
	if n == nil {
		return
	}
	t, _ := a.tableLayout(n)
	var output strings.Builder
	w := csv.NewWriter(&output)
	w.Comma = '\t'
	for row := r.Top; row <= r.Bottom; row++ {
		var cells []string
		for col := r.Left; col <= r.Right; col++ {
			cell := t.At(row, col)
			text := ""
			if cell != nil && cell.Row == row && cell.Column == col {
				var paragraphs []string
				for _, child := range blocks.Visible(cell.Node.Children) {
					paragraphs = append(paragraphs, child.Plain())
				}
				text = strings.Join(paragraphs, "\n")
			}
			cells = append(cells, text)
		}
		_ = w.Write(cells)
	}
	w.Flush()
	a.services.WriteClipboard(strings.TrimSuffix(output.String(), "\n"))
}

func (a *app) tableSelectionKey(key ui.Key, mods ui.Modifiers) bool {
	n, rect := a.tableSelectedCells()
	if n == nil {
		return false
	}
	if mods == 0 && key == ui.KeyEscape {
		a.focusTableCell(n.ID, rect.Top, rect.Left, false)
		return true
	}
	if isBlockDeleteKey(key, mods) {
		a.clearTableCells()
		return true
	}
	return false
}

func (a *app) clearTableCells() {
	n, rect := a.tableSelectedCells()
	if n == nil {
		return
	}
	a.tableChange(n.ID, func(t *blocks.Table) error {
		for _, cell := range t.Cells {
			if cell.Row >= rect.Top && cell.Row <= rect.Bottom && cell.Column >= rect.Left && cell.Column <= rect.Right {
				cell.Node.Children = []*blocks.Node{blocks.Paragraph("")}
			}
		}
		return nil
	})
}

// Spreadsheet text fills cells as one undoable operation. Ordinary multiline
// text remains in its current cell; tabs explicitly identify tabular input.
func (a *app) pasteTableCells(id, text string) bool {
	if a.mode != "Write" || !strings.Contains(text, "\t") && (a.blockEditor == nil || a.blockEditor.tableSelection == "") {
		return false
	}
	n, cell := a.tableCell(id)
	if cell == nil {
		return false
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma, r.FieldsPerRecord = '\t', -1
	values, err := r.ReadAll()
	if err != nil {
		// Plain tab-separated text may contain unquoted literal quote marks.
		values = nil
		for _, row := range strings.Split(strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n") {
			values = append(values, strings.Split(row, "\t"))
		}
	}
	width := 0
	for _, row := range values {
		width = max(width, len(row))
	}
	if len(values)+cell.Row > 1000 || width+cell.Column > 100 {
		a.fail(fmt.Errorf("pasted table exceeds 1000 rows or 100 columns"))
		return true
	}
	row, col := cell.Row, cell.Column
	if a.tableChange(n.ID, func(t *blocks.Table) error {
		for _, c := range t.Cells {
			if c.Row < row+len(values) && c.Row+c.Rows > row && c.Column < col+width && c.Column+c.Columns > col && (c.Rows > 1 || c.Columns > 1) {
				return fmt.Errorf("split merged cells before pasting a grid")
			}
		}
		for t.Rows < row+len(values) {
			if err := t.InsertRow(t.Rows); err != nil {
				return err
			}
		}
		for t.Columns < col+width {
			if err := t.InsertColumn(t.Columns); err != nil {
				return err
			}
		}
		for r, columns := range values {
			for c, value := range columns {
				t.At(row+r, col+c).Node.Children = []*blocks.Node{blocks.Paragraph(value)}
			}
		}
		return nil
	}) {
		a.focusTableCell(n.ID, row, col, false)
	}
	return true
}
