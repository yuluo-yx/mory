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

// nativeReadyHook is populated only by the e2e build to exercise system exporters.
var nativeReadyHook func(*app)

func main() {
	mygo.App.SetName("Mory")
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
		if _, err = a.lib.Create("Welcome to Mory", welcome, "markdown"); err != nil {
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
	options := mygo.WindowOptions{Title: "Mory", Width: 1280, Height: 820, MinWidth: 1000, MinHeight: 620, StateKey: "mory-main", BackgroundColor: "#fffefa", Content: ui.View(a.view)}
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
				} else if ext := strings.ToLower(filepath.Ext(path)); ext == ".md" || ext == ".xml" || ext == ".markdown" {
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

func (a *app) installMenu() {
	action := func(label, key string, fn func()) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Accelerator: a.keyFor(strings.TrimSuffix(label, "…"), key), Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
			fn()
			if a.win != nil {
				a.win.Update(func() {})
			}
		}}
	}
	formatItems := []*mygo.MenuItem{}
	for _, item := range a.actions() {
		if isTextAction(item.Name) && item.Name != "Undo" && item.Name != "Redo" && item.Name != "Paste" {
			formatItems = append(formatItems, action(item.Name, item.Key, item.Run))
		}
	}
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Mory", Submenu: []*mygo.MenuItem{{Role: mygo.RoleAbout}, updater.MenuItem(), mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(), {Role: mygo.RoleQuit}}},
		{Label: "File", Submenu: []*mygo.MenuItem{action("New note", "Cmd+N", func() { a.createNote("", "Untitled") }), action("Import…", "Cmd+O", a.chooseImport), action("Save", "Cmd+S", func() { a.save() }), action("Open local file", "Cmd+Shift+O", func() { a.chooseLocal(false) }), action("Quick note", "Cmd+Alt+N", a.quickNote), action("Open in new window", "", func() {
			if a.current != nil {
				a.openWindow(a.current)
			}
		}), action("Print", "Cmd+Alt+P", a.printDocument), action("Export…", "Cmd+Shift+E", a.exportMenuOpen), {Role: mygo.RoleClose}}},
		{Label: "Edit", Submenu: []*mygo.MenuItem{{Role: mygo.RoleUndo, Accelerator: a.keyFor("Undo", "Cmd+Z")}, {Role: mygo.RoleRedo, Accelerator: a.keyFor("Redo", "Cmd+Shift+Z")}, mygo.Separator(), {Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste, Accelerator: a.keyFor("Paste", "Cmd+V")}, {Role: mygo.RoleSelectAll}}}, {Label: "Format", Submenu: formatItems},
		{Label: "View", Submenu: []*mygo.MenuItem{action("Library", "Cmd+Shift+F", func() { a.home("All notes") }), action("Focus mode", "Cmd+Shift+L", func() { a.focus = !a.focus }), action("Insert block…", "Cmd+P", func() { a.insertOpen = true }), action("Settings…", "Cmd+,", func() { a.settingsOpen = true }), {Role: mygo.RoleToggleFullScreen}}},
		{Role: mygo.RoleWindowMenu}, {Label: "Help", Submenu: []*mygo.MenuItem{action("Shortcuts", "Cmd+/", func() { a.helpOpen = true })}},
	}))
}
