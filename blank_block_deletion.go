package main

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func blankTextBlock(n *blocks.Node) bool {
	if n == nil || n.Kind != "p" && headingLevel(n) == 0 {
		return false
	}
	return blocks.ToRich(n, blocks.RichOptions{}).Len() == 0 && len(blocks.Visible(n.Children)) == 0
}

// Lists share their outer paragraph boundary. Other containers, including table
// cells and callouts, retain their own editing boundary.
func (s *blockSession) adjacentBlankTarget(id string, direction int) *blocks.Node {
	parent, at := s.doc.Parent(id)
	for parent != nil {
		at += direction
		for at >= 0 && at < len(parent.Children) {
			n := parent.Children[at]
			if (n.Kind == "#raw" || n.Kind == "#text") && strings.TrimSpace(n.Text) == "" {
				at += direction
				continue
			}
			for n.Kind == "ol" || n.Kind == "ul" {
				children := blocks.Visible(n.Children)
				if len(children) == 0 {
					return nil
				}
				i := 0
				if direction < 0 {
					i = len(children) - 1
				}
				n = children[i]
			}
			return n
		}
		if parent.Kind != "ol" && parent.Kind != "ul" {
			return nil
		}
		parent, at = s.doc.Parent(parent.ID)
	}
	return nil
}

func (s *blockSession) blankDeletion(id string, direction int) (remove, keep *blocks.Node) {
	current := s.doc.Find(id)
	adjacent := s.adjacentBlankTarget(id, direction)
	if current == nil || adjacent == nil {
		return nil, nil
	}
	if blankTextBlock(adjacent) {
		return adjacent, current
	}
	if blankTextBlock(current) && (blocks.IsText(adjacent) && adjacent.Kind != "title" || adjacent.Kind == "pre") {
		return current, adjacent
	}
	return nil, nil
}

type blankDeletionInput struct {
	field         *blockField
	text          richtext.Text
	remove, keep  string
	caret, offset int
}

// Observe the actual caret at keydown, rather than registering a shortcut for
// the previous frame's selection. Native navigation can precede Backspace in
// the same frame; the rich editor would otherwise unstyle the heading first.
func (a *app) queueBlankDeletion(id string, field *blockField, direction int) bool {
	a.captureBlocks()
	s := a.blockEditor
	remove, keep := s.blankDeletion(id, direction)
	if remove == nil {
		return false
	}
	_, caret := field.doc.Selection()
	offset := caret
	if keep.ID != id {
		offset = 0
		if direction < 0 {
			offset = len([]rune(keep.Plain()))
		}
	}
	s.blankDeletionInput = &blankDeletionInput{field, field.doc.Text(), remove.ID, keep.ID, caret, offset}
	return true
}

func (a *app) applyBlankDeletionInput() {
	s := a.blockEditor
	p := s.blankDeletionInput
	if p == nil {
		return
	}
	s.blankDeletionInput = nil
	// Discard only the native boundary key's formatting side effect. The block
	// tree owns undo history, and the removal below is its single undoable edit.
	if !p.field.doc.Text().Equal(p.text) {
		p.field.doc.SetText(p.text)
		p.field.doc.SetSelection(p.caret, p.caret)
	}
	p.field.version = p.field.doc.Version()
	s.lastTextID = ""
	if a.fail(s.doc.Remove(p.remove)) {
		return
	}
	delete(s.fields, p.remove)
	a.focusBlock(p.keep, p.offset)
}
