package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/doctheme"
	"github.com/yuluo-yx/mory/internal/library"
)

func (a *app) reloadTheme() {
	name := a.lib.Settings.Theme
	if a.docTheme != nil && name == a.themeName && time.Since(a.themeChecked) < time.Second {
		return
	}
	a.themeChecked = time.Now()
	path := filepath.Join(a.lib.Dir, "themes", strings.ToLower(name)+".css")
	stamp := time.Time{}
	if info, err := os.Stat(path); err == nil {
		stamp = info.ModTime()
	}
	if a.docTheme != nil && name == a.themeName && stamp == a.themeStamp {
		return
	}
	theme, err := doctheme.Load(filepath.Join(a.lib.Dir, "themes"), name)
	if a.fail(err) {
		return
	}
	a.capture()
	a.docTheme, a.themeName, a.themeStamp = theme, name, stamp
	if a.blockEditor != nil {
		s := a.blockEditor
		anchor, caret := 0, 0
		if f := s.fields[s.active]; f != nil && f.doc != nil {
			anchor, caret = f.doc.Selection()
		}
		a.blockEditor.fields = map[string]*blockField{}
		if n := s.doc.Find(s.active); n != nil && blocks.IsText(n) {
			f := a.blockField(n, a.documentColor)
			f.doc.SetSelection(anchor, caret)
		}
	}
}

func (a *app) pageTheme(base *ui.Theme) *ui.Theme {
	t := *base
	if a.docTheme == nil {
		return &t
	}
	t.Text = a.docTheme.Color(".page", "color", t.Text)
	t.Background = a.docTheme.Color(".page", "background-color", t.Background)
	t.Accent = a.docTheme.Color("a", "color", t.Accent)
	t.Selection = a.docTheme.Color("::selection", "background-color", t.Selection)
	if base.Dark {
		// CSS colors remain authored in light mode; adapt contrast for native dark
		// documents while printing always uses the original stylesheet.
		t.Background = ui.Hex("#202321")
		t.Text = ui.Hex("#e8e8e5")
		t.Border = ui.Hex("#454a46")
		t.Surface = ui.Hex("#2c302c")
	}
	return &t
}

func (a *app) writingFont(selector string) ui.Font {
	font := ui.Font{Family: "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', sans-serif", Size: 16, LineHeight: 1.6}
	if a.docTheme != nil {
		font = a.docTheme.Font(selector, font)
	}
	font.Size *= a.lib.Settings.FontSize / 17
	return font
}

func (a *app) namedColor(kind, name string, fallback ui.Color) ui.Color {
	fallback = blockColor(name, fallback)
	if a.docTheme != nil {
		return a.docTheme.Named(kind, name, fallback)
	}
	return fallback
}

func (a *app) themeControls(c *ui.Context) {
	if ui.Select(c, &a.lib.Settings.Theme, doctheme.Names(filepath.Join(a.lib.Dir, "themes"))).Label("Writing style").FillWidth().Changed() {
		a.fail(a.lib.SaveSettings())
		a.reloadTheme()
	}
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Edit CSS").Clicked() {
			a.cssName = a.lib.Settings.Theme
			a.cssText = a.docTheme.Source
			a.cssOpen = true
			a.settingsOpen = false
		}
		if ui.Button(c, "Duplicate theme").Clicked() {
			a.cssName = strings.ToLower(a.lib.Settings.Theme) + "-copy"
			a.cssText = a.docTheme.Source
			a.cssOpen = true
			a.settingsOpen = false
		}
		if ui.Button(c, "Import CSS…").Clicked() {
			a.chooseTheme()
		}
	})
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Reset theme").Clicked() {
			path := filepath.Join(a.lib.Dir, "themes", strings.ToLower(a.lib.Settings.Theme)+".css")
			err := os.Remove(path)
			if err == nil || os.IsNotExist(err) {
				a.docTheme = nil
				a.reloadTheme()
			} else {
				a.fail(err)
			}
		}
		if ui.Button(c, "Open themes folder").Clicked() {
			a.reveal(filepath.Join(a.lib.Dir, "themes"))
		}
	})
	if a.docTheme != nil && len(a.docTheme.Warnings) > 0 {
		ui.Textf(c, "%d unsupported or invalid declarations", len(a.docTheme.Warnings)).FontSize(11).TextColor(c.Theme().TextMuted)
	}
}

func (a *app) themeOverlay(c *ui.Context) {
	ui.Modal(c, &a.cssOpen, func() {
		ui.Column(c).Width(720).MaxHeight(700).Gap(12).Children(func() {
			ui.Text(c, "Document stylesheet").FontSize(24)
			ui.TextInput(c, &a.cssName).Label("Theme name").FillWidth()
			ui.TextArea(c, &a.cssText).Label("Theme CSS").Font("monospace").FontSize(12).Height(360).FillWidth()
			parsed := doctheme.Parse(a.cssText)
			ui.Scroll(c).MaxHeight(100).Children(func() {
				for _, warning := range parsed.Warnings {
					ui.Text(c, warning).FontSize(11)
				}
			})
			ui.Row(c).Gap(10).Children(func() {
				if ui.PrimaryButton(c, "Apply stylesheet").Clicked() {
					name := strings.TrimSpace(a.cssName)
					if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
						a.problem = "Use a simple theme name."
					} else if !a.fail(library.AtomicWrite(filepath.Join(a.lib.Dir, "themes", strings.ToLower(name)+".css"), []byte(a.cssText))) {
						a.lib.Settings.Theme = name
						a.fail(a.lib.SaveSettings())
						a.docTheme = nil
						a.reloadTheme()
						a.cssOpen = false
					}
				}
				if ui.Button(c, "Cancel").Clicked() {
					a.cssOpen = false
				}
			})
		})
	})
}

func (a *app) chooseTheme() {
	if a.win == nil {
		return
	}
	go func() {
		paths, err := openDialog(mygo.OpenDialogOptions{Parent: a.win, Title: "Import document theme", Filters: []mygo.FileFilter{{Name: "CSS stylesheet", Extensions: []string{"css"}}}})
		a.win.Update(func() {
			if a.fail(err) || len(paths) == 0 {
				return
			}
			b, err := os.ReadFile(paths[0])
			if a.fail(err) {
				return
			}
			a.cssName = strings.TrimSuffix(filepath.Base(paths[0]), filepath.Ext(paths[0]))
			a.cssText = string(b)
			a.cssOpen = true
			a.settingsOpen = false
		})
	}()
}
