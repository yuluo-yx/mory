package blocks

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/yuluo-yx/mory/internal/document"
)

// ParseMarkdown preserves the original spelling of each top-level block. Only
// blocks changed in the native editor are serialized again.
func ParseMarkdown(source string) (*Node, error) {
	return parseDocusaurus(source, true)
}

func parseCommonMarkdown(source string) (*Node, error) {
	p := parser.New(parser.Default | parser.ExtDollarMath)
	p.Set(source)
	root := New("root")
	indexes := []int{}
	for i := range p.Children(0) {
		indexes = append(indexes, i)
	}
	cursor := 0
	for _, index := range indexes {
		block := p.Block(index)
		start := block.Start
		end := block.End
		if start < 0 || end < start || end > len(source) {
			return nil, fmt.Errorf("invalid Markdown source range")
		}
		if start > cursor {
			gap := New("#raw")
			gap.Text = source[cursor:start]
			root.Children = append(root.Children, gap)
		}
		raw := source[start:end]
		var nodes []*Node
		switch block.Kind {
		case parser.MathBlock:
			n := New("latex", Text(p.Lines(index)))
			n.Set("display", "true")
			nodes = []*Node{n}
		case parser.HTMLBlock:
			x, err := ParseXML(strings.TrimSpace(raw))
			if err == nil {
				nodes = Visible(x.Children)
			} else {
				n := New("raw-html", Text(raw))
				nodes = []*Node{n}
			}
		case parser.FootnoteDefinition:
			n := New("footnote", Text(strings.TrimSpace(raw)))
			nodes = []*Node{n}
		case parser.Table:
			if n, ok := compactMarkdownTable(p, index); ok {
				nodes = []*Node{n}
			} else {
				nodes = []*Node{markdownBlock(p, index)}
			}
		default:
			nodes = []*Node{markdownBlock(p, index)}
		}
		if len(nodes) == 1 {
			nodes[0].markdown = raw
			nodes[0].mdBaseline = nodes[0].signature()
		} else if len(nodes) > 1 {
			group := New("markdown-group", nodes...)
			group.markdown = raw
			group.mdBaseline = group.signature()
			nodes = []*Node{group}
		}
		root.Children = append(root.Children, nodes...)
		cursor = end
	}
	if len(indexes) == 0 && source != "" {
		n := Paragraph(source)
		n.markdown = source
		n.mdBaseline = n.signature()
		root.Children = append(root.Children, n)
	} else if cursor < len(source) {
		gap := New("#raw")
		gap.Text = source[cursor:]
		root.Children = append(root.Children, gap)
	}
	return root, nil
}

const compactTableRows = 200

// compactMarkdownTable avoids expanding thousands of ordinary Markdown table
// cells into rich-text nodes when the UI will show a compact preview. The
// original block spelling is still attached by parseCommonMarkdown, so saving
// an untouched document remains byte-for-byte stable.
func compactMarkdownTable(p *parser.Document, index int) (*Node, bool) {
	block := p.Block(index)
	if len(block.Align) == 0 {
		return nil, false
	}
	rows := make([]int, 0, compactTableRows)
	for row := range p.Children(index) {
		rows = append(rows, row)
	}
	if len(rows) < compactTableRows {
		return nil, false
	}
	info := &CompactTable{
		Rows:    len(rows),
		Columns: len(block.Align),
		Widths:  make([]float64, len(block.Align)),
		Preview: make(map[[2]int]string),
	}
	for i := range info.Widths {
		info.Widths[i] = 160
	}
	for rowPosition, row := range rows {
		if rowPosition >= 3 && rowPosition < len(rows)-3 {
			continue
		}
		column := 0
		for cell := range p.Children(row) {
			if column >= info.Columns {
				break
			}
			b := p.Block(cell)
			text := strings.TrimSpace(p.Source()[b.Start:b.End])
			info.Preview[[2]int{rowPosition, column}] = text
			column++
		}
	}
	n := New("table")
	n.Compact = info
	return n, true
}

