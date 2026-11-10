package main

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"
)

func settingsCard(c *ui.Context, title string, content func()) {
	ui.Column(c).FillWidth().Padding(16).Radius(10).Background(c.Theme().Surface).
		Border(1, c.Theme().Border.Alpha(0.6)).Gap(12).Children(func() {
		ui.Text(c, title).FontSize(14).Bold()
		content()
	})
}

func (a *app) settingsContent(c *ui.Context) {
	ui.Column(c).FillWidth().MaxWidth(840).Gap(16).Children(func() {
		switch a.settingsTab {
		case "Editor":
			a.editorSettings(c)
		case "Themes":
			settingsCard(c, a.tr("Appearance & Style"), func() {
				if a.selectText(c, &a.lib.Settings.Appearance, []string{"System", "Light", "Dark"}).Label(a.tr("Appearance")).FillWidth().Changed() {
					a.savePreferences()
				}
				a.themeControls(c)
			})
		default:
			a.generalSettings(c)
		}
	})
}

func (a *app) generalSettings(c *ui.Context) {
	settingsCard(c, a.tr("Language"), func() {
		language := "English"
		if a.lib.Settings.Language == "zh-CN" {
			language = "简体中文"
		}
		if ui.Select(c, &language, []string{"English", "简体中文"}).Label(a.tr("Application language")).FillWidth().Changed() {
			code := "en"
			if language == "简体中文" {
				code = "zh-CN"
			}
			a.setLanguage(code)
		}
	})
	settingsCard(c, a.tr("Library & Storage"), func() {
		ui.Text(c, a.lib.Dir).FontSize(13).TextColor(c.Theme().TextMuted)
		if ui.Button(c, a.tr("Reveal library in Finder")).AlignSelf(ui.Start).Clicked() {
			a.reveal(a.lib.Dir)
		}
	})
	settingsCard(c, a.tr("Default application"), func() {
		ui.Text(c, a.tr("Open .md, .markdown and .mdx files with Mory from Finder.")).FontSize(13).TextColor(c.Theme().TextMuted)
		if ui.Button(c, a.tr("Make Mory the default Markdown app")).AlignSelf(ui.Start).Clicked() {
			a.makeMarkdownDefault()
		}
		if a.associationStatus != "" {
			ui.Text(c, a.associationStatus).FontSize(12).TextColor(c.Theme().Accent)
		}
	})
	ui.Text(c, a.tr("Mory 0.1.0 · Made for macOS")).FontSize(12).TextColor(c.Theme().TextMuted)
}

func (a *app) editorSettings(c *ui.Context) {
	settingsCard(c, a.tr("Editor Preferences"), func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, a.tr("Text size: %.0f"), a.lib.Settings.FontSize).Grow(1)
			if ui.Button(c, "−").Label(a.tr("Smaller text")).Disabled(a.lib.Settings.FontSize <= 12).Clicked() {
				a.lib.Settings.FontSize = max(12, a.lib.Settings.FontSize-1)
				a.docTheme = nil
				a.savePreferences()
			}
			if ui.Button(c, "+").Label(a.tr("Larger text")).Disabled(a.lib.Settings.FontSize >= 30).Clicked() {
				a.lib.Settings.FontSize = min(30, a.lib.Settings.FontSize+1)
				a.docTheme = nil
				a.savePreferences()
			}
		})
		ui.Checkbox(c, &a.showToolbar, a.tr("Show formatting toolbar"))
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Page placement")).Grow(1)
			position := a.pagePosition()
			if a.selectText(c, &position, []string{"Left", "Center", "Right"}).Label(a.tr("Document position")).Width(140).Changed() {
				a.setPagePosition(position)
			}
		})
		ui.Text(c, a.tr("Position the cover, title and body together within the writing area.")).FontSize(12).TextColor(c.Theme().TextMuted)
		if a.current != nil {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.tr("Page width")).Grow(1)
				width := a.current.Width
				if width == "" {
					width = "Standard"
				}
				if a.selectText(c, &width, []string{"Standard", "Wide", "Full"}).Label(a.tr("Page width")).Width(140).Changed() {
					a.mutate(a.current, func() { a.current.Width = width })
				}
			})
		}
	})
	settingsCard(c, a.tr("Document outline"), func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Font size")).Grow(1)
			size := fmt.Sprintf("%.0f", a.outlineFontSize())
			if ui.Select(c, &size, []string{"12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22"}).Label(a.tr("Outline text size")).Width(80).Changed() {
				value, err := strconv.Atoi(size)
				if err == nil {
					a.setOutlineFontSize(float32(value))
				}
			}
			if ghostButton(c, a.tr("Use default size")).Disabled(a.outlineFontSize() == defaultOutlineFontSize).Clicked() {
				a.setOutlineFontSize(defaultOutlineFontSize)
			}
		})
		ui.Column(c).Padding(10, 12).Radius(7).Background(c.Theme().Background).Gap(6).Children(func() {
			ui.Text(c, a.tr("Outline preview")).Label(a.tr("Outline font preview")).FontSize(a.outlineFontSize())
			ui.Text(c, a.tr("Nested heading preview")).FontSize(a.outlineFontSize()).Padding(0, 0, 0, 12).TextColor(c.Theme().TextMuted)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Outline position")).Grow(1)
			side := a.outlineSide()
			if a.selectText(c, &side, []string{"Left", "Right"}).Label(a.tr("Outline placement")).Width(140).Changed() {
				a.setOutlineSide(side)
			}
		})
		ui.Checkbox(c, &a.outline, a.tr("Show outline"))
	})
}
