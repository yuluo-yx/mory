package main

import (
	"github.com/egoist/mygo/ui"
)

// Preferences occupy the workspace, retaining the document and its edit buffer.
func (a *app) settingsPage(c *ui.Context) {
	if !a.cssOpen && c.Shortcut(0, ui.KeyEscape) {
		a.settingsOpen, a.helpOpen = false, false
		a.shortcutDraft = nil
	}
	if a.helpOpen {
		a.settingsTab = "Keyboard shortcuts"
	}
	ui.Column(c).Fill().MinHeight(0).Padding(48, 32, 20).Gap(20).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, a.tr("Settings")).FontSize(28).Bold().Grow(1)
			if ui.Button(c, a.tr("Done")).Label(a.tr("Back from settings")).Clicked() {
				a.settingsOpen, a.helpOpen = false, false
				a.shortcutDraft = nil
			}
		})
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			for _, tab := range []string{"General", "Editor", "Themes", "Keyboard shortcuts"} {
				button := ghostButton(c, a.tr(tab)).Label(a.tr(tab)).Padding(8, 14)
				if a.settingsTab == tab || a.settingsTab == "" && tab == "General" {
					button.Background(c.Theme().SurfaceHover)
				}
				if button.Clicked() {
					a.settingsTab = tab
					a.helpOpen = false
				}
			}
		})
		if a.settingsTab == "Keyboard shortcuts" {
			a.shortcutSettings(c)
		} else {
			// Each category keeps its own scroll position.
			ui.Scroll(c.Key(a.settingsTab)).Grow(1).MinHeight(0).FillWidth().Children(func() { a.settingsContent(c) })
		}
	})
}

const defaultOutlineFontSize float32 = 14

func (a *app) pagePosition() string {
	if position := a.lib.Settings.PagePosition; position == "Left" || position == "Right" {
		return position
	}
	return "Center"
}

func (a *app) pageAlignment() ui.Align {
	switch a.pagePosition() {
	case "Left":
		return ui.Start
	case "Right":
		return ui.End
	default:
		return ui.Center
	}
}

func (a *app) setPagePosition(position string) {
	previous := a.lib.Settings.PagePosition
	a.lib.Settings.PagePosition = position
	if !a.savePreferences() {
		a.lib.Settings.PagePosition = previous
	}
}

// Missing and invalid legacy values use a readable default without rewriting
// settings until the user makes a change.
func (a *app) outlineFontSize() float32 {
	if size := a.lib.Settings.OutlineFontSize; size >= 12 && size <= 22 {
		return size
	}
	return defaultOutlineFontSize
}

func (a *app) setOutlineFontSize(size float32) {
	previous := a.lib.Settings.OutlineFontSize
	a.lib.Settings.OutlineFontSize = max(12, min(22, size))
	if !a.savePreferences() {
		a.lib.Settings.OutlineFontSize = previous
	}
}

func (a *app) outlineSide() string {
	if a.lib.Settings.OutlineSide == "Left" {
		return "Left"
	}
	return "Right"
}

func (a *app) setOutlineSide(side string) {
	previous := a.lib.Settings.OutlineSide
	a.lib.Settings.OutlineSide = side
	if !a.savePreferences() {
		a.lib.Settings.OutlineSide = previous
	}
}

// Reload preference version stamps in sibling windows without replacing notes.
func (a *app) savePreferences() bool {
	if a.fail(a.lib.SaveSettings()) {
		return false
	}
	for _, other := range appWindows {
		if other == a || other.lib.Dir != a.lib.Dir {
			continue
		}
		if other.fail(other.lib.ReloadSettings()) {
			continue
		}
		other.docTheme = nil
		if other.win != nil && !other.win.IsDestroyed() {
			other.win.Invalidate()
		}
	}
	if a.win != nil {
		a.applyLocale()
		a.installMenu()
	}
	return true
}

func (a *app) setLanguage(language string) {
	previous := a.lib.Settings.Language
	a.lib.Settings.Language = language
	if !a.savePreferences() {
		a.lib.Settings.Language = previous
	}
}

func outlinePositionIcon(side string) *ui.SVG {
	if side == "Left" {
		return iconSidebar()
	}
	return iconSidebarRight()
}
