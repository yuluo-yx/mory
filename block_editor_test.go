package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestNativeStructuredDocument(t *testing.T) {
	a := testApp(t)
	source := `<title icon="📘">Structured writing</title><h1>Rich documents</h1><p>Write <b>directly</b> in every block.</p><callout emoji="💡" background-color="light-yellow"><p>Keep the details that matter.</p></callout><grid><column width-ratio="0.5"><h2>Left column</h2><p>Ideas belong here.</p></column><column width-ratio="0.5"><h2>Right column</h2><p>And here, too.</p></column></grid><table><tr><th><p>Name</p></th><th><p>Status</p></th></tr><tr><td><p>Native editing</p></td><td><p>Ready</p></td></tr></table><latex display="true">\frac{a}{b}+\sqrt{x}</latex>`
	n, err := a.lib.Create("Structured writing", source, "xml")
	if err != nil {
		t.Fatal(err)
	}
	a.openNote(n)
	if a.blockEditor == nil || a.mode != "Write" {
		t.Fatal("missing native block editor")
	}
	tt := ui.NewTester(a.view, 1280, 900)
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveImage(t, tt, filepath.Join(dir, "structured.png"))
	}
	s := a.blockEditor
	var paragraph, table *blocks.Node
	s.doc.Root.Walk(func(n *blocks.Node) {
		if n.Kind == "p" && strings.HasPrefix(n.Plain(), "Write") {
			paragraph = n
		}
		if n.Kind == "table" {
			table = n
		}
	})
	if paragraph == nil || table == nil {
		t.Fatal("missing blocks")
	}
	field := a.blockField(paragraph, ui.Hex("#30362e"))
	field.doc.SetSelection(0, 0)
	field.doc.InsertText("Now ")
	a.capture()
	if !strings.Contains(a.current.Source, "Now Write") {
		t.Fatal(a.current.Source)
	}
	a.tableChange(table.ID, func(t *blocks.Table) error { return t.Merge(blocks.Rect{Top: 0, Left: 0, Bottom: 1, Right: 1}) })
	if !strings.Contains(a.current.Source, `rowspan="2"`) {
		t.Fatal("merge not persisted")
	}
	a.blockUndo(false)
	if strings.Contains(a.current.Source, `rowspan="2"`) {
		t.Fatal("merge undo failed")
	}
	a.blockUndo(true)
	if !strings.Contains(a.current.Source, `rowspan="2"`) {
		t.Fatal("merge redo failed")
	}
	a.changeMode("Source")
	a.changeMode("Write")
	if a.blockEditor == nil {
		t.Fatal("mode switch lost structured editor")
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
}

func TestNativeTableControls(t *testing.T) {
	a := testApp(t)
	a.createNote("| A | B |\n| --- | --- |\n| one | two |\n", "Table")
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Select cell 1 1")
	if err := tt.ClickWith(ui.Shift, "Select cell 2 2"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Merge cells")
	if !strings.Contains(a.current.Source, `colspan="2"`) {
		t.Fatal(a.current.Source)
	}
	click(t, tt, "Split cell")
	click(t, tt, "+ Row")
	click(t, tt, "+ Column")
	if a.problem != "" {
		t.Fatal(a.problem)
	}
}

func TestReferenceFixtureUnchanged(t *testing.T) {
	b, err := os.ReadFile("internal/blocks/testdata/all-blocks.xml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := blocks.Parse(string(b), "xml")
	if err != nil {
		t.Fatal(err)
	}
	if d.Source() != string(b) {
		t.Fatal("reference fixture changed")
	}
	a := testApp(t)
	n, err := a.lib.Create("Reference fixture", string(b), "xml")
	if err != nil {
		t.Fatal(err)
	}
	a.openNote(n)
	_ = ui.NewTester(a.view, 1280, 820)
	if a.problem != "" {
		t.Fatal(a.problem)
	}
	if !a.save() || a.current.Source != string(b) {
		t.Fatal("viewing changed the fixture")
	}
}
