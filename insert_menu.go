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
				for _, b := range []struct{ name, source string }{{"Heading", "\n## Heading\n"}, {"Bullet list", "\n- List item\n"}, {"To-do list", "\n- [ ] A small next step\n"}, {"Quote", "\n> A thought worth keeping.\n"}, {"Code block", "\n```go\nfunc main() {}\n```\n"}, {"Table", "\n| Column A | Column B |\n| --- | --- |\n| Value | Value |\n"}, {"Math", "\n$$\nE = mc^2\n$$\n"}, {"Callout", "\n> [!NOTE]\n> Something to remember.\n"}, {"Divider", "\n---\n"}} {
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
