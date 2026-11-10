package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/mathrender"
)

type blockField struct {
	doc     *richtext.Document
	code    *editor.State
	version uint64
	nodeID  string
}
type blockDrag struct{ ID string }
type blockSession struct {
	componentChoice        map[string]string
	doc                    *blocks.Document
	fields                 map[string]*blockField
	active, focus          string
	focusAt                int
	list                   ui.ListState
	split                  splitScrollState
	foldedHeadings         map[string]bool
	foldPresentation       bool
	restoreRich            bool
	restoreHeading         int
	selection              map[string]bool
	tableID                string
	cells                  blocks.Rect
	cellAnchor             [2]int
	properties             bool
	propertyID             string
	props                  map[string]string
	formulaOpen            bool
	formulaID, formulaText string
	version                uint64
	images                 map[string]*ui.Bitmap
	imageErrors            map[string]bool
	lastTextID             string
	lastText               time.Time
	lastTextVersion        uint64
	chosen                 ui.Selection[string]
}

func newBlockSession(source, format string) (*blockSession, error) {
	doc, err := blocks.Parse(source, format)
	if err != nil {
		return nil, err
	}
	return &blockSession{doc: doc, fields: map[string]*blockField{}, version: doc.Version, selection: map[string]bool{}, images: map[string]*ui.Bitmap{}, imageErrors: map[string]bool{}}, nil
}

func (a *app) structured() bool {
	return a.blockEditor != nil && (a.mode == "Write" || a.mode == "Read")
}

func (a *app) splitPreview(c *ui.Context) {
	if a.blockEditor == nil || a.blockEditor.doc.Source() != a.current.Source {
		s, err := newBlockSession(a.current.Source, a.current.Format)
		if err != nil {
			ui.Text(c, err.Error()).TextColor(c.Theme().Danger)
			return
		}
		a.blockEditor = s
	}
	if a.current.Format == "xml" || len(a.blockEditor.doc.Root.Children) > 0 {
		a.blockView(c, true)
	} else {
		ui.Scroll(c).Grow(1).Children(func() { markdown.View(c, a.current.Source, &a.preview).FillWidth() })
	}
}
func (a *app) enableBlocks() bool {
	if a.current == nil {
		return false
	}
	caret := 0
	if a.doc != nil {
		_, caret = a.doc.Selection()
	}
	a.capture()
	a.flushSource()
	session, err := newBlockSession(a.current.Source, a.current.Format)
	if a.fail(err) {
		return false
	}
	a.blockEditor = session
	remaining := caret
	session.doc.Root.Walk(func(n *blocks.Node) {
		if session.active != "" || !blocks.IsText(n) || n.Kind == "title" {
			return
		}
		length := blocks.ToRich(n, blocks.RichOptions{}).Len()
		if remaining <= length {
			session.active = n.ID
			field := a.blockField(n, a.documentColor)
			field.doc.SetSelection(remaining, remaining)
			a.doc = field.doc
		} else {
			remaining -= length + 1
		}
	})
	return true
}

func (a *app) captureBlocks() {
	s := a.blockEditor
	if s == nil {
		return
	}
	for id, f := range s.fields {
		n := s.doc.Find(id)
		if n == nil {
			delete(s.fields, id)
			continue
		}
		if f.doc != nil && f.doc.Version() != f.version {
			f.version = f.doc.Version()
			replacement := blocks.FromRich(f.doc.Text(), n)
			p, at := s.doc.Parent(id)
			if p == nil {
				continue
			}
			if err := s.doc.Splice(p.ID, at, 1, replacement...); a.fail(err) {
				continue
			}
			if len(replacement) == 1 && s.lastTextID == id && s.doc.Version == s.lastTextVersion+1 && time.Since(s.lastText) < 700*time.Millisecond {
				s.doc.Coalesce()
			}
			s.lastTextID, s.lastText = id, time.Now()
			s.lastTextVersion = s.doc.Version
			if len(replacement) > 1 {
				_, caret := f.doc.Selection()
				index, offset := 0, 0
				for start, paragraph := range f.doc.Text().Paragraphs() {
					if start <= caret {
						offset = caret - start
						s.focus = replacement[index].ID
						s.focusAt = offset
					}
					_ = paragraph
					index++
				}
				delete(s.fields, id)
			}
			if len(replacement) == 1 && f.doc.Text().String() == "/" {
				a.insertOpen = true
				a.insertQuery = ""
				a.slashPending = true
				a.slashOffset = 0
			}
		}
		if f.code != nil && f.code.Buffer.Version() != f.version {
			f.version = f.code.Buffer.Version()
			text := f.code.Buffer.String()
			a.fail(s.doc.Transform(id, func(n *blocks.Node) { n.Children = []*blocks.Node{blocks.New("code", blocks.Text(text))} }))
		}
	}
	if s.doc.Version != s.version {
		s.version = s.doc.Version
		a.current.Source = s.doc.Source()
		a.richSource = a.current.Source
		a.markDirty()
	}
}

