package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
	"github.com/yuluo-yx/mory/internal/library"
)

func (a *app) theme(c *ui.Context) {
	dark := c.Theme().Dark
	if a.lib.Settings.Appearance == "Dark" {
		dark = true
	} else if a.lib.Settings.Appearance == "Light" {
		dark = false
	}
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
	}
	t.Font = "Helvetica Neue"
	t.FontSize = 13
	t.Radius = 7
	if dark {
		t.Background = ui.Hex("#202321")
		t.Surface = ui.Hex("#282c29")
		t.Border = ui.Hex("#3c403c")
		t.Text = ui.Hex("#eeeae0")
		t.TextMuted = ui.Hex("#a4aaa2")
	} else {
		t.Background = ui.Hex("#fffefa")
		t.Surface = ui.Hex("#f3f2ed")
		t.Border = ui.Hex("#e4e3db")
		t.Text = ui.Hex("#30362e")
		t.TextMuted = ui.Hex("#83877d")
	}
	accent := "#667c54"
	t.Accent = ui.Hex(accent)
	t.AccentHover = t.Accent
	t.AccentPressed = t.Accent
	t.Selection = t.Accent.Alpha(0.22)
	t.Focus = t.Accent
	c.SetTheme(&t)
	a.reloadTheme()
	a.documentColor = a.pageTheme(&t).Text
}

func (a *app) view(c *ui.Context) {
	a.services = c.Services()
	a.theme(c)
	a.capture()
	if a.win != nil {
		a.refreshDisk()
		c.After(2 * time.Second)
	}
	if a.dirty {
		if time.Since(a.dirtySince) > 700*time.Millisecond {
			a.save()
		} else {
			c.After(750 * time.Millisecond)
		}
	}
	a.handleShortcuts(c)
	a.textFocused = false
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(c.Theme().Background).Children(func() {
		if !a.focus {
			a.sidebar(c)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			if a.current == nil {
				a.homeView(c)
			} else {
				a.editorView(c)
			}
		})
	})
	a.overlays(c)
}

func ghostButton(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).Background(ui.Transparent).Border(0, ui.Transparent).Padding(7, 10)
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(228).Shrink(0).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Padding(22, 14, 12).Gap(5).Children(func() {
		ui.Row(c).Padding(2, 10, 22).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Image(c, appIcon()).Width(34).Height(34).Fit(ui.Contain)
			ui.Text(c, "mory").FontSize(23).Bold().Grow(1)
			if ghostButton(c, "+").Label("New note").FontSize(21).Clicked() {
				a.createNote("", "Untitled")
			}
		})
		if ui.TextInput(c, &a.query).Label("Search notes").Placeholder("Search your notes…").FillWidth().Changed() {
			if a.save() {
				a.current = nil
			}
		}
		ui.Box(c).Height(12)
		for _, item := range []struct{ label, icon string }{{"All notes", "▤"}, {"Recent", "◷"}, {"Favorites", "☆"}, {"Trash", "⌫"}} {
			b := ghostButton(c, item.icon+"   "+item.label).Label(item.label).FillWidth().TextAlign(ui.Start).Padding(10, 12)
			if a.current == nil && a.section == item.label && a.folder == "" {
				b.Background(t.Accent.Alpha(0.12)).TextColor(t.Accent).Bold()
			}
			if b.Clicked() {
				a.home(item.label)
			}
		}
		ui.Row(c).Padding(24, 10, 8).Children(func() {
			ui.Text(c, "MY LIBRARY").FontSize(10).TextColor(t.TextMuted).Grow(1)
			ui.Textf(c, "%d", len(a.lib.Query("All notes", "", "", ""))).FontSize(10).TextColor(t.TextMuted)
			if ghostButton(c, "+").Label("New folder").Clicked() {
				a.folderFrom = ""
				a.folderName = ""
				a.folderOpen = true
			}
		})
		ui.Scroll(c).Grow(1).Gap(3).Children(func() {
			for _, folder := range a.lib.Folders() {
				button := ghostButton(c, "▱   "+folder).Label("Folder " + folder).TextAlign(ui.Start).FillWidth()
				button.PopoverMenu(func(m *ui.Menu) {
					if m.Item("Edit folder…").Chosen() {
						a.folderFrom = folder
						a.folderName = folder
						a.folderOpen = true
					}
				})
				if drag, ok := ui.Drop[noteDrag](button); ok {
					if n := a.lib.Get(drag.ID); n != nil {
						a.fail(a.lib.Move(n, folder, ""))
					}
				}
				if button.Clicked() {
					if a.save() {
						a.current = nil
						a.section = "All notes"
						a.folder = folder
					}
				}
			}
			seen := map[string]bool{}
			for _, n := range a.lib.Query("All notes", "", "", "Manual") {
				if n.Parent != "" && a.lib.Get(n.Parent) != nil && !a.lib.Get(n.Parent).Deleted {
					continue
				}
				a.treeNote(c, n, 0, seen)
			}
		})
		ui.Box(c).Height(1).FillWidth().Background(t.Border)
		if ghostButton(c, "↓   Import documents").FillWidth().TextAlign(ui.Start).Clicked() {
			a.chooseImport()
		}
		if ghostButton(c, "▱   Import folder").FillWidth().TextAlign(ui.Start).Clicked() {
			a.chooseLocal(true)
		}
		if ghostButton(c, "↗   Open local file").FillWidth().TextAlign(ui.Start).Clicked() {
			a.chooseLocal(false)
		}
		if ghostButton(c, "◇   Plugins").Label("Plugins").FillWidth().TextAlign(ui.Start).Clicked() {
			a.pluginsOpen = true
		}
		if ghostButton(c, "⚙   Settings").Label("Settings").FillWidth().TextAlign(ui.Start).Clicked() {
			a.settingsOpen = true
		}
		ui.Text(c, "LOCAL BY NATURE").FontSize(9).TextColor(t.TextMuted).Padding(14, 10, 0)
	})
}

