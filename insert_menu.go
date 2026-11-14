package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// insertMenuButton keeps hover and keyboard focus visible across appearances.
func insertMenuButton(c *ui.Context, label string) ui.Element {
	b := ghostButton(c, label).FillWidth().Height(36).Justify(ui.Start).TextAlign(ui.Start)
	if b.Hovered() || b.Focused() {
		b.Background(c.Theme().Accent.Alpha(.12)).TextColor(c.Theme().Accent)
	}
	return b
}

func (a *app) insertMenu(c *ui.Context) {
	ui.Modal(c, &a.insertOpen, func() {
		ui.Column(c).Width(400).Gap(12).Children(func() {
			ui.Text(c, a.tr("Give your idea a shape")).FontSize(22).Bold()
			ui.TextInput(c, &a.insertQuery).Label(a.tr("Search blocks")).Placeholder(a.tr("Search blocks…")).FillWidth().AutoFocus()
			ui.Scroll(c).Height(420).FillWidth().Gap(3).Children(func() {
				item := func(name string, run func()) {
					if !strings.Contains(strings.ToLower(name+" "+a.tr(name)), strings.ToLower(a.insertQuery)) {
						return
					}
					label := a.tr("Insert ") + name
					if name == "Image or attachment…" {
						label = a.tr(name)
					}
					if insertMenuButton(c.Key(name), a.tr(name)).Label(label).Disabled(a.current == nil).Clicked() {
						run()
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
			})
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
	}
}
