package main

import (
	"encoding/xml"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
	"github.com/yuluo-yx/mory/internal/typography"
)

const homePreviewBytes = 24 << 10

type homePreview struct {
	source, format string
	root           *blocks.Node
}

func (a *app) homeImage(note *library.Note, path string) ui.ImageSource {
	if path == "" || isRemoteImageURL(path) {
		return nil
	}
	base := a.lib.Dir
	if note.External != "" {
		base = filepath.Dir(note.External)
	}
	key := filepath.Join(base, path)
	if a.homeImages == nil {
		a.homeImages = map[string]ui.ImageSource{}
	}
	if image, ok := a.homeImages[key]; ok {
		return image
	}
	if len(a.homeImages) >= 64 {
		return nil
	}
	a.homeImages[key] = nil
	load := markdown.FileImages(base)
	if a.win == nil {
		image, _ := load(path)
		a.homeImages[key] = image
		return image
	}
	go func() {
		image, err := load(path)
		a.win.Update(func() {
			if err == nil {
				a.homeImages[key] = image
			}
		})
	}()
	return nil
}

// Preview work is bounded independently of document size. A clipped final XML
// block can still show its complete children; no truncated source is ever saved.
func previewSource(source string) string {
	if len(source) <= homePreviewBytes {
		return source
	}
	source = source[:homePreviewBytes]
	for !utf8.ValidString(source) {
		source = source[:len(source)-1]
	}
	if end := strings.LastIndexByte(source, '\n'); end > len(source)/2 {
		source = source[:end]
	}
	return source
}

func parseHomePreview(source, format string) *blocks.Node {
	if format != "xml" {
		root, err := blocks.ParseMarkdown(source)
		if err == nil {
			return root
		}
		return blocks.New("root", blocks.Paragraph(source))
	}
	root := blocks.New("root")
	stack := []*blocks.Node{root}
	decoder := xml.NewDecoder(strings.NewReader(source))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch token := token.(type) {
		case xml.StartElement:
			if len(stack) > 64 {
				return root
			}
			n := blocks.New(token.Name.Local)
			for _, attr := range token.Attr {
				n.Set(attr.Name.Local, attr.Value)
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, blocks.Text(string(token)))
		}
	}
	return root
}

