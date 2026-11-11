package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/mathrender"
)

func (a *app) tableChange(id string, fn func(*blocks.Table) error) {
	a.captureBlocks()
	err := a.blockEditor.doc.Transform(id, func(n *blocks.Node) {
		table := blocks.ReadTable(n)
		if a.fail(fn(table)) {
			return
		}
		table.Write(n)
	})
	a.fail(err)
	a.captureBlocks()
}

func (a *app) blockTable(c *ui.Context, n *blocks.Node, readOnly bool, depth int) {
	s := a.blockEditor
	t := c.Theme()
	table, simple := a.tableLayout(n)
	selection := blocks.Rect{Top: 0, Left: 0, Bottom: 0, Right: 0}
	if s.tableID == n.ID {
		selection = s.cells
	}
	selection = table.Expand(selection)
	if simple && table.Rows >= 200 {
		a.largeTableFallback(c, n, table, readOnly)
		return
	}
	if !readOnly {
		ui.Row(c).Wrap().Gap(5).Padding(4, 0).Children(func() {
			for _, action := range []struct {
				label string
				run   func(*blocks.Table) error
			}{{"+ Row", func(t *blocks.Table) error { return t.InsertRow(selection.Bottom + 1) }}, {"+ Column", func(t *blocks.Table) error { return t.InsertColumn(selection.Right + 1) }}, {"Merge cells", func(t *blocks.Table) error { return t.Merge(selection) }}, {"Split cell", func(t *blocks.Table) error { return t.Split(selection.Top, selection.Left) }}, {"Delete row", func(t *blocks.Table) error { return t.DeleteRows(selection.Top, selection.Bottom+1) }}, {"Delete column", func(t *blocks.Table) error { return t.DeleteColumns(selection.Left, selection.Right+1) }}} {
				if ghostButton(c, a.tr(action.label)).FontSize(11).Padding(5, 7).Clicked() {
					s.tableID = n.ID
					a.tableChange(n.ID, action.run)
				}
			}
			if ghostButton(c, a.tr("Table options")).FontSize(11).Clicked() {
				s.tableID = n.ID
				a.editBlockProperties(n)
			}
		})
	}
	tracks := make([]ui.Track, table.Columns)
	total := float32(0)
	for i, width := range table.Widths {
		tracks[i] = ui.Fixed(float32(width))
		total += float32(width)
	}
	ui.ScrollHorizontal(c).FillWidth().Children(func() {
		ui.Column(c).Width(total).Children(func() {
			if !readOnly {
				ui.Row(c).Width(total).Children(func() {
					for column, width := range table.Widths {
						ui.Row(c.Key(column)).Width(float32(width)).Gap(2).AlignItems(ui.Center).Children(func() {
							label := fmt.Sprintf("%c", 'A'+rune(column))
							if ghostButton(c, label).Label(a.tr("Select column ") + label).Grow(1).Clicked() {
								s.tableID = n.ID
								s.cells = blocks.Rect{Top: 0, Bottom: table.Rows - 1, Left: column, Right: column}
							}
							resize := ui.Box(c).Width(5).Height(24).Background(t.Border).Cursor(ui.CursorResizeEW)
							if dx, _, dragged := resize.Dragged(); dragged {
								a.tableChange(n.ID, func(t *blocks.Table) error { return t.Resize(column, width+float64(dx)) })
							}
						})
					}
				})
			}
			ui.Grid(c).ColumnTracks(tracks...).Width(total).Gap(0).Children(func() {
				for _, cell := range table.Cells {
					background := t.Background
					if cell.Node.Kind == "th" {
						background = a.docTheme.Color("th", "background-color", t.Surface)
					}
					if value := cell.Node.Attr("background-color"); value != "" {
						background = blockColor(value, background)
					}
					chosen := s.tableID == n.ID && cell.Row >= selection.Top && cell.Row <= selection.Bottom && cell.Column >= selection.Left && cell.Column <= selection.Right
					box := ui.Column(c.Key(cell.Node.ID)).ColumnStart(cell.Column+1).RowStart(cell.Row+1).ColumnSpan(cell.Columns).RowSpan(cell.Rows).Padding(7).Gap(2).MinHeight(55).Border(1, t.Border).Background(background)
					box.BorderColor(a.docTheme.Color("table", "border-color", t.Border))
					if cell.Node.Attr("vertical-align") == "middle" {
						box.Justify(ui.Center)
					} else if cell.Node.Attr("vertical-align") == "bottom" {
						box.Justify(ui.End)
					}
					if chosen && !readOnly {
						box.Border(1, t.Accent)
					}
					box.Children(func() {
						if !readOnly {
							button := ghostButton(c, fmt.Sprintf("%c%d", 'A'+rune(cell.Column), cell.Row+1)).Label(a.trf("Select cell %d %d", cell.Row+1, cell.Column+1)).FontSize(9).Padding(1, 3).TextColor(t.TextMuted).KeepFocus()
							if button.Clicked() {
								if button.ClickModifiers()&ui.Shift == 0 || s.tableID != n.ID {
									s.cellAnchor = [2]int{cell.Row, cell.Column}
								}
								s.tableID = n.ID
								s.cells = table.Expand(blocks.Rect{Top: min(s.cellAnchor[0], cell.Row), Left: min(s.cellAnchor[1], cell.Column), Bottom: max(s.cellAnchor[0], cell.Row), Right: max(s.cellAnchor[1], cell.Column)})
							}
						}
						children := blocks.Visible(cell.Node.Children)
						if len(children) == 0 {
							ui.Text(c, "")
						}
						for _, child := range children {
							a.blockNode(c, child, readOnly, depth+1)
						}
					})
					if !readOnly {
						box.PopoverMenu(func(m *ui.Menu) {
							s.tableID = n.ID
							if m.Item(a.tr("Merge selected cells")).Chosen() {
								a.tableChange(n.ID, func(t *blocks.Table) error { return t.Merge(s.cells) })
							}
							if m.Item(a.tr("Split cell")).Chosen() {
								a.tableChange(n.ID, func(t *blocks.Table) error { return t.Split(cell.Row, cell.Column) })
							}
							m.Submenu(a.tr("Cell background"), func(m *ui.Menu) {
								for _, name := range []string{"light-gray", "light-yellow", "light-green", "light-blue", "light-red"} {
									if m.Item(a.tr(name)).Chosen() {
										a.fail(s.doc.Transform(cell.Node.ID, func(n *blocks.Node) { n.Set("background-color", name) }))
									}
								}
							})
							if m.Item(a.tr("Move row up")).Disabled(cell.Row == 0).Chosen() {
								a.tableChange(n.ID, func(t *blocks.Table) error { return t.MoveRows(cell.Row, cell.Row+1, cell.Row-1) })
							}
							if m.Item(a.tr("Move row down")).Disabled(cell.Row == table.Rows-1).Chosen() {
								a.tableChange(n.ID, func(t *blocks.Table) error { return t.MoveRows(cell.Row, cell.Row+1, cell.Row+2) })
							}
							if m.Item(a.tr("Move column left")).Disabled(cell.Column == 0).Chosen() {
								a.tableChange(n.ID, func(t *blocks.Table) error { return t.MoveColumns(cell.Column, cell.Column+1, cell.Column-1) })
							}
							if m.Item(a.tr("Move column right")).Disabled(cell.Column == table.Columns-1).Chosen() {
								a.tableChange(n.ID, func(t *blocks.Table) error { return t.MoveColumns(cell.Column, cell.Column+1, cell.Column+2) })
							}
						})
					}
				}
			})
		})
	})
}

