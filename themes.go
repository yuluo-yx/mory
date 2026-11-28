package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/doctheme"
	"github.com/yuluo-yx/mory/internal/library"
	"github.com/yuluo-yx/mory/internal/typography"
)

// Reset selects the bundled style. Preserve user CSS, including a yuluo
// override, as a selectable custom file instead of deleting authored work.
func (a *app) resetTheme() {
	previous := a.lib.Settings.Theme
	path := filepath.Join(a.lib.Dir, "themes", doctheme.DefaultName+".css")
	backup := ""
	if _, err := os.Stat(path); err == nil {
		for suffix := 1; ; suffix++ {
			backup = filepath.Join(a.lib.Dir, "themes", fmt.Sprintf("yuluo-custom-%d.css", suffix))
			if _, err := os.Stat(backup); os.IsNotExist(err) {
				break
			} else if err != nil {
				a.fail(err)
				return
			}
		}
		if a.fail(os.Rename(path, backup)) {
			return
		}
	} else if !os.IsNotExist(err) {
		a.fail(err)
		return
	}
	a.lib.Settings.Theme = doctheme.DefaultName
	if !a.savePreferences() {
		a.lib.Settings.Theme = previous
		if backup != "" {
			a.fail(os.Rename(backup, path))
		}
		return
	}
	a.docTheme = nil
	a.reloadTheme()
}

func (a *app) reloadTheme() {
	dir := filepath.Join(a.lib.Dir, "themes")
	name := a.lib.Settings.Theme
	if a.docTheme != nil && name == a.themeName && time.Since(a.themeChecked) < time.Second {
		return
	}
	name = doctheme.ResolveName(dir, name)
	a.lib.Settings.Theme = name
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
	t.ScrollbarStyle = documentScrollbar(&t)
	if a.docTheme == nil {
		return &t
	}
	t.Text = a.docTheme.Color(".page", "color", t.Text)
	t.Background = a.docTheme.Color(".page", "background-color", t.Background)
	t.Accent = a.docTheme.LinkColor(base)
	t.Selection = a.docTheme.Color("::selection", "background-color", t.Selection)
	if base.Dark {
		// CSS colors remain authored in light mode; adapt contrast for native dark
		// documents while printing always uses the original stylesheet.
		t.Background = ui.Hex("#202321")
		t.Text = ui.Hex("#e8e8e5")
		t.Border = ui.Hex("#454a46")
		t.Surface = ui.Hex("#2c302c")
	}
	t.ScrollbarStyle = documentScrollbar(&t)
	return &t
}

func (a *app) writingFont(selector string) ui.Font {
	font := ui.Font{Family: typography.Text, Size: 16, LineHeight: 1.6}
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
	names := doctheme.Names(filepath.Join(a.lib.Dir, "themes"))
	if len(names) > 1 {
		previous := a.lib.Settings.Theme
		if ui.Select(c, &a.lib.Settings.Theme, names).Label(a.tr("Writing style")).FillWidth().Changed() {
			if !a.savePreferences() {
				a.lib.Settings.Theme = previous
			}
			a.reloadTheme()
		}
	} else {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, doctheme.DefaultName).Label(a.tr("Writing style")).Value(doctheme.DefaultName).Bold().Grow(1)
			ui.Text(c, a.tr("Built-in theme")).FontSize(c.Theme().FontSize - 1).TextColor(c.Theme().TextMuted)
		})
	}
	ui.Text(c, a.tr("Make it yours with CSS. Edit this style or import your own stylesheet.")).FontSize(c.Theme().FontSize - 1).TextColor(c.Theme().TextMuted)
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, a.tr("Edit CSS")).Clicked() {
			a.cssName = a.lib.Settings.Theme
			a.cssText = a.docTheme.Source
			a.cssOpen = true
			a.settingsOpen = false
		}
		if ui.Button(c, a.tr("Duplicate theme")).Clicked() {
			a.cssName = strings.ToLower(a.lib.Settings.Theme) + "-copy"
			a.cssText = a.docTheme.Source
			a.cssOpen = true
			a.settingsOpen = false
		}
		if ui.Button(c, a.tr("Import CSS…")).Clicked() {
			a.chooseTheme()
		}
	})
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, a.tr("Reset theme")).Clicked() {
			a.resetTheme()
		}
		if ui.Button(c, a.tr("Open themes folder")).Clicked() {
			a.reveal(filepath.Join(a.lib.Dir, "themes"))
		}
	})
	if a.docTheme != nil && len(a.docTheme.Warnings) > 0 {
		ui.Textf(c, a.tr("%d unsupported or invalid declarations"), len(a.docTheme.Warnings)).FontSize(c.Theme().FontSize - 2).TextColor(c.Theme().TextMuted)
	}
}

