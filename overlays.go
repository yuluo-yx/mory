package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *app) overlays(c *ui.Context) {
	a.libraryOverlays(c)
	a.themeOverlay(c)
	a.blockOverlays(c)
	ui.Modal(c, &a.pluginsOpen, func() {
		ui.Column(c).Width(480).Gap(14).Children(func() {
			ui.Text(c, a.tr("Your writing workbench")).FontSize(22).Bold().LetterSpacing(-0.3)
			ui.Text(c, a.tr("JavaScript transforms run locally in moejs.")).TextColor(c.Theme().TextMuted)
			for _, p := range a.plugins {
				ui.Row(c.Key(p.Name)).Gap(10).AlignItems(ui.Center).Children(func() {
					enabled := !a.lib.Settings.DisabledPlugins[p.Name]
					if ui.Checkbox(c, &enabled, a.tr(p.Name)).Grow(1).Changed() {
						a.lib.Settings.DisabledPlugins[p.Name] = !enabled
						a.savePreferences()
					}
					if ui.Button(c, a.tr("Run")).Label(a.tr("Run ") + p.Name).Disabled(!enabled || a.current == nil).Clicked() {
						a.runPlugin(p)
						if a.problem == "" {
							a.pluginsOpen = false
						}
					}
				})
			}
			ui.Text(c, a.tr("Add .js files exporting transform({ title, markdown }). Return { markdown } or { message }.")).FontSize(12)
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, a.tr("Open plugin folder")).Clicked() {
					a.reveal(a.lib.Dir + "/plugins")
				}
				if ui.Button(c, a.tr("Reload plugins")).Clicked() {
					a.reloadPlugins()
				}
				if ui.PrimaryButton(c, a.tr("Done")).Clicked() {
					a.pluginsOpen = false
				}
			})
		})
	})
	a.insertMenu(c)
	if !a.insertOpen {
		a.slashPending = false
	}
	ui.Modal(c, &a.linkOpen, func() {
		ui.Column(c).Width(380).Gap(14).Children(func() {
			ui.Text(c, a.tr("Add a link")).FontSize(22)
			ui.TextInput(c, &a.linkURL).Label(a.tr("Link URL")).Placeholder("https://example.com").FillWidth().AutoFocus()
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, a.tr("Apply link")).Clicked() && a.doc != nil {
					a.doc.SetLink(strings.TrimSpace(a.linkURL))
					a.linkOpen = false
					a.capture()
				}
				if ui.Button(c, a.tr("Remove link")).Clicked() && a.doc != nil {
					a.doc.SetLink("")
					a.linkOpen = false
					a.capture()
				}
			})
		})
	})
	ui.Modal(c, &a.renameOpen, func() {
		if a.target == nil {
			return
		}
		t := c.Theme()
		ui.Column(c).Width(440).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, a.tr("Document details")).FontSize(20).Bold().LetterSpacing(-0.3)
					ui.Text(c, a.tr("Configure note title, layout and metadata")).FontSize(12).TextColor(t.TextMuted)
				})
			})

			// Note Name Input Card
			ui.Column(c).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border.Alpha(0.6)).Gap(6).Children(func() {
				ui.Text(c, a.tr("Note name")).FontSize(12).Bold().TextColor(t.TextMuted)
				ui.TextInput(c, &a.renameTitle).Label(a.tr("Note name")).FillWidth().AutoFocus()
			})

			a.documentControls(c)

			// Actions Footer
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				if ui.Button(c, a.tr("Favorite")).Padding(5, 10).Radius(6).Clicked() {
					a.mutate(a.target, func() { a.target.Starred = !a.target.Starred })
					a.renameOpen = false
				}
				if ui.Button(c, a.tr("Pin")).Padding(5, 10).Radius(6).Clicked() {
					a.mutate(a.target, func() { a.target.Pinned = !a.target.Pinned })
					a.renameOpen = false
				}
				if ui.Button(c, a.tr("Duplicate")).Padding(5, 10).Radius(6).Clicked() {
					a.duplicate(a.target)
					a.renameOpen = false
				}
				if ui.Button(c, a.tr("Move…")).Padding(5, 10).Radius(6).Clicked() {
					a.moveFolder = a.target.Folder
					a.moveParent = a.target.Parent
					a.renameOpen = false
					a.moveOpen = true
				}
			})

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, a.tr("Move to Trash")).Padding(5, 10).Radius(6).TextColor(ui.Hex("#ff3b30")).Clicked() {
					a.trash(a.target)
					a.renameOpen = false
				}
				ui.Box(c).Grow(1)
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					if ui.Button(c, a.tr("Cancel")).Padding(5, 14).Radius(6).Clicked() {
						a.renameOpen = false
					}
					if ui.PrimaryButton(c, a.tr("Rename")).Padding(5, 16).Radius(6).Clicked() {
						if strings.TrimSpace(a.renameTitle) == "" {
							a.problem = "Give your note a name."
						} else {
							a.mutate(a.target, func() {
								a.target.Title = strings.TrimSpace(a.renameTitle)
								if a.target == a.current {
									a.syncMetadata()
								}
							})
							a.renameOpen = false
						}
					}
				})
			})
		})
	})
	ui.Modal(c, &a.moveOpen, func() {
		ui.Column(c).Width(400).Gap(14).Children(func() {
			ui.Text(c, a.tr("Move document")).FontSize(24)
			ui.Text(c, a.tr("Folder (leave empty for the library root)"))
			ui.TextInput(c, &a.moveFolder).Label(a.tr("Folder name")).FillWidth()
			ui.Text(c, a.tr("Parent document"))
			choices := []string{a.tr("None")}
			ids := map[string]string{"None": ""}
			selected := a.tr("None")
			for _, n := range a.lib.Notes {
				if n.Deleted || n == a.target {
					continue
				}
				label := n.Title + " · " + n.ID[:6]
				choices = append(choices, label)
				ids[label] = n.ID
				if n.ID == a.moveParent {
					selected = label
				}
			}
			if ui.Select(c, &selected, choices).Label(a.tr("Parent document")).FillWidth().Changed() {
				a.moveParent = ids[selected]
			}
			if ui.PrimaryButton(c, a.tr("Move")).Clicked() {
				if a.batchMove {
					a.batch("Move")
					a.batchMove = false
					a.moveOpen = false
				} else if a.target != nil && a.canParent(a.target.ID, a.moveParent) {
					a.mutate(a.target, func() { a.target.Folder = strings.TrimSpace(a.moveFolder); a.target.Parent = a.moveParent })
					a.moveOpen = false
				} else {
					a.problem = "A document cannot be moved inside its own descendants."
				}
			}
		})
	})
	ui.Modal(c, &a.deleteOpen, func() {
		if a.target == nil {
			return
		}
		ui.Column(c).Width(400).Gap(16).Children(func() {
			ui.Text(c, a.tr("In the Trash")).FontSize(24)
			ui.Text(c, a.target.Title)
			ui.Text(c, a.tr("Restore this note, or permanently delete it. Permanent deletion cannot be undone."))
			ui.Row(c).Gap(10).Children(func() {
				if ui.PrimaryButton(c, a.tr("Restore note")).Clicked() {
					a.fail(a.lib.TrashTree(a.target, false))
					a.deleteOpen = false
				}
				if ui.Button(c, a.tr("Delete permanently")).Clicked() {
					a.fail(a.lib.Purge(a.target))
					a.deleteOpen = false
				}
				if ui.Button(c, a.tr("Cancel")).Clicked() {
					a.deleteOpen = false
				}
			})
		})
	})
	ui.Modal(c, &a.exportOpen, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, a.tr("Take your words with you")).FontSize(22).Bold().LetterSpacing(-0.3)
			for _, format := range []string{"Markdown", "DocxXML", "HTML", "PDF"} {
				if ui.Button(c, a.tr("Export ")+format).FillWidth().Clicked() {
					a.exportOpen = false
					a.chooseExport(format)
				}
			}
		})
	})
	showError := a.problem != ""
	ui.Modal(c, &showError, func() {
		ui.Column(c).Width(440).Gap(16).Children(func() {
			ui.Text(c, a.tr("Something needs your attention")).FontSize(21).Bold()
			ui.Text(c, a.localizedProblem(a.problem))
			if a.dirty {
				ui.Text(c, a.tr("Your edits are still in memory. Export a copy before closing.")).TextColor(c.Theme().TextMuted)
				if ui.Button(c, a.tr("Export a copy")).Clicked() {
					a.problem = ""
					a.chooseExport("Markdown")
				}
			}
			if ui.PrimaryButton(c, a.tr("Dismiss")).Clicked() {
				a.problem = ""
				showError = false
			}
		})
	})
	if !showError {
		a.problem = ""
	}
}

func (a *app) canParent(id, parent string) bool {
	seen := map[string]bool{id: true}
	for parent != "" {
		if seen[parent] {
			return false
		}
		seen[parent] = true
		n := a.lib.Get(parent)
		if n == nil {
			return false
		}
		parent = n.Parent
	}
	return true
}
func (a *app) exportMenuOpen() {
	if a.current != nil {
		a.exportOpen = true
	}
}
