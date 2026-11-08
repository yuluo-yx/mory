package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

type shortcut struct {
	Name, Key string
	Run       func()
}

func (a *app) actions() []shortcut {
	edit := func(fn func(*richtext.Document)) func() {
		return func() {
			if a.current != nil && a.mode == "Write" && a.doc != nil {
				fn(a.doc)
				a.capture()
			}
		}
	}
	items := []shortcut{
		{"New note", "Cmd+N", func() { a.createNote("", "Untitled") }},
		{"Quick note", "Cmd+Alt+N", a.quickNote},
		{"Import", "Cmd+O", a.chooseImport},
		{"Open local file", "Cmd+Shift+O", func() { a.chooseLocal(false) }},
		{"Save", "Cmd+S", func() { a.save() }},
		{"Export", "Cmd+Shift+E", a.exportMenuOpen},
		{"Print", "Cmd+Alt+P", a.printDocument},
		{"Library", "Cmd+Shift+F", func() { a.home("All notes") }},
		{"Focus mode", "Cmd+Shift+L", func() { a.focus = !a.focus }},
		{"Insert block", "Cmd+P", func() { a.insertOpen = true; a.insertQuery = "" }},
		{"Settings", "Cmd+,", func() { a.settingsOpen = true }},
		{"Shortcuts", "Cmd+/", func() { a.helpOpen = true }},
		{"Find", "Cmd+F", func() {
			if a.current != nil {
				a.findOpen = true
			}
		}},
		{"Replace", "Cmd+Alt+F", func() {
			if a.current != nil {
				a.findOpen = true
			}
		}},
		{"Write mode", "Cmd+Alt+1", func() { a.changeMode("Write") }},
		{"Read mode", "Cmd+Alt+2", func() { a.changeMode("Read") }},
		{"Source mode", "Cmd+Alt+3", func() { a.changeMode("Source") }},
		{"Split mode", "Cmd+Alt+4", func() { a.changeMode("Split") }},
		{"Outline", "Cmd+Shift+H", func() { a.outline = !a.outline }},
		{"Bold", "Cmd+B", edit((*richtext.Document).ToggleBold)},
		{"Italic", "Cmd+I", edit((*richtext.Document).ToggleItalic)},
		{"Underline", "Cmd+U", edit((*richtext.Document).ToggleUnderline)},
		{"Strikethrough", "Cmd+Shift+X", edit((*richtext.Document).ToggleStrikethrough)},
		{"Inline code", "Cmd+Ctrl+C", edit((*richtext.Document).ToggleCode)},
		{"Link", "Cmd+K", edit(func(d *richtext.Document) { a.linkURL = d.Formatting().Link; a.linkOpen = true })},
		{"Bullets", "Cmd+Shift+8", edit(func(d *richtext.Document) { d.ToggleList(richtext.Bullets) })},
		{"Numbers", "Cmd+Shift+7", edit(func(d *richtext.Document) { d.ToggleList(richtext.Numbers) })},
		{"Tasks", "Cmd+Shift+9", edit(func(d *richtext.Document) { d.ToggleList(richtext.Tasks) })},
		{"Quote", "Cmd+Shift+B", edit((*richtext.Document).ToggleQuote)},
		{"Code block", "Cmd+Alt+C", edit((*richtext.Document).ToggleCodeBlock)},
		{"Indent", "Cmd+]", edit(func(d *richtext.Document) { d.Indent(1) })},
		{"Outdent", "Cmd+[", edit(func(d *richtext.Document) { d.Indent(-1) })},
		{"Align left", "Cmd+Shift+J", edit(func(d *richtext.Document) { d.SetAlign(ui.Start) })},
		{"Align center", "Cmd+Shift+M", edit(func(d *richtext.Document) { d.SetAlign(ui.Center) })},
		{"Align right", "Cmd+Shift+R", edit(func(d *richtext.Document) { d.SetAlign(ui.End) })},
		{"Undo", "Cmd+Z", func() {
			if a.structured() {
				a.blockUndo(false)
			} else if a.doc != nil && a.mode == "Write" {
				a.doc.Undo()
				a.capture()
			}
		}},
		{"Redo", "Cmd+Shift+Z", func() {
			if a.structured() {
				a.blockUndo(true)
			} else if a.doc != nil && a.mode == "Write" {
				a.doc.Redo()
				a.capture()
			}
		}},
		{"Paste", "Cmd+V", a.pasteStructured},
	}
	for level := 0; level <= 9; level++ {
		items = append(items, shortcut{fmt.Sprintf("Heading %d", level), fmt.Sprintf("Cmd+Ctrl+%d", level), func() { a.setHeading(level) }})
	}
	return items
}

func (a *app) keyFor(name, fallback string) string {
	if key, ok := a.lib.Settings.Keys[name]; ok {
		return key
	}
	return fallback
}

