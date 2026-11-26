package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func isBlockDeleteKey(key ui.Key, mods ui.Modifiers) bool {
	return mods == 0 && (key == ui.KeyBackspace || key == ui.KeyDelete)
}

// Ordinary Markdown may reopen in the rich editor. Give its empty paragraphs
// the same forward-delete behavior without joining or deleting adjacent text.
func exitEmptyRichBlock(doc *richtext.Document) bool {
	anchor, caret := doc.Selection()
	text := doc.Text()
	start, end := text.ParagraphRange(caret)
	style := text.ParagraphStyleAt(caret)
	if anchor != caret || start != end || style.List == richtext.NoList && style.Heading == 0 && !style.Quote && !style.CodeBlock {
		return false
	}
	doc.FormatParagraphs(func(p *richtext.ParagraphStyle) {
		switch {
		case p.List != richtext.NoList && p.Indent > 0:
			p.Indent--
		case p.List != richtext.NoList:
			p.List, p.Checked = richtext.NoList, false
		case p.Heading > 0:
			p.Heading = 0
		case p.Quote:
			p.Quote, p.QuoteDepth = false, 0
		case p.CodeBlock:
			p.CodeBlock = false
		}
	})
	return true
}

// Empty text is not enough to delete an enclosing block: attachments, opaque
// source, sibling paragraphs and nested containers must retain their contents.
func (a *app) emptyBlockTarget(id string) *blocks.Node {
	s := a.blockEditor
	if s == nil || a.mode != "Write" {
		return nil
	}
	n, field := s.doc.Find(id), s.fields[id]
	if n == nil || field == nil {
		return nil
	}
	if n.Kind == "pre" {
		if field.code == nil || strings.TrimSpace(field.code.Buffer.String()) != "" || len(field.code.Selections()) != 1 || !field.code.Selection().Empty() || n.Attr("caption") != "" {
			return nil
		}
		plain := true
		n.Walk(func(child *blocks.Node) {
			if child != n && child.Kind != "code" && child.Kind != "#text" {
				plain = false
			}
		})
		if !plain {
			return nil
		}
		return n
	}
	if field.doc == nil || field.doc.Text().Len() != 0 || len(blocks.Visible(n.Children)) != 0 {
		return nil
	}
	if headingLevel(n) > 0 {
		// Adjacent blank deletion already preserves the neighboring heading/list.
		if remove, _ := s.blankDeletion(id, -1); remove != nil {
			return nil
		}
		if remove, _ := s.blankDeletion(id, 1); remove != nil {
			return nil
		}
		return n
	}
	if n.Kind == "li" || n.Kind == "checkbox" {
		return n
	}
	parent, _ := s.doc.Parent(id)
	if n.Kind != "p" || parent == nil || parent.Kind != "blockquote" && parent.Kind != "callout" || parent.Attr("title") != "" {
		return nil
	}
	for _, child := range parent.Children {
		if child.ID == id {
			continue
		}
		if child.Kind != "#raw" && child.Kind != "#text" || strings.TrimSpace(child.Text) != "" {
			return nil
		}
	}
	return parent
}

// One deletion exits an empty structural block into ordinary body text. Keeping
// the field ID gives undo/redo and continued typing a stable caret destination.
func (a *app) exitEmptyBlock(id string) bool {
	if a.emptyBlockTarget(id) == nil {
		return false
	}
	a.captureBlocks()
	s := a.blockEditor
	target := a.emptyBlockTarget(id)
	if target == nil {
		return false
	}
	parent, at := s.doc.Parent(target.ID)
	if parent != nil && (parent.Kind == "ul" || parent.Kind == "ol") && (target.Kind == "li" || target.Kind == "checkbox") {
		return a.exitEmptyList(id)
	}
	if parent == nil {
		return false
	}
	paragraph := blocks.New("p")
	paragraph.ID = id
	if a.fail(s.doc.Splice(parent.ID, at, 1, paragraph)) {
		return true
	}
	s.lastTextID = ""
	delete(s.fields, id)
	a.focusBlock(id, 0)
	// Keep the text toolbar and editor identity stable when replacing lone code.
	a.doc = a.blockField(paragraph, a.documentColor).doc
	a.captureBlocks()
	return true
}
