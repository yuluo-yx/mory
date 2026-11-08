package main

import (
	"github.com/yuluo-yx/mory/internal/blocks"
	"time"
)

func (a *app) insertInline(kind string) {
	if a.current == nil {
		return
	}
	if a.blockEditor == nil && !a.enableBlocks() {
		return
	}
	s := a.blockEditor
	n := blocks.New(kind)
	switch kind {
	case "latex":
		n.Children = []*blocks.Node{blocks.Text("x^2")}
	case "time":
		at := time.Now().Add(time.Hour)
		n.Set("date", at.Format("2006-01-02"))
		n.Set("time", at.Format("15:04"))
	case "button":
		n.Set("text", "Button")
		n.Set("href", "https://example.com")
	case "doc-link":
		n.Set("title", "Document")
	}
	if field := s.fields[s.active]; field != nil && field.doc != nil {
		attachment := a.inlineObject(n, a.documentColor)
		attachment.Data = n.Clone()
		attachment.URL = "mory-inline:" + n.ID
		field.doc.InsertAttachment(attachment)
		a.captureBlocks()
	} else {
		a.insertNodes(blocks.New("p", n))
	}
	a.insertOpen = false
	if kind == "latex" {
		s.formulaID, s.formulaText, s.formulaOpen = n.ID, n.Plain(), true
	} else {
		a.editBlockProperties(n)
	}
}
