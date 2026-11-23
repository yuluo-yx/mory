package main

import (
	"github.com/egoist/mygo/ui"
)

const minPageWidth, maxPageWidth = 480, 3840

// documentPageWidth uses one preference for every document and window in the
// library. Legacy per-note widths remain stored but no longer affect layout.
// A zero limit fills the available space; narrow windows still shrink the page.
func (a *app) documentPageWidth() float32 {
	switch a.pageWidthMode() {
	case "Full":
		return 0
	case "Wide":
		return 1020
	case "Custom":
		if width := a.lib.Settings.PageWidth; width != 0 {
			return float32(max(minPageWidth, min(maxPageWidth, width)))
		}
	}
	return 820
}

func (a *app) pageWidthMode() string {
	switch mode := a.lib.Settings.PageWidthMode; mode {
	case "Wide", "Full", "Custom":
		return mode
	}
	return "Standard"
}

// Persist only preferences, so width changes neither rewrite documents nor
// create history. Failed writes keep the previous width active in this window.
func (a *app) setPageWidth(mode string, width int) {
	previousMode, previousWidth := a.lib.Settings.PageWidthMode, a.lib.Settings.PageWidth
	a.lib.Settings.PageWidthMode, a.lib.Settings.PageWidth = mode, width
	if !a.savePreferences() {
		a.lib.Settings.PageWidthMode, a.lib.Settings.PageWidth = previousMode, previousWidth
	}
}

// Both entry points control the same library preference, even without an open note.
func (a *app) pageWidthControls(c *ui.Context) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.tr("Page width")).Grow(1)
		mode := a.pageWidthMode()
		if a.selectText(c, &mode, []string{"Standard", "Wide", "Full", "Custom"}).Label(a.tr("Page width")).Width(140).Changed() {
			width := a.lib.Settings.PageWidth
			if mode == "Custom" && width == 0 {
				width = int(a.documentPageWidth())
				if width == 0 {
					width = 820
				}
			}
			a.setPageWidth(mode, width)
		}
	})
	ui.Text(c, a.tr("Applies to all documents.")).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
	if a.pageWidthMode() != "Custom" {
		return
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		value := float64(a.documentPageWidth())
		slider := ui.StepSlider(c, &value, minPageWidth, maxPageWidth, 20).Label(a.tr("Custom page width")).Grow(1).MinWidth(0)
		input := ui.NumberInput(c, &value, minPageWidth, maxPageWidth, 20).Label(a.tr("Page width in pixels")).Width(160)
		ui.Text(c, "px").FontSize(12).TextColor(c.Theme().TextMuted)
		if slider.Changed() || input.Changed() {
			a.setPageWidth("Custom", max(minPageWidth, min(maxPageWidth, int(value))))
		}
	})
	ui.Text(c, a.tr("Drag to resize the page. Narrow windows fit it automatically.")).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
}
