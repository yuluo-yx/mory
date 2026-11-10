package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

type shortcut struct {
	Name, Key string
	// Native responder-chain commands use menu roles and leave Run nil.
	Run func()
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
		{"Search notes", "Cmd+J", func() { a.home("All notes"); a.searchFocus = a.current == nil }},
		{"Import folder", "", func() { a.chooseLocal(true) }},
		{"Insert image or attachment", "", a.chooseAttachment},
		{"Plugins", "", func() { a.pluginsOpen = true }},
		{"New folder", "", func() { a.folderName, a.folderFrom = "", ""; a.folderOpen = true }},
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
		{"Formatting toolbar", "Cmd+Shift+T", func() { a.showToolbar = !a.showToolbar }},
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
		{"Cut", "Cmd+X", nil}, {"Copy", "Cmd+C", nil}, {"Select all", "Cmd+A", nil},
		{"Close window", "Cmd+W", nil}, {"Quit", "Cmd+Q", nil},
		{"Hide", "Cmd+H", nil}, {"Hide others", "Cmd+Alt+H", nil},
		{"Full screen", "Ctrl+Cmd+F", nil},
		{"Open in new window", "", func() {
			if a.current != nil {
				a.openWindow(a.current)
			}
		}},
		{"Outline on Left", "", func() { a.setOutlineSide("Left") }},
		{"Outline on Right", "", func() { a.setOutlineSide("Right") }},
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
			case "cmd", "command", "meta", "super":
				mods |= ui.Cmd
			case "ctrl", "control":
				mods |= ui.Ctrl
			case "alt", "option", "opt":
				mods |= ui.Alt
			case "shift":
				mods |= ui.Shift
			default:
				return 0, 0, fmt.Errorf("unknown modifier: %s", part)
			}
			continue
		}
		if len(part) >= 2 && part[0] == 'f' && func() bool {
			n, err := strconv.Atoi(part[1:])
			if err == nil && n >= 1 && n <= 12 {
				key = ui.KeyF1 + ui.Key(n-1)
				return true
			}
			return false
		}() {
		} else if len(part) == 1 && part[0] >= 'a' && part[0] <= 'z' {
			key = ui.KeyA + ui.Key(part[0]-'a')
		} else if len(part) == 1 && part[0] >= '0' && part[0] <= '9' {
			key = ui.Key0 + ui.Key(part[0]-'0')
		} else {
			key = map[string]ui.Key{",": ui.KeyComma, "/": ui.KeySlash, ".": ui.KeyPeriod, "[": ui.KeyBracketLeft, "]": ui.KeyBracketRight, "space": ui.KeySpace, "enter": ui.KeyEnter, "tab": ui.KeyTab, "escape": ui.KeyEscape, "backspace": ui.KeyBackspace, "delete": ui.KeyDelete, "left": ui.KeyLeft, "right": ui.KeyRight, "up": ui.KeyUp, "down": ui.KeyDown, "home": ui.KeyHome, "end": ui.KeyEnd, "return": ui.KeyEnter, "esc": ui.KeyEscape, "pageup": ui.KeyPageUp, "pagedown": ui.KeyPageDown, "-": ui.KeyMinus, "=": ui.KeyEqual, ";": ui.KeySemicolon, "\"": ui.KeyQuote, "backslash": ui.KeyBackslash, "`": ui.KeyBackquote}[part]
		}
	}
	if key == ui.KeyUnknown {
		return 0, 0, fmt.Errorf("invalid shortcut: %s", source)
	}
	return mods, key, nil
}

func (a *app) handleShortcuts(c *ui.Context) {
	if a.settingsOpen || a.helpOpen || a.cssOpen {
		return
	}
	actions := a.actions()
	assigned := map[string]bool{}
	for _, action := range actions {
		key, _ := canonicalKey(a.keyFor(action.Name, action.Key))
		if key != "" {
			assigned[key] = true
		}
		if action.Run == nil || isTextAction(action.Name) && (!a.textFocused || a.mode != "Write" || a.renameOpen || a.linkOpen) {
			continue
		}
		if mods, k, err := parseKey(key); err == nil && c.Shortcut(mods, k) {
			action.Run()
		}
	}
	// Consume disabled/rebound defaults only when no other action owns them.
	for _, action := range actions {
		if isTextAction(action.Name) && (!a.textFocused || a.mode != "Write") {
			continue
		}
		key, _ := canonicalKey(action.Key)
		if key != "" && !assigned[key] {
			if mods, k, err := parseKey(key); err == nil {
				_ = c.Shortcut(mods, k)
			}
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