func (n *Node) Markdown() string {
	if n.markdown != "" && n.signature() == n.mdBaseline {
		return n.markdown
	}
	if source, ok := extensionMarkdown(n); ok {
		return source
	}
	if n.Kind == "root" || n.Kind == "markdown-group" {
		var b strings.Builder
		for _, c := range n.Children {
			part := c.Markdown()
			b.WriteString(part)
			if c.markdown != "" && c.signature() == c.mdBaseline {
				continue
			}
			if !strings.HasSuffix(part, "\n\n") {
				if c.Kind != "#raw" && !strings.HasSuffix(part, "\n") {
					b.WriteByte('\n')
				}
			}
		}
		return b.String()
	}
	if n.Kind == "#text" {
		return n.Text
	}
	if n.Kind == "#raw" {
		return n.Text
	}
	if n.Kind == "pre" && n.Attr("code-meta") != "" {
		fence := "```"
		for strings.Contains(n.Plain(), fence) {
			fence += "`"
		}
		return fence + n.Attr("lang") + " " + n.Attr("code-meta") + "\n" + strings.TrimSuffix(n.Plain(), "\n") + "\n" + fence + "\n\n"
	}
	if len(n.Kind) == 2 && n.Kind[0] == 'h' && n.Kind[1] > '6' {
		return n.XML() + "\n\n"
	}
	if n.Kind == "title" || n.Kind == "source" || n.Kind == "video" || n.Kind == "whiteboard" || n.Kind == "sheet" {
		return n.XML() + "\n\n"
	}
	if n.Kind == "latex" {
		if n.Attr("display") == "true" {
			return "$$\n" + n.Plain() + "\n$$\n\n"
		}
		return "$" + n.Plain() + "$"
	}
	if n.Kind == "footnote" || n.Kind == "raw-html" {
		return n.Plain() + "\n\n"
	}
	if n.Kind == "grid" || n.Kind == "countdown" || n.Kind == "figure" || n.Kind == "sub-page-list" {
		return n.XML() + "\n\n"
	}
	styled := false
	n.Walk(func(c *Node) {
		if c.Kind == "time" || c.Kind == "button" || c.Kind == "cite" || c.Kind == "doc-link" {
			styled = true
		}
		for key := range c.Attrs {
			switch key {
			case "background-color", "text-color", "align", "image.crop", "folded", "width", "height":
				styled = true
			}
		}
	})
	if styled {
		return n.XML() + "\n\n"
	}
	if n.Kind == "table" {
		advanced := false
		n.Walk(func(c *Node) {
			if c.Int("rowspan", 1) > 1 || c.Int("colspan", 1) > 1 || c.Attr("background-color") != "" {
				advanced = true
			}
		})
		if advanced {
			return n.XML() + "\n\n"
		}
	}
	_, md, err := document.FromXML(n.XML())
	if err != nil {
		return n.XML() + "\n\n"
	}
	if id := n.Attr("heading-id"); id != "" {
		md = strings.TrimRight(md, "\n") + " {#" + id + "}\n"
	}
	return md + "\n"
}

func markdownBlock(p *parser.Document, index int) *Node {
	b := p.Block(index)
	n := New("p")
	switch b.Kind {
	case parser.Heading:
		n.Kind = "h" + strconv.Itoa(b.Level)
		n.Children = markdownInlines(p.Inlines(index))
		if match := headingID.FindStringSubmatch(n.Plain()); match != nil {
			n.Set("heading-id", match[1])
			if len(n.Children) > 0 {
				last := n.Children[len(n.Children)-1]
				if last.Kind == "#text" {
					last.Text = headingID.ReplaceAllString(last.Text, "")
				}
			}
		}
	case parser.Paragraph:
		n.Children = markdownInlines(p.Inlines(index))
		if len(n.Children) == 1 && n.Children[0].Kind == "img" {
			return n.Children[0]
		}
	case parser.ThematicBreak:
		n.Kind = "hr"
	case parser.CodeBlock:
		n.Kind = "pre"
		n.Set("lang", b.Language())
		n.Children = []*Node{New("code", Text(p.Lines(index)))}
		line := strings.TrimSpace(p.Source()[b.Start:lineEnd(p.Source(), b.Start)])
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			info := strings.TrimLeft(line, string(line[0]))
			_, meta, _ := strings.Cut(info, " ")
			n.Set("code-meta", meta)
			for _, m := range staticAttribute.FindAllStringSubmatch(meta, -1) {
				if m[1] == "title" {
					n.Set("caption", m[2]+m[3])
				}
			}
		}
	case parser.MathBlock:
		n.Kind = "latex"
		n.Set("display", "true")
		n.Children = []*Node{Text(p.Lines(index))}
	case parser.Quote:
		n.Kind = "blockquote"
		if b.Alert != 0 {
			n.Kind = "callout"
			n.Set("emoji", "💡")
			n.Set("alert", b.Alert.String())
		}
		for i := range p.Children(index) {
			if p.Block(i).Kind == parser.Paragraph && p.Lines(i) == "" {
				continue
			}
			n.Children = append(n.Children, markdownBlock(p, i))
		}
	case parser.List:
		n.Kind = "ul"
		if b.Ordered {
			n.Kind = "ol"
			if b.Number != 1 {
				n.Set("start", strconv.Itoa(b.Number))
			}
		}
		for i := range p.Children(index) {
			n.Children = append(n.Children, markdownBlock(p, i))
		}
	case parser.Item:
		n.Kind = "li"
		if b.Task != parser.NoTask {
			n.Kind = "checkbox"
			n.Set("done", strconv.FormatBool(b.Task == parser.Checked))
		}
		first := true
		for i := range p.Children(index) {
			child := markdownBlock(p, i)
			if first && child.Kind == "p" {
				n.Children = append(n.Children, child.Children...)
			} else {
				n.Children = append(n.Children, child)
			}
			first = false
		}
	case parser.Table:
		n.Kind = "table"
		cols := New("colgroup")
		for range b.Align {
			col := New("col")
			col.Set("width", "160")
			cols.Children = append(cols.Children, col)
		}
		n.Children = append(n.Children, cols)
		body := New("tbody")
		for i := range p.Children(index) {
			row := New("tr")
			column := 0
			for j := range p.Children(i) {
				tag := "td"
				if p.Block(i).Header {
					tag = "th"
				}
				cell := New(tag, New("p", markdownInlines(p.Inlines(j))...))
				if column < len(b.Align) {
					switch b.Align[column] {
					case parser.AlignCenter:
						cell.Set("align", "center")
					case parser.AlignRight:
						cell.Set("align", "right")
					}
				}
				row.Children = append(row.Children, cell)
				column++
			}
			body.Children = append(body.Children, row)
		}
		n.Children = append(n.Children, body)
	default:
		n.Children = []*Node{Text(p.Lines(index))}
	}
	return n
}