func (a *app) blockUndo(redo bool) {
	s := a.blockEditor
	if s == nil {
		return
	}
	a.captureBlocks()
	s.lastTextID = ""
	if redo {
		s.doc.Redo()
	} else {
		s.doc.Undo()
	}
	s.fields = map[string]*blockField{}
	a.captureBlocks()
}

func (a *app) blockField(n *blocks.Node, color ui.Color) *blockField {
	s := a.blockEditor
	if f := s.fields[n.ID]; f != nil {
		return f
	}
	text := blocks.ToRich(n, blocks.RichOptions{Color: func(name string) ui.Color { return a.namedColor("color", name, color) }, Background: func(name string) ui.Color { return a.namedColor("highlight", name, ui.Transparent) }, Object: func(object *blocks.Node) *richtext.Attachment { return a.inlineObject(object, color) }})
	if parent, _ := s.doc.Parent(n.ID); parent != nil && parent.Kind == "ol" {
		var paragraphs []richtext.Paragraph
		for _, p := range text.Paragraphs() {
			p.Style.List = richtext.NoList
			paragraphs = append(paragraphs, p)
		}
		text = richtext.TextOf(paragraphs...)
	}
	f := &blockField{nodeID: n.ID, doc: richtext.NewDocument(text)}
	f.version = f.doc.Version()
	s.fields[n.ID] = f
	if s.active == "" || s.active == n.ID {
		s.active = n.ID
		a.doc = f.doc
	}
	return f
}

func blockColor(name string, fallback ui.Color) ui.Color {
	if strings.HasPrefix(name, "#") {
		return ui.Hex(name)
	}
	palette := map[string]string{"gray": "#787774", "red": "#d44c47", "orange": "#d9730d", "yellow": "#b99527", "green": "#448361", "blue": "#337ea9", "purple": "#9065b0", "pink": "#c14c8a", "light-gray": "#f1f1ef", "light-red": "#fbe4e4", "light-orange": "#faebdd", "light-yellow": "#fbf3db", "light-green": "#edf3ec", "light-blue": "#e7f3f8", "light-purple": "#f4f0f7"}
	if hex := palette[name]; hex != "" {
		return ui.Hex(hex)
	}
	return fallback
}

func (a *app) inlineObject(n *blocks.Node, color ui.Color) *richtext.Attachment {
	label := n.Plain()
	switch n.Kind {
	case "latex":
		f, err := mathrender.Layout(n.Plain(), false, a.lib.Settings.FontSize, color, 0)
		if err == nil {
			return &richtext.Attachment{Name: n.Plain(), Width: f.Width, Height: f.Ascent + f.Depth, Draw: f.Draw}
		}
		label = "Invalid formula"
	case "time":
		label = n.Attr("date")
		if label == "" {
			label = n.Attr("time")
		}
		if label == "" {
			label = "Reminder"
		}
	case "cite", "doc-link":
		if title := n.Attr("title"); title != "" {
			label = "@" + title
		} else {
			label = "@Document"
		}
	case "button":
		if label == "" {
			label = n.Attr("text")
		}
		if label == "" {
			label = "Button"
		}
	case "img":
		path := n.Attr("href")
		if path == "" {
			path = n.Attr("path")
		}
		if image := a.blockImage(path); image != nil {
			return &richtext.Attachment{Name: n.Attr("caption"), Image: image, Width: float32(n.Float("width", 320))}
		}
	}
	if label == "" {
		label = n.Kind
	}
	font := ui.Font{Size: 13}
	layout := ui.ShapeText(label, font, 0)
	w, h := layout.Size()
	return &richtext.Attachment{Name: label, Width: w + 16, Height: h + 6, Draw: func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, color.Alpha(.08), 5)
		p.TextLayout(layout, r.X+8, r.Y+3, color)
	}}
}