func (a *app) treeNote(c *ui.Context, n *library.Note, depth int, seen map[string]bool) {
	if seen[n.ID] || depth > 20 {
		return
	}
	seen[n.ID] = true
	icon := "▫"
	if n.Icon != "" {
		icon = n.Icon
	}
	if n.Pinned {
		icon = "⌖"
	}
	b := ghostButton(c.Key("tree-"+n.ID), icon+"  "+n.Title).Label("Open "+n.Title).FillWidth().TextAlign(ui.Start).Padding(7, 8+float32(depth)*12).MaxHeight(35).Clip().Drag(noteDrag{n.ID})
	if drag, ok := ui.Drop[noteDrag](b); ok {
		if moving := a.lib.Get(drag.ID); moving != nil {
			a.fail(a.lib.Move(moving, n.Folder, n.ID))
		}
	}
	if a.current == n {
		b.Background(c.Theme().Accent.Alpha(0.1)).TextColor(c.Theme().Accent)
	}
	if b.Clicked() {
		a.openNote(n)
	}
	b.PopoverMenu(func(m *ui.Menu) { a.noteMenu(m, n) })
	for _, child := range a.lib.Notes {
		if !child.Deleted && child.Parent == n.ID {
			a.treeNote(c, child, depth+1, seen)
		}
	}
}