// Draw callbacks run only for visible cards. Cache bounded parsed fragments, not
// full editor documents, and paint them without interactive child controls.
func (a *app) drawHomePreview(p *ui.Painter, r ui.Rect, note *library.Note) {
	if note == nil {
		return
	}
	if a.homePreviews == nil || len(a.homePreviews) > 128 {
		a.homePreviews = map[string]*homePreview{}
	}
	source := previewSource(note.Source)
	preview := a.homePreviews[note.ID]
	if preview == nil || preview.source != source || preview.format != note.Format {
		preview = &homePreview{source: source, format: note.Format, root: parseHomePreview(source, note.Format)}
		a.homePreviews[note.ID] = preview
	}
	t := p.Theme()
	y := r.Y
	text := func(value string, size float32, weight int, color ui.Color, x, width float32) float32 {
		_, height := p.RichText(x, y, width, ui.Span{Text: value, Font: typography.Text, Size: size, Weight: weight, Color: color})
		return height
	}
	p.Clip(r, 0, func() {
		if image := a.homeImage(note, note.Cover); image != nil {
			p.Clip(ui.Rect{X: r.X, Y: y, W: r.W, H: 84}, 4, func() {
				p.Image(image, ui.Rect{X: r.X, Y: y, W: r.W, H: 84}, ui.Cover)
			})
			y += 94
		}
		var walk func(*blocks.Node, int)
		walk = func(n *blocks.Node, depth int) {
			if y >= r.Y+r.H || depth > 12 {
				return
			}
			if n.Kind == "p" && strings.TrimSpace(n.Plain()) == "" {
				for _, child := range n.Children {
					if child.Kind == "img" || child.Kind == "image" {
						walk(child, depth+1)
						return
					}
				}
			}
			size, weight := float32(10.5), 400
			x, width := r.X, r.W
			switch n.Kind {
			case "#raw", "frontmatter", "title":
				return
			case "#text":
				if strings.TrimSpace(n.Text) == "" {
					return
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				size, weight = float32(17-int(n.Kind[1]-'1')), 700
				y += 7
			case "li", "checkbox":
				marker := "•"
				if n.Kind == "checkbox" {
					marker = "☐"
					if n.Attr("done") == "true" {
						marker = "☑"
					}
				}
				text(marker, size, 400, t.TextMuted, x, 12)
				x, width = x+12, width-12
			case "pre":
				value := n.Plain()
				lines := strings.Split(value, "\n")
				value = strings.Join(lines[:min(6, len(lines))], "\n")
				spans := []ui.Span{{Text: value, Font: typography.Mono, Size: 9, Color: t.Text}}
				_, height := p.MeasureText(width-16, spans...)
				height = min(height, 88)
				p.Fill(ui.Rect{X: x, Y: y, W: width, H: height + 16}, t.Surface, 4)
				p.Clip(ui.Rect{X: x + 8, Y: y + 8, W: width - 16, H: height}, 0, func() { p.RichText(x+8, y+8, width-16, spans...) })
				y += height + 24
				return
			case "table":
				y += drawPreviewTable(p, n, ui.Rect{X: x, Y: y, W: width, H: r.Y + r.H - y}) + 10
				return
			case "blockquote", "callout":
				start := y
				for _, child := range n.Children {
					walk(child, depth+1)
				}
				p.Fill(ui.Rect{X: x - 5, Y: start, W: 2, H: max(12, y-start-6)}, t.Accent.Alpha(.45), 1)
				return
			case "hr":
				p.Line(x, y+5, x+width, y+5, 1, t.Border)
				y += 16
				return
			case "figure", "img", "image", "attachment":
				path := n.Attr("src")
				n.Walk(func(child *blocks.Node) {
					if path == "" {
						path = child.Attr("path")
					}
				})
				box := ui.Rect{X: x, Y: y, W: width, H: 84}
				p.Fill(box, t.Surface, 4)
				if image := a.homeImage(note, path); image != nil {
					p.Clip(box, 4, func() { p.Image(image, box, ui.Contain) })
				} else {
					p.Image(iconImage(), ui.Rect{X: x + width/2 - 12, Y: y + 22, W: 24, H: 24}, ui.Contain)
				}
				y += 94
				return
			case "p", "latex", "raw-html":
			default:
				for _, child := range n.Children {
					walk(child, depth+1)
				}
				return
			}
			value := n.Plain()
			if n.Kind == "#text" {
				value = n.Text
			}
			if value == "" {
				y += 6
				return
			}
			var spans []ui.Span
			if blocks.IsText(n) {
				for _, paragraph := range blocks.ToRich(n, blocks.RichOptions{}).Paragraphs() {
					for _, run := range paragraph.Runs {
						color := t.Text
						if run.Attributes.Link != "" {
							color = t.Accent
						}
						spans = append(spans, ui.Span{Text: run.Text, Font: typography.Text, Size: size, Weight: max(weight, run.Attributes.Weight), Italic: run.Attributes.Italic, Color: color})
					}
				}
			}
			if len(spans) == 0 {
				y += text(value, size, weight, t.Text, x, width) + 8
			} else {
				_, height := p.RichText(x, y, width, spans...)
				y += height + 8
			}
		}
		first := true
		for _, node := range preview.root.Children {
			if node.Kind == "#raw" || node.Kind == "frontmatter" || node.Kind == "title" {
				continue
			}
			if first && node.Kind == "h1" && strings.EqualFold(strings.TrimSpace(node.Plain()), strings.TrimSpace(note.Title)) {
				first = false
				continue
			}
			first = false
			walk(node, 0)
		}
	})
}

func drawPreviewTable(p *ui.Painter, node *blocks.Node, r ui.Rect) float32 {
	var rows [][]string
	if node.Compact != nil {
		table := node.Compact
		for row := 0; row < min(5, table.Rows); row++ {
			var cells []string
			for col := 0; col < min(4, table.Columns); col++ {
				cells = append(cells, table.Cells[row*table.Columns+col])
			}
			rows = append(rows, cells)
		}
	} else {
		var walk func(*blocks.Node)
		walk = func(n *blocks.Node) {
			if len(rows) >= 5 {
				return
			}
			if n.Kind == "tr" {
				var cells []string
				for _, cell := range n.Children {
					if (cell.Kind == "td" || cell.Kind == "th") && len(cells) < 4 {
						cells = append(cells, cell.Plain())
					}
				}
				rows = append(rows, cells)
				return
			}
			for _, child := range n.Children {
				walk(child)
			}
		}
		walk(node)
	}
	columns := 1
	for _, row := range rows {
		columns = max(columns, len(row))
	}
	width := r.W / float32(columns)
	for i, row := range rows {
		for j, value := range row {
			cell := ui.Rect{X: r.X + float32(j)*width, Y: r.Y + float32(i)*24, W: width, H: 24}
			if i == 0 {
				p.Fill(cell, p.Theme().Surface, 0)
			}
			p.Stroke(cell, p.Theme().Border, 0, .7)
			p.Clip(cell, 0, func() {
				p.RichText(cell.X+4, cell.Y+5, width-8, ui.Span{Text: value, Font: typography.Text, Size: 9, Color: p.Theme().Text})
			})
		}
	}
	return float32(len(rows)) * 24
}
