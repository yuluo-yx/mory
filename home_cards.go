package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func (a *app) folderCard(c *ui.Context, folder homeFolder, list bool, height float32) {
	t := c.Theme()
	card := ui.Column(c.Key("home-folder-" + folder.path)).Label(a.tr("Open folder ") + folder.path).
		Height(height).MinWidth(0).Focusable().Cursor(ui.CursorPointer).Radius(16)
	if list {
		card.Border(1, t.Border).Background(t.Surface)
		card.Children(func() {
			ui.Row(c).Fill().Padding(16).Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconFolder()).Size(24, 24).TextColor(t.TextMuted)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, library.FolderName(folder.path)).FontSize(15).Bold().SingleLine()
					ui.Textf(c, a.tr("%d documents"), folder.count).FontSize(12).TextColor(t.TextMuted)
				})
				ui.Icon(c, iconChevronRight()).Size(14, 14).TextColor(t.TextMuted)
			})
		})
	} else {
		card.Draw(func(p *ui.Painter, r ui.Rect) {
			paper, back, front := t.Background, ui.Hex("#e5e5e7"), ui.Hex("#f5f5f6")
			if t.Dark {
				back, front = ui.Hex("#39393d"), ui.Hex("#2b2b2f")
			}
			// The tab and layered sheets identify a folder before reading its label.
			p.Fill(ui.Rect{X: r.X, Y: r.Y + 18, W: r.W, H: r.H - 18}, back, 17)
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * .42, H: 45}, back, 17)
			for i := 2; i >= 0; i-- {
				inset := float32(12 + i*9)
				box := ui.Rect{X: r.X + inset, Y: r.Y + 44 - float32(i*7), W: r.W - inset*2, H: r.H - 62}
				p.Fill(box, paper, 10)
				p.Stroke(box, t.Border, 10, .7)
			}
			if folder.preview != nil {
				p.Clip(ui.Rect{X: r.X + 24, Y: r.Y + 58, W: r.W - 48, H: r.H - 130}, 0, func() {
					_, titleHeight := p.RichText(r.X+24, r.Y+58, r.W-48, ui.Span{Text: a.displayTitle(folder.preview.Title), Size: 14, Weight: 700, Color: t.Text})
					a.drawHomePreview(p, ui.Rect{X: r.X + 24, Y: r.Y + titleHeight + 70, W: r.W - 48, H: r.H - titleHeight - 140}, folder.preview)
				})
			} else {
				p.Image(iconFolder(), ui.Rect{X: r.X + r.W/2 - 22, Y: r.Y + 95, W: 44, H: 44}, ui.Contain)
			}
			p.FillGradient(ui.Rect{X: r.X, Y: r.Y + r.H*.48, W: r.W, H: r.H * .52}, ui.LinearGradient{From: front.Alpha(0), To: front, Angle: 180, End: .55}, 16)
			p.Stroke(ui.Rect{X: r.X, Y: r.Y + 18, W: r.W, H: r.H - 18}, t.Border.Alpha(.7), 16, .8)
		})
		card.Children(func() {
			ui.Box(c).Grow(1)
			ui.Column(c).FillWidth().Padding(14, 18, 18).Gap(5).Children(func() {
				ui.Text(c, library.FolderName(folder.path)).FontSize(16).Bold().SingleLine().Tooltip(folder.path)
				ui.Textf(c, a.tr("%d documents"), folder.count).FontSize(12).TextColor(t.TextMuted)
			})
		})
	}
	if card.Hovered() || card.FocusVisible() {
		card.Border(1, t.Accent.Alpha(.45))
	}
	if card.Clicked() || card.Shortcut(0, ui.KeyEnter) {
		a.openHomeFolder(folder.path)
	}
	card.PopoverMenu(func(m *ui.Menu) {
		if m.Item(a.tr("Open")).Chosen() {
			a.openHomeFolder(folder.path)
		}
		if m.Item(a.tr("New note")).Chosen() {
			a.openHomeFolder(folder.path)
			a.createNote("", "")
		}
		if m.Item(a.tr("Edit folder…")).Chosen() {
			a.folderFrom, a.folderName, a.folderOpen = folder.path, folder.path, true
		}
		if m.Item(a.tr("Open storage folder in Finder")).Chosen() {
			a.openStorageInFinder()
		}
		m.Separator()
		if m.Item(a.tr("Delete folder…")).Chosen() {
			a.folderDeleteTarget, a.folderDeleteOpen = folder.path, true
		}
	})
}

func (a *app) paperNoteCard(c *ui.Context, n *library.Note, height float32) {
	t := c.Theme()
	card := ui.Column(c.Key("card-"+n.ID)).Height(height).MinWidth(0).Radius(16).
		Border(1, t.Border).Background(t.Background).Label(a.tr("Note ") + a.displayTitle(n.Title)).
		Focusable().Cursor(ui.CursorPointer).Drag(noteDrag{n.ID})
	if card.Hovered() || card.FocusVisible() {
		card.Border(1, t.Accent.Alpha(.45)).Shadow(0, 5, 16, 0, ui.RGBA(0, 0, 0, .06))
	}
	if a.selected[n.ID] {
		card.Border(2, t.Accent)
	}
	if drag, ok := ui.Drop[noteDrag](card); ok {
		a.reorderNote(drag.ID, n.ID)
	}
	card.Children(func() {
		ui.Row(c).FillWidth().Padding(15, 15, 6, 18).Gap(6).AlignItems(ui.Start).Shrink(0).Children(func() {
			if a.selecting {
				selected := a.selected[n.ID]
				if ui.Checkbox(c, &selected, "").Label(a.tr("Select ") + a.displayTitle(n.Title)).Changed() {
					a.selected[n.ID] = selected
				}
			}
			ui.Text(c, a.displayTitle(n.Title)).FontSize(16).Bold().Grow(1).MinWidth(0).MaxHeight(42).Clip().Tooltip(a.displayTitle(n.Title))
			if n.Pinned {
				ui.Icon(c, iconPin()).Size(12, 12).TextColor(t.TextMuted)
			}
			if n.Starred {
				ui.Icon(c, iconStar()).Size(12, 12).TextColor(ui.Hex("#ff9500"))
			}
			more := ghostButton(c, "").Label(a.tr("Actions ") + a.displayTitle(n.Title)).Padding(3).Radius(6)
			more.Children(func() { ui.Icon(c, iconMore()).Size(14, 14).TextColor(t.TextMuted) })
			if more.Clicked() {
				a.openNoteActions(n)
			}
		})
		ui.Column(c).FillWidth().Grow(1).MinHeight(0).Padding(4, 18, 10).Clip().Children(func() {
			ui.Box(c).Fill().Clip().Draw(func(p *ui.Painter, r ui.Rect) {
				a.drawHomePreview(p, r, n)
				p.FillGradient(ui.Rect{X: r.X, Y: r.Y + r.H - 26, W: r.W, H: 26}, ui.LinearGradient{From: t.Background.Alpha(0), To: t.Background, Angle: 180}, 0)
			})
		})
		ui.Row(c).Padding(0, 18, 13).FillWidth().Shrink(0).Children(func() {
			ui.Text(c, a.noteSummary(n)).FontSize(10).TextColor(t.TextMuted).SingleLine().Grow(1).MinWidth(0)
		})
	})
	if card.Clicked() || card.Shortcut(0, ui.KeyEnter) {
		if a.selecting {
			a.selected[n.ID] = !a.selected[n.ID]
		} else {
			a.openNote(n)
		}
	}
	card.PopoverMenu(func(m *ui.Menu) { a.noteMenu(m, n) })
}
