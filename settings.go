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
		ui.Row(c).Gap(8).Children(func() {
			for _, tab := range []string{"General", "Keyboard shortcuts"} {
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
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() { a.settingsContent(c) })
		}
	})
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
