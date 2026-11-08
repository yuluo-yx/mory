package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestFormattingCommandRegistry(t *testing.T) {
	for _, name := range []string{"Bold", "Italic", "Underline", "Strikethrough", "Inline code", "Link", "Bullets", "Numbers", "Tasks", "Quote", "Code block", "Indent", "Outdent", "Align left", "Align center", "Align right", "Heading 0", "Heading 1", "Heading 6", "Heading 7", "Heading 8", "Heading 9", "Undo", "Redo", "Find", "Replace", "Write mode", "Read mode", "Source mode", "Split mode", "Outline", "Export", "Focus mode", "Settings", "Shortcuts", "Insert block", "Save", "Library"} {
		t.Run(name, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Editable text", "Commands")
			a.doc.SetSelection(0, a.doc.Text().Len())
			a.doc.InsertText("Changed text")
			a.doc.SetSelection(0, a.doc.Text().Len())
			a.capture()
			found := false
			for _, action := range a.actions() {
				if action.Name == name {
					found = true
					action.Run()
					break
				}
			}
			if !found {
				t.Fatal("missing command")
			}
			if a.problem != "" {
				t.Fatal(a.problem)
			}
			if isTextAction(name) && name != "Undo" && name != "Redo" && name != "Link" && a.current != nil && a.current.Source == "" {
				t.Fatal("formatting erased text")
			}
		})
	}
}

func TestInlineInsertionAndToolbarMenus(t *testing.T) {
	for _, kind := range []string{"latex", "time", "button", "doc-link"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Inline content", "Inline")
			a.doc.SetSelection(7, 7)
			a.insertInline(kind)
			tt := ui.NewTester(a.view, 1440, 1100)
			if kind == "latex" {
				click(t, tt, "Apply equation")
			} else {
				click(t, tt, "Apply properties")
			}
			marker := "<" + kind
			if kind == "latex" {
				marker = "$x^2$"
			}
			if !strings.Contains(a.current.Source, marker) {
				t.Fatal("inline object absent", a.current.Source)
			}
		})
	}
	a := testApp(t)
	a.createNote("Formatted", "Toolbar")
	tt := ui.NewTester(a.view, 1440, 1000)
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyA)
	for _, menu := range []struct{ button, item string }{{"Color", "red"}, {"Highlight", "light-yellow"}, {"Align", "Center"}, {"Color", "Default"}, {"Highlight", "None"}, {"Align", "Right"}} {
		click(t, tt, menu.button)
		if err := tt.ChooseMenuItem(menu.item); err != nil {
			t.Fatal(err)
		}
	}
	if a.doc.Formatting().Paragraph.Align != ui.End {
		t.Fatal("alignment not applied")
	}
	a.findText = "Formatted"
	a.replaceText = "Replaced"
	a.replaceAll()
	if !strings.Contains(a.current.Source, "Replaced") {
		t.Fatal("rich replace all")
	}
}

func TestNativeTableContextOperations(t *testing.T) {
	for _, action := range []string{"Move row up", "Move row down", "Move column left", "Move column right", "Merge selected cells", "Split cell", "Cell background"} {
		t.Run(action, func(t *testing.T) {
			table := blocks.NewTable(3, 3)
			model := blocks.ReadTable(table)
			model.At(1, 1).Node.Children = []*blocks.Node{blocks.Paragraph("Selected cell")}
			model.Write(table)
			a, tt := xmlApp(t, table.XML())
			click(t, tt, "Select cell 2 2")
			if err := tt.RightClick("Select cell 2 2"); err != nil {
				t.Fatal(err)
			}
			path := []string{action}
			if action == "Cell background" {
				path = append(path, "light-green")
			}
			if err := tt.ChooseMenuItem(path...); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a.problem != "" {
				t.Fatal(a.problem)
			}
			if !strings.Contains(a.current.Source, "Selected cell") {
				t.Fatal("table action lost content")
			}
		})
	}
	a, tt := xmlApp(t, blocks.NewTable(3, 3).XML())
	click(t, tt, "Select column B")
	if a.blockEditor.cells.Bottom != 2 {
		t.Fatal("column selection")
	}
	click(t, tt, "Table options")
	click(t, tt, "Cancel")
	table := firstBlock(a, "table")
	cell := blocks.ReadTable(table).At(0, 0).Node
	a.editBlockProperties(cell)
	tt.Frame()
	click(t, tt, "Apply properties")
}

func TestCropDraggingAndQuickLookThumbnail(t *testing.T) {
	a, tt := xmlApp(t, `<p>Picture</p>`)
	a.insertAsset(pngBytes(t), "pic.png")
	tt.Frame()
	click(t, tt, "Image size, crop & caption")
	for _, name := range []string{"Crop corner 1", "Crop corner 4"} {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatal(name)
		}
		x, y := r.X+r.W/2, r.Y+r.H/2
		tt.Press(x, y)
		tt.Move(x+12, y+8)
		tt.Release(x+12, y+8)
	}
	if a.blockEditor.props["image.crop"] == "" {
		t.Fatal("dragging did not set crop")
	}
	click(t, tt, "Apply properties")
	n := firstBlock(a, "img")
	if n.Attr("image.crop") == "" {
		t.Fatal("crop not saved")
	}
	path := n.Attr("href")
	thumbnail := quickLookThumbnail(a.lib.Dir, path)
	if len(thumbnail) == 0 {
		t.Fatal("macOS did not generate an image thumbnail")
	}
	if quickLookThumbnail(a.lib.Dir, "missing") != nil {
		t.Fatal("missing preview")
	}
	if quickLookThumbnail(filepath.Join(a.lib.Dir, "missing"), "file") != nil {
		t.Fatal("invalid preview root")
	}
	entries, _ := os.ReadDir(a.lib.Dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".preview-") {
			t.Fatal("thumbnail temporary files leaked")
		}
	}
}

func TestNestedAndUnknownNativeBlocks(t *testing.T) {
	source := `<title>All blocks</title><h7>Deep heading</h7><p>Parent<p>Child</p></p><ul><li>Bullet<ul><li>Nested</li></ul></li></ul><ol><li seq="3">Third</li><li>Fourth</li></ol><checkbox done="true">Done</checkbox><blockquote><p>Quoted</p></blockquote><hr/><sheet token="local"/><whiteboard type="blank"/><sub-page-list/><figure><source path="assets/a.pdf" name="PDF"/></figure><p><img path="missing"/><time time="12:00"/><cite/><button/><widget>Unknown</widget></p>`
	a, tt := xmlApp(t, source)
	tt.SetSize(1440, 2200)
	tt.Frame()
	if !tt.HasText("All blocks") {
		t.Fatal("document missing")
	}
	if a.current.Source != source {
		t.Fatal("rendering rewrote source")
	}
	a.changeMode("Read")
	tt.Frame()
	a.changeMode("Write")
	tt.Frame()
	p := firstBlock(a, "p")
	f := a.blockField(p, a.documentColor)
	f.doc.SetSelection(0, 0)
	a.blockEditor.active = p.ID
	a.insertInline("button")
	tt.Frame()
	click(t, tt, "Apply properties")
}
