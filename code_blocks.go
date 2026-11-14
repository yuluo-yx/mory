package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// leaveCode moves into a neighboring text block. Explicit mouse insertion uses
// an empty paragraph, keeping a following heading or populated paragraph intact.
func (a *app) leaveCode(id string, direction int, insert bool) bool {
	s := a.blockEditor
	if s == nil || a.mode != "Write" {
		return false
	}
	a.captureBlocks()
	n := s.doc.Find(id)
	if n == nil || n.Kind != "pre" {
		return false
	}
	parent, at := s.doc.Parent(id)
	if parent == nil {
		return false
	}
	step := at + direction
	for step >= 0 && step < len(parent.Children) && parent.Children[step].Kind == "#raw" {
		step += direction
	}
	var target *blocks.Node
	if step >= 0 && step < len(parent.Children) {
		candidate := parent.Children[step]
		if insert {
			if candidate.Kind == "p" && len(candidate.Children) == 0 {
				target = candidate
			}
			if candidate.Kind == "p" && len(candidate.Children) == 1 && candidate.Children[0].Kind == "#text" && candidate.Plain() == "" {
				target = candidate
			}
		} else if blocks.IsText(candidate) || candidate.Kind == "pre" {
			target = candidate
		}
	}
	if target == nil {
		if direction < 0 {
			return false
		}
		target = blocks.Paragraph("")
		s.lastTextID = ""
		nodes := []*blocks.Node{target}
		// Lossless Markdown stores the newline after a fence as a raw sibling.
		// Insert after that separator; if none exists, add one in the same undo
		// transaction so prose cannot become part of the closing fence line.
		separated := strings.HasSuffix(n.Markdown(), "\n")
		for _, gap := range parent.Children[at+1 : step] {
			separated = separated || strings.Contains(gap.Text, "\n")
		}
		if s.doc.Format == "markdown" && !separated {
			gap := blocks.New("#raw")
			gap.Text = "\n\n"
			nodes = append([]*blocks.Node{gap}, nodes...)
		}
		if a.fail(s.doc.Splice(parent.ID, step, 0, nodes...)) {
			return false
		}
		a.captureBlocks()
	}
	s.chosen.Clear()
	offset := 0
	if direction < 0 {
		offset = len([]rune(target.Plain()))
	}
	a.focusBlock(target.ID, offset)
	if blocks.IsText(target) {
		a.doc = a.blockField(target, a.documentColor).doc
	}
	return true
}

// Keep ordinary Enter and Shift+arrows inside the code editor. Only an
// unmodified arrow crossing its text boundary changes the focused block.
func (a *app) codeBoundaryInput(id string, field *blockField, event ui.InputEvent) bool {
	if event.Kind != ui.InputKeyDown || event.Mods != 0 || len(field.code.Selections()) != 1 {
		return false
	}
	selection := field.code.Selection()
	if !selection.Empty() {
		return false
	}
	length := len([]rune(field.code.Buffer.String()))
	if selection.Head == length && (event.Key == ui.KeyDown || event.Key == ui.KeyRight) {
		return a.leaveCode(id, 1, false)
	}
	if selection.Head == 0 && (event.Key == ui.KeyUp || event.Key == ui.KeyLeft) {
		return a.leaveCode(id, -1, false)
	}
	return false
}

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