func (a *app) blockImage(path string) *ui.Bitmap {
	if path == "" || a.blockEditor == nil {
		return nil
	}
	s := a.blockEditor
	if bitmap := s.images[path]; bitmap != nil {
		return bitmap
	}
	if s.imageErrors[path] {
		return nil
	}
	root, err := os.OpenRoot(a.assetRoot())
	if err != nil {
		return nil
	}
	defer root.Close()
	data, err := root.ReadFile(filepath.FromSlash(path))
	if err != nil {
		s.imageErrors[path] = true
		return nil
	}
	bitmap, err := ui.DecodeBitmap(data)
	if err != nil {
		s.imageErrors[path] = true
		return nil
	}
	s.images[path] = bitmap
	return bitmap
}

func (a *app) blockView(c *ui.Context, readOnly bool) {
	s := a.blockEditor
	a.captureBlocks()
	visible := s.visibleBlocks()
	s.list.Key = func(i int) any { return visible[i].ID }
	s.list.Selection = &s.chosen
	if s.foldPresentation {
		s.list.Selection = nil
		s.list.Reorder = nil
	}
	s.list.MenuKeepsSelection = true
	if !readOnly && !s.foldPresentation {
		s.list.Reorder = func(rows []int, to int) {
			if len(rows) == 0 {
				return
			}
			dest := len(s.doc.Root.Children)
			if to < len(visible) {
				_, dest = s.doc.Parent(visible[to].ID)
			}
			ids := []string{}
			for _, row := range rows {
				ids = append(ids, visible[row].ID)
			}
			for _, id := range ids {
				a.fail(s.doc.Move(id, s.doc.Root.ID, dest))
			}
			a.captureBlocks()
		}
	}
	syncSplit := readOnly && a.mode == "Split"
	if syncSplit {
		s.split.sync(c, a.code, &s.list, a.current.Source, a.current.Format, visible)
	}
	list := ui.List(c, &s.list, len(visible), func(i int) {
		if syncSplit {
			ui.Column(c).FillWidth().Draw(func(_ *ui.Painter, r ui.Rect) {
				s.split.rows[i] = r
			}).Children(func() { a.blockNode(c, visible[i], readOnly, 0) })
		} else {
			a.blockNode(c, visible[i], readOnly, 0)
		}
	}).Grow(1).Gap(7)
	if syncSplit {
		list.FillWidth().Padding(4, 0, 48).TrackScroll(&s.split.scroll).Draw(func(_ *ui.Painter, r ui.Rect) {
			s.split.viewport = r
			clear(s.split.rows)
		}).DrawOver(func(_ *ui.Painter, _ ui.Rect) {
			s.split.geometryScroll = s.split.scroll.Y
			// Programmatic movement must not be echoed back as user input.
			if s.split.driven {
				s.split.lastPreview = s.split.scroll.Y
				s.split.driven = false
			}
		})
	} else {
		list.Margin(0, 0, 0, -headingGutterWidth).Padding(4, 0, 48, headingGutterWidth)
	}
	if !readOnly && !s.foldPresentation {
		ui.Row(c).Gap(8).Padding(6).Children(func() {
			if s.chosen.Len() > 1 {
				if ghostButton(c, a.tr("Copy blocks")).Clicked() {
					a.copyBlocks(c, false)
				}
				if ghostButton(c, a.tr("Cut blocks")).Clicked() {
					a.copyBlocks(c, true)
				}
				if ghostButton(c, a.tr("Delete selected blocks")).Clicked() {
					a.deleteBlocks()
				}
			}
			if ghostButton(c, a.tr("+ Add block")).Clicked() {
				a.insertOpen = true
				a.insertQuery = ""
			}
			if ghostButton(c, a.tr("Undo")).KeepFocus().Clicked() {
				a.blockUndo(false)
			}
			if ghostButton(c, a.tr("Redo")).KeepFocus().Clicked() {
				a.blockUndo(true)
			}
		})
	}
}

