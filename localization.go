package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
)

// UI message keys stay in English; document contents and persisted enum values
// are never translated. MyGo supplies the matching native control translations.
//
//go:embed resources/locales/zh-CN.json
var chineseJSON []byte

var chineseMessages = func() map[string]string {
	var messages map[string]string
	if err := json.Unmarshal(chineseJSON, &messages); err != nil {
		panic(err)
	}
	return messages
}()

func (a *app) tr(message string) string {
	if a.lib.Settings.Language == "zh-CN" {
		if translated, ok := chineseMessages[message]; ok {
			return translated
		}
	}
	return message
}

func (a *app) trf(message string, values ...any) string { return fmt.Sprintf(a.tr(message), values...) }

func (a *app) dateText(value time.Time, withTime bool) string {
	layout := "Jan 2, 2006"
	if a.lib.Settings.Language == "zh-CN" {
		layout = "2006-01-02"
	}
	if withTime {
		layout += " 15:04"
	}
	return value.Format(layout)
}

func (a *app) statusText(message string) string {
	for _, prefix := range []string{"Exported ", "Editing original file · ", "Reminder: "} {
		if rest, ok := strings.CutPrefix(message, prefix); ok {
			return a.tr(prefix) + rest
		}
	}
	for _, format := range []string{"Imported %d documents and their metadata", "Imported %d documents"} {
		var count int
		if n, err := fmt.Sscanf(message, format, &count); n == 1 && err == nil && fmt.Sprintf(format, count) == message {
			return a.trf(format, count)
		}
	}
	return a.tr(message)
}

func (a *app) applyLocale() {
	locale := "en"
	if a.lib.Settings.Language == "zh-CN" {
		locale = "zh-Hans"
	}
	mygo.App.SetLocale(locale)
}

// SelectBase separates translated labels from the stable values stored on disk.
func (a *app) selectText(c *ui.Context, value *string, options []string) ui.Element {
	t := c.Theme()
	sel := ui.SelectBase(c, value)
	sel.Trigger.Padding(6, 10).Gap(8).Radius(6).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Text(c, a.tr(*value)).Grow(1)
		ui.Text(c, "⌄").TextColor(t.TextMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Padding(4).Radius(8).Background(t.Background).Border(1, t.Border)
		for _, option := range options {
			item := sel.Item(option).Padding(6, 10).Radius(4)
			if item.Highlighted() {
				item.Background(t.Accent).TextColor(t.AccentText)
			}
			item.Children(func() { ui.Text(c, a.tr(option)) })
		}
	})
	return sel.Trigger
}

// Only known UI message fragments are translated in diagnostic summaries.
func (a *app) localizedProblem(message string) string {
	if a.lib.Settings.Language != "zh-CN" {
		return message
	}
	for _, prefix := range []string{"Shortcut conflict: ", "global shortcut ", "shortcut needs Cmd, Ctrl or Alt: ", "invalid shortcut: ", "unknown modifier: ", "Enter a positive number for "} {
		if rest, ok := strings.CutPrefix(message, prefix); ok {
			return a.tr(prefix) + rest
		}
	}
	if text := a.tr(message); text != message {
		return text
	}
	return a.trf("Operation failed. Details: %s", message)
}