func (a *app) homeView(c *ui.Context) {
	t := c.Theme()
	notes := a.lib.Query(a.section, a.query, a.folder, a.order)
	ui.Row(c).Height(55).Shrink(0).Padding(12, 30).AlignItems(ui.Center).BorderWidth(0, 0, 1).BorderColor(t.Border).Children(func() {
		ui.Text(c, "Your space").TextColor(t.TextMuted).Grow(1)
		ui.Text(c, time.Now().Format("Monday, January 2")).FontSize(12).TextColor(t.TextMuted)
	})
	ui.Column(c).Grow(1).Padding(36, 38, 22).Gap(22).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				title := a.section
				if a.folder != "" {
					title = a.folder
				}
				if a.query != "" {
					title = "Search results"
				}
				ui.Text(c, title).FontSize(32).Font("Georgia")
				ui.Textf(c, "%d notes · A place for everything on your mind.", len(notes)).TextColor(t.TextMuted).FontSize(13)
			})
			if a.section != "Trash" && ui.PrimaryButton(c, "+  New note").Label("Create note").Padding(11, 18).Clicked() {
				a.createNote("", "Untitled")
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "YOUR NOTES").FontSize(10).TextColor(t.TextMuted).Grow(1)
			ui.Select(c, &a.order, []string{"Modified", "Created", "Title", "Manual"}).Width(118).Label("Sort notes")
			if ghostButton(c, "▦").Label("Grid view").Clicked() {
				a.lib.Settings.List = false
				a.fail(a.lib.SaveSettings())
			}
			if ghostButton(c, "☰").Label("List view").Clicked() {
				a.lib.Settings.List = true
				a.fail(a.lib.SaveSettings())
			}
		})
		a.batchBar(c, notes)
		ui.Scroll(c).Grow(1).Gap(12).Children(func() {
			if len(notes) == 0 {
				ui.Column(c).Padding(60, 20).Gap(14).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "A little space for something new.").Font("Georgia").FontSize(23)
					ui.Text(c, "Create a note, import a document, or try another search.").TextColor(t.TextMuted)
					if ghostButton(c, "Create a blank note").Clicked() {
						a.createNote("", "Untitled")
					}
				})
				return
			}
			if a.lib.Settings.List {
				for _, n := range notes {
					a.noteCard(c, n, true)
				}
			} else {
				ui.Grid(c).Columns(3).Gap(16).Children(func() {
					for _, n := range notes {
						a.noteCard(c, n, false)
					}
				})
			}
		})
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "●  On this Mac").FontSize(11).TextColor(t.Accent).Grow(1)
			ui.Text(c, "⌘N  New note     ⌘O  Import").FontSize(11).TextColor(t.TextMuted)
		})
	})
}

func (a *app) noteCard(c *ui.Context, n *library.Note, list bool) {
	t := c.Theme()
	height := float32(210)
	if list {
		height = 96
	}
	card := ui.Column(c.Key("card-"+n.ID)).Height(height).MinWidth(0).Padding(18).Gap(10).Radius(10).Border(1, t.Border).Background(t.Background).Label("Note " + n.Title).Focusable().Drag(noteDrag{n.ID})
	if a.selected[n.ID] {
		card.Border(2, t.Accent)
	}
	if drag, ok := ui.Drop[noteDrag](card); ok {
		a.reorderNote(drag.ID, n.ID)
	}
	card.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			if a.selecting {
				selected := a.selected[n.ID]
				if ui.Checkbox(c, &selected, "").Label("Select " + n.Title).Changed() {
					a.selected[n.ID] = selected
				}
			}
			icon := "▤"
			if n.Format == "xml" {
				icon = "▧"
			}
			ui.Text(c, icon).TextColor(t.Accent).Grow(1)
			if n.Pinned {
				ui.Text(c, "⌖").TextColor(t.TextMuted)
			}
			if n.Starred {
				ui.Text(c, "★").TextColor(t.Accent)
			}
			if ghostButton(c, "···").Label("Actions "+n.Title).Padding(0, 4).Clicked() {
				a.target = n
				a.renameTitle = n.Title
				a.renameOpen = true
			}
		})
		ui.Text(c, n.Title).Font("Georgia").FontSize(18).MaxHeight(27).Clip()
		if !list {
			preview := n.Source
			if n.Format == "xml" {
				_, preview, _ = document.FromXML(preview)
			}
			preview = strings.NewReplacer("#", "", "**", "", "\n", " ").Replace(preview)
			rs := []rune(preview)
			if len(rs) > 145 {
				preview = string(rs[:145]) + "…"
			}
			ui.Text(c, preview).FontSize(12).TextColor(t.TextMuted).Grow(1).Clip()
		}
		ui.Text(c, n.Updated.Format("Jan 2")+"  ·  "+formatLabel(n)).FontSize(10).TextColor(t.TextMuted)
	})
	if card.Clicked() {
		if a.selecting {
			a.selected[n.ID] = !a.selected[n.ID]
		} else if n.Deleted {
			a.target = n
			a.deleteOpen = true
		} else {
			a.openNote(n)
		}
	}
	card.PopoverMenu(func(m *ui.Menu) { a.noteMenu(m, n) })
}

func formatLabel(n *library.Note) string {
	if n.Format == "xml" {
		return "DocxXML"
	}
	return "Markdown"
}