func (a *app) tableLayout(n *blocks.Node) (*blocks.Table, bool) {
	s := a.blockEditor
	if cached, ok := s.tables[n.ID]; ok && cached.node == n {
		return cached.table, cached.simple
	}
	table := blocks.ReadTable(n)
	if n.Compact != nil {
		s.tables[n.ID] = tableCache{node: n, table: table, simple: true}
		return table, true
	}
	simple := len(table.Cells) == table.Rows*table.Columns
	if simple {
		for _, cell := range table.Cells {
			if cell.Rows != 1 || cell.Columns != 1 {
				simple = false
				break
			}
		}
	}
	s.tables[n.ID] = tableCache{node: n, table: table, simple: simple}
	return table, simple
}

func (a *app) largeTableFallback(c *ui.Context, n *blocks.Node, table *blocks.Table, readOnly bool) {
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(16).Gap(8).Background(t.Surface).Border(1, t.Border).Radius(8).Children(func() {
		ui.Textf(c, a.tr("Large table: %d rows × %d columns"), table.Rows, table.Columns).Bold()
		ui.Text(c, a.largeTablePreview(table)).Font("Menlo").FontSize(11).TextColor(t.TextMuted).FillWidth().Clip()
		if readOnly {
			ui.Text(c, a.tr("This table is shown in a compact preview to keep scrolling responsive.")).FontSize(11).TextColor(t.TextMuted)
		} else {
			ui.Text(c, a.tr("This table uses a compact preview to keep editing responsive.")).FontSize(11).TextColor(t.TextMuted)
		}
		if ghostButton(c, a.tr("Open Source mode")).Clicked() {
			a.changeMode("Source")
		}
	})
}

