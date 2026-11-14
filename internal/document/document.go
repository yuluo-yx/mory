// Package document handles lossless source documents and display conversions.
package document

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/markdown/parser"
	xhtml "golang.org/x/net/html"
)

type Heading struct {
	Level, Offset int
	Text          string
}

// RichSafe rejects constructs the framework's paragraph editor would flatten.
// Source editing always retains the original bytes, including unknown XML blocks.
func RichSafe(source string) bool {
	if strings.Contains(source, ":::") || strings.Contains(source, "{#") {
		return false
	}
	d := parser.Parse(source)
	for i := 1; i < d.Len(); i++ {
		switch d.Block(i).Kind {
		case parser.CodeBlock:
			// The paragraph editor discards fence languages and cannot highlight code.
			return false
		case parser.Table, parser.HTMLBlock, parser.MathBlock, parser.FootnoteDefinition, parser.ThematicBreak:
			return false
		}
		for _, inline := range d.Inlines(i) {
			if inline.Kind == parser.Image {
				return false
			}
		}
	}
	for _, line := range strings.Split(source, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "import ") || strings.HasPrefix(s, "export ") {
			return false
		}
		if strings.HasPrefix(s, "[") && strings.Contains(s, "]:") {
			return false
		}
	}
	return !strings.Contains(source, "<") && !strings.Contains(source, "$") && !strings.Contains(source, "[!")
}

func Headings(source string) []Heading {
	base := 0
	if strings.HasPrefix(source, "---\n") || strings.HasPrefix(source, "---\r\n") {
		for offset := strings.IndexByte(source, '\n') + 1; offset < len(source); {
			end := strings.IndexByte(source[offset:], '\n')
			if end < 0 {
				end = len(source) - offset - 1
			}
			end += offset + 1
			if line := strings.TrimSpace(source[offset:end]); line == "---" || line == "..." {
				base = end
				source = source[end:]
				break
			}
			offset = end
		}
	}
	d := parser.Parse(source)
	var result []Heading
	for i := 1; i < d.Len(); i++ {
		b := d.Block(i)
		if b.Kind == parser.Heading {
			// Use rendered inline text so escapes, links and emphasis match the page.
			text := strings.TrimSpace(parser.PlainText(d.Inlines(i)))
			text = regexp.MustCompile(`\s+\{#[^{}\s]+\}\s*$`).ReplaceAllString(text, "")
			result = append(result, Heading{Level: b.Level, Offset: base + b.Start, Text: text})
		}
	}
	return result
}

func Counts(source string) (words, characters int) {
	inWord := false
	for _, r := range source {
		characters++
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			words++
			inWord = false
		} else if unicode.IsSpace(r) || unicode.IsPunct(r) {
			inWord = false
		} else if !inWord {
			words++
			inWord = true
		}
	}
	return
}

// XML nodes retain the ordered mixture of text and elements for conversion.
type node struct {
	name     string
	attrs    map[string]string
	text     string
	children []*node
}

func parseXML(source string) (*node, error) {
	decoder := xml.NewDecoder(strings.NewReader("<mory-root>" + source + "</mory-root>"))
	root := &node{name: "root"}
	stack := []*node{root}
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid DocxXML: %w", err)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			n := &node{name: v.Name.Local, attrs: map[string]string{}}
			for _, a := range v.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			p := stack[len(stack)-1]
			p.children = append(p.children, n)
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			p := stack[len(stack)-1]
			p.children = append(p.children, &node{text: string(v)})
		}
	}
	return root, nil
}

func (n *node) plain() string {
	if n.name == "" {
		return n.text
	}
	var b strings.Builder
	for _, c := range n.children {
		b.WriteString(c.plain())
	}
	return b.String()
}

