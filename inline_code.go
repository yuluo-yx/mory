package main

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

type inlineCodeInput struct {
	doc     *richtext.Document
	start   int
	text    string
	version uint64
}

// Native shortcuts handle single keystrokes. Remember bulk input and plain-text
// paste so their code spans can be formatted after the editor consumes them.
func (a *app) queueInlineCode(doc *richtext.Document, text string) {
	a.applyInlineCodeInput()
	if !strings.Contains(text, "`") || utf8.RuneCountInString(text) <= 1 {
		return
	}
	anchor, caret := doc.Selection()
	if doc.Text().ParagraphStyleAt(caret).CodeBlock || doc.Formatting().Code == richtext.FormatOn {
		return
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	a.inlineCodeInput = &inlineCodeInput{doc, min(anchor, caret), text, doc.Version()}
}

func (a *app) applyInlineCodeInput() {
	pending := a.inlineCodeInput
	if pending == nil || pending.doc.Version() == pending.version {
		return
	}
	a.inlineCodeInput = nil
	doc := pending.doc
	text := doc.Text()
	end := pending.start + utf8.RuneCountInString(pending.text)
	// Rich clipboard formats may already have been parsed by the native editor.
	if end > text.Len() || text.Slice(pending.start, end).String() != pending.text {
		return
	}
	start, _ := text.ParagraphRange(pending.start)
	_, stop := text.ParagraphRange(end)
	source := text.Slice(start, stop).String()
	parsed := parser.Parse(source)
	type span struct {
		start, end int
		text       string
		attributes richtext.Attributes
	}
	var spans []span
	for i := 1; i < parsed.Len(); i++ {
		for _, inline := range parsed.Inlines(i) {
			if inline.Kind != parser.Code {
				continue
			}
			left := start + utf8.RuneCountInString(source[:inline.Start])
			right := start + utf8.RuneCountInString(source[:inline.End])
			if right <= pending.start || left >= end || text.AttributesAt(left).Code {
				continue
			}
			attrs := text.AttributesAt(left)
			attrs.Code = true
			spans = append(spans, span{left, right, inline.Text, attrs})
		}
	}
	if len(spans) == 0 {
		return
	}
	// Work backwards so parser byte offsets, converted to rune offsets, stay valid.
	doc.Edit(func(edit *richtext.Edit) {
		for i := len(spans) - 1; i >= 0; i-- {
			s := spans[i]
			edit.ReplaceString(s.start, s.end, s.text, s.attributes)
		}
	})
	if anchor, caret := doc.Selection(); anchor == caret {
		doc.Format(func(attrs *richtext.Attributes) { attrs.Code = false })
	}
}