func (a *app) largeTablePreview(table *blocks.Table) string {
	rows := make([]int, 0, 6)
	for row := 0; row < table.Rows && len(rows) < 3; row++ {
		rows = append(rows, row)
	}
	for row := max(3, table.Rows-3); row < table.Rows; row++ {
		if row >= 0 {
			rows = append(rows, row)
		}
	}
	var b strings.Builder
	for i, row := range rows {
		if i == 3 && table.Rows > 6 {
			b.WriteString("…\n")
		}
		for column := 0; column < table.Columns; column++ {
			if column > 0 {
				b.WriteString(" | ")
			}
			text := ""
			if cell := table.At(row, column); cell != nil {
				text = cell.Node.Plain()
			} else if table.Preview != nil {
				text = table.Preview[[2]int{row, column}]
			}
			runes := []rune(text)
			if len(runes) > 96 {
				text = string(runes[:96]) + "…"
			}
			b.WriteString(text)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (a *app) blockColumns(c *ui.Context, n *blocks.Node, readOnly bool, depth int) {
	columns := n.Elements()
	if len(columns) == 0 {
		return
	}
	ui.Row(c).Gap(16).FillWidth().AlignItems(ui.Stretch).Children(func() {
		for _, column := range columns {
			ratio := column.Float("width-ratio", 1/float64(len(columns)))
			ui.Column(c.Key(column.ID)).Grow(float32(ratio)).MinWidth(90).Gap(8).BorderWidth(0, 0, 0, 1).BorderColor(c.Theme().Border).Padding(0, 8).Children(func() {
				for _, child := range blocks.Visible(column.Children) {
					a.blockNode(c, child, readOnly, depth+1)
				}
				if !readOnly {
					ui.Row(c).Gap(4).Children(func() {
						if ghostButton(c, a.tr("+ Block")).Clicked() {
							p := blocks.Paragraph("")
							a.fail(a.blockEditor.doc.Splice(column.ID, len(column.Children), 0, p))
							a.blockEditor.focus = p.ID
						}
						if ghostButton(c, a.tr("Width…")).Clicked() {
							a.editBlockProperties(column)
						}
					})
				}
			})
		}
	})
	if !readOnly {
		ui.Row(c).Gap(8).Children(func() {
			if ghostButton(c, a.tr("+ Column")).Clicked() {
				column := blocks.New("column", blocks.Paragraph(""))
				column.Set("width-ratio", fmt.Sprint(1/float64(len(columns)+1)))
				a.fail(a.blockEditor.doc.Splice(n.ID, len(n.Children), 0, column))
			}
			if len(columns) > 2 && ghostButton(c, a.tr("Remove last column")).Clicked() {
				last := columns[len(columns)-1]
				previous := columns[len(columns)-2]
				a.fail(a.blockEditor.doc.Transaction(func() error {
					if err := a.blockEditor.doc.Splice(previous.ID, len(previous.Children), 0, last.Clone().Children...); err != nil {
						return err
					}
					return a.blockEditor.doc.Remove(last.ID)
				}))
			}
		})
	}
}

func (a *app) blockPicture(c *ui.Context, n *blocks.Node, readOnly bool) {
	path := n.Attr("href")
	if path == "" {
		path = n.Attr("src")
	}
	if path == "" {
		path = n.Attr("path")
	}
	bitmap := a.blockImage(path)
	if bitmap == nil {
		ui.Text(c, a.tr("Image: ")+path).TextColor(c.Theme().TextMuted)
	} else {
		if crop := n.Attr("image.crop"); crop != "" {
			bitmap = a.croppedImage(path, crop, bitmap)
		}
		w, h := bitmap.Size()
		width := float32(n.Float("width", float64(min(w, 600))))
		height := float32(n.Float("height", float64(width)*float64(h)/float64(max(w, 1))))
		align := ui.Start
		if n.Attr("align") == "center" {
			align = ui.Center
		} else if n.Attr("align") == "right" {
			align = ui.End
		}
		ui.Row(c).Justify(align).FillWidth().Children(func() {
			ui.Image(c, bitmap).Width(width).MaxWidthPercent(100).Height(height).Fit(ui.Contain).Label(n.Attr("caption"))
		})
	}
	if caption := n.Attr("caption"); caption != "" {
		ui.Text(c, caption).TextColor(c.Theme().TextMuted).FontSize(12)
	}
	if !readOnly && ghostButton(c, a.tr("Image size, crop & caption")).Clicked() {
		a.editBlockProperties(n)
	}
}

func (a *app) croppedImage(path, crop string, fallback *ui.Bitmap) *ui.Bitmap {
	s := a.blockEditor
	key := path + "|" + crop
	if bitmap := s.images[key]; bitmap != nil {
		return bitmap
	}
	var left, top, right, bottom float64
	_, err := fmt.Sscanf(strings.NewReplacer(",", " ", "[", " ", "]", " ").Replace(crop), "%f %f %f %f", &left, &top, &right, &bottom)
	if err != nil || right <= left || bottom <= top {
		return fallback
	}
	root, err := os.OpenRoot(a.assetRoot())
	if err != nil {
		return fallback
	}
	defer root.Close()
	data, err := root.ReadFile(path)
	if err != nil {
		return fallback
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fallback
	}
	bounds := source.Bounds()
	rect := image.Rect(int(left*float64(bounds.Dx())), int(top*float64(bounds.Dy())), int(right*float64(bounds.Dx())), int(bottom*float64(bounds.Dy()))).Intersect(bounds)
	if rect.Empty() {
		return fallback
	}
	out := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(out, out.Bounds(), source, rect.Min, draw.Src)
	bitmap := ui.NewBitmap(out)
	s.images[key] = bitmap
	return bitmap
}

func (a *app) editBlockProperties(n *blocks.Node) {
	s := a.blockEditor
	s.propertyID = n.ID
	s.props = map[string]string{}
	for k, v := range n.Attrs {
		s.props[k] = v
	}
	s.properties = true
}

func (a *app) blockOverlays(c *ui.Context) {
	s := a.blockEditor
	if s == nil {
		return
	}
	ui.Modal(c, &s.formulaOpen, func() {
		ui.Column(c).Width(520).Gap(14).Children(func() {
			ui.Text(c, a.tr("Edit equation")).FontSize(24)
			ui.TextArea(c, &s.formulaText).Label(a.tr("LaTeX source")).Font("Menlo").Height(110).FillWidth().AutoFocus()
			formula, err := mathrender.Layout(s.formulaText, true, 22, c.Theme().Text, 0)
			if err != nil {
				ui.Text(c, err.Error()).TextColor(c.Theme().Danger).MaxHeight(90).Clip()
			} else {
				ui.Box(c).FillWidth().Height(formula.Ascent + formula.Depth + 20).Draw(func(p *ui.Painter, r ui.Rect) { formula.Draw(p, r) })
			}
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, a.tr("Apply equation")).Disabled(err != nil).Clicked() {
					a.fail(s.doc.Transform(s.formulaID, func(n *blocks.Node) { n.Children = []*blocks.Node{blocks.Text(s.formulaText)} }))
					s.formulaOpen = false
					s.fields = map[string]*blockField{}
					a.captureBlocks()
				}
				if ui.Button(c, a.tr("Cancel")).Clicked() {
					s.formulaOpen = false
				}
			})
		})
	})
	ui.Modal(c, &s.properties, func() {
		n := s.doc.Find(s.propertyID)
		if n == nil {
			s.properties = false
			return
		}
		ui.Column(c).Width(440).Gap(12).Children(func() {
			ui.Text(c, a.tr("Block properties · ")+n.Kind).FontSize(22)
			keys := []string{"align", "background-color"}
			switch n.Kind {
			case "img":
				keys = []string{"width", "height", "caption", "align", "image.crop"}
			case "column":
				keys = []string{"width-ratio"}
			case "callout":
				keys = []string{"emoji", "background-color", "border-color", "text-color"}
			case "pre":
				keys = []string{"lang", "caption", "wrap"}
			case "countdown":
				keys = []string{"duration", "color", "notify"}
			case "title":
				keys = []string{"icon", "cover", "cover-x", "cover-y"}
			case "table":
				keys = []string{"align", "background-color"}
			case "time":
				keys = []string{"date", "time", "is-whole-day"}
			case "button":
				keys = []string{"text", "href", "color"}
			case "doc-link", "cite":
				keys = []string{"title", "doc-id"}
			case "td", "th":
				keys = []string{"vertical-align", "background-color"}
			}
			if n.Kind == "doc-link" || n.Kind == "cite" {
				labels := []string{a.tr("Choose document")}
				ids := map[string]string{}
				for _, note := range a.lib.Notes {
					if !note.Deleted {
						label := note.Title + " · " + note.ID[:6]
						labels = append(labels, label)
						ids[label] = note.ID
					}
				}
				choice := a.tr("Choose document")
				if ui.Select(c, &choice, labels).Label(a.tr("Referenced document")).FillWidth().Changed() {
					if note := a.lib.Get(ids[choice]); note != nil {
						s.props["doc-id"] = note.ID
						s.props["title"] = note.Title
					}
				}
				if note := a.lib.Get(s.props["doc-id"]); note != nil && ui.Button(c, a.tr("Open referenced document")).Clicked() {
					s.properties = false
					a.openNote(note)
					return
				}
			}
			for _, key := range keys {
				ui.Row(c.Key(key)).Gap(10).AlignItems(ui.Center).Children(func() {
					label := key
					if key == "image.crop" {
						label = "Crop: left top right bottom"
					}
					ui.Text(c, a.tr(label)).Width(180).FontSize(12)
					value := s.props[key]
					if ui.TextInput(c, &value).Label(a.tr("Property ") + key).Grow(1).Changed() {
						s.props[key] = value
					}
				})
			}
			if n.Kind == "img" {
				ui.Text(c, a.tr("Crop uses fractions from 0 to 1; e.g. 0.1 0.1 0.9 0.9.")).FontSize(11).TextColor(c.Theme().TextMuted)
				a.cropEditor(c, n)
			}
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, a.tr("Apply properties")).Clicked() {
					valid := true
					for _, key := range []string{"width", "height", "width-ratio", "duration"} {
						if value := s.props[key]; value != "" {
							number, err := strconv.ParseFloat(value, 64)
							if err != nil || number <= 0 || math.IsNaN(number) || math.IsInf(number, 0) {
								a.problem = "Enter a positive number for " + key
								valid = false
							}
						}
					}
					if valid {
						a.fail(s.doc.Transform(n.ID, func(n *blocks.Node) {
							for _, key := range keys {
								n.Set(key, s.props[key])
							}
						}))
						s.properties = false
						s.fields = map[string]*blockField{}
						a.captureBlocks()
					}
				}
				if ui.Button(c, a.tr("Cancel")).Clicked() {
					s.properties = false
				}
			})
		})
	})
}

