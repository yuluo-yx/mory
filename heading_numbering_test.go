package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestNextHeadingNumber(t *testing.T) {
	for _, tc := range []struct {
		name     string
		previous []numberedHeading
		level    int
		want     string
	}{
		{"no seed", []numberedHeading{{2, "Overview"}}, 3, ""},
		{"child", []numberedHeading{{2, "1. Overview"}}, 3, "1.1 "},
		{"sibling", []numberedHeading{{2, "1. Overview"}, {3, "1.1 First"}}, 3, "1.2 "},
		{"parent sibling", []numberedHeading{{2, "1. Overview"}, {3, "1.1 First"}}, 2, "2. "},
		{"reset children", []numberedHeading{{2, "1. Old"}, {3, "1.4 Old"}, {2, "2. New"}}, 3, "2.1 "},
		{"unnumbered section", []numberedHeading{{2, "1. Old"}, {2, "New"}}, 3, ""},
		{"skip level", []numberedHeading{{2, "1. Overview"}}, 4, "1.1.1 "},
		{"unicode space", []numberedHeading{{2, "9.\u00a0Overview"}}, 2, "10. "},
		{"no preceding headings", nil, 2, ""},
		{"higher than seed", []numberedHeading{{2, "1. Overview"}}, 1, ""},
		{"invalid number", []numberedHeading{{2, "1.x Overview"}}, 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextHeadingNumber(tc.previous, tc.level); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHeadingNumberReordering(t *testing.T) {
	for _, tc := range []struct {
		name     string
		headings []numberedHeading
		want     []string
	}{
		{"deleted first", []numberedHeading{{2, "2. Second"}, {2, "3. Third"}}, []string{"1.", "2."}},
		{"inserted sibling", []numberedHeading{{2, "1. Parent"}, {3, "1.1 New"}, {3, "1.1 Old"}, {2, "2. Next"}}, []string{"1.", "1.1", "1.2", "2."}},
		{"promoted child", []numberedHeading{{2, "1. Parent"}, {2, "1.1 Child"}, {3, "1.2 Descendant"}, {2, "2. Next"}}, []string{"1.", "2.", "2.1", "3."}},
		{"removed parent", []numberedHeading{{3, "1.2 Child"}, {3, "1.3 Sibling"}}, []string{"1.", "2."}},
		{"skipped levels", []numberedHeading{{2, "1. Parent"}, {4, "7.9 Deep"}, {4, "7.10 Sibling"}}, []string{"1.", "1.1.1", "1.1.2"}},
		{"unnumbered boundary", []numberedHeading{{2, "3. First"}, {2, "Plain"}, {3, "9. Restart"}}, []string{"1.", "", "1."}},
		{"unnumbered document", []numberedHeading{{1, "Plain"}, {2, "Child"}}, []string{"", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reorderedHeadingPrefixes(tc.headings); !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func currentHeadingTitles(a *app) []string {
	var titles []string
	if a.structured() {
		a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
			if headingLevel(n) > 0 {
				titles = append(titles, n.Plain())
			}
		})
	} else {
		for _, p := range a.doc.Text().Paragraphs() {
			if p.Style.Heading > 0 {
				titles = append(titles, pText(p))
			}
		}
	}
	return titles
}

func headingNodes(doc *blocks.Document) []*blocks.Node {
	var nodes []*blocks.Node
	doc.Root.Walk(func(n *blocks.Node) {
		if headingLevel(n) > 0 {
			nodes = append(nodes, n)
		}
	})
	return nodes
}

func TestHeadingReorderingEditsAndHistory(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, operation := range []string{"delete first", "insert middle", "promote", "demote"} {
			t.Run(fmt.Sprintf("%t/%s", structured, operation), func(t *testing.T) {
				a := testApp(t)
				a.createNote("## 1. First\n\n## 2. **Second** 世界\n\n## 3. Third", "Reordering")
				if structured && !a.enableBlocks() {
					t.Fatal(a.problem)
				}
				before := currentHeadingTitles(a)
				var want []string
				if structured {
					s := a.blockEditor
					children := headingNodes(s.doc)
					var err error
					switch operation {
					case "delete first":
						err = s.doc.Remove(children[0].ID)
					case "insert middle":
						parent, at := s.doc.Parent(children[1].ID)
						err = s.doc.Splice(parent.ID, at, 0, blocks.New("h2", blocks.Text("2. Inserted")))
					case "promote", "demote":
						kind := "h1"
						if operation == "demote" {
							kind = "h3"
						}
						err = s.doc.Transform(children[1].ID, func(n *blocks.Node) { n.Kind = kind })
						delete(s.fields, children[1].ID)
					}
					if err != nil {
						t.Fatal(err)
					}
				} else {
					doc := a.doc
					var starts []int
					for at, p := range doc.Text().Paragraphs() {
						if p.Style.Heading > 0 {
							starts = append(starts, at)
						}
					}
					switch operation {
					case "delete first":
						doc.SetSelection(0, starts[1])
						doc.InsertText("")
					case "insert middle":
						doc.Edit(func(e *richtext.Edit) { e.ReplaceString(starts[1], starts[1], "2. Inserted\n", richtext.Attributes{}) })
					case "promote", "demote":
						doc.SetSelection(starts[1], starts[1])
						level := 1
						if operation == "demote" {
							level = 3
						}
						doc.SetHeading(level)
					}
				}
				switch operation {
				case "delete first":
					want = []string{"1. Second 世界", "2. Third"}
				case "insert middle":
					want = []string{"1. First", "2. Inserted", "3. Second 世界", "4. Third"}
				case "promote":
					want = []string{"1.1 First", "2. Second 世界", "2.1 Third"}
				case "demote":
					want = []string{"1. First", "1.1 Second 世界", "2. Third"}
				}
				a.capture()
				if got := currentHeadingTitles(a); !slices.Equal(got, want) {
					t.Fatalf("got %v, want %v", got, want)
				}
				if !strings.Contains(a.current.Source, "**Second** 世界") {
					t.Fatal("lost inline formatting", a.current.Source)
				}
				if structured {
					a.blockUndo(false)
				} else {
					a.doc.Undo()
					a.capture()
				}
				if got := currentHeadingTitles(a); !slices.Equal(got, before) {
					t.Fatalf("undo: %v, want %v", got, before)
				}
				if structured {
					a.blockUndo(true)
				} else {
					a.doc.Redo()
					a.capture()
				}
				if got := currentHeadingTitles(a); !slices.Equal(got, want) {
					t.Fatalf("redo: %v, want %v", got, want)
				}
				if !a.save() {
					t.Fatal(a.problem)
				}
				a.loadEditors()
				if got := currentHeadingTitles(a); !slices.Equal(got, want) {
					t.Fatalf("reopen: %v, want %v", got, want)
				}
			})
		}
	}
}

func TestMovedAndFoldedHeadingNumbers(t *testing.T) {
	a := testApp(t)
	a.createNote("## 1. First\n\n### 1.1 Child\n\n## 2. Second\n\n## 3. Third", "Move headings")
	if !a.enableBlocks() {
		t.Fatal(a.problem)
	}
	s := a.blockEditor
	children := headingNodes(s.doc)
	a.toggleHeading(children[0])
	// Keep the folded child unloaded; normalization must still update it.
	delete(s.fields, children[1].ID)
	if err := s.doc.Move(children[3].ID, s.doc.Root.ID, 0); err != nil {
		t.Fatal(err)
	}
	a.capture()
	want := []string{"1. Third", "2. First", "2.1 Child", "3. Second"}
	if got := currentHeadingTitles(a); !slices.Equal(got, want) {
		t.Fatalf("move: %v", got)
	}
	a.blockUndo(false)
	if got := currentHeadingTitles(a); !slices.Equal(got, []string{"1. First", "1.1 Child", "2. Second", "3. Third"}) {
		t.Fatalf("undo move: %v", got)
	}
	a.blockUndo(true)
	if got := currentHeadingTitles(a); !slices.Equal(got, want) {
		t.Fatalf("redo move: %v", got)
	}
}

func TestHeadingPrefixCaretAndSelection(t *testing.T) {
	text := richtext.NewText("1.2.3 Title 世界")
	for _, tc := range []struct{ anchor, caret, wantAnchor, wantCaret int }{
		{0, 0, 0, 0}, {4, 4, 2, 2}, {5, 5, 2, 2}, {13, 6, 10, 3},
	} {
		got, anchor, caret := replaceHeadingPrefixes(text, []headingPrefixEdit{{0, 5, "2."}}, tc.anchor, tc.caret)
		if got.String() != "2. Title 世界" || anchor != tc.wantAnchor || caret != tc.wantCaret {
			t.Fatalf("%+v: %q %d %d", tc, got.String(), anchor, caret)
		}
	}
}

func TestHeadingNumberingPreservesImportedSpacing(t *testing.T) {
	text := richtext.TextOf(richtext.Paragraph{Style: richtext.ParagraphStyle{Heading: 2}, Runs: []richtext.Run{{Text: "  9.\u00a0Title"}}})
	edits := headingPrefixChanges(text, []string{"1."})
	got, _, _ := replaceHeadingPrefixes(text, edits, 0, 0)
	if got.String() != "  1.\u00a0Title" {
		t.Fatal(got.String())
	}
}

func BenchmarkHeadingReordering(b *testing.B) {
	headings := make([]numberedHeading, 10000)
	for i := range headings {
		headings[i] = numberedHeading{2 + i%3, "9. Imported heading"}
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = reorderedHeadingPrefixes(headings)
	}
}

func TestExistingParagraphBecomesNumberedHeading(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		a.createNote("## 1. Parent\n\nDraft", "Convert heading")
		label := "Document editor"
		if structured {
			if !a.enableBlocks() {
				t.Fatal(a.problem)
			}
			children := a.blockEditor.doc.Root.Children
			label = "Block text " + children[len(children)-1].ID
		}
		tt := ui.NewTester(a.view, 1280, 900)
		click(t, tt, label)
		tt.Key(ui.Cmd, ui.KeyEnd)
		a.setHeading(3)
		tt.Frame()
		if got := currentHeadingTitles(a); !slices.Equal(got, []string{"1. Parent", "1.1 Draft"}) {
			t.Fatalf("structured=%t: %v", structured, got)
		}
		tt.Type(" continued")
		a.capture()
		if got := currentHeadingTitles(a); !slices.Equal(got, []string{"1. Parent", "1.1 Draft continued"}) {
			t.Fatalf("caret: %v", got)
		}
	}
}

func TestAutomaticHeadingInput(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, toolbar := range []bool{false, true} {
			a := testApp(t)
			a.createNote("## 1. Overview\n\n### 1.1 First\n\nDraft", "Numbered headings")
			label := "Document editor"
			if structured {
				if !a.enableBlocks() {
					t.Fatal(a.problem)
				}
				root := a.blockEditor.doc.Root
				label = "Block text " + root.Children[len(root.Children)-1].ID
			}
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, label)
			tt.Key(ui.Cmd, ui.KeyEnd)
			for range 5 {
				tt.Key(0, ui.KeyBackspace)
			}
			if toolbar {
				a.setHeading(3)
				tt.Frame()
			} else {
				for _, r := range "### " {
					tt.Type(string(r))
				}
			}
			_, caret := a.doc.Selection()
			start, end := a.doc.Text().ParagraphRange(caret)
			if got := a.doc.Text().Slice(start, end).String(); got != "1.2 " {
				t.Fatalf("structured=%t toolbar=%t: %q", structured, toolbar, got)
			}
			if !structured && !toolbar {
				tt.Key(ui.Cmd, ui.KeyZ)
				if a.doc.Text().Slice(start, a.doc.Text().Len()).String() != "" {
					t.Fatal("undo did not remove the generated prefix")
				}
				tt.Frame()
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				if a.doc.Text().Slice(start, a.doc.Text().Len()).String() != "1.2 " {
					t.Fatal("redo did not restore the prefix")
				}
			}
			tt.Type("Second")
			a.capture()
			if !a.save() {
				t.Fatal(a.problem)
			}
			parsed := a.richText(a.current.Source)
			found := false
			for _, p := range parsed.Paragraphs() {
				if p.Style.Heading == 3 && pText(p) == "1.2 Second" {
					found = true
				}
			}
			if !found {
				t.Fatal("numbered heading did not persist", a.current.Source)
			}
		}
	}
}

func TestInsertedHeadingContinuesNumbers(t *testing.T) {
	a := testApp(t)
	a.createNote("## 1. First", "Insert numbered heading")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyEnd)
	click(t, tt, "Insert block")
	click(t, tt, "Insert Heading")
	if a.doc.Text().String() != "2. " {
		t.Fatalf("inserted heading: %q", a.doc.Text().String())
	}
	tt.Type("Next")
	a.capture()
	if a.doc.Text().String() != "2. Next" {
		t.Fatal("new heading did not retain typing focus", a.doc.Text().String())
	}
}
