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
			a.capture()
			a.flushSource()
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
	next.createNote("", "")
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
			_, err = a.duplicateWithAssets(n)
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

// Ending selection discards only UI state; batch actions run when explicitly chosen.
func (a *app) endSelection() {
	a.selecting = false
	a.selected = map[string]bool{}
	if a.batchMove {
		a.batchMove, a.moveOpen = false, false
		a.moveFolder, a.moveParent = "", ""
	}
}

func (a *app) batchBar(c *ui.Context, notes []*library.Note) {
	t := c.Theme()
	if !a.selecting && (a.section != "Trash" || len(notes) == 0) {
		return
	}

	ui.Row(c).FillWidth().Wrap().Padding(6, 14).Radius(8).Background(t.Surface).Border(1, t.Border.Alpha(0.7)).Gap(10).AlignItems(ui.Center).Children(func() {
		if a.selecting {
			selectedCount := 0
			for _, n := range notes {
				if a.selected[n.ID] {
					selectedCount++
				}
			}
			allSelected := len(notes) > 0 && selectedCount == len(notes)
			selectLabel := "Select all"
			if allSelected {
				selectLabel = "Deselect all"
			}
			if ghostButton(c, a.tr(selectLabel)).Padding(4, 8).Radius(5).Disabled(len(notes) == 0).Clicked() {
				for _, n := range notes {
					if allSelected {
						delete(a.selected, n.ID)
					} else {
						a.selected[n.ID] = true
					}
				}
				selectedCount = len(notes)
				if allSelected {
					selectedCount = 0
				}
			}
			ui.Textf(c, a.tr("%d selected"), selectedCount).FontSize(12).Bold().TextColor(t.Accent)
			ui.Box(c).Height(14).Width(1).Background(t.Border)
			if a.section == "Trash" {
				if ghostButton(c, a.tr("Restore selected")).Padding(4, 8).Radius(5).Disabled(selectedCount == 0).Clicked() {
					a.batch("Restore")
				}
			} else {
				if ghostButton(c, a.tr("Duplicate selected")).Padding(4, 8).Radius(5).Disabled(selectedCount == 0).Clicked() {
					a.batch("Duplicate")
				}
				if ghostButton(c, a.tr("Move selected…")).Padding(4, 8).Radius(5).Disabled(selectedCount == 0).Clicked() {
					a.batchMove = true
					a.moveOpen = true
					a.moveParent = ""
					a.moveFolder = ""
					a.target = nil
				}
				if ghostButton(c, a.tr("Trash selected")).Padding(4, 8).Radius(5).Disabled(selectedCount == 0).Clicked() {
					a.batch("Trash")
				}
			}
			ui.Box(c).Grow(1)
			ui.Row(c).Gap(6).Shrink(0).Children(func() {
				cancel := ghostButton(c, "").Label(a.tr("Cancel selection")).Tooltip(a.tr("Clear selection and exit management")).Padding(4, 10).Radius(5)
				cancel.Children(func() { ui.Text(c, a.tr("Cancel")).Label(a.tr("Cancel selection")).SingleLine() })
				if cancel.Clicked() {
					a.endSelection()
				}
				if ghostButton(c, a.tr("Done")).Padding(4, 10).Radius(5).Background(t.Background).Clicked() {
					a.endSelection()
				}
			})
		}
		if a.section == "Trash" && len(notes) > 0 {
			ui.Box(c).Grow(1)
			if ghostButton(c, a.tr("Empty Trash…")).Padding(4, 8).Radius(5).TextColor(ui.Hex("#ff3b30")).Clicked() {
				a.emptyTrashOpen = true
			}
		}
	})
}