func (a *app) cropEditor(c *ui.Context, n *blocks.Node) {
	path := n.Attr("href")
	if path == "" {
		path = n.Attr("path")
	}
	if path == "" {
		path = n.Attr("src")
	}
	bitmap := a.blockImage(path)
	if bitmap == nil {
		return
	}
	left, top, right, bottom := float32(0), float32(0), float32(1), float32(1)
	_, _ = fmt.Sscanf(a.blockEditor.props["image.crop"], "%f %f %f %f", &left, &top, &right, &bottom)
	w, h := bitmap.Size()
	width := float32(380)
	height := min(float32(180), width*float32(h)/float32(max(1, w)))
	ui.Box(c).Width(width).Height(height).Clip().Children(func() {
		ui.Image(c, bitmap).Width(width).Height(height).Fit(ui.FillBox)
		ui.Box(c).Absolute().Left(left*width).Top(top*height).Width((right-left)*width).Height((bottom-top)*height).Border(2, c.Theme().Accent)
		for i, point := range [][2]float32{{left, top}, {right, top}, {left, bottom}, {right, bottom}} {
			handle := ui.Box(c.Key(i)).Absolute().Left(point[0]*width-5).Top(point[1]*height-5).Size(10, 10).Background(c.Theme().Accent).Label(fmt.Sprintf("Crop corner %d", i+1))
			if dx, dy, ok := handle.Dragged(); ok {
				if i%2 == 0 {
					left = max(0, min(right-.02, left+dx/width))
				} else {
					right = min(1, max(left+.02, right+dx/width))
				}
				if i < 2 {
					top = max(0, min(bottom-.02, top+dy/height))
				} else {
					bottom = min(1, max(top+.02, bottom+dy/height))
				}
				a.blockEditor.props["image.crop"] = fmt.Sprintf("%.4f %.4f %.4f %.4f", left, top, right, bottom)
			}
		}
	})
}

