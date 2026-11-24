package main

import "github.com/egoist/mygo/ui/richtext"

// MyGo 0.9.2 records a native replacement's redo selection before collapsing
// its caret. Collapse text-changing redos so continued typing cannot replace
// the restored words. Formatting-only redos retain the user's selection.
func redoRichText(doc *richtext.Document) {
	before := doc.Text().String()
	doc.Redo()
	anchor, caret := doc.Selection()
	if anchor != caret && doc.Text().String() != before {
		end := max(anchor, caret)
		doc.SetSelection(end, end)
	}
}
