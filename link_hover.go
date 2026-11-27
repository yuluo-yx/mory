package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

type linkHoverState struct {
	document, active, pending string
	seen, pressed             bool
}

func hasRichLinks(doc *richtext.Document) bool {
	if doc != nil {
		for _, p := range doc.Text().Paragraphs() {
			for _, run := range p.Runs {
				if run.Attributes.Link != "" && run.Attributes.Attachment == nil {
					return true
				}
			}
		}
	}
	return false
}

// Navigation runs at the next frame's start, never halfway through building a
// document. A stationary pointer cannot repeatedly open a URL or follow a chain
// of note links after navigation replaces the document under it.
func (a *app) beginLinkHover(c *ui.Context) {
	s := &a.linkHover
	if c.Modifiers() != ui.Ctrl || s.pressed || a.current == nil {
		s.active, s.pending = "", ""
	} else if s.pending != "" {
		link := s.pending
		s.pending = ""
		if a.current.ID == s.document {
			a.openLink(link)
		}
	}
	s.seen = false
}

func (a *app) endLinkHover() {
	if !a.linkHover.seen {
		a.linkHover.active = ""
	}
}

func (a *app) hoverDocumentLink(c *ui.Context, ed ui.Element, doc *richtext.Document, font ui.Font, theme *richtext.Theme, padding ...float32) {
	if c.Modifiers() != ui.Ctrl || a.linkHover.pressed || a.current == nil {
		return
	}
	x, y, over := ed.PointerPosition()
	if !over || ed.Dragging() {
		return
	}
	var inset float32
	if len(padding) != 0 {
		inset = padding[0]
	}
	width := ed.LayoutBounds().W - 2*inset
	if x < inset || x >= inset+width {
		return
	}
	link := richLinkAt(doc, font, theme, width, x-inset, y)
	if link == "" || strings.HasPrefix(link, "mory-inline:") {
		return
	}
	s := &a.linkHover
	s.seen = true
	ed.Cursor(ui.CursorPointer)
	if s.active != "" {
		if s.document == a.current.ID && s.active != link {
			s.active, s.pending = link, link
			c.AnimationFrame()
		}
		return
	}
	s.active, s.pending, s.document = link, link, a.current.ID
	c.AnimationFrame()
}

// MyGo 0.9.2 exposes visible paragraph positions and its text shaper, but not
// link hit testing. Shape only the hovered paragraph with the same font/insets
// as EditorBase, then require a link's actual glyph range (not nearest caret).
// This leaves selection, IME, undo, scrolling and document attributes untouched.
func richLinkAt(doc *richtext.Document, font ui.Font, theme *richtext.Theme, width, x, y float32) string {
	if width <= 0 {
		return ""
	}
	for _, box := range doc.VisibleParagraphs() {
		if y < box.Top || y >= box.Top+box.Height {
			continue
		}
		text := doc.Text()
		_, end := text.ParagraphRange(box.Start)
		for _, p := range text.Slice(box.Start, end).Paragraphs() {
			st := p.Style
			left := float32(st.Indent) * font.Size * theme.Indent
			if st.List != richtext.NoList {
				left += font.Size * theme.Indent
			}
			if st.Quote {
				left += font.Size * theme.Indent * .75 * float32(max(1, st.QuoteDepth))
			}
			right := float32(0)
			if st.CodeBlock {
				left += font.Size * theme.CodeBlockPadding[0]
				right = font.Size * theme.CodeBlockPadding[0]
			}
			if x < left || x >= width-right {
				return ""
			}
			f := font
			if st.Heading > 0 {
				h := theme.Headings[min(st.Heading, 6)-1]
				f.Size *= h.Size
				f.Weight = h.Weight
			} else if st.CodeBlock {
				f.Family, f.Size = theme.CodeFont, font.Size*theme.CodeSize
			}
			spans := make([]ui.Span, 0, len(p.Runs))
			for _, run := range p.Runs {
				a := run.Attributes
				span := ui.Span{Text: run.Text, Font: a.Font, Size: a.Size, Weight: a.Weight, Italic: a.Italic}
				if a.Code {
					span.Font = theme.CodeFont
					if span.Size == 0 {
						span.Size = font.Size * theme.CodeSize
					}
				}
				if at := a.Attachment; at != nil {
					span.ObjectWidth, span.ObjectHeight = at.Width, at.Height
					if span.ObjectWidth > width-left-right {
						span.ObjectHeight *= (width - left - right) / span.ObjectWidth
						span.ObjectWidth = width - left - right
					}
				}
				spans = append(spans, span)
			}
			layout := ui.ShapeRichTextWith(spans, f, ui.TextShape{Width: max(1, width-left-right), Align: st.Align})
			offset := 0
			for _, run := range p.Runs {
				next := offset + ui.UTF16Len(run.Text)
				if run.Attributes.Link != "" && run.Attributes.Attachment == nil {
					for _, r := range layout.SelectionRects(ui.TextInputRange{Start: offset, End: next}) {
						if r.Contains(x-left, y-box.Top) {
							return run.Attributes.Link
						}
					}
				}
				offset = next
			}
			return ""
		}
	}
	return ""
}
