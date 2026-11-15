package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

var calloutTypes = []string{"note", "info", "tip", "important", "warning", "caution", "danger"}

func (a *app) changeCalloutType(id, kind string) {
	if a.mode != "Write" || a.blockEditor == nil {
		return
	}
	a.captureBlocks()
	a.fail(a.blockEditor.doc.Transform(id, func(n *blocks.Node) { blocks.SetCalloutType(n, kind) }))
	a.captureBlocks()
}

// The last text field exits its nearest callout. Earlier fields keep ordinary
// paragraph navigation, and Shift+arrows keep extending the text selection.
func (a *app) calloutBoundaryInput(id string, field *blockField, event ui.InputEvent) bool {
	if event.Kind != ui.InputKeyDown || event.Mods != 0 || (event.Key != ui.KeyDown && event.Key != ui.KeyRight) {
		return false
	}
	anchor, caret := field.doc.Selection()
	if anchor != caret || caret != field.doc.Text().Len() {
		return false
	}
	s := a.blockEditor
	for parent, _ := s.doc.Parent(id); parent != nil; parent, _ = s.doc.Parent(parent.ID) {
		if parent.Kind != "callout" {
			continue
		}
		last := ""
		parent.Walk(func(n *blocks.Node) {
			if blocks.IsText(n) || n.Kind == "pre" {
				last = n.ID
			}
		})
		if last == id {
			return a.leaveBlock(parent.ID, 1, false)
		}
		return false
	}
	return false
}

// The entry sits outside the container so its placement shows where typing goes.
func (a *app) writeBelowBlock(c *ui.Context, n *blocks.Node, label string) {
	below := ui.Row(c.Key("below-" + n.Kind)).FillWidth().Height(24).AlignItems(ui.Center).Focusable().Cursor(ui.CursorText).
		Label(a.tr(label)).Tooltip(a.tr(label))
	if below.Hovered() || below.Focused() {
		below.Background(c.Theme().Accent.Alpha(.06)).Radius(4).Children(func() {
			ui.Text(c, a.tr(label)).FontSize(11).TextColor(c.Theme().TextMuted)
		})
	}
	if below.Clicked() {
		a.leaveBlock(n.ID, 1, true)
	}
}