// FromXML converts supported blocks for reading/export. The caller keeps the
// original XML as the authoritative editable document, never this projection.
func FromXML(source string) (title, markdown string, err error) {
	root, err := parseXML(source)
	if err != nil {
		return "", "", err
	}
	var findTitle func(*node)
	findTitle = func(n *node) {
		if n.name == "title" && title == "" {
			title = n.plain()
		}
		for _, c := range n.children {
			findTitle(c)
		}
	}
	findTitle(root)
	return title, strings.TrimSpace(render(root, 0)) + "\n", nil
}

func render(n *node, depth int) string {
	if n.name == "" {
		return n.text
	}
	var b strings.Builder
	for _, c := range n.children {
		b.WriteString(render(c, depth+1))
	}
	s := b.String()
	wrap := func(mark string) string { return mark + s + mark }
	switch n.name {
	case "title":
		return ""
	case "p":
		return strings.TrimSpace(s) + "\n\n"
	case "h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8", "h9":
		return strings.Repeat("#", min(int(n.name[1]-'0'), 6)) + " " + strings.TrimSpace(s) + "\n\n"
	case "b", "strong":
		return wrap("**")
	case "em", "i":
		return wrap("*")
	case "del", "s":
		return wrap("~~")
	case "u":
		return "<u>" + s + "</u>"
	case "code":
		return "`" + s + "`"
	case "br":
		return "  \n"
	case "a":
		return "[" + s + "](" + n.attrs["href"] + ")"
	case "latex":
		return "$" + n.plain() + "$"
	case "hr":
		return "\n---\n\n"
	case "pre":
		code := n.plain()
		fence := "```"
		for strings.Contains(code, fence) {
			fence += "`"
		}
		return fence + n.attrs["lang"] + "\n" + strings.TrimSuffix(code, "\n") + "\n" + fence + "\n\n"
	case "ul", "ol":
		var rows []string
		index := 1
		for _, c := range n.children {
			if c.name != "li" {
				continue
			}
			prefix := "- "
			if n.name == "ol" {
				prefix = fmt.Sprintf("%d. ", index)
				index++
			}
			item := strings.TrimSpace(render(c, depth+1))
			rows = append(rows, prefix+strings.ReplaceAll(item, "\n", "\n  "))
		}
		return strings.Join(rows, "\n") + "\n\n"
	case "checkbox":
		mark := " "
		if n.attrs["done"] == "true" {
			mark = "x"
		}
		return "- [" + mark + "] " + strings.TrimSpace(s) + "\n"
	case "blockquote", "callout":
		prefix := ""
		if n.name == "callout" {
			prefix = "> [!NOTE]\n"
		}
		return prefix + "> " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n> ") + "\n\n"
	case "img":
		url := n.attrs["href"]
		if url == "" {
			url = n.attrs["src"]
		}
		if url == "" {
			url = n.attrs["path"]
		}
		return "![" + n.attrs["caption"] + "](" + url + ")\n\n"
	case "bookmark", "file", "attachment":
		name := n.attrs["title"]
		if name == "" {
			name = n.attrs["name"]
		}
		url := n.attrs["href"]
		if url == "" {
			url = n.attrs["path"]
		}
		return "[" + name + "](" + url + ")\n\n"
	case "table":
		var rows [][]string
		var walk func(*node)
		walk = func(p *node) {
			if p.name == "tr" {
				var row []string
				for _, c := range p.children {
					if c.name == "td" || c.name == "th" {
						cell := strings.TrimSpace(render(c, depth+1))
						row = append(row, strings.ReplaceAll(strings.ReplaceAll(cell, "|", "\\|"), "\n", "<br>"))
					}
				}
				rows = append(rows, row)
				return
			}
			for _, c := range p.children {
				walk(c)
			}
		}
		walk(n)
		if len(rows) == 0 {
			return ""
		}
		width := 0
		for _, row := range rows {
			width = max(width, len(row))
		}
		var out strings.Builder
		for i, row := range rows {
			for len(row) < width {
				row = append(row, "")
			}
			out.WriteString("| " + strings.Join(row, " | ") + " |\n")
			if i == 0 {
				out.WriteString("|" + strings.Repeat(" --- |", width) + "\n")
			}
		}
		return out.String() + "\n"
	case "grid", "column":
		return s + "\n"
	case "whiteboard", "sheet":
		return "[" + n.name + "]\n\n"
	default:
		return s
	}
}

