package main

import (
	"github.com/egoist/mygo/ui"
)

var settingsCategories = []struct {
	name string
	icon func() *ui.SVG
}{
	{"General", iconSliders},
	{"Editor", iconFileText},
	{"Themes", iconImage},
	{"Keyboard shortcuts", iconKeyboard},
	{"About", iconInfo},
}

func (a *app) closeSettings() {
	a.settingsOpen, a.helpOpen = false, false
	a.shortcutDraft = nil
}

// Preferences replace library navigation without changing the current document,
// editor mode, focus mode or the user's collapsed-sidebar preference.
func (a *app) settingsNavigation(c *ui.Context) {
	t := c.Theme()
	back := ghostButton(c, "").Label(a.tr("Back from settings")).FillWidth().Padding(9, 10).Radius(7)
	if back.Hovered() {
		back.Background(t.SurfaceHover)
	}
	back.Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconChevronLeft()).Width(16).Height(16).TextColor(t.TextMuted)
			ui.Text(c, a.tr("Back to workspace")).FontSize(sidebarFontSize)
		})
	})
	if back.Clicked() {
		a.closeSettings()
	}
	ui.Text(c, a.tr("Settings")).FontSize(22).Bold().Padding(18, 10, 14)
	selected := 0
	for i, category := range settingsCategories {
		if a.settingsTab == category.name {
			selected = i
		}
	}
	tabs := ui.TabsBase(c, &selected, len(settingsCategories))
	tabs.List.Column().FillWidth().Gap(4).Orientation(ui.Vertical).FocusGroup(ui.Vertical).
		Label(a.tr("Settings categories")).Children(func() {
		for i, category := range settingsCategories {
			tab := tabs.Tab(i).Label(a.tr(category.name)).FillWidth().Padding(11, 12).Radius(8).
				Gap(12).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			color := t.TextMuted
			if selected == i {
				tab.Background(t.Accent.Alpha(0.14))
				color = t.Accent
			} else if tab.Hovered() {
				tab.Background(t.SurfaceHover)
			}
			tab.Children(func() {
				ui.Icon(c, category.icon()).Width(18).Height(18).TextColor(color)
				text := ui.Text(c, a.tr(category.name)).FontSize(sidebarFontSize).SingleLine().Grow(1).MinWidth(0)
				if selected == i {
					text.TextColor(t.Accent).Bold()
				}
			})
		}
	})
	a.settingsTab = settingsCategories[selected].name
	if tabs.List.Changed() {
		a.helpOpen = false
		a.settingsOpen = true
	}
}

// Preferences occupy the workspace, retaining the document and its edit buffer.
func (a *app) settingsPage(c *ui.Context) {
	// Scope larger interface text to preferences, leaving document typography intact.
	settingsTheme := *c.Theme()
	settingsTheme.FontSize += 2
	ui.Themed(c, &settingsTheme).Column().Fill().MinHeight(0).Padding(48, 32, 20).Gap(24).Label(a.tr("Settings content")).Children(func() {
		ui.Text(c, a.tr(a.settingsTab)).Label(a.tr("Settings section title")).Value(a.tr(a.settingsTab)).FontSize(30).Bold().Shrink(0)
		if a.settingsTab == "Keyboard shortcuts" {
			ui.Column(c).FillWidth().MaxWidth(840).Grow(1).MinHeight(0).Children(func() { a.shortcutSettings(c) })
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
