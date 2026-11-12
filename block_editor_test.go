package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/doctheme"
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

func TestSplitSourceLines(t *testing.T) {
	for _, test := range []struct {
		format, source string
		want           []int
	}{
		{"markdown", "\n# Heading\n\nSame paragraph.\n\nSame paragraph.\n", []int{1, 3, 5, 7}},
		{"markdown", "# Heading\n\n```go\nvar x = 1\n```\n\n- one\n- two\n", []int{0, 2, 6, 9}},
		{"xml", "<title>Title</title>\n<h1>Heading</h1>\n<p>Same</p>\n<p>Same</p>", []int{1, 2, 3, 4}},
	} {
		t.Run(test.format+test.source[:1], func(t *testing.T) {
			d, err := blocks.Parse(test.source, test.format)
			if err != nil {
				t.Fatal(err)
			}
			var visible []*blocks.Node
			for _, n := range blocks.Visible(d.Root.Children) {
				if n.Kind != "title" {
					visible = append(visible, n)
				}
			}
			if got := splitSourceLines(test.source, test.format, visible); fmt.Sprint(got) != fmt.Sprint(test.want) {
				t.Fatalf("source lines = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSplitScrollFollowsBothPanes(t *testing.T) {
	for _, format := range []string{"markdown", "xml"} {
		t.Run(format, func(t *testing.T) {
			a := testApp(t)
			var source strings.Builder
			for i := range 35 {
				if format == "xml" {
					fmt.Fprintf(&source, "<h1>Section %d</h1>\n<p>%s</p>\n", i, strings.Repeat("Variable height paragraph. ", 1+i%8))
				} else {
					fmt.Fprintf(&source, "# Section %d\n\n%s\n\n", i, strings.Repeat("Variable height paragraph. ", 1+i%8))
				}
			}
			n, err := a.lib.Create("Scroll synchronization", source.String(), format)
			if err != nil {
				t.Fatal(err)
			}
			a.openNote(n)
			a.changeMode("Split")
			tt := ui.NewTester(a.view, 1280, 820)
			settle := func() {
				for range 8 {
					tt.Frame()
				}
			}
			settle()
			s := &a.blockEditor.split
			// Jump over unmeasured virtual rows, then into the middle of a block.
			for _, target := range []int{40, 12, 20} {
				y := a.code.LineTop(s.lines[target])
				if target == 20 {
					y += (a.code.LineTop(s.lines[target+1]) - y) * 0.4
				}
				a.code.SetScrollY(y)
				settle()
				r, ok := s.rows[target]
				if !ok || s.pending || s.scroll.Y <= 0 {
					t.Fatalf("source did not reveal preview block %d: rows=%v pending=%t scroll=%v", target, s.rows, s.pending, s.scroll.Y)
				}
				want := float64(s.viewport.Y)
				if target == 20 {
					want -= float64(r.H+7) * 0.4
				}
				if math.Abs(float64(r.Y)-want) > 1.5 || math.Abs(a.code.ScrollY()-y) > 0.5 {
					t.Fatalf("source/preview anchor drift: row=%+v viewport=%+v source=%v want=%v rowY=%v", r, s.viewport, a.code.ScrollY(), y, want)
				}
			}
			// A real wheel event in the preview drives the source in the other direction.
			before := a.code.ScrollY()
			tt.Scroll(s.viewport.X+40, s.viewport.Y+80, 0, 170)
			settle()
			if a.code.ScrollY() <= before {
				t.Fatal("preview scrolling did not move the source")
			}
			stableSource, stablePreview := a.code.ScrollY(), s.scroll.Y
			settle()
			if math.Abs(a.code.ScrollY()-stableSource) > 0.5 || math.Abs(float64(s.scroll.Y-stablePreview)) > 0.5 {
				t.Fatal("synchronized panes kept feeding scroll changes back to each other")
			}
			tt.Scroll(s.viewport.X+40, s.viewport.Y+80, 0, -100000)
			settle()
			if a.code.ScrollY() > 0.5 || s.scroll.Y > 0.5 {
				t.Fatalf("return to top did not synchronize: source=%v preview=%v", a.code.ScrollY(), s.scroll.Y)
			}
			// Wheel input on the source, not only API-driven jumps, moves the preview.
			ed, _ := tt.Find("Source editor")
			tt.Scroll(ed.X+100, ed.Y+80, 0, 300)
			settle()
			if a.code.ScrollY() <= 0 || s.scroll.Y <= 0 {
				t.Fatal("source wheel input did not synchronize")
			}
			// Read mode has its own remembered list position. Re-entering Split
			// must restore the source anchor instead of inheriting that position.
			a.changeMode("Read")
			tt.Frame()
			if a.blockEditor != nil {
				a.blockEditor.list.ScrollTo(55, ui.Start)
				tt.Frame()
			}
			a.changeMode("Split")
			settle()
			s = &a.blockEditor.split
			target := s.target
			if r, ok := s.rows[target]; !ok || r.Y > s.viewport.Y+1 || r.Y+r.H+7 < s.viewport.Y {
				t.Fatalf("re-entering split lost the source anchor: target=%d rows=%v", target, s.rows)
			}
			// Reparse after an edit while scrolled; then sync the freshly built rows.
			updated := source.String() + "\n"
			a.code.Buffer.Set(updated)
			settle()
			s = &a.blockEditor.split
			if a.current.Source != updated || s.pending || !s.initialized {
				t.Fatal("editing left the scroll mapping stale")
			}
			// A distant end jump must settle even when the last row cannot be top-aligned.
			a.code.SetScrollY(math.MaxFloat64)
			settle()
			if s.pending || s.scroll.Y <= 0 || math.Abs(float64(s.scroll.Y-s.scroll.MaxY)) > 1 {
				t.Fatalf("end jump did not settle: offset=%v max=%v pending=%t", s.scroll.Y, s.scroll.MaxY, s.pending)
			}
		})
	}
}

func TestDocusaurusCodeHighlights(t *testing.T) {
	for _, tc := range []struct {
		source, meta string
		want         []int
	}{
		{"a\nb\nc\nd", `title="test" {1,3-4} showLineNumbers`, []int{0, 2, 3}},
		{"a\nb\nc", `{0,2-1000000000,7-5, 1-}`, []int{1, 2}},
		{"// highlight-next-line\na\nb\n// highlight-start\nc\nd\n// highlight-end\ne", "", []int{1, 4, 5}},
		{"# highlight-next-line\na\n/* highlight-start */\nb\n<!-- highlight-end -->\nc", "", []int{1, 3}},
		{"const s = 'highlight-start';\na", "", nil},
	} {
		got := []int(nil)
		for _, mark := range docusaurusCodeHighlights(tc.source, tc.meta, ui.Hex("#123456")) {
			got = append(got, mark.First)
			if mark.Last != mark.First {
				t.Fatal(mark)
			}
		}
		if !slices.Equal(got, tc.want) {
			t.Fatalf("%q: %v != %v", tc.source, got, tc.want)
		}
	}
}

func TestDocusaurusAnchorNavigation(t *testing.T) {
	a := testApp(t)
	a.createNote("# Overview {#overview}\n\n## Target {#stable-id}\n\nBody", "Anchors")
	tt := ui.NewTester(a.view, 1280, 820)
	s := a.blockEditor
	a.toggleHeading(s.doc.Root.Children[0])
	a.openLink("#stable-id")
	tt.Frame()
	if s.headingFolded(s.doc.Root.Children[0]) || s.active == "" {
		t.Fatal("anchor did not reveal heading")
	}
	a.openLink("#missing")
	if a.preview.Anchor != "#missing" {
		t.Fatal("fallback missing")
	}
	a.createNote("<Tabs groupId=\"navigation\"><TabItem value=\"first\">First.</TabItem>\n<TabItem value=\"second\"><details><summary>Nested</summary>\n\n## Deep {#deep}\n\nTarget body.\n\n</details></TabItem></Tabs>", "Nested anchors")
	tt.Frame()
	a.openLink("#deep")
	tt.Frame()
	s = a.blockEditor
	if s.componentChoice["group:navigation"] != "second" {
		t.Fatal("anchor did not select containing tab")
	}
	var target *blocks.Node
	s.doc.Root.Walk(func(n *blocks.Node) {
		if n.Attr("heading-id") == "deep" {
			target = n
		}
	})
	if target == nil {
		t.Fatal("missing deep heading")
	}
	if _, ok := tt.Find("Block text " + target.ID); !ok {
		t.Fatal("anchor did not open details")
	}
}

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