var invalidFilename = regexp.MustCompile(`[\x00-\x1f/:\\]`)

func Filename(title string) string {
	title = invalidFilename.ReplaceAllString(strings.TrimSpace(title), "-")
	if title == "" || title == "." || title == ".." {
		return "Untitled"
	}
	r := []rune(title)
	if len(r) > 80 {
		title = string(r[:80])
	}
	return title
}

func Validate(source, format string) error {
	if !utf8.ValidString(source) {
		return fmt.Errorf("document is not UTF-8")
	}
	if len(source) > 32<<20 {
		return fmt.Errorf("document exceeds 32 MiB")
	}
	if format == "xml" {
		_, err := parseXML(source)
		return err
	}
	return nil
}

// ToXML wraps CommonMark HTML in the DocxXML subset. XML imports
// export their original source instead, preserving extensions and merged cells.
func ToXML(title, markdown string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(title))
	root, _ := xhtml.Parse(strings.NewReader(parser.Parse(markdown).HTML()))
	var body *xhtml.Node
	var find func(*xhtml.Node)
	find = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)
	var renderXML func(*xhtml.Node) string
	escape := func(s string) string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
	renderXML = func(n *xhtml.Node) string {
		if n.Type == xhtml.TextNode {
			return escape(n.Data)
		}
		if n.Type != xhtml.ElementNode {
			return ""
		}
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(renderXML(c))
		}
		tag := n.Data
		attrs := ""
		if tag == "body" {
			return b.String()
		}
		if tag == "input" {
			return ""
		}
		if tag == "li" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "input" {
					tag = "checkbox"
					done := "false"
					for _, attr := range c.Attr {
						if attr.Key == "checked" {
							done = "true"
						}
					}
					attrs = ` done="` + done + `"`
					break
				}
			}
		}
		if tag == "ul" && strings.Contains(b.String(), "<checkbox ") {
			return b.String()
		}
		for _, attr := range n.Attr {
			switch attr.Key {
			case "href", "src", "title", "colspan", "rowspan":
				key := attr.Key
				if tag == "img" && key == "src" {
					key = "href"
				}
				attrs += " " + key + `="` + escape(attr.Val) + `"`
			}
		}
		if tag == "pre" && n.FirstChild != nil {
			for _, attr := range n.FirstChild.Attr {
				if attr.Key == "class" && strings.HasPrefix(attr.Val, "language-") {
					attrs += ` lang="` + escape(strings.TrimPrefix(attr.Val, "language-")) + `"`
				}
			}
		}
		if tag == "br" || tag == "hr" || tag == "img" {
			return "<" + tag + attrs + "/>"
		}
		return "<" + tag + attrs + ">" + b.String() + "</" + tag + ">"
	}
	return "<title>" + escaped.String() + "</title>\n" + renderXML(body)
}

func HTML(title, markdown string) string {
	return "<!doctype html><html><head><meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; style-src 'unsafe-inline'; img-src data:;\"><title>" + html.EscapeString(title) + "</title><style>body{font:16px/1.65 Georgia,serif;color:#292927;max-width:800px;margin:48px auto;padding:0 36px}h1,h2,h3{line-height:1.25}pre{white-space:pre-wrap;background:#f4f3ef;padding:18px}code{font-family:Menlo,monospace}table{border-collapse:collapse;width:100%}td,th{border:1px solid #ddd;padding:8px}img{max-width:100%}blockquote{border-left:3px solid #b5b5a5;padding-left:20px;color:#666}@media print{body{margin:0;max-width:none}pre,blockquote,tr{break-inside:avoid}}</style></head><body><h1>" + html.EscapeString(title) + "</h1>" + parser.Parse(markdown).HTML() + "</body></html>"
}
