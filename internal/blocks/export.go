package blocks

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

type ExportOptions struct {
	Title, Cover, Icon, CSS string
	Math                    func(string, bool) (string, error)
}

func safeURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "", "https", "http", "mailto", "tel":
		return html.EscapeString(s)
	}
	return ""
}

// HTML renders the structured tree directly, keeping merged cells, columns and
// inline objects. Unknown nodes retain their text without executing raw HTML.
func (d *Document) HTML(opts ExportOptions) (string, error) {
	var renderErr error
	var render func(*Node, bool) string
	render = func(n *Node, inline bool) string {
		var children strings.Builder
		for _, c := range n.Children {
			children.WriteString(render(c, inline || IsText(n)))
		}
		content := children.String()
		tag := n.Kind
		attrs := ""
		for _, name := range []string{"rowspan", "colspan"} {
			if v := n.Int(name, 0); v > 0 {
				attrs += " " + name + `="` + strconv.Itoa(v) + `"`
			}
		}
		style := ""
		for _, name := range []string{"text-color", "background-color"} {
			v := n.Attr(name)
			if v != "" {
				key := name
				if key == "text-color" {
					key = "color"
				}
				style += key + ":" + cssColor(v) + ";"
			}
		}
		if align := n.Attr("align"); align == "left" || align == "right" || align == "center" {
			style += "text-align:" + align + ";"
		}
		switch tag {
		case "#text":
			return html.EscapeString(n.Text)
		case "#raw", "title":
			return ""
		case "root", "markdown-group":
			return content
		case "latex":
			if opts.Math != nil {
				s, err := opts.Math(n.Plain(), !inline)
				if err == nil {
					return s
				}
				renderErr = err
			}
			return "<code>" + html.EscapeString(n.Plain()) + "</code>"
		case "grid":
			tag = "div"
			attrs += ` class="columns"`
		case "column":
			tag = "div"
			style += fmt.Sprintf("flex:%.4f;min-width:0;", n.Float("width-ratio", 1))
		case "callout":
			tag = "aside"
			attrs += ` class="callout"`
			content = html.EscapeString(n.Attr("emoji")) + content
		case "checkbox":
			tag = "div"
			attrs += ` class="task"`
			mark := "☐ "
			if n.Attr("done") == "true" {
				mark = "☑ "
				style += "text-decoration:line-through;"
			}
			content = mark + content
		case "pre":
			attrs += ` class="code"`
			content = "<code>" + html.EscapeString(n.Plain()) + "</code>"
			if n.Attr("wrap") == "true" {
				style += "white-space:pre-wrap;"
			}
		case "img":
			path := n.Attr("href")
			if path == "" {
				path = n.Attr("src")
			}
			if path == "" {
				path = n.Attr("path")
			}
			if width := n.Int("width", 0); width > 0 {
				style += fmt.Sprintf("width:%dpx;", width)
			}
			return `<figure style="` + style + `"><img src="` + safeURL(path) + `" alt="` + html.EscapeString(n.Attr("caption")) + `"><figcaption>` + html.EscapeString(n.Attr("caption")) + `</figcaption></figure>`
		case "a":
			attrs += ` href="` + safeURL(n.Attr("href")) + `"`
		case "source", "file", "attachment", "bookmark", "figure", "video":
			path := n.Attr("href")
			if path == "" {
				path = n.Attr("path")
			}
			if path == "" {
				path = n.Attr("src")
			}
			label := n.Attr("name")
			if label == "" {
				label = n.Attr("title")
			}
			if label == "" {
				label = path
			}
			tag = "a"
			attrs += ` href="` + safeURL(path) + `"`
			content = html.EscapeString(label) + content
		case "time":
			tag = "time"
			content = html.EscapeString(n.Attr("date")+" "+n.Attr("time")) + content
		case "cite", "doc-link":
			tag = "span"
			attrs += ` class="reference"`
			content = "@" + html.EscapeString(n.Attr("title")) + content
		case "button":
			tag = "a"
			attrs += ` class="button" href="` + safeURL(n.Attr("href")) + `"`
			if content == "" {
				content = html.EscapeString(n.Attr("text"))
			}
		case "countdown":
			tag = "div"
			content = "Timer: " + html.EscapeString(n.Attr("duration")) + " seconds"
		case "sub-page-list":
			return ""
		case "raw-html":
			nodes, err := ParseHTML(n.Plain())
			if err != nil {
				return html.EscapeString(n.Text)
			}
			return render(nodes, false)
		case "h7", "h8", "h9":
			tag = "h6"
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "b", "strong", "em", "i", "u", "s", "del", "sup", "sub", "span", "code", "blockquote", "table", "tbody", "thead", "tfoot", "tr", "td", "th", "br", "hr":
		default:
			tag = "div"
		}
		if style != "" {
			attrs += ` style="` + html.EscapeString(style) + `"`
		}
		if tag == "hr" || tag == "br" {
			return "<" + tag + attrs + ">"
		}
		return "<" + tag + attrs + ">" + content + "</" + tag + ">"
	}
	body := render(d.Root, false)
	cover := ""
	if opts.Cover != "" {
		cover = `<img class="cover" src="` + safeURL(opts.Cover) + `" alt="">`
	}
	page := `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:;"><title>` + html.EscapeString(opts.Title) + `</title><style>` + exportCSS + opts.CSS + `</style></head><body class="page">` + cover + `<h1 class="title">` + html.EscapeString(opts.Icon+" "+opts.Title) + `</h1>` + body + `</body></html>`
	return page, renderErr
}

