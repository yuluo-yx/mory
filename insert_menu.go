package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// insertMenuButton is also used by component controls outside the insert menu.
func insertMenuButton(c *ui.Context, label string) ui.Element {
	b := ghostButton(c, label).FillWidth().Height(36).Justify(ui.Start).TextAlign(ui.Start)
	if b.Hovered() || b.Focused() {
		b.Background(c.Theme().Accent.Alpha(.12)).TextColor(c.Theme().Accent)
	}
	return b
}

type insertAction struct {
	name string
	run  func()
}

type insertNavigation struct {
	query, selected, pending string
	reveal, pointerKnown     bool
	pointerX, pointerY       float32
}

func (a *app) insertActions(query string) []insertAction {
	var actions []insertAction
	query = strings.ToLower(strings.TrimSpace(query))
	item := func(name string, run func()) {
		if strings.Contains(strings.ToLower(name+" "+a.tr(name)), query) {
			actions = append(actions, insertAction{name, run})
		}
	}
	for _, name := range []string{"Heading", "Bullet list", "To-do list", "Quote", "Code block", "Callout"} {
		item(name, func() { a.insertEmptyBlock(name) })
	}
	for _, b := range []struct{ name, source string }{{"Table", "\n| Column A | Column B |\n| --- | --- |\n| Value | Value |\n"}, {"Math", "\n$$\nE = mc^2\n$$\n"}, {"Divider", "\n---\n"}} {
		item(b.name, func() { a.insert(b.source) })
	}
	item("Image or attachment…", func() { a.insertOpen = false; a.chooseAttachment() })
	item("Columns", func() { a.insertNodes(blocks.NewColumns(2)) })
	item("Countdown", func() { n := blocks.New("countdown"); n.Set("duration", "300"); a.insertNodes(n) })
	for _, b := range []struct{ name, kind string }{{"Inline equation", "latex"}, {"Reminder", "time"}, {"Button", "button"}, {"Document link", "doc-link"}} {
		item(b.name, func() { a.insertInline(b.kind) })
	}
	item("Child pages", func() { a.insertNodes(blocks.New("sub-page-list")) })
	return actions
}

func (a *app) insertMenu(c *ui.Context) {
	ui.Modal(c, &a.insertOpen, func() {
		panel := ui.Column(c).Width(400).Gap(12)
		navigation := ui.Local(panel, "navigation", func() insertNavigation { return insertNavigation{query: a.insertQuery} })
		results := func(query string) []insertAction {
			actions := a.insertActions(query)
			if navigation.query != query {
				navigation.query, navigation.selected = query, ""
				if len(actions) > 0 {
					navigation.selected = actions[0].name
				}
				navigation.reveal = true
			}
			return actions
		}
		// A stationary pointer must not overwrite keyboard navigation when the
		// selected row scrolls underneath it. Real pointer movement takes over.
		x, y, over := panel.PointerPosition()
		pointerMoved := over && (!navigation.pointerKnown || x != navigation.pointerX || y != navigation.pointerY)
		navigation.pointerX, navigation.pointerY, navigation.pointerKnown = x, y, true
		panel.Children(func() {
			ui.Text(c, a.tr("Give your idea a shape")).FontSize(22).Bold()
			search := ui.TextInput(c, &a.insertQuery).Label(a.tr("Search blocks")).Placeholder(a.tr("Search blocks…")).FillWidth().AutoFocus()
			search.OnTextKey(func(key ui.TextKey) bool {
				if key.Mods != 0 || key.Key != ui.KeyUp && key.Key != ui.KeyDown && key.Key != ui.KeyEnter {
					return false
				}
				// TextKey includes text queued before this key, even between frames.
				actions := results(key.Text())
				if a.current == nil || len(actions) == 0 {
					return true
				}
				index := -1
				for i, action := range actions {
					if action.name == navigation.selected {
						index = i
						break
					}
				}
				switch key.Key {
				case ui.KeyEnter:
					navigation.pending = actions[max(0, index)].name
				case ui.KeyDown:
					navigation.selected = actions[(index+1)%len(actions)].name
				case ui.KeyUp:
					if index < 0 {
						index = 0
					}
					navigation.selected = actions[(index-1+len(actions))%len(actions)].name
				}
				navigation.reveal = true
				return true
			})
			actions := results(a.insertQuery)
			ui.Scroll(c).Height(420).FillWidth().Gap(3).Role(ui.RoleListBox).Label(a.tr("Block options")).Children(func() {
				for _, action := range actions {
					name := action.name
					label := a.tr("Insert ") + name
					if name == "Image or attachment…" {
						label = a.tr(name)
					}
					button := ghostButton(c.Key(name), a.tr(name)).FillWidth().Height(36).Justify(ui.Start).TextAlign(ui.Start).
						Role(ui.RoleOption).Label(label).Disabled(a.current == nil).KeepFocus().TabStop(false)
					if pointerMoved && button.Hovered() && navigation.selected != name {
						navigation.selected = name
						c.AnimationFrame()
					}
					selected := navigation.selected == name
					button.AccessSelected(selected)
					if selected {
						button.Background(c.Theme().Accent.Alpha(.12)).TextColor(c.Theme().Accent)
						search.ActiveDescendant(button)
						if navigation.reveal {
							button.ScrollIntoView()
							navigation.reveal = false
						}
					}
					if button.Clicked() {
						navigation.pending = name
					}
				}
				if len(actions) == 0 {
					ui.Text(c, a.tr("No matching blocks")).TextColor(c.Theme().TextMuted).Padding(8)
				}
			})
			if name := navigation.pending; name != "" {
				navigation.pending = ""
				for _, action := range actions {
					if action.name == name && a.current != nil {
						action.run()
						// Keyboard submission is not a button click. Build the closed
						// dialog and new editor promptly so its caret receives typing.
						c.AnimationFrame()
						break
					}
				}
			}
		})
	})
}

// Instructional examples must never become editable document content. Build an
// empty structural block and focus its text field without inserting hint text.
func (a *app) insertEmptyBlock(name string) {
	if a.current == nil {
		return
	}
	if a.slashPending {
		if a.blockEditor != nil {
			if f := a.blockEditor.fields[a.blockEditor.active]; f != nil && f.doc != nil {
				f.doc.SetSelection(0, f.doc.Text().Len())
				f.doc.InsertText("")
			}
		} else {
			a.doc.SetSelection(a.slashOffset, a.slashOffset+1)
			a.doc.InsertText("")
		}
		a.capture()
		a.slashPending = false
	}
	text := blocks.New("p")
	node := text
	switch name {
	case "Heading":
		text.Kind = "h2"
	case "Bullet list":
		text.Kind = "li"
		node = blocks.New("ul", text)
	case "To-do list":
		text.Kind = "checkbox"
	case "Quote":
		node = blocks.New("blockquote", text)
	case "Code block":
		text.Kind = "pre"
	case "Callout":
		node = blocks.New("callout", text)
		node.Set("alert", "NOTE")
		node.Set("emoji", "💡")
	}
	a.insertNodes(node)
	if s := a.blockEditor; s != nil {
		s.active, s.focus, s.focusAt = text.ID, text.ID, 0
		if name == "Heading" {
			doc := a.blockField(text, a.documentColor).doc
			version := doc.Version()
			s.lastTextID = ""
			a.numberHeading(doc, 2)
			numbered := doc.Version() != version
			s.focusAt = doc.Text().Len()
			a.captureBlocks()
			if numbered {
				// Insertion and its generated prefix are one user command.
				s.doc.Coalesce()
			}
		}
	}
}
