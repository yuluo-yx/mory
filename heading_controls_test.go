package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestEditingBesideFoldedSectionPreservesHiddenText(t *testing.T) {
	a := testApp(t)
	a.createNote("# Parent\n\n**Hidden formatting.**\n\n# Next\n\nVisible paragraph.\n", "Folded editing")
	tt := ui.NewTester(a.view, 1280, 820)
	a.foldRichHeading(0)
	tt.Frame()
	s := a.blockEditor
	for _, n := range s.doc.Root.Children {
		if n.Plain() == "Visible paragraph." {
			field := a.blockField(n, a.documentColor)
			field.doc.SetSelection(field.doc.Text().Len(), field.doc.Text().Len())
			field.doc.InsertText(" Edited.")
			break
		}
	}
	a.capture()
	if !strings.Contains(a.current.Source, "**Hidden formatting.**") || !strings.Contains(a.current.Source, "Visible paragraph. Edited.") || strings.Contains(a.current.Source, "folded=") {
		t.Fatal("editing an expanded section damaged hidden content", a.current.Source)
	}
	a.changeMode("Source")
	if !strings.Contains(a.code.Buffer.String(), "**Hidden formatting.**") {
		t.Fatal("source mode lost hidden content")
	}
}

func TestHeadingHoverAndNestedFolding(t *testing.T) {
	a := testApp(t)
	const source = "# Parent\n\nParent body.\n\n## Child\n\nChild body.\n\n# Next\n\nNext body.\n"
	a.createNote(source, "Heading controls")
	a.outline = false
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	ed, _ := tt.Find("Document editor")
	for _, box := range a.doc.VisibleParagraphs() {
		if a.doc.Text().ParagraphStyleAt(box.Start).Heading == 2 {
			tt.Move(ed.X+60, ed.Y+box.Top+5)
			break
		}
	}
	tt.Frame()
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "heading-hover.png"))
	}
	click(t, tt, "Collapse section Child")
	tt.Frame()
	s := a.blockEditor
	if s == nil {
		t.Fatal("folding did not enter the block presentation")
	}
	byText := map[string]*blocks.Node{}
	for _, n := range blocks.Visible(s.doc.Root.Children) {
		byText[n.Plain()] = n
	}
	if _, shown := tt.Find("Block text " + byText["Child body."].ID); shown {
		t.Fatal("collapsed child content is still rendered")
	}
	parent, _ := tt.Find("Block text " + byText["Parent"].ID)
	tt.Move(parent.X+50, parent.Y+8)
	tt.Frame()
	click(t, tt, "Collapse section Parent")
	tt.Frame()
	if _, shown := tt.Find("Block text " + byText["Child"].ID); shown {
		t.Fatal("collapsed parent still shows child headings")
	}
	if _, shown := tt.Find("Block text " + byText["Next"].ID); !shown {
		t.Fatal("collapse hid the following peer section")
	}
	if a.current.Source != source || a.dirty {
		t.Fatal("folding changed the document instead of its presentation")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "heading-folded.png"))
	}
	click(t, tt, "Expand section Parent")
	tt.Frame()
	if !s.headingFolded(byText["Child"]) {
		t.Fatal("expanding the parent discarded the child's fold state")
	}
	click(t, tt, "Expand section Child")
	tt.Frame()
	if a.blockEditor != nil || a.current.Source != source || a.doc.Text().String() != "Parent\nParent body.\nChild\nChild body.\nNext\nNext body." {
		t.Fatal("expanding all headings did not restore the complete editor")
	}
}

func TestHeadingFoldPreservesSourceAcrossModes(t *testing.T) {
	a := testApp(t)
	const source = "# Parent\n\n**Keep this formatting.**\n\n# Next\n\nLast section.\n"
	a.createNote(source, "Folded modes")
	tt := ui.NewTester(a.view, 1280, 820)
	a.foldRichHeading(0)
	tt.Frame()
	for _, mode := range []string{"Source", "Split", "Read", "Write"} {
		a.changeMode(mode)
		tt.Frame()
		if a.current.Source != source {
			t.Fatalf("%s lost content from the collapsed section", mode)
		}
	}
	if a.doc.Text().String() != "Parent\nKeep this formatting.\nNext\nLast section." {
		t.Fatal("mode switching restored only the active heading")
	}
}

func TestXMLHeadingFoldDoesNotRewriteDocument(t *testing.T) {
	a, tt := xmlApp(t, "<h1>Parent</h1><p>Hidden</p><h2>Child</h2><p>Nested</p><h1>Next</h1><p>Visible</p>")
	source := a.current.Source
	s := a.blockEditor
	parent := s.doc.Root.Children[0]
	field, _ := tt.Find("Block text " + parent.ID)
	tt.Move(field.X+30, field.Y+8)
	tt.Frame()
	click(t, tt, "Collapse section Parent")
	tt.Frame()
	if _, shown := tt.Find("Block text " + s.doc.Root.Children[1].ID); shown {
		t.Fatal("XML section did not collapse")
	}
	click(t, tt, "Expand section Parent")
	tt.Frame()
	if a.current.Source != source || parent.Attr("folded") != "" || a.dirty {
		t.Fatal("fold controls serialized view state into XML")
	}
	a.toggleHeading(parent)
	a.focusBlock(s.doc.Root.Children[2].ID, 0)
	tt.Frame()
	if s.headingFolded(parent) {
		t.Fatal("outline navigation did not reveal a hidden heading")
	}
}
