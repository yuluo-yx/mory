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