func (a *app) noteMenu(m *ui.Menu, n *library.Note) {
	if n.Deleted {
		if m.Item("Restore").Chosen() {
			a.fail(a.lib.TrashTree(n, false))
		}
		if m.Item("Delete permanently…").Chosen() {
			a.target = n
			a.deleteOpen = true
		}
		return
	}
	if m.Item("Open").Chosen() {
		a.openNote(n)
	}
	if m.Item("Open in new window").Chosen() {
		a.openWindow(n)
	}
	if m.Item("Rename…").Chosen() {
		a.target = n
		a.renameTitle = n.Title
		a.renameOpen = true
	}
	if m.Item("Move…").Chosen() {
		a.target = n
		a.moveFolder = n.Folder
		a.moveParent = n.Parent
		a.moveOpen = true
	}
	if m.Item("Duplicate").Chosen() {
		a.duplicate(n)
	}
	if m.Item("Favorite").Checked(n.Starred).Chosen() {
		a.mutate(n, func() { n.Starred = !n.Starred })
	}
	if m.Item("Pin to top").Checked(n.Pinned).Chosen() {
		a.mutate(n, func() { n.Pinned = !n.Pinned })
	}
	m.Separator()
	if m.Item("Move to Trash").Chosen() {
		a.trash(n)
	}
}

func (a *app) editorView(c *ui.Context) {
	t := c.Theme()
	n := a.current
	ui.Row(c).Height(55).Shrink(0).Padding(10, 22).Gap(9).AlignItems(ui.Center).BorderWidth(0, 0, 1).BorderColor(t.Border).Children(func() {
		if ghostButton(c, "‹").Label("Back to library").FontSize(21).Clicked() {
			a.home("All notes")
		}
		ui.Text(c, n.Title).MaxWidth(210).MaxHeight(24).Clip().TextColor(t.TextMuted).Grow(1)
		for _, mode := range []string{"Write", "Source", "Split", "Read"} {
			b := ghostButton(c, mode).Padding(6, 10)
			if a.mode == mode {
				b.Background(t.Surface).TextColor(t.Accent)
			}
			if b.Clicked() {
				a.changeMode(mode)
			}
		}
		if ghostButton(c, "☆").Label("Toggle favorite").Clicked() {
			a.mutate(n, func() { n.Starred = !n.Starred })
		}
		if ghostButton(c, "☷").Label("Toggle outline").Clicked() {
			a.outline = !a.outline
		}
		if ghostButton(c, "↗").Label("Export note").Clicked() {
			a.exportMenuOpen()
		}
		if ghostButton(c, "···").Label("Document actions").Clicked() {
			a.target = n
			a.renameTitle = n.Title
			a.renameOpen = true
		}
	})
	if a.current == nil {
		return
	}
	if a.mode == "Write" {
		a.toolbar(c)
	}
	if a.findOpen && a.mode == "Write" {
		a.findBar(c)
	}
	ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		column := ui.Column(c).Grow(1).MinWidth(0).Padding(26, 32, 10)
		if n.Width != "Full" && a.mode != "Split" {
			width := float32(820)
			if n.Width == "Wide" {
				width = 1020
			}
			column.MaxWidth(width)
		}
		column.Children(func() {
			base := c.Theme()
			c.SetTheme(a.pageTheme(base))
			defer c.SetTheme(base)
			column.Background(c.Theme().Background)
			a.documentHeader(c)
			switch a.mode {
			case "Write":
				if a.blockEditor != nil {
					a.blockView(c, false)
					break
				}
				font := a.writingFont(".page")
				ed := richtext.EditorBase(c.Key(n.ID), a.doc, richtext.EditorOptions{Label: "Document editor", Placeholder: "Start writing…", Font: font, OnLink: a.openLink, OnLinkCommand: func() { a.linkURL = a.doc.Formatting().Link; a.linkOpen = true }, Theme: a.docTheme.Rich(c.Theme(), font)}).Grow(1).FillWidth()
				a.textFocused = ed.Focused()

			case "Source":
				editor.View(c.Key(n.ID), a.code).Label("Source editor").Fill()
			case "Split":
				ui.Row(c).Grow(1).MinHeight(0).Gap(22).Children(func() {
					editor.View(c.Key(n.ID), a.code).Label("Source editor").Grow(1).MinWidth(0)
					ui.Box(c).Width(1).Background(t.Border)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() { a.splitPreview(c) })
				})
			case "Read":
				if a.blockEditor != nil {
					a.blockView(c, true)
				} else {
					ui.Scroll(c).Grow(1).Children(func() { markdown.View(c, a.markdownSource(), &a.preview).FillWidth() })
				}
			}
		})
		if a.outline && !a.focus && a.mode != "Split" {
			a.outlineView(c)
		}
	})
	a.capture()
	ui.Row(c).Height(32).Shrink(0).Padding(5, 24).Gap(16).BorderWidth(1, 0, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.status).FontSize(10).TextColor(t.TextMuted).Grow(1)
		if a.statsSource != n.Source {
			text := n.Source
			if a.structured() {
				text = a.blockEditor.doc.Root.Plain()
			}
			a.words, a.characters = document.Counts(text)
			a.statsSource = n.Source
		}
		ui.Textf(c, "%d words  ·  %d characters", a.words, a.characters).FontSize(10).TextColor(t.TextMuted)
		if ghostButton(c, "Focus").Label("Toggle focus mode").Padding(0, 5).FontSize(10).Clicked() {
			a.focus = !a.focus
		}
	})
}