func (a *app) insertNodes(nodes ...*blocks.Node) {
	if a.blockEditor == nil && !a.enableBlocks() {
		return
	}
	s := a.blockEditor
	a.captureBlocks()
	s.lastTextID = ""
	parent := s.doc.Root
	index := len(parent.Children)
	if active := s.doc.Find(s.active); active != nil {
		if p, at := s.doc.Parent(active.ID); p != nil {
			parent = p
			index = at + 1
			if f := s.fields[active.ID]; f != nil && f.doc != nil && blocks.IsText(active) {
				anchor, caret := f.doc.Selection()
				start, end := min(anchor, caret), max(anchor, caret)
				before := blocks.FromRich(f.doc.Text().Slice(0, start), active)
				after := blocks.FromRich(f.doc.Text().Slice(end, f.doc.Text().Len()), active)
				var replacement []*blocks.Node
				if start > 0 {
					replacement = append(replacement, before...)
				}
				replacement = append(replacement, nodes...)
				if end < f.doc.Text().Len() {
					for _, n := range after {
						blocks.FreshIDs(n)
					}
					replacement = append(replacement, after...)
				}
				if len(replacement) > 0 {
					a.fail(s.doc.Splice(parent.ID, at, 1, replacement...))
					s.fields = map[string]*blockField{}
					s.active = nodes[0].ID
					a.captureBlocks()
					a.insertOpen = false
					a.mode = "Write"
					return
				}
			}
		}
	}
	if len(nodes) > 0 {
		a.fail(s.doc.Splice(parent.ID, index, 0, nodes...))
		s.active = nodes[0].ID
	}
	a.captureBlocks()
	a.insertOpen = false
	a.mode = "Write"
}
