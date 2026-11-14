package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

const minPageWidth, maxPageWidth = 480, 3840

// documentPageWidth returns the preferred logical width, including page padding.
// A zero limit lets the page fill the available space; narrow windows still shrink it.
func documentPageWidth(n *library.Note) float32 {
	switch n.Width {
	case "Full":
		return 0
	case "Wide":
		return 1020
	case "Custom":
		if n.PageWidth != 0 {
			return float32(max(minPageWidth, min(maxPageWidth, n.PageWidth)))
		}
	}
	return 820
}

// pageWidthControls is shared by document details and editor preferences.
func (a *app) pageWidthControls(c *ui.Context, n *library.Note) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.tr("Page width")).Grow(1)
		width := n.Width
		if width == "" {
			width = "Standard"
		}
		if a.selectText(c, &width, []string{"Standard", "Wide", "Full", "Custom"}).Label(a.tr("Page width")).Width(140).Changed() {
			previous := documentPageWidth(n)
			a.mutate(n, func() {
				if width == "Custom" && n.PageWidth == 0 {
					n.PageWidth = int(previous)
					if n.PageWidth == 0 {
						n.PageWidth = 820
					}
				}
				n.Width = width
			})
		}
	})
	if n.Width != "Custom" {
		return
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		value := float64(documentPageWidth(n))
		slider := ui.StepSlider(c, &value, minPageWidth, maxPageWidth, 20).Label(a.tr("Custom page width")).Grow(1).MinWidth(0)
		input := ui.NumberInput(c, &value, minPageWidth, maxPageWidth, 20).Label(a.tr("Page width in pixels")).Width(160)
		ui.Text(c, "px").FontSize(12).TextColor(c.Theme().TextMuted)
		if slider.Changed() || input.Changed() {
			a.mutate(n, func() { n.PageWidth = max(minPageWidth, min(maxPageWidth, int(value))) })
		}
	})
	ui.Text(c, a.tr("Drag to resize the page. Narrow windows fit it automatically.")).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
}