func (a *app) blockNode(c *ui.Context, n *blocks.Node, readOnly bool, depth int) {
	if depth > 80 {
		return
	}
	s := a.blockEditor
	t := c.Theme()
	row := ui.Row(c.Key(n.ID)).FillWidth().MinWidth(0).AlignItems(ui.Start).Gap(6)
	row.Children(func() {
		if !readOnly && depth == 0 && !s.foldPresentation {
			handle := ghostButton(c.Key("handle"), "⠿").Label(a.tr("Block actions ") + n.Kind).Padding(3).Width(21).Drag(blockDrag{n.ID})
			if headingLevel(n) > 0 {
				// The heading toggle occupies this gutter slot without moving the text.
				handle.Opacity(0)
			}
			if handle.Clicked() {
				s.active = n.ID
			}
			handle.PopoverMenu(func(m *ui.Menu) { a.blockMenu(m, n) })
		}
		body := ui.Column(c).Grow(1).MinWidth(0).Gap(5)
		if color := n.Attr("background-color"); color != "" && n.Kind != "callout" && n.Kind != "table" {
			body.Background(a.namedColor("background", color, ui.Transparent)).Padding(8).Radius(5)
		}
		if _, over := ui.DragOver[blockDrag](body); over {
			body.Border(1, t.Accent)
		}
		if drag, ok := ui.Drop[blockDrag](body); ok && !readOnly {
			p, index := s.doc.Parent(n.ID)
			if p != nil {
				a.fail(s.doc.Move(drag.ID, p.ID, index))
				a.captureBlocks()
			}
		}
		body.Children(func() {
			switch {
			case blocks.IsText(n):
				a.blockText(c, n, readOnly)
			case n.Kind == "table":
				a.blockTable(c, n, readOnly, depth)
			case n.Kind == "grid":
				a.blockColumns(c, n, readOnly, depth)
			case n.Kind == "pre":
				a.blockCode(c, n, readOnly)
			case n.Kind == "details" || n.Kind == "tabs" || n.Kind == "tab-item":
				a.docusaurusComponent(c, n, readOnly, depth)
			case n.Kind == "mdx-source":
				ui.Text(c, a.tr("MDX source · edit in Source mode")).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, n.Plain()).Font("Menlo").FontSize(12)
			case n.Kind == "latex":
				a.blockMath(c, n, readOnly)
			case n.Kind == "img":
				a.blockPicture(c, n, readOnly)
			case n.Kind == "hr":
				ui.Box(c).Height(1).FillWidth().Background(t.Border).Margin(12, 0)
			case n.Kind == "callout":
				accent := t.Accent
				if color := map[string]string{"note": "#708090", "tip": "#2e8555", "info": "#3088bd", "warning": "#b78000", "caution": "#b78000", "danger": "#d12d3f"}[n.Attr("directive")]; color != "" {
					accent = ui.Hex(color)
				}
				ui.Row(c).Padding(14).Radius(8).Background(blockColor(n.Attr("background-color"), accent.Alpha(.08))).Border(1, blockColor(n.Attr("border-color"), accent.Alpha(.3))).Gap(12).Children(func() {
					emoji := n.Attr("emoji")
					if emoji == "" {
						emoji = "💡"
					}
					ui.Text(c, emoji).FontSize(22)
					ui.Column(c).Grow(1).Children(func() {
						if n.Attr("directive") != "" {
							title := n.Attr("title")
							if title == "" {
								title = strings.ToUpper(n.Attr("directive"))
							}
							ui.Text(c, title).Bold()
						}
						for _, child := range blocks.Visible(n.Children) {
							a.blockNode(c, child, readOnly, depth+1)
						}
					})
				})
			case n.Kind == "blockquote":
				ui.Column(c).BorderWidth(0, 0, 0, 3).BorderColor(t.Accent.Alpha(.5)).Padding(4, 0, 4, 14).Children(func() {
					for _, child := range blocks.Visible(n.Children) {
						a.blockNode(c, child, readOnly, depth+1)
					}
				})
			case n.Kind == "ul" || n.Kind == "ol" || n.Kind == "column" || n.Kind == "markdown-group":
				for i, child := range blocks.Visible(n.Children) {
					if n.Kind == "ol" && child.Kind == "li" {
						ui.Row(c).Gap(8).Children(func() {
							ui.Textf(c, a.tr("%d."), i+1).TextColor(t.TextMuted)
							ui.Column(c).Grow(1).Children(func() { a.blockNode(c, child, readOnly, depth+1) })
						})
					} else {
						a.blockNode(c, child, readOnly, depth+1)
					}
				}
			case n.Kind == "sub-page-list":
				for _, note := range a.lib.Notes {
					if note.Parent == a.current.ID && !note.Deleted {
						if ghostButton(c, note.Title).Clicked() {
							a.openNote(note)
						}
					}
				}
			case n.Kind == "source" || n.Kind == "figure" || n.Kind == "bookmark" || n.Kind == "file" || n.Kind == "attachment" || n.Kind == "video":
				a.blockFile(c, n)
			case n.Kind == "countdown":
				a.blockCountdown(c, n, readOnly)
			default:
				ui.Column(c).Padding(12).Radius(6).Border(1, t.Border).Gap(8).Children(func() {
					ui.Text(c, n.Kind).Bold()
					if n.Plain() != "" {
						ui.Text(c, n.Plain())
					}
					if !readOnly && ghostButton(c, a.tr("Edit block properties")).Clicked() {
						a.editBlockProperties(n)
					}
				})
			}
		})
	})
	if level := headingLevel(n); level > 0 && depth == 0 && a.mode != "Split" {
		row.Children(func() {
			left := float32(-headingGutterWidth)
			if !readOnly && !s.foldPresentation {
				left += 27
			}
			top := float32(4)
			if field := s.fields[n.ID]; field != nil && field.doc != nil {
				if boxes := field.doc.VisibleParagraphs(); len(boxes) > 0 {
					top = boxes[0].Top + max(0, min(boxes[0].Height, 40)-24)/2
				}
			}
			button := a.headingToggle(c.Key("heading-toggle"), level, n.Plain(), s.headingFolded(n)).Absolute().Left(left).Top(top)
			if !row.Hovered() && !button.Hovered() && !button.Focused() && !s.headingFolded(n) {
				button.Opacity(0)
			}
			if button.Clicked() {
				a.toggleHeading(n)
				c.AnimationFrame()
			}
		})
	}
}

