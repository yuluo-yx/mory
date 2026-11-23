package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Enter before a heading inserts body space without creating an outline entry.
// Keep the heading's identity, formatting, and caret in one undoable edit.
func (a *app) headingStartEnter(ed ui.Element, doc *richtext.Document) {
	anchor, caret := doc.Selection()
	text := doc.Text()
	start, end := text.ParagraphRange(caret)
	if anchor != caret || caret != start || start == end || text.ParagraphStyleAt(start).Heading == 0 {
		return
	}
	if !ed.Shortcut(0, ui.KeyEnter) {
		return
	}
	a.capture()
	doc.Edit(func(e *richtext.Edit) {
		e.ReplaceString(start, start, "\n", richtext.Attributes{})
		e.FormatParagraphs(start, start, func(p *richtext.ParagraphStyle) {
			*p = richtext.ParagraphStyle{ID: p.ID}
		})
		e.Select(start+1, start+1)
	})
	a.capture()
	if s := a.blockEditor; s != nil {
		// Reveal the inserted blank row even when the list anchors to the
		// original heading and would otherwise scroll the new row away.
		for i, n := range s.visibleBlocks() {
			if n.ID == s.active && i > 0 {
				s.list.ScrollIntoView(i - 1)
				break
			}
		}
	}
}
