package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestSplitPanesStayBelowDocumentHeader(t *testing.T) {
	for _, format := range []string{"markdown", "xml"} {
		for _, cover := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cover=%t", format, cover), func(t *testing.T) {
				a := testApp(t)
				source := "# Preview heading\n\n" + strings.Repeat("A paragraph in the preview.\n\n", 40)
				if format == "xml" {
					source = "<h1>Preview heading</h1>" + strings.Repeat("<p>A paragraph in the preview.</p>", 40)
				}
				n, err := a.lib.Create("Split preview", source, format)
				if err != nil {
					t.Fatal(err)
				}
				if cover {
					image, err := os.ReadFile("resources/icon.png")
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(a.lib.Dir, "assets", "cover.png"), image, 0o600); err != nil {
						t.Fatal(err)
					}
					n.Cover = "assets/cover.png"
				}
				a.openNote(n)
				a.changeMode("Split")
				tt := ui.NewTester(a.view, 1280, 820)
				for _, size := range [][2]int{{1000, 620}, {1280, 820}, {1600, 1000}} {
					tt.SetSize(size[0], size[1])
					tt.Frame()
					ed, ok := tt.Find("Source editor")
					if !ok || ed.W < 200 || ed.H < 100 {
						t.Fatalf("source editor has no usable viewport at %v: %+v", size, ed)
					}
					title, ok := tt.Find("Document title")
					if !ok || ed.Y < title.Y+title.H || ed.Y+ed.H > float32(size[1]-30) {
						t.Fatalf("source editor overlaps the header or status bar at %v: editor=%+v title=%+v", size, ed, title)
					}
					preview, ok := tt.Find("Block text " + a.blockEditor.doc.Root.Children[0].ID)
					if !ok || preview.X < ed.X+ed.W || preview.Y < ed.Y || preview.Y+preview.H > ed.Y+ed.H {
						t.Fatalf("preview escaped its pane at %v: editor=%+v preview=%+v", size, ed, preview)
					}
					if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
						saveImage(t, tt, filepath.Join(dir, fmt.Sprintf("split-%s-%t-%d.png", format, cover, size[0])))
					}
				}
				// Editing the visible source pane must refresh the adjacent rendered document.
				click(t, tt, "Source editor")
				tt.Key(ui.Cmd, ui.KeyA)
				updated := "# Updated preview\n\nEdited in split mode."
				if format == "xml" {
					updated = "<h1>Updated preview</h1><p>Edited in split mode.</p>"
				}
				tt.Type(updated)
				tt.Frame()
				tt.Frame()
				if a.current.Source != updated || !strings.Contains(a.blockEditor.doc.Root.Plain(), "Updated preview") {
					t.Fatal("split preview did not follow source edits")
				}
			})
		}
	}
}

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