func (a *app) blockText(c *ui.Context, n *blocks.Node, readOnly bool) {
	s := a.blockEditor
	field := a.blockField(n, c.Theme().Text)
	font := a.writingFont(".page")
	for parent, _ := s.doc.Parent(n.ID); parent != nil && parent != s.doc.Root; parent, _ = s.doc.Parent(parent.ID) {
		if parent.Kind == "td" || parent.Kind == "th" {
			font = a.docTheme.Font("table", font)
			break
		}
	}
	richTheme := a.docTheme.Rich(c.Theme(), font)
	if len(n.Kind) == 2 && n.Kind[0] == 'h' && n.Kind[1] > '6' {
		richTheme.Headings[5].Size = a.docTheme.Length(n.Kind, "font-size", font.Size, font.Size, font.Size) / font.Size
	}
	ed := richtext.EditorBase(c.Key("text"), field.doc, richtext.EditorOptions{Label: "Block text " + n.ID, Placeholder: "Write something…", ReadOnly: readOnly, Font: font, Theme: richTheme, OnLink: a.openLink, OnLinkCommand: func() { a.doc = field.doc; a.linkURL = field.doc.Formatting().Link; a.linkOpen = true }}).FillWidth()
	width := ed.LayoutBounds().W
	if width < 80 {
		width = 600
	}
	height := float32(0)
	for _, p := range field.doc.Text().Paragraphs() {
		f := font
		if p.Style.Heading > 0 {
			f.Size *= []float32{1.8, 1.45, 1.2, 1.1, 1, .9}[p.Style.Heading-1]
		}
		text := pText(p)
		if text == "" {
			text = " "
		}
		_, h := ui.ShapeText(text, f, max(40, width-28)).Size()
		for _, run := range p.Runs {
			if run.Attributes.Attachment != nil {
				h = max(h, run.Attributes.Attachment.Height+6)
			}
		}
		height += h + 8
	}
	ed.Height(max(32, height+10))
	if ed.Focused() {
		a.textFocused = true
		s.active = n.ID
		a.doc = field.doc
	}
	if s.focus == n.ID {
		ed.Focus()
		field.doc.SetSelection(min(s.focusAt, field.doc.Text().Len()), min(s.focusAt, field.doc.Text().Len()))
		s.focus = ""
	}
	if !readOnly {
		ed.ObserveInput(func(event ui.InputEvent) {
			if event.Kind == ui.InputCommand && (event.Text == "undo" || event.Text == "redo") {
				a.blockUndo(event.Text == "redo")
				return
			}
			if event.Kind != ui.InputKeyDown {
				return
			}
			anchor, caret := field.doc.Selection()
			if anchor != caret {
				return
			}
			if event.Key == ui.KeyBackspace && caret == 0 && event.Mods == 0 {
				a.mergeBlock(n.ID, -1)
			} else if event.Key == ui.KeyDelete && caret == field.doc.Text().Len() && event.Mods == 0 {
				a.mergeBlock(n.ID, 1)
			} else if (event.Key == ui.KeyUp || event.Key == ui.KeyLeft) && caret == 0 {
				a.focusAdjacent(n.ID, -1)
			} else if (event.Key == ui.KeyDown || event.Key == ui.KeyRight) && caret == field.doc.Text().Len() {
				a.focusAdjacent(n.ID, 1)
			}
		})
	}
	if n.Attr("folded") != "true" {
		for _, child := range n.Children {
			if blocks.IsBlock(child) {
				a.blockNode(c, child, readOnly, 1)
			}
		}
	}
}

