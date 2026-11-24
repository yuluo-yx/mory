package main

import (
	"strings"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

const selectAllHint = "Press Ctrl+A or ⌘A again to select the entire document body"

func isSelectAllInput(event ui.InputEvent) bool {
	return event.Kind == ui.InputCommand && event.Text == "selectAll" ||
		event.Kind == ui.InputKeyDown && event.Key == ui.KeyA && (event.Mods == ui.Cmd || event.Mods == ui.Ctrl)
}

// Select All first belongs to the focused editor. Only a second invocation
// without intervening input promotes that selection to the document body.
func (a *app) observeBlockSelectAll(id string, event ui.InputEvent) {
	s := a.blockEditor
	if isSelectAllInput(event) {
		if event.Repeat {
			return
		}
		if s.selectAllID == id {
			s.selectAllPending = true
			s.selectAllID = ""
		} else {
			s.selectAllID = id
			a.status = selectAllHint
		}
		return
	}
	if event.Kind == ui.InputKeyDown {
		// Releasing and pressing the shortcut's modifier between invocations
		// must not count as an intervening editing action.
		switch event.Code {
		case ui.CodeControlLeft, ui.CodeControlRight, ui.CodeSuperLeft, ui.CodeSuperRight,
			ui.CodeShiftLeft, ui.CodeShiftRight, ui.CodeAltLeft, ui.CodeAltRight:
			return
		}
	}
	switch event.Kind {
	case ui.InputKeyDown, ui.InputText, ui.InputCompose, ui.InputCommand, ui.InputPointerDown, ui.InputSelect:
		s.selectAllID = ""
		if a.status == selectAllHint {
			a.status = ""
		}
	}
}

func (a *app) selectAllBlocks() {
	s := a.blockEditor
	a.captureBlocks()
	s.chosen.Clear()
	// Include folded and offscreen blocks without instantiating their editors.
	for _, n := range blocks.Visible(s.doc.Root.Children) {
		if n.Kind != "title" {
			s.chosen.Add(n.ID)
		}
	}
	s.focus = ""
	a.status = "Entire document body selected"
}

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
	var ids []string
	for id := range s.chosen.All() {
		ids = append(ids, id)
	}
	a.removeBlocks(ids)
}

// Every block-deletion entry point shares one undo transaction and restores a
// reachable caret, including when no body blocks remain.
func (a *app) removeBlocks(ids []string) {
	s := a.blockEditor
	if s == nil || len(ids) == 0 {
		return
	}
	a.captureBlocks()
	index := len(s.visibleBlocks())
	for i, n := range s.visibleBlocks() {
		for _, id := range ids {
			if n.ID == id {
				index = min(index, i)
			}
		}
	}
	if a.fail(s.doc.Transaction(func() error {
		for _, id := range ids {
			if s.doc.Find(id) != nil {
				if err := s.doc.Remove(id); err != nil {
					return err
				}
			}
		}
		return nil
	})) {
		return
	}
	s.fields = map[string]*blockField{}
	a.focusAfterBlockRemoval(index)
	a.captureBlocks()
}

func (a *app) focusAfterBlockRemoval(index int) {
	s := a.blockEditor
	s.chosen.Clear()
	s.active, s.focus, s.focusAt = "", "", 0
	visible := s.visibleBlocks()
	if len(visible) == 0 {
		if s.emptyBody == nil {
			s.emptyBody = blocks.Paragraph("")
		}
		s.active, s.focus = s.emptyBody.ID, s.emptyBody.ID
		// Keep the formatting toolbar present before the next view is built.
		// Adding it a pass later changes the editor's identity and loses focus.
		a.doc = a.blockField(s.emptyBody, a.documentColor).doc
		return
	}
	for offset := range len(visible) {
		n := visible[(min(index, len(visible)-1)+offset)%len(visible)]
		n.Walk(func(child *blocks.Node) {
			if s.focus == "" && (blocks.IsText(child) || child.Kind == "pre") && child.Kind != "title" {
				s.active, s.focus = child.ID, child.ID
			}
		})
		if s.focus != "" {
			a.focusBlock(s.focus, 0)
			return
		}
	}
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
		var matches []int
		for offset, runeOffset := 0, 0; offset < len(text); {
			index := strings.Index(text[offset:], a.findText)
			if index < 0 {
				break
			}
			runeOffset += len([]rune(text[offset : offset+index]))
			matches = append(matches, runeOffset)
			offset += index + len(a.findText)
			runeOffset += len([]rune(a.findText))
		}
		// Replace original matches backwards in one undo step. Replacement text
		// may contain the query and must never become another match in this edit.
		a.doc.Edit(func(edit *richtext.Edit) {
			length := len([]rune(a.findText))
			for index := len(matches) - 1; index >= 0; index-- {
				start := matches[index]
				edit.ReplaceString(start, start+length, a.replaceText, edit.Text().AttributesAt(start))
			}
		})
		a.capture()
	}
}
