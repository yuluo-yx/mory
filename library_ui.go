package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/document"
	"github.com/yuluo-yx/mory/internal/library"
)

type noteDrag struct{ ID string }

// Each window has its own edit buffer and library version stamps. Disk reloads
// only replace clean buffers, so a second window cannot silently overwrite edits.
var appWindows []*app

func (a *app) refreshDisk() {
	if time.Since(a.lastDiskCheck) < 2*time.Second {
		return
	}
	a.lastDiskCheck = time.Now()
	a.checkReminders(time.Now())
	if a.current != nil && a.lib.Changed(a.current) {
		if a.dirty {
			_ = a.lib.Recover(a.current)
			return
		}
		if a.fail(a.lib.Reload(a.current.ID)) {
			return
		}
		a.readMetadata()
		a.loadEditors()
		a.status = "Reloaded changes from disk"
	}
	if a.current == nil {
		fresh, err := library.Open(a.lib.Dir)
		if !a.fail(err) {
			a.lib = fresh
		}
	}
}

func (a *app) openWindow(n *library.Note) {
	if !a.save() {
		return
	}
	next, err := newApp(a.lib.Dir)
	if a.fail(err) {
		return
	}
	next.openNote(next.lib.Get(n.ID))
	next.newWindow()
	next.win.Focus()
}

func (a *app) quickNote() {
	for _, w := range appWindows {
		if w.quick && !w.win.IsDestroyed() {
			w.win.Show()
			w.win.Focus()
			return
		}
	}
	next, err := newApp(a.lib.Dir)
	if a.fail(err) {
		return
	}
	next.quick, next.focus, next.outline = true, true, false
	next.createNote("", "Quick note")
	next.newWindow()
	next.win.Focus()
}

func (a *app) selectedNotes() []*library.Note {
	var out []*library.Note
	for _, n := range a.lib.Query(a.section, a.query, a.folder, a.order) {
		if a.selected[n.ID] {
			out = append(out, n)
		}
	}
	return out
}

func (a *app) batch(action string) {
	if !a.save() {
		return
	}
	for _, n := range a.selectedNotes() {
		var err error
		switch action {
		case "Duplicate":
			_, err = a.lib.Duplicate(n)
		case "Trash":
			err = a.lib.TrashTree(n, true)
		case "Restore":
			err = a.lib.TrashTree(n, false)
		case "Move":
			err = a.lib.Move(n, a.moveFolder, a.moveParent)
		}
		if a.fail(err) {
			return
		}
	}
	a.selected = map[string]bool{}
	a.status = "Batch operation completed"
}

func (a *app) batchBar(c *ui.Context, notes []*library.Note) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ghostButton(c, "Select notes").Clicked() {
			a.selecting = !a.selecting
			a.selected = map[string]bool{}
		}
		if a.selecting {
			if ghostButton(c, "Select all").Clicked() {
				for _, n := range notes {
					a.selected[n.ID] = true
				}
			}
			ui.Textf(c, "%d selected", len(a.selectedNotes()))
			if a.section == "Trash" {
				if ghostButton(c, "Restore selected").Clicked() {
					a.batch("Restore")
				}
			} else {
				if ghostButton(c, "Duplicate selected").Clicked() {
					a.batch("Duplicate")
				}
				if ghostButton(c, "Move selected…").Clicked() {
					a.batchMove = true
					a.moveOpen = true
					a.moveParent = ""
					a.moveFolder = ""
					a.target = nil
				}
				if ghostButton(c, "Trash selected").Clicked() {
					a.batch("Trash")
				}
			}
		}
		if a.section == "Trash" && len(notes) > 0 && ghostButton(c, "Empty Trash…").Clicked() {
			a.emptyTrashOpen = true
		}
	})
}