func (a *app) focusAdjacent(id string, direction int) {
	s := a.blockEditor
	var all []*blocks.Node
	s.doc.Root.Walk(func(n *blocks.Node) {
		if blocks.IsText(n) && n.Kind != "title" {
			all = append(all, n)
		}
	})
	for i, n := range all {
		if n.ID == id && i+direction >= 0 && i+direction < len(all) {
			dest := all[i+direction]
			s.focus = dest.ID
			s.focusAt = 0
			if direction < 0 {
				s.focusAt = len([]rune(dest.Plain()))
			}
			return
		}
	}
}
func (a *app) mergeBlock(id string, direction int) {
	s := a.blockEditor
	a.captureBlocks()
	p, index := s.doc.Parent(id)
	if p == nil {
		return
	}
	other := index + direction
	if other < 0 || other >= len(p.Children) || !blocks.IsText(p.Children[other]) {
		return
	}
	left, right := index, other
	if left > right {
		left, right = right, left
	}
	first, second := p.Children[left], p.Children[right]
	caret := len([]rune(first.Plain()))
	merged := first.Clone()
	merged.Children = append(merged.Children, second.Clone().Children...)
	a.fail(s.doc.Splice(p.ID, left, right-left+1, merged))
	s.fields = map[string]*blockField{}
	s.focus = merged.ID
	s.focusAt = caret
	a.captureBlocks()
}

func (a *app) blockMenu(m *ui.Menu, n *blocks.Node) {
	s := a.blockEditor
	if m.Item(a.tr("Insert paragraph below")).Chosen() {
		p, index := s.doc.Parent(n.ID)
		if p != nil {
			fresh := blocks.Paragraph("")
			a.fail(s.doc.Splice(p.ID, index+1, 0, fresh))
			s.focus = fresh.ID
		}
	}
	if m.Item(a.tr("Duplicate block")).Chosen() {
		p, index := s.doc.Parent(n.ID)
		if p != nil {
			copy := n.Clone()
			blocks.FreshIDs(copy)
			a.fail(s.doc.Splice(p.ID, index+1, 0, copy))
		}
	}
	folded := n.Attr("folded") == "true"
	if headingLevel(n) > 0 {
		folded = s.headingFolded(n)
	}
	if m.Item(a.tr("Fold content")).Checked(folded).Chosen() {
		if headingLevel(n) > 0 {
			a.toggleHeading(n)
		} else {
			a.fail(s.doc.Transform(n.ID, func(n *blocks.Node) {
				if folded {
					n.Set("folded", "")
				} else {
					n.Set("folded", "true")
				}
			}))
		}
	}
	if m.Item(a.tr("Properties…")).Chosen() {
		a.editBlockProperties(n)
	}
	m.Submenu(a.tr("Background"), func(m *ui.Menu) {
		for _, color := range []string{"Default", "light-gray", "light-red", "light-orange", "light-yellow", "light-green", "light-blue", "light-purple"} {
			if m.Item(a.tr(color)).Chosen() {
				a.fail(s.doc.Transform(n.ID, func(n *blocks.Node) {
					if color == "Default" {
						n.Set("background-color", "")
					} else {
						n.Set("background-color", color)
					}
				}))
			}
		}
	})
	m.Separator()
	if m.Item(a.tr("Delete block")).Chosen() {
		a.fail(s.doc.Remove(n.ID))
	}
	a.captureBlocks()
}

