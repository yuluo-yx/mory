package blocks

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// RichOptions lets the view supply native drawings for inline objects and the
// document theme's named colors without coupling the model to a window.
type RichOptions struct {
	Color      func(string) ui.Color
	Background func(string) ui.Color
	Object     func(*Node) *richtext.Attachment
}

func ToRich(n *Node, options RichOptions) richtext.Text {
	p := richtext.Paragraph{Style: richtext.ParagraphStyle{ID: n.ID}}
	if len(n.Kind) == 2 && n.Kind[0] == 'h' {
		p.Style.Heading = min(6, int(n.Kind[1]-'0'))
	}
	if n.Kind == "checkbox" {
		p.Style.List = richtext.Tasks
		p.Style.Checked = n.Attr("done") == "true"
	}
	if n.Kind == "li" {
		p.Style.List = richtext.Bullets
	}
	switch n.Attr("align") {
	case "center":
		p.Style.Align = ui.Center
	case "right":
		p.Style.Align = ui.End
	}
	var walk func(*Node, richtext.Attributes)
	walk = func(n *Node, a richtext.Attributes) {
		switch n.Kind {
		case "#text":
			p.Runs = append(p.Runs, richtext.Run{Text: n.Text, Attributes: a})
			return
		case "br":
			p.Runs = append(p.Runs, richtext.Run{Text: "\n", Attributes: a})
			return
		case "b", "strong":
			a.Weight = 700
		case "em", "i":
			a.Italic = true
		case "u":
			a.Underline = true
		case "s", "del":
			a.Strikethrough = true
		case "code":
			a.Code = true
		case "a":
			a.Link = n.Attr("href")
		case "span":
			if options.Color != nil {
				if v := n.Attr("text-color"); v != "" {
					a.Color = options.Color(v)
				}
				if v := n.Attr("background-color"); v != "" {
					if options.Background != nil {
						a.Background = options.Background(v)
					} else {
						a.Background = options.Color(v)
					}
				}
			}
		default:
			attachment := &richtext.Attachment{Name: n.Plain(), Width: 90, Height: 22, Data: n.Clone(), URL: "mory-inline:" + n.ID}
			if attachment.Name == "" {
				attachment.Name = n.Kind
			}
			if options.Object != nil {
				if object := options.Object(n); object != nil {
					attachment = object
					attachment.Data = n.Clone()
					attachment.URL = "mory-inline:" + n.ID
				}
			}
			a.Attachment = attachment
			p.Runs = append(p.Runs, richtext.Run{Text: string(richtext.ObjectReplacement), Attributes: a})
			return
		}
		for _, child := range n.Children {
			walk(child, a)
		}
	}
	for _, c := range n.Children {
		if !IsBlock(c) && c.Kind != "#raw" {
			walk(c, richtext.Attributes{})
		}
	}
	return richtext.TextOf(p)
}

func colorName(c ui.Color) string { return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A) }

// FromRich preserves inline objects and each paragraph's identity. Native text
// editing never replaces a formula, reminder or document link by its label.
func FromRich(text richtext.Text, original *Node) []*Node {
	var nodes []*Node
	for _, p := range text.Paragraphs() {
		n := New("p")
		if p.Style.ID != "" {
			n.ID = p.Style.ID
		}
		if original != nil && n.ID == original.ID {
			n.Attrs = map[string]string{}
			for k, v := range original.Attrs {
				n.Attrs[k] = v
			}
			if original.Kind == "li" {
				n.Kind = "li"
			}
		}
		if p.Style.Heading > 0 {
			n.Kind = "h" + strconv.Itoa(p.Style.Heading)
			if original != nil && len(original.Kind) == 2 && original.Kind[0] == 'h' && original.Kind[1] > '6' && p.Style.Heading == 6 {
				n.Kind = original.Kind
			}
		}
		switch p.Style.List {
		case richtext.Tasks:
			n.Kind = "checkbox"
			n.Set("done", strconv.FormatBool(p.Style.Checked))
		case richtext.Bullets:
			n.Kind = "li"
			n.Set("list", "bullet")
		case richtext.Numbers:
			n.Kind = "li"
			n.Set("list", "number")
		}
		switch p.Style.Align {
		case ui.Center:
			n.Set("align", "center")
		case ui.End:
			n.Set("align", "right")
		default:
			n.Set("align", "")
		}
		for _, run := range p.Runs {
			a := run.Attributes
			var parts []*Node
			if a.Attachment != nil {
				if original, ok := a.Attachment.Data.(*Node); ok {
					parts = []*Node{original.Clone()}
				} else {
					img := New("img")
					img.Set("href", a.Attachment.URL)
					img.Set("caption", a.Attachment.Name)
					parts = []*Node{img}
				}
			} else {
				lines := strings.Split(run.Text, "\n")
				for i, line := range lines {
					if i > 0 {
						parts = append(parts, New("br"))
					}
					if line != "" {
						parts = append(parts, Text(line))
					}
				}
			}
			wrap := func(tag string) { parts = []*Node{New(tag, parts...)} }
			if a.Code {
				wrap("code")
			}
			if a.Strikethrough {
				wrap("del")
			}
			if a.Underline {
				wrap("u")
			}
			if a.Italic {
				wrap("em")
			}
			if a.Weight >= 600 {
				wrap("b")
			}
			if a.Color != (ui.Color{}) || a.Background != (ui.Color{}) {
				wrap("span")
				if a.Color != (ui.Color{}) {
					parts[0].Set("text-color", colorName(a.Color))
				}
				if a.Background != (ui.Color{}) {
					parts[0].Set("background-color", colorName(a.Background))
				}
			}
			if a.Link != "" {
				wrap("a")
				parts[0].Set("href", a.Link)
			}
			n.Children = append(n.Children, parts...)
		}
		if p.Style.CodeBlock {
			n.Kind = "pre"
			n.Children = []*Node{New("code", Text(text.PlainText()))}
		}
		if p.Style.Quote {
			n = New("blockquote", n)
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		nodes = []*Node{Paragraph("")}
	}
	// Nested paragraphs/lists belong to the original block, not its inline text.
	if original != nil {
		for _, c := range original.Children {
			if IsBlock(c) {
				nodes[len(nodes)-1].Children = append(nodes[len(nodes)-1].Children, c.Clone())
			}
		}
	}
	return nodes
}