func cssColor(v string) string {
	named := map[string]string{"red": "#d44c47", "orange": "#d9730d", "yellow": "#b99527", "green": "#448361", "blue": "#337ea9", "purple": "#9065b0", "gray": "#787774", "light-gray": "#f1f1ef", "light-red": "#fbe4e4", "light-orange": "#faebdd", "light-yellow": "#fbf3db", "light-green": "#edf3ec", "light-blue": "#e7f3f8", "light-purple": "#f4f0f7"}
	if color := named[v]; color != "" {
		return color
	}
	if strings.HasPrefix(v, "#") && (len(v) == 4 || len(v) == 5 || len(v) == 7 || len(v) == 9) {
		if _, err := strconv.ParseUint(v[1:], 16, 32); err == nil {
			return v
		}
	}
	return "inherit"
}

const exportCSS = `body{font:16px/1.6 Georgia,serif;color:#292927;max-width:820px;margin:48px auto;padding:0 36px}h1,h2,h3{line-height:1.25}pre{overflow-wrap:anywhere;background:#f4f3ef;padding:18px}code{font-family:Menlo,monospace}table{border-collapse:collapse;width:100%}td,th{border:1px solid #ddd;padding:8px;vertical-align:top}th{background:#f1f2f0}img{max-width:100%;object-fit:contain}.cover{width:100%;height:200px;object-fit:cover}.columns{display:flex;gap:24px}.callout{border:1px solid #ddd;border-radius:8px;background:#f3f5ef;padding:16px}blockquote{border-left:3px solid #b5b5a5;padding-left:20px;color:#666}figure{margin:12px 0}figcaption{font-size:12px;color:#666}.button,.reference{padding:2px 6px;background:#eee;border-radius:4px}math[display=block]{display:block;text-align:center;margin:18px 0}@media print{body{margin:0;max-width:none}pre,blockquote,tr{break-inside:avoid}}`

