package main

import "github.com/egoist/mygo/ui"

// Borrow existing page whitespace for the overlay scrollbar, preserving the
// authored text width and the title's alignment while keeping its hit area clear.
const documentScrollGutter float32 = 16

func documentScrollbar(theme *ui.Theme) ui.ScrollbarStyle {
	return ui.ScrollbarStyle{
		TrackWidth: 10, ThumbWidth: 4, Radius: 2, MinLength: 28,
		Thumb: theme.Text.Alpha(.20), Hover: theme.Text.Alpha(.35), Active: theme.Text.Alpha(.5),
	}
}

func documentScroll(c *ui.Context) ui.Element {
	return ui.Scroll(c).Label("Document preview").Grow(1).MinHeight(0).Margin(0, -documentScrollGutter, 0, 0).Padding(0, documentScrollGutter, 0, 0)
}

// Reserve header space inside the existing virtual editor. The cover and title
// are painted above that space, so scrolling never allocates a full-height text
// editor or turns a large block document into an eagerly built scroll view.
func (a *app) pageHeaderSpace() float32 {
	height := a.pageHeaderHeight
	if height == 0 {
		height = 140
	}
	if a.current != nil && a.current.Cover != "" && a.loadAssetBitmap(a.current.Cover) != nil {
		height += documentCoverHeight
	}
	return height
}

func (a *app) updatePageHeaderOffset(c *ui.Context) {
	offset := a.pageScroll.Y
	if a.mode == "Source" || a.mode == "Split" {
		offset = float32(a.code.ScrollY())
	} else if a.mode == "Write" && a.blockEditor == nil && a.doc != nil {
		offset = a.pageHeaderSpace()
		if boxes := a.doc.VisibleParagraphs(); len(boxes) > 0 && boxes[0].Index == 0 {
			offset = max(0, a.pageHeaderSpace()+1-boxes[0].Top)
		}
	}
	if a.pageHeaderOffset != offset {
		a.pageHeaderOffset = offset
		c.AnimationFrame()
	}
}

func (a *app) pageHeaderOverlay(c *ui.Context) {
	space := a.pageHeaderSpace()
	ui.Column(c.Key("page-header-overlay")).Absolute().ZIndex(1).Top(-a.pageHeaderOffset).Left(0).Right(0).Height(space).PassThrough().
		HandleInput(func(event ui.InputEvent) bool {
			if event.Kind != ui.InputScroll || event.DY == 0 || event.Mods&ui.Shift != 0 {
				return false
			}
			if a.mode == "Source" || a.mode == "Split" {
				a.code.SetScrollY(a.code.ScrollY() + float64(event.DY))
			} else if a.mode == "Write" && a.blockEditor == nil {
				a.doc.ScrollBy(event.DY)
			} else {
				a.pageScroll.Y = max(0, min(a.pageScroll.MaxY, a.pageScroll.Y+event.DY))
			}
			return true
		}).Children(func() {
		a.documentCover(c)
		ui.Row(c).FillWidth().Justify(a.pageAlignment()).PassThrough().Children(func() {
			group := ui.Row(c).Grow(1).MinWidth(0).PassThrough()
			outline := a.outline && !a.focus && a.mode != "Split"
			if width := a.documentPageWidth(); width > 0 && a.mode != "Split" {
				if outline {
					width += a.outlineWidth()
				}
				group.MaxWidth(width)
			}
			group.Children(func() {
				if outline && a.outlineSide() == "Left" {
					ui.Box(c).Width(a.outlineWidth()).Shrink(0)
				}
				ui.Themed(c, a.pageTheme(c.Theme())).Label(a.tr("Document title area")).Grow(1).MinWidth(0).Padding(24, 40, 0).Background(c.Theme().Background).PassThrough().Draw(func(_ *ui.Painter, r ui.Rect) {
					if r.H != a.pageHeaderHeight {
						a.pageHeaderHeight = r.H
						c.AnimationFrame()
					}
				}).Children(func() { a.documentHeader(c) })
				if outline && a.outlineSide() == "Right" {
					ui.Box(c).Width(a.outlineWidth()).Shrink(0)
				}
			})
		})
	})
}