func (a *app) blockCode(c *ui.Context, n *blocks.Node, readOnly bool) {
	s := a.blockEditor
	f := s.fields[n.ID]
	if f == nil {
		b := editor.NewBuffer(n.Plain())
		b.SetLanguage(codeLanguage(n.Attr("lang")))
		f = &blockField{nodeID: n.ID, code: &editor.State{Buffer: b, SoftWrap: n.Attr("wrap") == "true"}, version: b.Version()}
		s.fields[n.ID] = f
	}
	if title := n.Attr("caption"); title != "" {
		ui.Text(c, title).FontSize(12).Bold().Padding(8).FillWidth().Background(c.Theme().Surface)
	}
	ui.Row(c).Gap(8).Children(func() {
		language := n.Attr("lang")
		if ui.TextInput(c, &language).Label(a.tr("Code language")).Width(120).Disabled(readOnly).Changed() {
			a.fail(s.doc.Transform(n.ID, func(n *blocks.Node) { n.Set("lang", language) }))
			f.code.Buffer.SetLanguage(codeLanguage(language))
		}
		wrap := n.Attr("wrap") == "true"
		if ui.Checkbox(c, &wrap, a.tr("Wrap")).Disabled(readOnly).Changed() {
			a.fail(s.doc.Transform(n.ID, func(n *blocks.Node) { n.Set("wrap", strconv.FormatBool(wrap)) }))
			f.code.SoftWrap = wrap
		}
		if ghostButton(c, a.tr("Copy code")).Clicked() {
			c.Services().WriteClipboard(f.code.Buffer.String())
		}
	})
	f.code.ReadOnly = readOnly
	f.code.Font = a.writingFont("pre")
	f.code.Theme = editor.DefaultTheme(c.Theme())
	f.code.Theme.Background = a.docTheme.Color("pre", "background-color", c.Theme().Surface)
	f.code.Theme.Text = a.docTheme.Color("pre", "color", c.Theme().Text)
	for kind, selector := range map[editor.TokenKind]string{editor.TokenComment: "comment", editor.TokenKeyword: "keyword", editor.TokenTag: "tag", editor.TokenString: "string", editor.TokenNumber: "number", editor.TokenConstant: "boolean", editor.TokenType: "class-name", editor.TokenAttribute: "attr-name", editor.TokenFunction: "function"} {
		style := f.code.Theme.Tokens[kind]
		style.Color = a.docTheme.Color(".token."+selector, "color", style.Color)
		f.code.Theme.Tokens[kind] = style
	}
	f.code.Theme.LineNumber = a.docTheme.Color(".line-numbers", "color", c.Theme().TextMuted)
	if a.docTheme.Value(".line-numbers", "display") == "none" {
		f.code.LineNumbers = editor.LineNumbersOff
	} else {
		f.code.LineNumbers = editor.LineNumbersOn
	}
	if strings.Contains(n.Attr("code-meta"), "showLineNumbers") {
		f.code.LineNumbers = editor.LineNumbersOn
	}
	f.code.LineBackgrounds = docusaurusCodeHighlights(f.code.Buffer.String(), n.Attr("code-meta"), c.Theme().Accent.Alpha(.10))
	if c.Theme().Dark {
		f.code.Theme = editor.DefaultTheme(c.Theme())
	}
	lines := strings.Count(f.code.Buffer.String(), "\n") + 1
	editor.View(c.Key("code"), f.code).Height(float32(min(24, max(3, lines)))*22 + 12).FillWidth().Label(a.tr("Code block"))
}

