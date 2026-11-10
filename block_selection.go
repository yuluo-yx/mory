package main

import (
	"strings"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func (a *app) copyBlocks(c *ui.Context, cut bool) {
	s := a.blockEditor
	if s == nil {
		return
	}
	a.captureBlocks()
	root := blocks.New("root")
	for _, n := range s.doc.Root.Children {
		if s.chosen.Has(n.ID) {
			root.Children = append(root.Children, n.Clone())
		}
	}
	d, _ := blocks.Parse(root.XML(), "xml")
	html, _ := d.HTML(blocks.ExportOptions{})
	c.Services().WriteClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte(html)), transfer.Bytes(transfer.Text, []byte(root.Markdown())), transfer.Bytes(transfer.Format("application/vnd.mory.docxml"), []byte(root.XML())))))
	if cut {
		a.deleteBlocks()
	}
}

func (a *app) deleteBlocks() {
	s := a.blockEditor
	if s == nil {
		return
	}
	a.fail(s.doc.Transaction(func() error {
		for id := range s.chosen.All() {
			if s.doc.Find(id) != nil {
				if err := s.doc.Remove(id); err != nil {
					return err
				}
			}
		}
		return nil
	}))
	s.chosen.Clear()
	s.fields = map[string]*blockField{}
	a.captureBlocks()
}

func (a *app) focusBlock(id string, offset int) {
	s := a.blockEditor
	if s == nil {
		return
	}
	s.active, s.focus, s.focusAt = id, id, offset
	top := id
	for {
		p, _ := s.doc.Parent(top)
		if p == nil || p == s.doc.Root {
			break
		}
		if p.Kind == "details" || p.Kind == "tabs" {
			if s.componentChoice == nil {
				s.componentChoice = map[string]string{}
			}
			if p.Kind == "details" {
				s.componentChoice[p.ID] = "open"
			} else if child := s.doc.Find(top); child != nil && child.Kind == "tab-item" {
				key := p.ID
				if group := p.Attr("groupId"); group != "" {
					key = "group:" + group
				}
				s.componentChoice[key] = child.Attr("value")
			}
		}
		top = p.ID
	}
	s.revealHeadingParents(top)
	for i, n := range s.visibleBlocks() {
		if n.ID == top {
			s.list.ScrollTo(i, ui.Center)
			break
		}
	}
}

func (a *app) findStructured() {
	s := a.blockEditor
	var texts []*blocks.Node
	s.doc.Root.Walk(func(n *blocks.Node) {
		if blocks.IsText(n) && n.Kind != "title" {
			texts = append(texts, n)
		}
	})
	start := 0
	for i, n := range texts {
		if n.ID == s.active {
			start = i
			break
		}
	}
	for step := 0; step <= len(texts); step++ {
		if len(texts) == 0 {
			break
		}
		n := texts[(start+step)%len(texts)]
		f := a.blockField(n, a.documentColor)
		from := 0
		if step == 0 {
			_, from = f.doc.Selection()
		}
		runes := []rune(f.doc.Text().String())
		from = min(from, len(runes))
		tail := string(runes[from:])
		index := strings.Index(tail, a.findText)
		if index >= 0 {
			offset := from + len([]rune(tail[:index]))
			a.focusBlock(n.ID, offset)
			f.doc.SetSelection(offset, offset+len([]rune(a.findText)))
			s.focus = ""
			a.doc = f.doc
			return
		}
	}
	a.status = "No matches"
}

func (a *app) replaceAll() {
	if a.current == nil || a.findText == "" {
		return
	}
	if a.structured() {
		s := a.blockEditor
		a.captureBlocks()
		a.fail(s.doc.Transaction(func() error {
			var ids []string
			s.doc.Root.Walk(func(n *blocks.Node) {
				if n.Kind == "#text" && strings.Contains(n.Text, a.findText) {
					ids = append(ids, n.ID)
				}
			})
			for _, id := range ids {
				if err := s.doc.Transform(id, func(n *blocks.Node) { n.Text = strings.ReplaceAll(n.Text, a.findText, a.replaceText) }); err != nil {
					return err
				}
			}
			return nil
		}))
		s.fields = map[string]*blockField{}
		a.captureBlocks()
	} else {
		text := a.doc.Text().String()
		count := strings.Count(text, a.findText)
		for range count {
			a.findNext()
			a.doc.InsertText(a.replaceText)
		}
		a.capture()
	}
}