func markdownInlines(inlines []parser.Inline) []*Node {
	var render func(int, int) string
	render = func(start, end int) string {
		var out strings.Builder
		for i := start; i < end; {
			in := inlines[i]
			next := max(i+1, min(in.Next, end))
			children := ""
			if next > i+1 {
				children = render(i+1, next)
			}
			switch in.Kind {
			case parser.Text, parser.Mention, parser.Reference:
				out.WriteString(escape(in.Text))
			case parser.SoftBreak:
				out.WriteByte(' ')
			case parser.LineBreak:
				out.WriteString("<br/>")
			case parser.Code:
				out.WriteString("<code>" + escape(in.Text) + "</code>")
			case parser.Emphasis:
				out.WriteString("<em>" + children + "</em>")
			case parser.Strong:
				out.WriteString("<b>" + children + "</b>")
			case parser.Strikethrough:
				out.WriteString("<del>" + children + "</del>")
			case parser.Link:
				out.WriteString(`<a href="` + escape(in.Dest) + `">` + children + "</a>")
			case parser.Image:
				out.WriteString(`<img href="` + escape(in.Dest) + `" caption="` + escape(strings.TrimSpace(stripTags(children))) + `"/>`)
			case parser.Math:
				out.WriteString("<latex>" + escape(in.Text) + "</latex>")
			case parser.HTML:
				out.WriteString(strings.ReplaceAll(in.Text, "<br>", "<br/>"))
			case parser.FootnoteRef:
				out.WriteString(`<a href="#fn-` + escape(in.Text) + `">` + escape(in.Text) + "</a>")
			}
			i = next
		}
		return out.String()
	}
	markup := render(0, len(inlines))
	root, err := ParseXML(markup)
	if err != nil {
		return []*Node{Text(stripTags(markup))}
	}
	return root.Children
}
func stripTags(source string) string {
	root, err := ParseXML(source)
	if err == nil {
		return root.Plain()
	}
	return source
}

func NewTable(rows, columns int) *Node {
	rows = max(1, rows)
	columns = max(1, columns)
	table := New("table")
	cols := New("colgroup")
	for range columns {
		col := New("col")
		col.Set("width", "160")
		cols.Children = append(cols.Children, col)
	}
	table.Children = append(table.Children, cols)
	body := New("tbody")
	for r := range rows {
		row := New("tr")
		for range columns {
			tag := "td"
			if r == 0 {
				tag = "th"
			}
			row.Children = append(row.Children, New(tag, Paragraph("")))
		}
		body.Children = append(body.Children, row)
	}
	table.Children = append(table.Children, body)
	return table
}
func NewColumns(count int) *Node {
	grid := New("grid")
	for range max(2, count) {
		column := New("column", Paragraph(""))
		column.Set("width-ratio", fmt.Sprint(1/float64(max(2, count))))
		grid.Children = append(grid.Children, column)
	}
	return grid
}
