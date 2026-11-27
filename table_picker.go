package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func (a *app) tablePicker(c *ui.Context) {
	ui.Modal(c, &a.tablePickerOpen, func() {
		panel := ui.Column(c).Width(300).Gap(12)
		navigation := ui.Local(panel, "pointer", func() insertNavigation { return insertNavigation{} })
		x, y, over := panel.PointerPosition()
		moved := over && (!navigation.pointerKnown || x != navigation.pointerX || y != navigation.pointerY)
		navigation.pointerX, navigation.pointerY, navigation.pointerKnown = x, y, true
		panel.Children(func() {
			ui.Text(c, a.tr("Insert table")).FontSize(22).Bold()
			ui.Text(c, a.trf("%d rows × %d columns", a.tableInsertRows, a.tableInsertColumns)).Label(a.tr("Table dimensions")).TextColor(c.Theme().TextMuted)
			ui.Column(c).Gap(3).Children(func() {
				for row := 1; row <= 9; row++ {
					ui.Row(c).Gap(3).Children(func() {
						for col := 1; col <= 9; col++ {
							cell := ghostButton(c, "").Label(fmt.Sprintf("Table size %d %d", row, col)).Width(28).Height(22).Padding(0).Border(1, c.Theme().Border).Radius(3).KeepFocus().TabStop(false)
							if cell.Hovered() && moved {
								a.tableInsertRows, a.tableInsertColumns = row, col
							}
							if row <= a.tableInsertRows && col <= a.tableInsertColumns {
								cell.Background(c.Theme().Accent.Alpha(.18)).BorderColor(c.Theme().Accent)
							}
							if cell.Clicked() {
								a.insertTable(row, col)
							}
						}
					})
				}
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, a.tr("Cancel")).Clicked() {
					a.tablePickerOpen = false
				}
				confirm := ui.PrimaryButton(c, a.tr("Create table")).AutoFocus()
				for _, move := range []struct {
					key      ui.Key
					row, col int
				}{{ui.KeyUp, -1, 0}, {ui.KeyDown, 1, 0}, {ui.KeyLeft, 0, -1}, {ui.KeyRight, 0, 1}} {
					if confirm.Shortcut(0, move.key) {
						a.tableInsertRows = max(1, min(9, a.tableInsertRows+move.row))
						a.tableInsertColumns = max(1, min(9, a.tableInsertColumns+move.col))
					}
				}
				if confirm.Clicked() {
					a.insertTable(a.tableInsertRows, a.tableInsertColumns)
				}
			})
		})
	})
}

func (a *app) insertTable(rows, columns int) {
	if a.current == nil {
		return
	}
	a.consumeSlash()
	n := blocks.NewTable(max(1, min(9, rows)), max(1, min(9, columns)))
	a.insertNodes(n)
	a.tablePickerOpen = false
	a.focusTableCell(n.ID, 0, 0, false)
}