func (a *app) themeOverlay(c *ui.Context) {
	ui.Modal(c, &a.cssOpen, func() {
		ui.Column(c).Width(720).MaxHeight(700).Gap(12).Children(func() {
			ui.Text(c, a.tr("Document stylesheet")).FontSize(24)
			submit := formSubmitted(ui.TextInput(c, &a.cssName).Label(a.tr("Theme name")).FillWidth())
			a.stylesheetField(c)
			parsed := doctheme.Parse(a.cssText)
			ui.Scroll(c).MaxHeight(100).Children(func() {
				for _, warning := range parsed.Warnings {
					ui.Text(c, warning).FontSize(11)
				}
			})
			ui.Row(c).Gap(10).Children(func() {
				if ui.PrimaryButton(c, a.tr("Apply stylesheet")).Clicked() || submit {
					name := strings.TrimSpace(a.cssName)
					if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
						a.problem = "Use a simple theme name."
					} else if !a.fail(library.AtomicWrite(filepath.Join(a.lib.Dir, "themes", strings.ToLower(name)+".css"), []byte(a.cssText))) {
						a.lib.Settings.Theme = name
						a.savePreferences()
						a.docTheme = nil
						a.reloadTheme()
						a.cssOpen = false
					}
				}
				if ui.Button(c, a.tr("Cancel")).Clicked() {
					a.cssOpen = false
				}
			})
		})
	})
	if !a.cssOpen {
		// A new edit/import session starts with its own selection and undo history.
		a.cssEditor = nil
	}
}

type stylesheetEditor struct {
	state  editor.State
	source string
}

func (a *app) stylesheetField(c *ui.Context) {
	if a.cssEditor == nil {
		buffer := editor.NewBuffer(a.cssText)
		buffer.SetLanguage(editor.LanguageFor("theme.css"))
		a.cssEditor = &stylesheetEditor{
			state:  editor.State{Buffer: buffer, Font: ui.Font{Family: typography.Mono, Size: 13}, SoftWrap: true, NoScrollPastEnd: true},
			source: a.cssText,
		}
	} else if a.cssText != a.cssEditor.source {
		// Imported or replaced drafts must reach the buffer before it is drawn.
		a.cssEditor.state.Buffer.Set(a.cssText)
	}
	editor.View(c.Key("stylesheet-editor"), &a.cssEditor.state).Label(a.tr("Theme CSS")).
		Height(360).FillWidth().Border(1, c.Theme().Border).Radius(8).Clip().AutoFocus()
	// Validation and Apply always consume the same draft shown by the editor.
	a.cssText = a.cssEditor.state.Buffer.String()
	a.cssEditor.source = a.cssText
}

func (a *app) chooseTheme() {
	if a.win == nil {
		return
	}
	options := mygo.OpenDialogOptions{Parent: a.win, Title: a.tr("Import document theme"), Filters: []mygo.FileFilter{{Name: "CSS stylesheet", Extensions: []string{"css"}}}}
	go func() {
		paths, err := openDialog(options)
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
