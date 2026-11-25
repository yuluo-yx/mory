package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

const (
	defaultSidebarWidth = 240
	minSidebarWidth     = 200
	maxSidebarWidth     = 480
	minWorkspaceWidth   = 480
)

func (a *app) preferredSidebarWidth() float32 {
	width := a.lib.Settings.SidebarWidth
	if width == 0 {
		return defaultSidebarWidth
	}
	return float32(max(minSidebarWidth, min(maxSidebarWidth, width)))
}

// Window resizing only limits the visible width; it keeps the user's preference.
func sidebarWidthLimit(windowWidth float32) float32 {
	return max(minSidebarWidth, min(maxSidebarWidth, windowWidth-minWorkspaceWidth))
}

func (a *app) visibleSidebarWidth(windowWidth float32) float32 {
	width := a.preferredSidebarWidth()
	if a.sidebarDragging {
		width = a.sidebarDragWidth
	}
	return max(minSidebarWidth, min(sidebarWidthLimit(windowWidth), width))
}

func (a *app) setSidebarWidth(width float32) {
	previous := a.lib.Settings.SidebarWidth
	a.lib.Settings.SidebarWidth = max(minSidebarWidth, min(maxSidebarWidth, int(math.Round(float64(width)))))
	if !a.savePreferences() {
		a.lib.Settings.SidebarWidth = previous
	}
}

// An overlay straddles the existing one-pixel border without changing layout.
// MyGo captures the pointer for the whole drag, even outside this eight-DIP grip.
func (a *app) sidebarResizer(c *ui.Context, sidebar ui.Element) {
	windowWidth, _ := c.Size()
	width := a.visibleSidebarWidth(windowWidth)
	edge := ui.Box(c.Key("sidebar-resizer")).Absolute().Left(width-4).Top(0).Bottom(0).Width(8).
		Cursor(ui.CursorResizeEW).Focusable().Role(ui.RoleSplitter).Label(a.tr("Resize sidebar")).
		Tooltip(a.tr("Drag to resize sidebar. Double-click to reset.")).
		Range(minSidebarWidth, float64(sidebarWidthLimit(windowWidth)), float64(width))
	dx, _, held := edge.Dragged()
	switch {
	case edge.DoubleClicked():
		a.sidebarDragging = false
		a.setSidebarWidth(defaultSidebarWidth)
	case edge.PressCanceled():
		a.sidebarDragging = false
	case held:
		if !a.sidebarDragging {
			a.sidebarDragWidth, a.sidebarDragging = width, true
			a.sidebarDragMoved = false
		}
		// Retain overshoot so reversing beyond a limit does not make it jump.
		a.sidebarDragWidth += dx
		a.sidebarDragMoved = a.sidebarDragMoved || dx != 0
		edge.Focus(ui.FocusOptions{PreventScroll: true})
	case a.sidebarDragging:
		a.sidebarDragging = false
		if a.sidebarDragMoved && width != a.preferredSidebarWidth() {
			a.setSidebarWidth(width)
		}
	}
	if edge.Shortcut(0, ui.KeyLeft) {
		a.setSidebarWidth(max(minSidebarWidth, width-10))
	}
	if edge.Shortcut(0, ui.KeyRight) {
		a.setSidebarWidth(min(sidebarWidthLimit(windowWidth), width+10))
	}
	width = a.visibleSidebarWidth(windowWidth)
	sidebar.Width(width)
	edge.Left(width - 4)
	edge.Draw(func(p *ui.Painter, r ui.Rect) {
		if held || edge.Hovered() || edge.FocusVisible() {
			p.Fill(ui.Rect{X: r.X + 3, Y: r.Y, W: 2, H: r.H}, c.Theme().Accent, 0)
		}
	})
}
