package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/doctheme"
)

func TestDefaultApplicationSetting(t *testing.T) {
	old := setMarkdownDefault
	t.Cleanup(func() { setMarkdownDefault = old })
	a := testApp(t)
	a.settingsOpen = true
	tt := ui.NewTester(a.view, 1280, 820)
	calls := 0
	setMarkdownDefault = func() error { calls++; return nil }
	click(t, tt, "Make Mory the default Markdown app")
	if calls != 1 || !tt.HasText("Mory is now the default Markdown application.") {
		t.Fatal("missing completion feedback")
	}
	setMarkdownDefault = func() error { return errors.New("association rejected") }
	click(t, tt, "Make Mory the default Markdown app")
	if a.problem != "association rejected" {
		t.Fatal(a.problem)
	}
}

func TestDocusaurusComponentsAndTheme(t *testing.T) {
	a := testApp(t)
	source := "---\ntitle: Guide\n---\n\n# Guide {#guide}\n\n:::tip Handy\nHelpful text.\n:::\n\n<details><summary>More</summary>\n\nHidden detail.\n\n</details>\n\n<Tabs groupId=\"platform\"><TabItem value=\"mac\" label=\"macOS\" default>Mac instructions.</TabItem>\n<TabItem value=\"linux\" label=\"Linux\">Linux instructions.</TabItem></Tabs>\n"
	a.createNote(source, "Docusaurus guide")
	a.lib.Settings.Theme = "Docusaurus"
	tt := ui.NewTester(a.view, 1280, 1000)
	visibleText := func(text string) bool {
		found := false
		a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
			if blocks.IsText(n) && n.Plain() == text {
				_, found = tt.Find("Block text " + n.ID)
			}
		})
		return found
	}
	click(t, tt, "Toggle details More")
	tt.Frame()
	if !visibleText("Hidden detail.") {
		if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
			saveImage(t, tt, filepath.Join(dir, "docusaurus-detail.png"))
		}
		t.Fatalf("details did not open: %v", a.blockEditor.componentChoice)
	}
	click(t, tt, "Toggle details More")
	click(t, tt, "Tab Linux")
	tt.Frame()
	if !visibleText("Linux instructions.") {
		t.Fatal("tab did not switch")
	}
	if a.current.Source != source {
		t.Fatal("preview changed source")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "docusaurus.png"))
	}
	for _, mode := range []string{"Read", "Source", "Split", "Write"} {
		a.changeMode(mode)
		tt.Frame()
		if a.current.Source != source {
			t.Fatalf("mode %s changed source", mode)
		}
	}
	theme := doctheme.Parse(doctheme.Builtin("Docusaurus"))
	if len(theme.Warnings) != 0 {
		t.Fatal(theme.Warnings)
	}
}

func TestInsertMenuHoverAndSearch(t *testing.T) {
	a := testApp(t)
	a.createNote("Body", "Insert menu")
	a.insertOpen = true
	tt := ui.NewTester(a.view, 900, 700)
	r, ok := tt.Find("Insert Heading")
	if !ok {
		t.Fatal("missing menu item")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "insert-hover.png"))
	}
	a.insertQuery = "Document link"
	tt.Frame()
	if !tt.HasText("Document link") || tt.HasText("Heading") || tt.HasText("Columns") {
		t.Fatal("search did not filter every item")
	}
	a.insertQuery = "Columns"
	tt.Frame()
	click(t, tt, "Insert Columns")
	if a.insertOpen {
		t.Fatal("insert did not close menu")
	}
}

func TestChildNoteAndDocusaurusCodeEditing(t *testing.T) {
	a := testApp(t)
	a.createNote("Parent body", "Parent")
	parent := a.current
	parent.Folder = "Docs"
	a.createChildNote(parent)
	if a.current.Parent != parent.ID || a.current.Folder != "Docs" {
		t.Fatal("child lost hierarchy")
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
	const source = "```js title=\"demo.js\" {1} showLineNumbers\nlet value = 1;\n```\n"
	a.createNote(source, "Code metadata")
	tt := ui.NewTester(a.view, 1280, 820)
	if a.blockEditor == nil {
		t.Fatal("code metadata entered lossy rich editor")
	}
	var code *blocks.Node
	a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
		if n.Kind == "pre" {
			code = n
		}
	})
	field := a.blockEditor.fields[code.ID]
	if field == nil || len(field.code.LineBackgrounds) != 1 {
		t.Fatal("missing code highlighting")
	}
	field.code.Buffer.Set("let value = 2;\n")
	tt.Frame()
	a.captureBlocks()
	if !strings.Contains(a.current.Source, `title="demo.js" {1} showLineNumbers`) || !strings.Contains(a.current.Source, "let value = 2;") {
		t.Fatal(a.current.Source)
	}
}
