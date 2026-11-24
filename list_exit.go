package main

import (
	"strconv"

	"github.com/yuluo-yx/mory/internal/blocks"
)

// Empty list items end the list before native paragraph conversion can turn
// them back into list items. Split the container without dropping later items.
func (a *app) exitEmptyList(id string) bool {
	session := a.blockEditor
	if session == nil {
		return false
	}
	item := session.doc.Find(id)
	field := session.fields[id]
	if item == nil || item.Kind != "li" || field == nil || field.doc == nil || field.doc.Text().Len() != 0 || len(blocks.Visible(item.Children)) != 0 {
		return false
	}
	a.captureBlocks()
	list, index := session.doc.Parent(id)
	if list == nil || list.Kind != "ul" && list.Kind != "ol" {
		return false
	}
	container, position := session.doc.Parent(list.ID)
	if container == nil {
		return false
	}
	paragraph := blocks.Paragraph("")
	paragraph.ID = id
	var replacement []*blocks.Node
	if index > 0 {
		before := list.Clone()
		before.Children = before.Children[:index]
		replacement = append(replacement, before)
	}
	replacement = append(replacement, paragraph)
	if index+1 < len(list.Children) {
		after := list.Clone()
		after.ID = blocks.New(list.Kind).ID
		after.Children = after.Children[index+1:]
		if list.Kind == "ol" {
			after.Set("start", strconv.Itoa(list.Int("start", 1)+index+1))
		}
		replacement = append(replacement, after)
	}
	if a.fail(session.doc.Splice(container.ID, position, 1, replacement...)) {
		return true
	}
	session.lastTextID = ""
	delete(session.fields, id)
	a.focusBlock(id, 0)
	a.captureBlocks()
	return true
}
