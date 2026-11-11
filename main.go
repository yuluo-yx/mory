// Mory is a macOS writing app rendered by MyGo's native GPU UI.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

// nativeReadyHook is populated only by E2E and native performance builds.
var nativeReadyHook func(*app)

func main() {
	mygo.App.SetName("Mory")
	mygo.App.SetLanguages("en", "zh-Hans")
	mygo.Use(native.New(updater.Options{}))
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	dir := os.Getenv("MORY_LIBRARY")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		dir = filepath.Join(home, "Documents", "Mory")
	}
	a, err := newApp(dir)
	if err != nil {
		log.Fatal(err)
	}
	if len(a.lib.Notes) == 0 {
		if _, err = a.welcomeNote(); err != nil {
			log.Fatal(err)
		}
	}
	mygo.App.OnOpenFile(func(path string) {
		a.openExternal(path)
		if a.win != nil {
			if a.win.IsDestroyed() {
				a.newWindow()
			}
			a.win.Update(func() {})
			a.win.Show()
			a.win.Focus()
		}
	})
	mygo.App.WhenReady(func() {
		a.applyLocale()
		a.newWindow()
		a.installMenu()
		a.registerQuickNote()
		if nativeReadyHook != nil {
			nativeReadyHook(a)
		}
	})
	mygo.App.OnActivate(func(visible bool) {
		if !visible && a.win != nil {
			if a.win.IsDestroyed() {
				a.newWindow()
			}
			a.win.Show()
			a.win.Focus()
		}
	})
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		for _, a := range appWindows {
			if !a.save() {
				e.PreventDefault()
				if a.win != nil {
					a.win.Update(func() {})
				}
			}
		}
	})
	if err = mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (a *app) newWindow() {
	options := mygo.WindowOptions{
		Title:          "Mory",
		Width:          1280,
		Height:         820,
		MinWidth:       1000,
		MinHeight:      620,
		StateKey:       "mory-main",
		TitleBarStyle:  mygo.TitleBarHiddenInset,
		TitleBarHeight: 48,
		Vibrancy:       mygo.VibrancySidebar,
		Content:        ui.View(a.view),
	}
	if a.quick {
		options.Title = "Mory Quick Note"
		options.Width = 800
		options.Height = 620
		options.MinWidth = 600
		options.MinHeight = 400
		options.StateKey = "mory-quick"
		options.Panel = true
	}
	a.win = mygo.NewWindow(options)
	appWindows = append(appWindows, a)
	closed := make(chan struct{})
	a.win.OnClosed(func() { close(closed) })
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-closed:
				return
			case <-ticker.C:
				mygo.RunOnMain(func() {
					if a.win.IsDestroyed() {
						return
					}
					if a.dirty || time.Since(a.lastDiskCheck) >= 2*time.Second {
						a.backgroundTick()
						a.win.Invalidate()
					}
				})
			}
		}
	}()
	a.win.OnFocus(func() { a.installMenu(); a.lastDiskCheck = time.Time{}; a.refreshDisk() })
	a.win.OnFileDrop(func(e *mygo.FileDropEvent) {
		a.win.Update(func() {
			for _, path := range e.Paths {
				info, err := os.Stat(path)
				if a.fail(err) {
					continue
				}
				if info.IsDir() {
					a.importFolder(path)
				} else if ext := strings.ToLower(filepath.Ext(path)); ext == ".md" || ext == ".xml" || ext == ".markdown" || ext == ".mdx" {
					a.importFile(path)
				} else if a.current != nil {
					a.attachFile(path)
				}
			}
		})
	})
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.save() {
			e.PreventDefault()
			a.win.Update(func() {})
		}
	})
}

func (a *app) installMenu() { mygo.App.SetMenu(a.buildMenu(updater.MenuItem())) }

// Native menus and shortcut settings share stable action names.
func (a *app) buildMenu(updateItem *mygo.MenuItem) *mygo.Menu {
	actions := map[string]shortcut{}
	for _, item := range a.actions() {
		actions[item.Name] = item
	}
	action := func(name string) *mygo.MenuItem {
		item := actions[name]
		label := name
		if name == "Import" || name == "Export" || name == "Settings" {
			label += "…"
		}
		return &mygo.MenuItem{Label: a.tr(label), Accelerator: a.keyFor(name, item.Key), Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
			if item.Run != nil {
				item.Run()
			}
			if a.win != nil {
				a.win.Update(func() {})
			}
		}}
	}
	roles := map[*mygo.MenuItem]string{}
	role := func(name string, r mygo.MenuRole) *mygo.MenuItem {
		item := &mygo.MenuItem{Role: r, Accelerator: a.keyFor(name, actions[name].Key)}
		roles[item] = item.Accelerator
		return item
	}
	formats := []*mygo.MenuItem{}
	for _, item := range a.actions() {
		if isTextAction(item.Name) && item.Name != "Undo" && item.Name != "Redo" && item.Name != "Paste" {
			formats = append(formats, action(item.Name))
		}
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Mory", Submenu: []*mygo.MenuItem{{Role: mygo.RoleAbout}, updateItem, mygo.Separator(), action("Settings"), mygo.Separator(), role("Hide", mygo.RoleHide), role("Hide others", mygo.RoleHideOthers), {Role: mygo.RoleUnhide}, mygo.Separator(), role("Quit", mygo.RoleQuit)}},
		{Label: a.tr("File"), Submenu: []*mygo.MenuItem{action("New note"), action("Import"), action("Open local file"), action("Save"), action("Quick note"), action("Open in new window"), action("Print"), action("Export"), role("Close window", mygo.RoleClose)}},
		{Label: a.tr("Edit"), Submenu: []*mygo.MenuItem{role("Undo", mygo.RoleUndo), role("Redo", mygo.RoleRedo), mygo.Separator(), role("Cut", mygo.RoleCut), role("Copy", mygo.RoleCopy), role("Paste", mygo.RolePaste), role("Select all", mygo.RoleSelectAll), mygo.Separator(), action("Find"), action("Replace")}},
		{Label: a.tr("Format"), Submenu: formats},
		{Label: a.tr("View"), Submenu: []*mygo.MenuItem{action("Library"), action("Focus mode"), action("Formatting toolbar"), action("Outline"), action("Outline on Left"), action("Outline on Right"), action("Write mode"), action("Read mode"), action("Source mode"), action("Split mode"), action("Insert block"), role("Full screen", mygo.RoleToggleFullScreen)}},
		{Role: mygo.RoleWindowMenu}, {Label: a.tr("Help"), Submenu: []*mygo.MenuItem{action("Open welcome guide"), action("Shortcuts")}},
	})
	// MyGo fills role defaults during construction; explicitly clear disabled keys afterwards.
	for item, key := range roles {
		item.Accelerator = key
	}
	return menu
}
