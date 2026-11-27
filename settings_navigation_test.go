package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSettingsSidebarLayoutAndCategories(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, appearance := range []string{"Light", "Dark"} {
			t.Run(language+"/"+appearance, func(t *testing.T) {
				a := testApp(t)
				a.lib.Settings.Language, a.lib.Settings.Appearance = language, appearance
				a.createNote("Unchanged body", "Working document")
				a.settingsOpen = true
				tt := ui.NewTester(a.view, 1280, 820)
				for _, size := range [][2]int{{1000, 620}, {1280, 820}, {2560, 1440}} {
					tt.SetSize(size[0], size[1])
					for _, bar := range []ui.TitleBar{{Height: 48, Left: 78}, {}} {
						tt.SetTitleBar(bar)
						for _, category := range settingsCategories {
							click(t, tt, a.tr(category.name))
							content, _ := tt.Find(a.tr("Settings content"))
							title, ok := tt.Access(a.tr("Settings section title"))
							if !ok || title.Value != a.tr(category.name) {
								t.Fatal("content title does not match the selected category", title)
							}
							lastY := float32(0)
							for _, nav := range settingsCategories {
								item, ok := tt.AccessByRole(ui.RoleTab, a.tr(nav.name))
								if !ok || item.Bounds.X+item.Bounds.W >= content.X || item.Bounds.Y <= lastY || item.Selected != (nav.name == category.name) {
									t.Fatal("category is not a vertical selected sidebar tab", nav.name, item, content)
								}
								lastY = item.Bounds.Y
							}
							for _, label := range []string{"My documents", "Open Working document", "Toggle sidebar", "Document editor"} {
								if _, ok := tt.Find(a.tr(label)); ok {
									t.Fatal("library UI is still present in settings", label)
								}
							}
							if category.name == "Keyboard shortcuts" {
								for _, label := range []string{"Save shortcuts", "Global quick note shortcut", "Back from settings"} {
									r, ok := tt.Find(a.tr(label))
									if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
										t.Fatal("settings control exceeds the window", size, label, r)
									}
								}
							}
							if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && language == "zh-CN" && size[0] == 1280 && bar.Height > 0 && (category.name == "About" || category.name == "General") {
								saveImage(t, tt, filepath.Join(dir, fmt.Sprintf("settings-%s-%s.png", category.name, appearance)))
							}
						}
					}
				}
			})
		}
	}
}

func TestSettingsReturnRestoresWorkspace(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		for _, focus := range []bool{false, true} {
			for _, mode := range []string{"Write", "Source", "Split", "Read"} {
				t.Run(fmt.Sprintf("hidden=%t/focus=%t/%s", hidden, focus, mode), func(t *testing.T) {
					a := testApp(t)
					a.createNote("Keep this document", "Draft")
					note := a.current
					a.doc.SetSelection(a.doc.Text().Len(), a.doc.Text().Len())
					a.doc.InsertText(" with pending edits")
					a.changeMode(mode)
					a.sidebarHidden, a.focus = hidden, focus
					tt := ui.NewTester(a.view, 1280, 820)
					for _, escape := range []bool{false, true} {
						tt.Key(ui.Cmd, ui.KeyComma)
						click(t, tt, "About")
						if !tt.HasText("Your work stays local") {
							t.Fatal("about page missing")
						}
						if escape {
							tt.Key(0, ui.KeyEscape)
						} else {
							click(t, tt, "Back from settings")
						}
						if a.current != note || a.mode != mode || a.sidebarHidden != hidden || a.focus != focus || note.Source != "Keep this document with pending edits" || a.settingsOpen || a.helpOpen {
							t.Fatal("settings return changed workspace or document state")
						}
					}
				})
			}
		}
	}
}

func TestSettingsKeyboardNavigationAndAboutLink(t *testing.T) {
	a := testApp(t)
	a.folder, a.query = "Work", "draft"
	a.helpOpen = true
	tt := ui.NewTester(a.view, 1280, 820)
	if a.settingsTab != "Keyboard shortcuts" {
		t.Fatal("shortcut help did not select its category")
	}
	click(t, tt, "General")
	for _, step := range []struct {
		key ui.Key
		tab string
	}{{ui.KeyDown, "Editor"}, {ui.KeyEnd, "About"}, {ui.KeyUp, "Keyboard shortcuts"}, {ui.KeyHome, "General"}} {
		tt.Key(0, step.key)
		if a.settingsTab != step.tab || a.helpOpen || !a.settingsOpen {
			t.Fatal("keyboard navigation did not select the category", step.tab, a.settingsTab)
		}
	}
	click(t, tt, "About")
	previous := openExternal
	t.Cleanup(func() { openExternal = previous })
	url := ""
	openExternal = func(value string) error { url = value; return nil }
	click(t, tt, "Project on GitHub")
	if url != "https://github.com/yuluo-yx/mory" {
		t.Fatal("wrong project link", url)
	}
	click(t, tt, "Back from settings")
	if a.current != nil || a.folder != "Work" || a.query != "draft" || a.shortcutDraft != nil {
		t.Fatal("settings changed the library scope")
	}
}