// ParseHTML converts web and office clipboard fragments through the HTML5
// parser. Layout tags become structured blocks; active content is discarded.
func ParseHTML(source string) (*Node, error) {
	d, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return nil, err
	}
	var walk func(*xhtml.Node) []*Node
	walk = func(h *xhtml.Node) []*Node {
		if h.Type == xhtml.TextNode {
			return []*Node{Text(h.Data)}
		}
		if h.Type != xhtml.ElementNode && h.Type != xhtml.DocumentNode {
			return nil
		}
		switch h.Data {
		case "script", "style", "head", "iframe", "object", "embed":
			return nil
		}
		var children []*Node
		for c := h.FirstChild; c != nil; c = c.NextSibling {
			children = append(children, walk(c)...)
		}
		tag := h.Data
		switch tag {
		case "html", "body", "":
			return children
		case "div", "section", "article":
			tag = "p"
			for _, c := range children {
				if IsBlock(c) {
					return children
				}
			}
		case "input":
			tag = "checkbox"
		case "aside":
			tag = "callout"
		case "img":
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "b", "strong", "em", "i", "u", "s", "del", "sup", "sub", "span", "a", "code", "pre", "blockquote", "table", "tbody", "thead", "tfoot", "tr", "td", "th", "br", "hr", "latex", "grid", "column", "callout", "checkbox", "time", "button":
		default:
			return children
		}
		n := New(tag, children...)
		for _, attr := range h.Attr {
			switch attr.Key {
			case "href", "src":
				if safeURL(attr.Val) != "" {
					key := attr.Key
					if tag == "img" {
						key = "href"
					}
					n.Set(key, attr.Val)
				}
			case "rowspan", "colspan", "width", "height", "align", "title":
				n.Set(attr.Key, attr.Val)
			case "checked":
				n.Set("done", "true")
			case "data-lark-language", "data-language", "data-lang":
				n.Set("lang", attr.Val)
			case "class":
				if strings.HasPrefix(attr.Val, "language-") {
					n.Set("lang", strings.TrimPrefix(attr.Val, "language-"))
				}
			case "style":
				for _, pair := range strings.Split(attr.Val, ";") {
					key, value, ok := strings.Cut(pair, ":")
					if !ok {
						continue
					}
					key, value = strings.TrimSpace(key), strings.TrimSpace(value)
					switch key {
					case "color":
						n.Set("text-color", value)
					case "background-color":
						n.Set(key, value)
					case "text-align":
						n.Set("align", value)
					case "font-weight":
						if value == "bold" || value == "700" || value == "600" {
							n.Children = []*Node{New("b", n.Children...)}
						}
					case "font-style":
						if value == "italic" {
							n.Children = []*Node{New("em", n.Children...)}
						}
					case "text-decoration":
						if strings.Contains(value, "underline") {
							n.Children = []*Node{New("u", n.Children...)}
						}
					}
				}
			}
		}
		if tag == "pre" {
			for _, c := range children {
				if c.Attr("lang") != "" {
					n.Set("lang", c.Attr("lang"))
				}
			}
		}
		if tag == "li" {
			for _, c := range children {
				if c.Kind == "checkbox" {
					n.Kind = "checkbox"
					n.Set("done", c.Attr("done"))
					n.Children = slicesWithout(children, c)
					break
				}
			}
		}
		if tag == "span" && (n.Attr("text-color") != "" || n.Attr("background-color") != "") {
			return []*Node{n}
		}
		return []*Node{n}
	}
	root := New("root", walk(d)...)
	var out []*Node
	var inline []*Node
	flush := func() {
		if len(inline) > 0 {
			out = append(out, New("p", inline...))
			inline = nil
		}
	}
	for _, n := range root.Children {
		if IsBlock(n) || n.Kind == "img" {
			flush()
			out = append(out, n)
		} else if n.Kind != "#text" || strings.TrimSpace(n.Text) != "" {
			inline = append(inline, n)
		}
	}
	flush()
	root.Children = out
	return root, nil
}

func slicesWithout(nodes []*Node, target *Node) []*Node {
	var out []*Node
	for _, n := range nodes {
		if n != target {
			out = append(out, n)
		}
	}
	return out
}