func parseKey(source string) (ui.Modifiers, ui.Key, error) {
	var mods ui.Modifiers
	key := ui.KeyUnknown
	parts := strings.Split(source, "+")
	for i, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if i < len(parts)-1 {
			switch part {
			case "cmd", "command":
				mods |= ui.Cmd
			case "ctrl", "control":
				mods |= ui.Ctrl
			case "alt", "option":
				mods |= ui.Alt
			case "shift":
				mods |= ui.Shift
			default:
				return 0, 0, fmt.Errorf("unknown modifier: %s", part)
			}
			continue
		}
		if len(part) == 1 && part[0] >= 'a' && part[0] <= 'z' {
			key = ui.KeyA + ui.Key(part[0]-'a')
		} else if len(part) == 1 && part[0] >= '0' && part[0] <= '9' {
			key = ui.Key0 + ui.Key(part[0]-'0')
		} else {
			key = map[string]ui.Key{",": ui.KeyComma, "/": ui.KeySlash, ".": ui.KeyPeriod, "[": ui.KeyBracketLeft, "]": ui.KeyBracketRight, "space": ui.KeySpace, "enter": ui.KeyEnter, "tab": ui.KeyTab, "escape": ui.KeyEscape, "backspace": ui.KeyBackspace, "delete": ui.KeyDelete, "left": ui.KeyLeft, "right": ui.KeyRight, "up": ui.KeyUp, "down": ui.KeyDown, "home": ui.KeyHome, "end": ui.KeyEnd}[part]
		}
	}
	if key == ui.KeyUnknown {
		return 0, 0, fmt.Errorf("invalid shortcut: %s", source)
	}
	return mods, key, nil
}

func (a *app) handleShortcuts(c *ui.Context) {
	for _, action := range a.actions() {
		if isTextAction(action.Name) && (!a.textFocused || a.mode != "Write" || a.settingsOpen || a.helpOpen || a.renameOpen || a.linkOpen) {
			continue
		}
		if a.current == nil && action.Name == "Find" {
			continue
		}
		key := a.keyFor(action.Name, action.Key)
		if key != action.Key {
			if mods, k, err := parseKey(action.Key); err == nil {
				_ = c.Shortcut(mods, k)
			}
		}
		if key == "" {
			continue
		}
		mods, k, err := parseKey(key)
		if err == nil && c.Shortcut(mods, k) {
			action.Run()
		}
	}
}

func isTextAction(name string) bool {
	if strings.HasPrefix(name, "Heading ") {
		return true
	}
	switch name {
	case "Bold", "Italic", "Underline", "Strikethrough", "Inline code", "Link", "Bullets", "Numbers", "Tasks", "Quote", "Code block", "Indent", "Outdent", "Align left", "Align center", "Align right", "Undo", "Redo", "Paste":
		return true
	}
	return false
}

func (a *app) setHeading(level int) {
	if a.current == nil || a.doc == nil || a.mode != "Write" {
		return
	}
	if level > 6 && a.blockEditor == nil {
		if !a.enableBlocks() {
			return
		}
	}
	if a.blockEditor != nil {
		s := a.blockEditor
		n := s.doc.Find(s.active)
		if n == nil || !blocks.IsText(n) {
			return
		}
		a.captureBlocks()
		a.fail(s.doc.Transform(n.ID, func(n *blocks.Node) {
			n.Kind = "p"
			if level > 0 {
				n.Kind = fmt.Sprintf("h%d", level)
			}
		}))
		delete(s.fields, n.ID)
		a.captureBlocks()
	} else {
		a.doc.SetHeading(level)
		a.capture()
	}
}

var quickKey string

func (a *app) registerQuickNote() {
	key := a.lib.Settings.QuickNote
	if key == "" {
		key = "Alt+Space"
	}
	if key == quickKey {
		return
	}
	if err := mygo.GlobalShortcut.Register(key, a.quickNote); a.fail(err) {
		return
	}
	if quickKey != "" {
		mygo.GlobalShortcut.Unregister(quickKey)
	}
	quickKey = key
}

func (a *app) shortcutSettings(c *ui.Context) {
	ui.Column(c).Width(560).MaxHeight(680).Gap(12).Children(func() {
		ui.Text(c, "Keyboard shortcuts").FontSize(24)
		ui.Text(c, "Use Cmd, Ctrl, Alt and Shift separated by +. Leave a field empty to disable it.").FontSize(12)
		ui.Scroll(c).MaxHeight(500).Gap(6).Children(func() {
			for _, action := range a.actions() {
				ui.Row(c.Key(action.Name)).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, action.Name).Grow(1)
					value := a.keyFor(action.Name, action.Key)
					if ui.TextInput(c, &value).Label("Shortcut " + action.Name).Width(190).Changed() {
						a.lib.Settings.Keys[action.Name] = value
					}
				})
			}
			ui.Text(c, "Global quick note")
			ui.TextInput(c, &a.lib.Settings.QuickNote).Label("Global quick note shortcut").Placeholder("Alt+Space").FillWidth()
		})
		ui.Row(c).Gap(12).Children(func() {
			if ui.Button(c, "Restore defaults").Clicked() {
				a.lib.Settings.Keys = map[string]string{}
				a.lib.Settings.QuickNote = ""
			}
			if ui.PrimaryButton(c, "Save shortcuts").Clicked() {
				used := map[string]string{}
				valid := true
				for _, action := range a.actions() {
					key := a.keyFor(action.Name, action.Key)
					if key == "" {
						continue
					}
					mods, k, err := parseKey(key)
					if a.fail(err) {
						valid = false
						break
					}
					identity := fmt.Sprintf("%d:%d", mods, k)
					if other := used[identity]; other != "" {
						a.problem = "Shortcut conflict: " + other + " and " + action.Name
						valid = false
						break
					}
					used[identity] = action.Name
				}
				if valid && !a.fail(a.lib.SaveSettings()) {
					a.helpOpen = false
					if a.win != nil {
						a.installMenu()
						a.registerQuickNote()
					}
				}
			}
			if ui.Button(c, "Done").Clicked() {
				a.helpOpen = false
			}
		})
	})
}
