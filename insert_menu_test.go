package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

var insertOptionNames = []string{"Heading", "Bullet list", "To-do list", "Quote", "Code block", "Callout", "Table", "Math", "Divider", "Image or attachment…", "Columns", "Countdown", "Inline equation", "Reminder", "Button", "Document link", "Child pages"}

func requireInsertSelection(t *testing.T, a *app, tt *ui.Tester, want string) {
	t.Helper()
	for _, name := range insertOptionNames {
		label := a.tr("Insert ") + name
		if name == "Image or attachment…" {
			label = a.tr(name)
		}
		info, ok := tt.Access(label)
		if name == want {
			if !ok || !info.Selected || info.Offscreen || info.Bounds.H < 35 {
				t.Fatalf("%s is not selected and fully visible: %+v", name, info)
			}
		} else if ok && info.Selected {
			t.Fatalf("%s remained selected alongside %s", name, want)
		}
	}
	if !tt.Focused(a.tr("Search blocks")) {
		t.Fatal("navigation took focus away from search")
	}
}

func TestInsertMenuKeyboardCanReachFirstOption(t *testing.T) {
	a := testApp(t)
	a.createNote("", "Keyboard insertion")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Insert block")
	// Park the pointer over Math, matching the reported stuck highlight.
	r, ok := tt.Find("Insert Math")
	if !ok {
		t.Fatal("missing Math option")
	}
	tt.Move(r.X+20, r.Y+r.H/2)
	for range 7 {
		tt.Key(0, ui.KeyUp)
	}
	tt.Key(0, ui.KeyEnter)
	if a.insertOpen || a.blockEditor == nil || firstBlock(a, "h2") == nil {
		t.Fatal("Up could not reach and insert Heading from Math")
	}
	tt.Type("Authored heading")
	a.capture()
	if firstBlock(a, "h2").Plain() != "Authored heading" {
		t.Fatal("keyboard insertion lost the editing caret")
	}
}

func TestInsertMenuTraversesEveryOptionAndScrollsBack(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		t.Run(language, func(t *testing.T) {
			a := testApp(t)
			a.lib.Settings.Language = language
			a.createNote("", "All insert options")
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, a.tr("Insert block"))
			for _, name := range insertOptionNames {
				tt.Key(0, ui.KeyDown)
				requireInsertSelection(t, a, tt, name)
			}
			for i := len(insertOptionNames) - 2; i >= 0; i-- {
				tt.Key(0, ui.KeyUp)
				requireInsertSelection(t, a, tt, insertOptionNames[i])
			}
			tt.Key(0, ui.KeyUp)
			requireInsertSelection(t, a, tt, "Child pages")
			tt.Key(0, ui.KeyDown)
			requireInsertSelection(t, a, tt, "Heading")
		})
	}
}

func TestInsertMenuFiltersWhileNavigating(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Language = "zh-CN"
	a.createNote("", "Search insert options")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, a.tr("Insert block"))
	tt.Key(0, ui.KeyUp)
	requireInsertSelection(t, a, tt, "Child pages")
	tt.Type("列表")
	requireInsertSelection(t, a, tt, "Bullet list")
	tt.Key(0, ui.KeyDown)
	requireInsertSelection(t, a, tt, "To-do list")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("  CODE  ")
	requireInsertSelection(t, a, tt, "Code block")
	tt.Key(0, ui.KeyUp)
	requireInsertSelection(t, a, tt, "Code block")
	tt.Key(0, ui.KeyEnter)
	tt.Type("authored code")
	a.capture()
	if a.insertOpen || firstCodeBlock(t, a).Plain() != "authored code" {
		t.Fatal("Enter did not insert the filtered option with a live caret", a.current.Source)
	}
}

func TestInsertMenuNoResultsAndReopen(t *testing.T) {
	a := testApp(t)
	a.createNote("Keep body", "No results")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Insert block")
	tt.Type("no-such-block")
	for _, key := range []ui.Key{ui.KeyUp, ui.KeyDown, ui.KeyEnter} {
		tt.Key(0, key)
	}
	if !a.insertOpen || !tt.HasText("No matching blocks") || a.current.Source != "Keep body" {
		t.Fatal("empty search inserted content or closed the menu")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	requireInsertSelection(t, a, tt, "Heading")
	tt.Key(0, ui.KeyUp)
	requireInsertSelection(t, a, tt, "Child pages")
	tt.Key(0, ui.KeyEscape)
	if a.insertOpen {
		t.Fatal("Escape did not dismiss the menu")
	}
	click(t, tt, "Insert block")
	tt.Key(0, ui.KeyDown)
	requireInsertSelection(t, a, tt, "Heading")
	tt.Type("quote")
	tt.Key(0, ui.KeyEnter)
	tt.Type("Quoted")
	a.capture()
	if !strings.Contains(a.current.Source, "> Quoted") || !strings.Contains(a.current.Source, "Keep body") {
		t.Fatal("reopened menu lost its selection or document text", a.current.Source)
	}
}

func TestInsertMenuMouseTakesOverKeyboardSelection(t *testing.T) {
	a := testApp(t)
	a.createNote("", "Pointer selection")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Insert block")
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	r, _ := tt.Find("Insert Callout")
	tt.Move(r.X+20, r.Y+r.H/2)
	requireInsertSelection(t, a, tt, "Callout")
	tt.Key(0, ui.KeyUp)
	requireInsertSelection(t, a, tt, "Code block")
	click(t, tt, "Insert Callout")
	tt.Type("Pointer content")
	a.capture()
	if a.insertOpen || firstBlock(a, "callout").Plain() != "Pointer content" {
		t.Fatal("click inserted the keyboard option instead of the clicked one")
	}
}

func TestInsertMenuSlashKeyboardInsertion(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		a.createNote("Draft", "Slash keyboard")
		label := "Document editor"
		if structured {
			if !a.enableBlocks() {
				t.Fatal(a.problem)
			}
			label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
		}
		tt := ui.NewTester(a.view, 1280, 900)
		click(t, tt, label)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("/")
		tt.Type("quote")
		requireInsertSelection(t, a, tt, "Quote")
		tt.Key(0, ui.KeyEnter)
		tt.Type("Authored quote")
		a.capture()
		if a.insertOpen || strings.Contains(a.current.Source, "/") || !strings.Contains(a.current.Source, "> Authored quote") {
			t.Fatal("keyboard slash insertion kept the slash or lost focus", a.current.Source)
		}
	}
}