func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	doc := a.doc
	f := doc.Formatting()
	ui.Row(c).Padding(8, 28).Gap(4).Wrap().BorderWidth(0, 0, 1).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		style := "Body"
		if f.Paragraph.Heading > 0 {
			style = fmt.Sprintf("Heading %d", f.Paragraph.Heading)
		}
		if ui.Select(c, &style, []string{"Body", "Heading 1", "Heading 2", "Heading 3", "Heading 4", "Heading 5", "Heading 6", "Heading 7", "Heading 8", "Heading 9"}).Label("Paragraph style").KeepFocus().Width(125).Changed() {
			level := 0
			fmt.Sscanf(style, "Heading %d", &level)
			a.setHeading(level)
		}
		for _, b := range []struct {
			text, label string
			on          bool
			run         func()
		}{{"B", "Bold", f.Bold == richtext.FormatOn, doc.ToggleBold}, {"I", "Italic", f.Italic == richtext.FormatOn, doc.ToggleItalic}, {"U", "Underline", f.Underline == richtext.FormatOn, doc.ToggleUnderline}, {"S", "Strikethrough", f.Strikethrough == richtext.FormatOn, doc.ToggleStrikethrough}, {"‹›", "Inline code", f.Code == richtext.FormatOn, doc.ToggleCode}, {"•", "Bullet list", f.Paragraph.List == richtext.Bullets, func() { doc.ToggleList(richtext.Bullets) }}, {"1.", "Numbered list", f.Paragraph.List == richtext.Numbers, func() { doc.ToggleList(richtext.Numbers) }}, {"☑", "Task list", f.Paragraph.List == richtext.Tasks, func() { doc.ToggleList(richtext.Tasks) }}, {"❝", "Quote", f.Paragraph.Quote, doc.ToggleQuote}, {"{}", "Code block", f.Paragraph.CodeBlock, doc.ToggleCodeBlock}} {
			button := ghostButton(c, b.text).Label(b.label).Tooltip(b.label).KeepFocus().Padding(5, 9)
			if b.on {
				button.Background(t.Accent.Alpha(0.15))
			}
			if button.Clicked() {
				b.run()
			}
		}
		if ghostButton(c, "Link").KeepFocus().Clicked() {
			a.linkURL = doc.Formatting().Link
			a.linkOpen = true
		}
		if ghostButton(c, "+ Insert").Label("Insert block").KeepFocus().Clicked() {
			a.insertOpen = true
			a.insertQuery = ""
		}
		ghostButton(c, "Color").KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
			for _, color := range []string{"Default", "gray", "red", "orange", "yellow", "green", "blue", "purple"} {
				if m.Item(color).Chosen() {
					value := blockColor(color, t.Text)
					if color == "Default" {
						value = ui.Color{}
					}
					doc.Format(func(attr *richtext.Attributes) { attr.Color = value })
					a.capture()
				}
			}
		})
		ghostButton(c, "Highlight").KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
			for _, color := range []string{"None", "light-gray", "light-red", "light-orange", "light-yellow", "light-green", "light-blue", "light-purple"} {
				if m.Item(color).Chosen() {
					value := blockColor(color, ui.Transparent)
					doc.Format(func(attr *richtext.Attributes) { attr.Background = value })
					a.capture()
				}
			}
		})
		ghostButton(c, "Align").KeepFocus().PopoverMenuButton(func(m *ui.Menu) {
			for _, item := range []struct {
				name  string
				align ui.Align
			}{{"Left", ui.Start}, {"Center", ui.Center}, {"Right", ui.End}} {
				if m.Item(item.name).Chosen() {
					doc.SetAlign(item.align)
					a.capture()
				}
			}
		})
	})
}