func (a *app) libraryOverlays(c *ui.Context) {
	ui.Modal(c, &a.folderOpen, func() {
		ui.Column(c).Width(380).Gap(12).Children(func() {
			ui.Text(c, "Folder").FontSize(24)
			ui.TextInput(c, &a.folderName).Label("Folder title").FillWidth().AutoFocus()
			if ui.PrimaryButton(c, "Save folder").Clicked() && !a.fail(a.lib.RenameFolder(a.folderFrom, a.folderName)) {
				a.folderOpen = false
			}
			if a.folderFrom != "" && ui.Button(c, "Remove folder, keep notes").Clicked() && !a.fail(a.lib.DeleteFolder(a.folderFrom)) {
				a.folderOpen = false
				a.folder = ""
			}
		})
	})
	ui.Modal(c, &a.emptyTrashOpen, func() {
		ui.Column(c).Width(400).Gap(12).Children(func() {
			ui.Text(c, "Empty Trash permanently?").FontSize(24)
			ui.Text(c, "All trashed library notes will be deleted. Original external files are kept.")
			if ui.Button(c, "Empty Trash").Clicked() && !a.fail(a.lib.EmptyTrash()) {
				a.emptyTrashOpen = false
				a.selected = map[string]bool{}
			}
			if ui.Button(c, "Cancel").Clicked() {
				a.emptyTrashOpen = false
			}
		})
	})
	ui.Modal(c, &a.historyOpen, func() {
		ui.Column(c).Width(560).MaxHeight(600).Gap(12).Children(func() {
			ui.Text(c, "Document history").FontSize(24)
			ui.Scroll(c).MaxHeight(440).Gap(8).Children(func() {
				if len(a.revisions) == 0 {
					ui.Text(c, "No previous saved revisions yet.")
				}
				for _, revision := range a.revisions {
					ui.Row(c.Key(revision.Name)).Gap(12).Children(func() {
						ui.Text(c, revision.Note.Updated.Format("Jan 2, 2006 15:04:05")).Grow(1)
						ui.Textf(c, "%d characters", len([]rune(revision.Note.Source)))
						if ui.Button(c, "Restore").Clicked() && a.current != nil {
							a.current.Source = revision.Note.Source
							a.loadEditors()
							a.markDirty()
							a.save()
							a.historyOpen = false
						}
					})
				}
			})
			if ui.Button(c, "Close").Clicked() {
				a.historyOpen = false
			}
		})
	})
	ui.Modal(c, &a.recoveryOpen, func() {
		ui.Column(c).Width(480).Gap(12).Children(func() {
			ui.Text(c, "Recover unsaved documents").FontSize(24)
			for i, revision := range a.recoveries {
				ui.Row(c.Key(revision.Name)).Gap(8).Children(func() {
					ui.Text(c, revision.Note.Title).Grow(1)
					if ui.Button(c, "Recover as a copy").Clicked() {
						n, err := a.lib.Create(revision.Note.Title+" recovered", revision.Note.Source, revision.Note.Format)
						if !a.fail(err) {
							a.openNote(n)
							a.fail(os.Remove(filepath.Join(a.lib.Dir, "recovery", revision.Name)))
							a.recoveries = slices.Delete(a.recoveries, i, i+1)
							a.recoveryOpen = len(a.recoveries) > 0
						}
					}
				})
			}
			if ui.Button(c, "Later").Clicked() {
				a.recoveryOpen = false
			}
		})
	})
}

func (a *app) chooseLocal(folder bool) {
	if a.win == nil {
		return
	}
	go func() {
		paths, err := openDialog(mygo.OpenDialogOptions{Parent: a.win, Title: "Open local documents", Directory: folder, Multiple: !folder})
		a.win.Update(func() {
			if a.fail(err) {
				return
			}
			for _, path := range paths {
				if folder {
					a.importFolder(path)
				} else {
					a.openExternal(path)
				}
			}
		})
	}()
}

func (a *app) openExternal(path string) {
	if !a.save() {
		return
	}
	b, err := os.ReadFile(path)
	if a.fail(err) {
		return
	}
	format := "markdown"
	if strings.EqualFold(filepath.Ext(path), ".xml") {
		format = "xml"
	}
	if a.fail(document.Validate(string(b), format)) {
		return
	}
	n, err := a.lib.OpenExternal(path)
	if !a.fail(err) {
		a.openNote(n)
		a.status = "Editing original file · " + path
	}
}

func (a *app) importFolder(path string) {
	if _, err := os.Stat(filepath.Join(path, "index.json")); err == nil {
		a.importIndexedLibrary(path)
		return
	}
	count := 0
	err := filepath.WalkDir(path, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if file != path && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(file))
		if ext != ".md" && ext != ".markdown" && ext != ".xml" {
			return nil
		}
		a.importFileFrom(file, path)
		if a.problem != "" {
			return fmt.Errorf("%s", a.problem)
		}
		rel, _ := filepath.Rel(path, filepath.Dir(file))
		if rel == "." {
			rel = ""
		}
		a.current.Folder = filepath.ToSlash(rel)
		if err := a.lib.Save(a.current); err != nil {
			return err
		}
		count++
		return nil
	})
	if !a.fail(err) {
		a.status = fmt.Sprintf("Imported %d documents", count)
	}
}

func (a *app) assetRoot() string {
	if a.current != nil && a.current.External != "" {
		return filepath.Dir(a.current.External)
	}
	return a.lib.Dir
}

func (a *app) reorderNote(from, to string) {
	if from == to || !a.save() {
		return
	}
	notes := a.lib.Query(a.section, a.query, a.folder, "Manual")
	var ids []string
	for _, n := range notes {
		if n.ID == from {
			continue
		}
		if n.ID == to {
			ids = append(ids, from)
		}
		ids = append(ids, n.ID)
	}
	if !a.fail(a.lib.Reorder(ids)) {
		a.order = "Manual"
	}
}
