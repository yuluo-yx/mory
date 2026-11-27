package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestCommandSlashCyclesModesAndPreservesPendingEdits(t *testing.T) {
	for _, source := range []string{"Body 世界", "<p>Body 世界</p>", "```go\nBody 世界\n```\n"} {
		t.Run(source, func(t *testing.T) {
			a := testApp(t)
			a.createNote(source, "Cycle")
			tt := ui.NewTester(a.view, 1280, 900)
			label := "Document editor"
			if strings.HasPrefix(source, "<") {
				label = "Block text " + firstBlock(a, "p").ID
			} else if strings.HasPrefix(source, "```") {
				label = "Code editor"
			}
			click(t, tt, label)
			tt.Type("Pending ")
			for round := range 2 {
				for _, mode := range []string{"Source", "Read", "Split", "Write"} {
					tt.Key(ui.Cmd, ui.KeySlash)
					if a.mode != mode || a.helpOpen {
						t.Fatalf("Cmd+/ reached %s instead of %s; help=%t", a.mode, mode, a.helpOpen)
					}
					if mode == "Source" && round == 0 {
						if !tt.Focused("Source editor") {
							t.Fatal("source did not receive keyboard focus")
						}
						tt.Key(ui.Cmd, ui.KeyEnd)
						tt.Type("\n\nSource edit 世界")
					}
					if mode == "Split" && !tt.Focused("Source editor") {
						t.Fatal("split source did not receive keyboard focus")
					}
					a.capture()
					a.flushSource()
					if !strings.Contains(a.current.Source, "Pending") || !strings.Contains(a.current.Source, "Source edit 世界") {
						t.Fatal("cycling lost pending edits", a.current.Source)
					}
				}
				tt.Type("Continue ")
				a.capture()
				if !strings.Contains(a.current.Source, "Continue") {
					t.Fatal("returning to Write lost typing focus", a.current.Source)
				}
			}
			if !a.save() {
				t.Fatal(a.problem)
			}
			stored, err := library.Open(a.lib.Dir)
			if err != nil || stored.Get(a.current.ID).Source != a.current.Source {
				t.Fatal("cycled document did not persist", err)
			}
		})
	}
}

func TestModeCycleLegacyBindingAndNativeMenu(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Keys["Shortcuts"] = "Command+/"
	a.createNote("Body", "Cycle")
	tt := ui.NewTester(a.view, 1280, 900)
	tt.Key(ui.Cmd, ui.KeySlash)
	if a.mode != "Source" || a.helpOpen || a.shortcutHint("Shortcuts") != "" {
		t.Fatal("legacy help binding still owns Cmd+/")
	}
	keys, _, err := a.validateShortcuts(a.lib.Settings.Keys, "")
	if err != nil || keys["Shortcuts"] != "" || keys["Cycle editor mode"] != "Cmd+/" {
		t.Fatal("legacy settings cannot be saved without conflicts", keys, err)
	}
	if a.lib.Settings.Keys["Shortcuts"] != "Command+/" {
		t.Fatal("resolving defaults mutated stored settings")
	}
	for _, menu := range a.buildMenu(&mygo.MenuItem{Label: "Updates"}).Items() {
		for _, item := range menu.Submenu {
			if item.Label == "Cycle editor mode" {
				if item.Accelerator != "Cmd+/" {
					t.Fatal("native menu binding differs from local shortcut")
				}
				item.Click(item, nil)
				if a.mode != "Read" {
					t.Fatal("native menu did not cycle")
				}
			}
		}
	}
	a.lib.Settings.Keys["Cycle editor mode"] = "Ctrl+Alt+M"
	a.lib.Settings.Keys["Shortcuts"] = "Cmd+Shift+/"
	tt.Key(ui.Cmd, ui.KeySlash)
	if a.mode != "Read" || a.helpOpen {
		t.Fatal("rebound default remained active")
	}
	tt.Key(ui.Ctrl|ui.Alt, ui.KeyM)
	if a.mode != "Split" {
		t.Fatal("custom cycle shortcut did not run")
	}
	a.lib.Settings.Keys["Cycle editor mode"] = ""
	tt.Key(ui.Cmd, ui.KeySlash)
	if a.mode != "Split" || a.shortcutHint("Shortcuts") != "Cmd+Shift+/" {
		t.Fatal("disable or custom help binding was lost")
	}
}

func TestModeCycleGuards(t *testing.T) {
	a := testApp(t)
	a.cycleEditorMode()
	if a.mode != "Write" || a.helpOpen {
		t.Fatal("cycle without a document changed the workspace")
	}
	a.createNote("Body", "Cycle")
	for _, flag := range []*bool{&a.settingsOpen, &a.helpOpen, &a.cssOpen} {
		*flag = true
		a.cycleEditorMode()
		if a.mode != "Write" {
			t.Fatal("cycle escaped settings")
		}
		*flag = false
	}
	a.createNote(strings.Repeat("x", maxWriteLineBytes+1), "Long line")
	a.cycleEditorMode()
	if a.mode != "Source" || a.modeCycleFocus {
		t.Fatal("cycle bypassed the source-only guard")
	}
}