func (a *app) outlineView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Width(176).Shrink(0).Padding(34, 12).Gap(10).Children(func() {
		ui.Text(c, "ON THIS PAGE").FontSize(9).TextColor(t.TextMuted).Padding(0, 8, 10)
		if a.structured() {
			a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
				if len(n.Kind) == 2 && n.Kind[0] == 'h' && n.Kind[1] >= '1' && n.Kind[1] <= '9' {
					if ghostButton(c.Key(n.ID), n.Plain()).Label("Heading "+n.Plain()).FontSize(11).Padding(5, 8+float32(n.Kind[1]-'1')*8).FillWidth().Clicked() {
						a.focusBlock(n.ID, 0)
					}
				}
			})
			return
		}
		source := a.markdownSource()
		if a.headingSource != source {
			a.headingSource = source
			a.headings = document.Headings(source)
		}
		for _, h := range a.headings {
			if ghostButton(c.Key(h.Offset), h.Text).Label("Heading "+h.Text).FontSize(11).Padding(5, 8+float32(h.Level-1)*8).MaxHeight(38).Clip().TextAlign(ui.Start).FillWidth().Clicked() {
				if a.mode == "Write" {
					for pos, p := range a.doc.Text().Paragraphs() {
						if p.Style.Heading == h.Level && strings.TrimSpace(pText(p)) == h.Text {
							a.doc.SetSelection(pos, pos)
							break
						}
					}
				} else if a.mode == "Source" {
					a.code.Select(h.Offset, h.Offset)
				} else {
					a.preview.Anchor = markdown.Slug(h.Text)
					a.preview.AnchorVersion++
				}
			}
		}
	})
}

func pText(p richtext.Paragraph) string {
	var b strings.Builder
	for _, r := range p.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

func (a *app) findBar(c *ui.Context) {
	ui.Row(c).Padding(8, 28).Gap(8).Children(func() {
		ui.TextInput(c, &a.findText).Label("Find text").Placeholder("Find…").Width(180)
		ui.TextInput(c, &a.replaceText).Label("Replace text").Placeholder("Replace with…").Width(180)
		if ui.Button(c, "Next").Clicked() {
			a.findNext()
		}
		if ui.Button(c, "Replace").Clicked() {
			anchor, caret := a.doc.Selection()
			start, end := min(anchor, caret), max(anchor, caret)
			r := []rune(a.doc.Text().String())
			if start < end && string(r[start:end]) == a.findText {
				a.doc.InsertText(a.replaceText)
				a.capture()
			}
			a.findNext()
		}
		if ghostButton(c, "×").Label("Close find").Clicked() {
			a.findOpen = false
		}
		if ui.Button(c, "Replace all").Clicked() {
			a.replaceAll()
		}
	})
}

func (a *app) findNext() {
	if a.findText == "" {
		return
	}
	if a.structured() {
		a.findStructured()
		return
	}
	text := a.doc.Text().String()
	_, caret := a.doc.Selection()
	r := []rune(text)
	start := min(caret, len(r))
	part := string(r[start:])
	offset := strings.Index(part, a.findText)
	if offset < 0 {
		start = 0
		part = text
		offset = strings.Index(text, a.findText)
	}
	if offset >= 0 {
		pos := start + len([]rune(part[:offset]))
		a.doc.SetSelection(pos, pos+len([]rune(a.findText)))
	} else {
		a.status = "No matches"
	}
}
