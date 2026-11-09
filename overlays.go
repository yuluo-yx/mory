package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func (a *app) overlays(c *ui.Context) {
	a.libraryOverlays(c)
	a.themeOverlay(c)
	a.blockOverlays(c)
	if a.focus {
		ui.Modal(c, &a.settingsOpen, func() {
			a.settingsContent(c)
		})
	}
	ui.Modal(c, &a.pluginsOpen, func() {
		ui.Column(c).Width(480).Gap(14).Children(func() {
			ui.Text(c, "Your writing workbench").FontSize(22).Bold().LetterSpacing(-0.3)
			ui.Text(c, "JavaScript transforms run locally in moejs.").TextColor(c.Theme().TextMuted)
			for _, p := range a.plugins {
				ui.Row(c.Key(p.Name)).Gap(10).AlignItems(ui.Center).Children(func() {
					enabled := !a.lib.Settings.DisabledPlugins[p.Name]
					if ui.Checkbox(c, &enabled, p.Name).Grow(1).Changed() {
						a.lib.Settings.DisabledPlugins[p.Name] = !enabled
						a.fail(a.lib.SaveSettings())
					}
					if ui.Button(c, "Run").Label("Run " + p.Name).Disabled(!enabled || a.current == nil).Clicked() {
						a.runPlugin(p)
						if a.problem == "" {
							a.pluginsOpen = false
						}
					}
				})
			}
			ui.Text(c, "Add .js files exporting transform({ title, markdown }). Return { markdown } or { message }.").FontSize(12)
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Open plugin folder").Clicked() {
					a.reveal(a.lib.Dir + "/plugins")
				}
				if ui.Button(c, "Reload plugins").Clicked() {
					a.reloadPlugins()
				}
				if ui.PrimaryButton(c, "Done").Clicked() {
					a.pluginsOpen = false
				}
			})
		})
	})
	ui.Modal(c, &a.insertOpen, func() {
		ui.Column(c).Width(400).Gap(12).Children(func() {
			ui.Text(c, "Give your idea a shape").FontSize(22).Bold().LetterSpacing(-0.3)
			ui.TextInput(c, &a.insertQuery).Label("Search blocks").Placeholder("Search blocks…").FillWidth().AutoFocus()
			for _, b := range []struct{ name, source string }{{"Heading", "\n## Heading\n"}, {"Bullet list", "\n- List item\n"}, {"To-do list", "\n- [ ] A small next step\n"}, {"Quote", "\n> A thought worth keeping.\n"}, {"Code block", "\n```go\nfunc main() {}\n```\n"}, {"Table", "\n| Column A | Column B |\n| --- | --- |\n| Value | Value |\n"}, {"Math", "\n$$\nE = mc^2\n$$\n"}, {"Callout", "\n> [!NOTE]\n> Something to remember.\n"}, {"Divider", "\n---\n"}} {
				if strings.Contains(strings.ToLower(b.name), strings.ToLower(a.insertQuery)) {
					if ghostButton(c, b.name).Label("Insert " + b.name).FillWidth().TextAlign(ui.Start).Disabled(a.current == nil).Clicked() {
						a.insert(b.source)
					}
				}
			}
			if ghostButton(c, "Image or attachment…").FillWidth().TextAlign(ui.Start).Disabled(a.current == nil).Clicked() {
				a.insertOpen = false
				a.chooseAttachment()
			}
			if ghostButton(c, "Columns").Label("Insert Columns").FillWidth().TextAlign(ui.Start).Disabled(a.current == nil).Clicked() {
				a.insertNodes(blocks.NewColumns(2))
			}
			if ghostButton(c, "Countdown").Label("Insert Countdown").FillWidth().TextAlign(ui.Start).Disabled(a.current == nil).Clicked() {
				n := blocks.New("countdown")
				n.Set("duration", "300")
				a.insertNodes(n)
			}
			for _, item := range []struct{ name, kind string }{{"Inline equation", "latex"}, {"Reminder", "time"}, {"Button", "button"}, {"Document link", "doc-link"}} {
				if ghostButton(c, item.name).Label("Insert " + item.name).FillWidth().Disabled(a.current == nil).Clicked() {
					a.insertInline(item.kind)
				}
			}
			if ghostButton(c, "Child pages").Label("Insert Child pages").FillWidth().Disabled(a.current == nil).Clicked() {
				a.insertNodes(blocks.New("sub-page-list"))
			}
		})
	})
	if !a.insertOpen {
		a.slashPending = false
	}
	ui.Modal(c, &a.linkOpen, func() {
		ui.Column(c).Width(380).Gap(14).Children(func() {
			ui.Text(c, "Add a link").FontSize(22)
			ui.TextInput(c, &a.linkURL).Label("Link URL").Placeholder("https://example.com").FillWidth().AutoFocus()
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "Apply link").Clicked() && a.doc != nil {
					a.doc.SetLink(strings.TrimSpace(a.linkURL))
					a.linkOpen = false
					a.capture()
				}
				if ui.Button(c, "Remove link").Clicked() && a.doc != nil {
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
		ui.Column(c).Width(400).Gap(16).Children(func() {
			ui.Text(c, "Document details").FontSize(22).Bold().LetterSpacing(-0.3)
			ui.TextInput(c, &a.renameTitle).Label("Note name").FillWidth().AutoFocus()
			a.documentControls(c)
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "Rename").Clicked() {
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
				if ui.Button(c, "Move…").Clicked() {
					a.moveFolder = a.target.Folder
					a.moveParent = a.target.Parent
					a.renameOpen = false
					a.moveOpen = true
				}
				if ui.Button(c, "Duplicate").Clicked() {
					a.duplicate(a.target)
					a.renameOpen = false
				}
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Favorite").Clicked() {
					a.mutate(a.target, func() { a.target.Starred = !a.target.Starred })
					a.renameOpen = false
				}
				if ui.Button(c, "Pin").Clicked() {
					a.mutate(a.target, func() { a.target.Pinned = !a.target.Pinned })
					a.renameOpen = false
				}
				if ui.Button(c, "Move to Trash").Clicked() {
					a.trash(a.target)
					a.renameOpen = false
				}
			})
		})
	})
	ui.Modal(c, &a.moveOpen, func() {
		ui.Column(c).Width(400).Gap(14).Children(func() {
			ui.Text(c, "Move document").FontSize(24)
			ui.Text(c, "Folder (leave empty for the library root)")
			ui.TextInput(c, &a.moveFolder).Label("Folder name").FillWidth()
			ui.Text(c, "Parent document")
			choices := []string{"None"}
			ids := map[string]string{"None": ""}
			selected := "None"
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
			if ui.Select(c, &selected, choices).Label("Parent document").FillWidth().Changed() {
				a.moveParent = ids[selected]
			}
			if ui.PrimaryButton(c, "Move").Clicked() {
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
			ui.Text(c, "In the Trash").FontSize(24)
			ui.Text(c, a.target.Title)
			ui.Text(c, "Restore this note, or permanently delete it. Permanent deletion cannot be undone.")
			ui.Row(c).Gap(10).Children(func() {
				if ui.PrimaryButton(c, "Restore note").Clicked() {
					a.fail(a.lib.TrashTree(a.target, false))
					a.deleteOpen = false
				}
				if ui.Button(c, "Delete permanently").Clicked() {
					a.fail(a.lib.Purge(a.target))
					a.deleteOpen = false
				}
				if ui.Button(c, "Cancel").Clicked() {
					a.deleteOpen = false
				}
			})
		})
	})
	ui.Modal(c, &a.exportOpen, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, "Take your words with you").FontSize(22).Bold().LetterSpacing(-0.3)
			for _, format := range []string{"Markdown", "DocxXML", "HTML", "PDF"} {
				if ui.Button(c, "Export "+format).FillWidth().Clicked() {
					a.exportOpen = false
					a.chooseExport(format)
				}
			}
		})
	})
	ui.Modal(c, &a.helpOpen, func() { a.shortcutSettings(c) })
	showError := a.problem != ""
	ui.Modal(c, &showError, func() {
		ui.Column(c).Width(440).Gap(16).Children(func() {
			ui.Text(c, "Something needs your attention").FontSize(21).Bold()
			ui.Text(c, a.problem)
			if a.dirty {
				ui.Text(c, "Your edits are still in memory. Export a copy before closing.").TextColor(c.Theme().TextMuted)
				if ui.Button(c, "Export a copy").Clicked() {
					a.problem = ""
					a.chooseExport("Markdown")
				}
			}
			if ui.PrimaryButton(c, "Dismiss").Clicked() {
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

func (a *app) settingsContent(c *ui.Context) {
		ui.Column(c).Width(460).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Settings").FontSize(22).Bold().LetterSpacing(-0.3)
					ui.Text(c, "Preferences for writing and appearance").FontSize(12).TextColor(c.Theme().TextMuted)
				})
			})

			// Appearance & Style Section
			ui.Column(c).Padding(12).Radius(8).Background(c.Theme().Surface).Gap(10).Border(1, c.Theme().Border.Alpha(0.6)).Children(func() {
				ui.Text(c, "Appearance & Style").Bold().FontSize(13)
				if ui.Select(c, &a.lib.Settings.Appearance, []string{"System", "Light", "Dark"}).Label("Appearance").FillWidth().Changed() {
					a.fail(a.lib.SaveSettings())
				}
				a.themeControls(c)
			})

			// Editor & Typography Section
			ui.Column(c).Padding(12).Radius(8).Background(c.Theme().Surface).Gap(10).Border(1, c.Theme().Border.Alpha(0.6)).Children(func() {
				ui.Text(c, "Editor Preferences").Bold().FontSize(13)
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Textf(c, "Text size: %.0f", a.lib.Settings.FontSize).Grow(1).FontSize(13)
					if ui.Button(c, "−").Label("Smaller text").Padding(4, 10).Radius(5).Clicked() {
						a.lib.Settings.FontSize = max(12, a.lib.Settings.FontSize-1)
						a.docTheme = nil
						a.fail(a.lib.SaveSettings())
					}
					if ui.Button(c, "+").Label("Larger text").Padding(4, 10).Radius(5).Clicked() {
						a.lib.Settings.FontSize = min(30, a.lib.Settings.FontSize+1)
						a.docTheme = nil
						a.fail(a.lib.SaveSettings())
					}
				})
				ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
					if ui.Checkbox(c, &a.showToolbar, "Show formatting toolbar").Changed() {}
					if ui.Checkbox(c, &a.outline, "Show outline").Changed() {}
				})
				if a.current != nil {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Page width").FontSize(13)
						width := a.current.Width
						if width == "" {
							width = "Standard"
						}
						if ui.Select(c, &width, []string{"Standard", "Wide", "Full"}).Label("Page width").Width(120).Changed() {
							a.mutate(a.current, func() { a.current.Width = width })
						}
					})
				}
			})

			// Shortcuts & Keybindings Section
			ui.Column(c).Padding(12).Radius(8).Background(c.Theme().Surface).Gap(10).Border(1, c.Theme().Border.Alpha(0.6)).Children(func() {
				ui.Text(c, "Shortcuts & Keybindings").Bold().FontSize(13)
				if ui.Checkbox(c, &a.lib.Settings.ShowShortcuts, "Show shortcut hints beside buttons").Changed() {
					a.fail(a.lib.SaveSettings())
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					if ui.Button(c, "Keyboard shortcuts").Padding(5, 10).Radius(5).Clicked() {
						a.helpOpen = true
						a.settingsOpen = false
					}
					ui.Text(c, "Custom keys (Cmd, Ctrl, Alt, Shift)").FontSize(11).TextColor(c.Theme().TextMuted)
				})
			})

			// Library & Storage Section
			ui.Column(c).Padding(12).Radius(8).Background(c.Theme().Surface).Gap(10).Border(1, c.Theme().Border.Alpha(0.6)).Children(func() {
				ui.Text(c, "Library & Storage").Bold().FontSize(13)
				ui.Text(c, a.lib.Dir).FontSize(11).TextColor(c.Theme().TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, "Reveal library in Finder").Padding(5, 10).Radius(5).Clicked() {
						a.reveal(a.lib.Dir)
					}
				})
			})

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Mory 0.1.0 · Made for macOS").FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
				if ui.PrimaryButton(c, "Done").Padding(6, 16).Radius(6).Clicked() {
					a.settingsOpen = false
				}
			})
		})
}
