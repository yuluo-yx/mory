package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestCodeBoundaryArrowsEnterAdjacentText(t *testing.T) {
	for _, key := range []ui.Key{ui.KeyDown, ui.KeyRight} {
		a := testApp(t)
		a.createNote("## Before\n\n```go\n世界\n```\n\n## After\n", "Boundary")
		tt := ui.NewTester(a.view, 1280, 1000)
		code := firstCodeBlock(t, a)
		click(t, tt, "Code editor")
		field := a.blockEditor.fields[code.ID]
		end := len([]rune(field.code.Buffer.String()))
		field.code.Select(end, end)
		tt.Key(0, key)
		tt.Type("New ")
		a.capture()
		if !strings.Contains(a.current.Source, "## New After") || code.Plain() != "世界\n" {
			t.Fatal("arrow failed to leave code for the next block", a.current.Source)
		}
		a.doc.SetSelection(0, 0)
		tt.Key(0, ui.KeyUp)
		if !tt.Focused("Code editor") {
			t.Fatal("up from following text skipped the code block")
		}
		click(t, tt, "Code editor")
		field.code.Select(0, 0)
		tt.Key(0, ui.KeyUp)
		tt.Type(" more")
		a.capture()
		if !strings.Contains(a.current.Source, "## Before more") {
			t.Fatal("up did not enter previous text", a.current.Source)
		}
	}
}

func TestCodeAtDocumentEndCreatesWritableParagraph(t *testing.T) {
	a := testApp(t)
	const original = "```go\npackage main\n```\n"
	a.createNote(original, "Last code")
	tt := ui.NewTester(a.view, 1280, 1000)
	code := firstCodeBlock(t, a)
	click(t, tt, "Code editor")
	f := a.blockEditor.fields[code.ID]
	end := len([]rune(f.code.Buffer.String()))
	f.code.Select(end, end)
	tt.Key(0, ui.KeyDown)
	tt.Type("Outside")
	tt.Type(" code 世界")
	a.capture()
	if code.Plain() != "package main\n" || !strings.Contains(a.current.Source, "```\nOutside code 世界") {
		t.Fatal("typing stayed in code", a.current.Source)
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
	a.blockUndo(false)
	a.blockUndo(false)
	if a.current.Source != original {
		t.Fatalf("exit cannot be undone independently: %q", a.current.Source)
	}
}

func TestClickBelowCodeInsertsBeforeHeadingAndReusesEmptyParagraph(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		a := testApp(t)
		a.lib.Settings.Language = language
		a.createNote("```go\ncontent\n```\n\n## Keep heading\n", "Mouse exit")
		tt := ui.NewTester(a.view, 1280, 1000)
		code := firstCodeBlock(t, a)
		click(t, tt, a.tr("Write below code"))
		count := len(a.blockEditor.doc.Root.Children)
		click(t, tt, a.tr("Write below code"))
		if len(a.blockEditor.doc.Root.Children) != count {
			t.Fatal("repeated click created duplicate empty paragraphs")
		}
		tt.Type("Paragraph")
		tt.Type(" below")
		a.capture()
		if code.Plain() != "content\n" || !strings.Contains(a.current.Source, "Paragraph below") || !strings.Contains(a.current.Source, "## Keep heading") {
			t.Fatal(a.current.Source)
		}
		if strings.Index(a.current.Source, "Paragraph below") > strings.Index(a.current.Source, "## Keep heading") {
			t.Fatal("paragraph was inserted after the heading")
		}
		a.loadEditors()
		tt.Frame()
		if !strings.Contains(a.blockEditor.doc.Source(), "Paragraph below") {
			t.Fatal("reopening lost inserted paragraph")
		}
	}
}

func TestCodeEnterAndSelectionStayInsideCode(t *testing.T) {
	a := testApp(t)
	a.createNote("```go\nfirst\nsecond\n```\n", "Code navigation")
	tt := ui.NewTester(a.view, 1280, 1000)
	code := firstCodeBlock(t, a)
	click(t, tt, "Code editor")
	f := a.blockEditor.fields[code.ID]
	f.code.Select(2, 2)
	tt.Key(0, ui.KeyDown)
	if !tt.Focused("Code editor") {
		t.Fatal("arrow in code moved out too early")
	}
	end := len([]rune(f.code.Buffer.String()))
	f.code.Select(end, end)
	tt.Key(0, ui.KeyEnter)
	tt.Type("third")
	a.capture()
	if !strings.Contains(firstCodeBlock(t, a).Plain(), "third") || !tt.Focused("Code editor") {
		t.Fatal("Enter exited code")
	}
	end = len([]rune(f.code.Buffer.String()))
	f.code.Select(end, end)
	tt.Key(ui.Shift, ui.KeyRight)
	if !tt.Focused("Code editor") {
		t.Fatal("selection shortcut exited code")
	}
	f.code.SetSelections(editor.Selection{Anchor: 0, Head: 0}, editor.Selection{Anchor: end, Head: end})
	tt.Key(0, ui.KeyDown)
	if !tt.Focused("Code editor") {
		t.Fatal("multiple cursors exited code")
	}
}

func TestNestedCodeExitStaysInContainerAndReadOnlyHasNoEntry(t *testing.T) {
	a, tt := xmlApp(t, `<callout><pre lang="go"><code>x</code></pre></callout><h2>After</h2>`)
	code := firstCodeBlock(t, a)
	parent, _ := a.blockEditor.doc.Parent(code.ID)
	click(t, tt, "Write below code")
	tt.Type("Nested paragraph")
	a.capture()
	var paragraph *blocks.Node
	for _, n := range parent.Children {
		if n.Kind == "p" {
			paragraph = n
		}
	}
	if paragraph == nil || paragraph.Plain() != "Nested paragraph" {
		t.Fatal("code exit escaped the container", a.current.Source)
	}
	a.changeMode("Read")
	tt.Frame()
	if _, ok := tt.Access("Write below code"); ok {
		t.Fatal("read-only code offers insertion")
	}
	before := a.current.Source
	if a.leaveCode(code.ID, 1, true) || a.current.Source != before {
		t.Fatal("read-only exit changed source")
	}
}

func TestCodeExitKeepsClosingFenceSeparateOnReopen(t *testing.T) {
	for _, source := range []string{"```go\nx\n```", "```go\nx\n```\n", "```go\nx\n```\n\n## After\n"} {
		a := testApp(t)
		a.createNote(source, "Fence separator")
		tt := ui.NewTester(a.view, 1280, 1000)
		click(t, tt, "Write below code")
		tt.Type("Outside code")
		if !a.save() {
			t.Fatal(a.problem)
		}
		a.loadEditors()
		tt.Frame()
		if got := firstCodeBlock(t, a).Plain(); got != "x\n" {
			t.Fatalf("prose was parsed as code: %q", got)
		}
		var found bool
		a.blockEditor.doc.Root.Walk(func(n *blocks.Node) { found = found || n.Kind == "p" && n.Plain() == "Outside code" })
		if !found {
			t.Fatal("reopened document lost the ordinary paragraph", a.current.Source)
		}
	}
}