func (a *app) libraryOverlays(c *ui.Context) {
	ui.Modal(c, &a.folderOpen, func() {
		t := c.Theme()
		titleText := a.tr("New folder")
		if a.folderFrom != "" {
			titleText = a.tr("Edit folder…")
		}
		ui.Column(c).Width(380).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, titleText).FontSize(18).Bold().LetterSpacing(-0.3)
					ui.Text(c, a.tr("Organize notes into a workspace folder")).FontSize(12).TextColor(t.TextMuted)
				})
			})

			ui.Column(c).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border.Alpha(0.6)).Gap(6).Children(func() {
				ui.Text(c, a.tr("Folder title")).FontSize(12).Bold().TextColor(t.TextMuted)
				ui.TextInput(c, &a.folderName).Label(a.tr("Folder title")).Placeholder(a.tr("e.g. Work, Ideas, Journal…")).FillWidth().AutoFocus()
			})

			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				if a.folderFrom != "" {
					if ui.Button(c, a.tr("Remove folder, keep notes")).Padding(5, 10).Radius(6).TextColor(ui.Hex("#ff3b30")).Clicked() && !a.fail(a.lib.DeleteFolder(a.folderFrom)) {
						a.folderOpen = false
						a.folder = ""
					}
				}
				ui.Box(c).Grow(1)
				if ui.Button(c, a.tr("Cancel")).Padding(5, 12).Radius(6).Clicked() {
					a.folderOpen = false
				}
				if ui.PrimaryButton(c, a.tr("Save folder")).Padding(5, 14).Radius(6).Clicked() && !a.fail(a.lib.RenameFolder(a.folderFrom, a.folderName)) {
					a.folderOpen = false
				}
			})
		})
	})
	ui.Modal(c, &a.emptyTrashOpen, func() {
		ui.Column(c).Width(400).Gap(12).Children(func() {
			ui.Text(c, a.tr("Empty Trash permanently?")).FontSize(24)
			ui.Text(c, a.tr("All trashed library notes will be deleted. Original external files are kept."))
			if ui.Button(c, a.tr("Empty Trash")).Clicked() && !a.fail(a.lib.EmptyTrash()) {
				a.emptyTrashOpen = false
				a.selected = map[string]bool{}
			}
			if ui.Button(c, a.tr("Cancel")).Clicked() {
				a.emptyTrashOpen = false
			}
		})
	})
	ui.Modal(c, &a.historyOpen, func() {
		ui.Column(c).Width(560).MaxHeight(600).Gap(12).Children(func() {
			ui.Text(c, a.tr("Document history")).FontSize(24)
			policy := a.lib.HistoryPolicy()
			summary := a.trf("Every %d min · %d days · Up to %d versions / %d MiB per document. The latest version is always kept.", policy.IntervalMinutes, policy.RetentionDays, policy.MaxVersions, policy.MaxMiB)
			if policy.Disabled {
				summary = a.tr("Automatic history is off. Existing versions and checkpoints made before restoring are kept.")
			}
			ui.Text(c, summary).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
			ui.Scroll(c).MaxHeight(440).Gap(8).Children(func() {
				if len(a.revisions) == 0 {
					ui.Text(c, a.tr("No previous saved revisions yet."))
				}
				for _, revision := range a.revisions {
					ui.Row(c.Key(revision.Name)).Gap(12).Children(func() {
						ui.Text(c, a.dateText(revision.Note.Updated, true)).Grow(1)
						ui.Textf(c, a.tr("%d characters"), len([]rune(revision.Note.Source)))
						if ui.Button(c, a.tr("Restore")).Clicked() && a.current != nil {
							// Flush pending edits before taking the pre-restore checkpoint.
							if !a.save() {
								return
							}
							a.setCurrentSource(revision.Note.Source)
							a.loadEditors()
							a.markDirty()
							if a.saveWithHistory(true) {
								a.historyOpen = false
							}
						}
					})
				}
			})
			if ui.Button(c, a.tr("Close")).Clicked() {
				a.historyOpen = false
			}
		})
	})
	ui.Modal(c, &a.recoveryOpen, func() {
		ui.Column(c).Width(480).Gap(12).Children(func() {
			ui.Text(c, a.tr("Recover unsaved documents")).FontSize(24)
			for i, revision := range a.recoveries {
				ui.Row(c.Key(revision.Name)).Gap(8).Children(func() {
					ui.Text(c, a.displayTitle(revision.Note.Title)).Grow(1)
					if ui.Button(c, a.tr("Recover as a copy")).Clicked() {
						title := revision.Note.Title
						if strings.TrimSpace(title) != "" {
							title += " recovered"
						}
						n, err := a.lib.Create(title, revision.Note.Source, revision.Note.Format)
						if !a.fail(err) {
							a.openNote(n)
							a.fail(os.Remove(filepath.Join(a.lib.Dir, "recovery", revision.Name)))
							a.recoveries = slices.Delete(a.recoveries, i, i+1)
							a.recoveryOpen = len(a.recoveries) > 0
						}
					}
				})
			}
			if ui.Button(c, a.tr("Later")).Clicked() {
				a.recoveryOpen = false
			}
		})
	})
}

func (a *app) chooseLocal(folder bool) {
	if a.win == nil {
		return
	}
	options := mygo.OpenDialogOptions{Parent: a.win, Title: a.tr("Open local documents"), Directory: folder, Multiple: !folder}
	go func() {
		paths, err := openDialog(options)
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
		if ext != ".md" && ext != ".markdown" && ext != ".mdx" && ext != ".xml" {
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
