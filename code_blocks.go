package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// Remember the fence before the paragraph editor's Enter handler removes it.
func (a *app) observeCodeFence(doc *richtext.Document, event ui.InputEvent) {
	if event.Kind == ui.InputText {
		a.queueInlineCode(doc, event.Text)
	}
	if event.Kind != ui.InputKeyDown || event.Key != ui.KeyEnter || event.Mods != 0 {
		return
	}
	a.codeFenceDoc, a.codeFenceInfo = nil, ""
	anchor, caret := doc.Selection()
	text := doc.Text()
	start, end := text.ParagraphRange(caret)
	if anchor != caret || caret != end || text.ParagraphStyleAt(caret).CodeBlock {
		return
	}
	line := text.Slice(start, end).String()
	if strings.HasPrefix(line, "```") && !strings.Contains(line[3:], "`") {
		a.codeFenceDoc, a.codeFenceInfo = doc, strings.TrimSpace(line[3:])
	}
}

func (a *app) applyCodeFence(doc *richtext.Document, node *blocks.Node) {
	if a.codeFenceDoc != doc {
		return
	}
	info := a.codeFenceInfo
	a.codeFenceDoc, a.codeFenceInfo = nil, ""
	if fields := strings.Fields(info); len(fields) > 0 {
		node.Set("lang", fields[0])
		node.Set("code-meta", strings.TrimSpace(strings.TrimPrefix(info, fields[0])))
	}
}

// Switch a newly created rich-text code paragraph to the existing block editor.
// Existing fenced documents take this path at load time through RichSafe instead.
func (a *app) promoteRichCode() bool {
	doc := a.doc
	start := -1
	for offset, p := range doc.Text().Paragraphs() {
		if p.Style.CodeBlock {
			start = offset
			break
		}
	}
	if start < 0 {
		return false
	}
	s, err := newBlockSession(doc.Text().Markdown(), a.current.Format, a.sourceRevision)
	if a.fail(err) {
		return false
	}
	var code *blocks.Node
	s.doc.Root.Walk(func(n *blocks.Node) {
		if code == nil && n.Kind == "pre" {
			code = n
		}
	})
	if code == nil {
		return false
	}
	a.applyCodeFence(doc, code)
	_, caret := doc.Selection()
	s.active, s.focus, s.focusAt = code.ID, code.ID, max(0, caret-start)
	a.blockEditor = s
	a.setCurrentSource(s.doc.Source())
	s.sourceRevision = a.sourceRevision
	a.richSource = "\x00"
	a.markDirty()
	return true
}

func (a *app) codeBlockActive() bool {
	if s := a.blockEditor; s != nil {
		if n := s.doc.Find(s.active); n != nil && n.Kind == "pre" {
			return true
		}
	}
	return a.doc != nil && a.doc.Formatting().Paragraph.CodeBlock
}

func (a *app) toggleCodeBlock() {
	if a.current == nil || a.mode != "Write" {
		return
	}
	if s := a.blockEditor; s != nil {
		if n := s.doc.Find(s.active); n != nil && n.Kind == "pre" {
			a.captureBlocks()
			if a.fail(s.doc.Transform(n.ID, func(n *blocks.Node) {
				n.Kind, n.Attrs = "p", nil
				n.Children = []*blocks.Node{blocks.Text(n.Plain())}
			})) {
				return
			}
			delete(s.fields, n.ID)
			s.focus, s.focusAt = n.ID, 0
			a.captureBlocks()
			return
		}
	}
	if a.doc != nil {
		a.doc.ToggleCodeBlock()
		a.capture()
	}
}
