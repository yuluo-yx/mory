package main

import (
	"fmt"
	"maps"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Empty bindings deliberately disable an action, including the global hotkey.
func canonicalKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	mods, _, err := parseKey(value)
	if err != nil {
		return "", err
	}
	if mods&(ui.Cmd|ui.Ctrl|ui.Alt) == 0 {
		return "", fmt.Errorf("shortcut needs Cmd, Ctrl or Alt: %s", value)
	}
	parts := []string{}
	for _, mod := range []struct {
		bit  ui.Modifiers
		name string
	}{{ui.Cmd, "Cmd"}, {ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}} {
		if mods&mod.bit != 0 {
			parts = append(parts, mod.name)
		}
	}
	key := strings.ToLower(strings.TrimSpace(value[strings.LastIndex(value, "+")+1:]))
	aliases := map[string]string{"return": "enter", "esc": "escape"}
	if alias := aliases[key]; alias != "" {
		key = alias
	}
	if len(key) == 1 {
		key = strings.ToUpper(key)
	} else {
		key = strings.ToUpper(key[:1]) + key[1:]
	}
	return strings.Join(append(parts, key), "+"), nil
}

func (a *app) validateShortcuts(keys map[string]string, quick string) (map[string]string, string, error) {
	result, used := map[string]string{}, map[string]string{}
	validate := func(name, value string) (string, error) {
		key, err := canonicalKey(value)
		if err != nil {
			return "", err
		}
		if key == "" {
			return "", nil
		}
		if other := used[key]; other != "" {
			return "", fmt.Errorf("Shortcut conflict: %s and %s", other, name)
		}
		used[key] = name
		return key, nil
	}
	for _, action := range a.actions() {
		value, ok := keys[action.Name]
		if !ok {
			value = action.Key
		}
		key, err := validate(action.Name, value)
		if err != nil {
			return nil, "", err
		}
		result[action.Name] = key
	}
	key, err := validate("Global quick note", quick)
	return result, key, err
}

var quickKey string
var registerGlobal = mygo.GlobalShortcut.Register
var unregisterGlobal = mygo.GlobalShortcut.Unregister

func (a *app) shortcutHint(name string) string {
	for _, action := range a.actions() {
		if action.Name == name {
			return a.keyFor(name, action.Key)
		}
	}
	return ""
}

func (a *app) shortcutHints(names ...string) string {
	var hints []string
	for _, name := range names {
		if key := a.shortcutHint(name); key != "" {
			hints = append(hints, key+"  "+a.tr(name))
		}
	}
	return strings.Join(hints, "     ")
}

// Register the replacement before releasing the old key, so failures preserve it.
func (a *app) applyQuickKey(key string) error {
	key, err := canonicalKey(key)
	if err != nil {
		return err
	}
	if key == quickKey {
		return nil
	}
	if key != "" {
		if err := registerGlobal(key, a.quickNote); err != nil {
			return fmt.Errorf("global shortcut %s unavailable: %w", key, err)
		}
	}
	if quickKey != "" {
		unregisterGlobal(quickKey)
	}
	quickKey = key
	return nil
}

func (a *app) registerQuickNote() { a.fail(a.applyQuickKey(a.lib.Settings.QuickNote)) }

func (a *app) saveShortcuts() bool {
	keys, quick, err := a.validateShortcuts(a.shortcutDraft, a.quickDraft)
	if a.fail(err) {
		return false
	}
	previous, oldQuick := a.lib.Settings.Keys, a.lib.Settings.QuickNote
	if a.win != nil && a.fail(a.applyQuickKey(quick)) {
		return false
	}
	a.lib.Settings.Keys, a.lib.Settings.QuickNote = keys, quick
	if !a.savePreferences() {
		a.lib.Settings.Keys, a.lib.Settings.QuickNote = previous, oldQuick
		if a.win != nil {
			a.fail(a.applyQuickKey(oldQuick))
		}
		return false
	}
	a.shortcutDraft = nil
	return true
}

func (a *app) shortcutSettings(c *ui.Context) {
	if a.shortcutDraft == nil {
		a.shortcutDraft = maps.Clone(a.lib.Settings.Keys)
		a.quickDraft = a.lib.Settings.QuickNote
	}
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Gap(12).Children(func() {
		if ui.Checkbox(c, &a.lib.Settings.ShowShortcuts, a.tr("Show shortcut hints beside buttons")).Changed() {
			a.savePreferences()
		}
		ui.Text(c, a.tr("Use Cmd, Ctrl, Alt and Shift separated by +. Leave a field empty to disable it.")).FontSize(12)
		ui.Column(c).Padding(14).Radius(8).Background(c.Theme().Surface).Gap(8).Children(func() {
			ui.Text(c, a.tr("Global quick note")).Bold()
			ui.Text(c, a.tr("Works outside Mory. Disabled by default to avoid launcher conflicts.")).FontSize(12)
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				enabled := strings.TrimSpace(a.quickDraft) != ""
				if ui.Checkbox(c, &enabled, a.tr("Enable global quick note")).Changed() {
					if enabled {
						a.quickDraft = "Ctrl+Alt+Space"
					} else {
						a.quickDraft = ""
					}
				}
				input := ui.TextInput(c, &a.quickDraft).Label(a.tr("Global quick note shortcut"))
				input.Placeholder(a.tr("Enter a shortcut, for example: Ctrl+Alt+Space"))
				input.Disabled(!enabled).Grow(1)
			})
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Gap(6).Children(func() {
			for _, action := range a.actions() {
				ui.Row(c.Key(action.Name)).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, a.tr(action.Name)).Grow(1)
					value, ok := a.shortcutDraft[action.Name]
					if !ok {
						value = action.Key
					}
					if ui.TextInput(c, &value).Label(a.tr("Shortcut ") + a.tr(action.Name)).Width(190).Changed() {
						a.shortcutDraft[action.Name] = value
					}
				})
			}
		})
		ui.Row(c).Gap(12).Children(func() {
			if ui.Button(c, a.tr("Restore defaults")).Clicked() {
				a.shortcutDraft = map[string]string{}
				a.quickDraft = ""
			}
			if ui.PrimaryButton(c, a.tr("Save shortcuts")).Clicked() {
				if a.saveShortcuts() {
					a.helpOpen, a.settingsOpen = false, false
				}
			}
			if ui.Button(c, a.tr("Cancel")).Clicked() {
				a.helpOpen, a.settingsOpen = false, false
				a.shortcutDraft = nil
			}
		})
	})
}