func codeLanguage(name string) *editor.Language {
	ext := map[string]string{"python": "py", "javascript": "js", "typescript": "ts", "rust": "rs", "ruby": "rb", "shell": "sh", "bash": "sh", "csharp": "cs", "cpp": "cpp", "plaintext": "txt", "markdown": "md"}[strings.ToLower(name)]
	if ext == "" {
		ext = name
	}
	return editor.LanguageFor("code." + ext)
}

func (a *app) blockMath(c *ui.Context, n *blocks.Node, readOnly bool) {
	formula, err := mathrender.Layout(n.Plain(), true, a.lib.Settings.FontSize+2, c.Theme().Text, 0)
	if err != nil {
		ui.Text(c, err.Error()).TextColor(c.Theme().Danger)
	} else {
		ui.Box(c).Height(formula.Ascent + formula.Depth + 24).FillWidth().Draw(func(p *ui.Painter, r ui.Rect) {
			formula.Draw(p, ui.Rect{X: r.X + max(0, (r.W-formula.Width)/2), Y: r.Y + 12, W: r.W, H: r.H})
		}).Label(a.tr("Equation ") + n.Plain())
	}
	if !readOnly && ghostButton(c, a.tr("Edit equation")).Clicked() {
		s := a.blockEditor
		s.formulaID = n.ID
		s.formulaText = n.Plain()
		s.formulaOpen = true
	}
}

func (a *app) blockFile(c *ui.Context, n *blocks.Node) {
	source := n
	if n.Kind == "figure" {
		for _, child := range n.Children {
			if child.Kind == "source" {
				source = child
				break
			}
		}
	}
	name := source.Attr("name")
	if name == "" {
		name = source.Attr("title")
	}
	path := source.Attr("path")
	if path == "" {
		path = source.Attr("href")
	}
	if path == "" {
		path = source.Attr("src")
	}
	if name == "" {
		name = filepath.Base(path)
	}
	if ghostButton(c, "↗  "+name).Clicked() {
		a.openLink(path)
	}
	if path != "" && !strings.Contains(path, "://") {
		preview := n.Attr("view-type") == "Preview" || n.Kind == "video"
		if preview {
			if bitmap := a.fileThumbnail(path); bitmap != nil {
				ui.Image(c, bitmap).MaxWidth(600).Height(240).Fit(ui.Contain).Label(a.tr("Attachment thumbnail ") + name)
			}
		}
		ui.Row(c).Gap(8).Children(func() {
			if ghostButton(c, a.tr("Quick Look")).Label(a.tr("Preview ") + name).Clicked() {
				a.quickLook(path)
			}
			if a.mode == "Write" {
				if ui.Checkbox(c, &preview, a.tr("Preview thumbnail")).Changed() {
					a.fail(a.blockEditor.doc.Transform(n.ID, func(n *blocks.Node) {
						if preview {
							n.Set("view-type", "Preview")
						} else {
							n.Set("view-type", "Card")
						}
					}))
					a.captureBlocks()
				}
			}
		})
	}
}

func (a *app) blockCountdown(c *ui.Context, n *blocks.Node, readOnly bool) {
	duration := n.Int("duration", 300)
	start := int64(n.Int("start-time", 0))
	remaining := duration
	if start > 0 {
		remaining = max(0, duration-int(time.Now().UnixMilli()-start)/1000)
		c.After(time.Second)
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Textf(c, a.tr("%02d:%02d"), remaining/60, remaining%60).FontSize(30).Font("Menlo")
		if !readOnly {
			if ghostButton(c, a.tr("Start / reset")).Clicked() {
				a.fail(a.blockEditor.doc.Transform(n.ID, func(n *blocks.Node) { n.Set("start-time", fmt.Sprint(time.Now().UnixMilli())) }))
			}
			if ghostButton(c, a.tr("Stop")).Clicked() {
				a.fail(a.blockEditor.doc.Transform(n.ID, func(n *blocks.Node) { n.Set("start-time", "") }))
			}
		}
	})
}
