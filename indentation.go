package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Claim Tab only in editable body fields, before native focus navigation runs.
func (a *app) bodyIndentShortcuts(ed ui.Element, doc *richtext.Document) {
	if doc.Formatting().Paragraph.CodeBlock {
		return // Code editors own their literal tabs and selection indentation.
	}
	indent := ed.Shortcut(0, ui.KeyTab)
	outdent := ed.Shortcut(ui.Shift, ui.KeyTab)
	if !indent && !outdent {
		return
	}
	if a.blockEditor != nil {
		// A formatting command must not coalesce with surrounding typing.
		a.captureBlocks()
		a.blockEditor.lastTextID = ""
	}
	level := 1
	if outdent {
		level = -1
	}
	doc.Indent(level)
	a.capture()
	if a.blockEditor != nil {
		a.blockEditor.lastTextID = ""
	}
}
