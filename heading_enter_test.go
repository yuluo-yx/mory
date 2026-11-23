package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestHeadingStartEnterLayout(t *testing.T) {
	for _, mode := range []string{"rich", "blocks", "xml"} {
		for _, long := range []bool{false, true} {
			for _, target := range []string{"1. Section", "2. Next"} {
				t.Run(fmt.Sprintf("%s/long=%t/%s", mode, long, target), func(t *testing.T) {
					a := testApp(t)
					source := "## 1. Section\n\n- Parent\n    - Child\n\n## 2. Next\n\nBody"
					if long {
						source += strings.Repeat("\n\nMore body text.", 40)
					}
					format := "markdown"
					if mode == "xml" {
						root, err := blocks.ParseMarkdown(source)
						if err != nil {
							t.Fatal(err)
						}
						source, format = root.XML(), "xml"
					}
					note, err := a.lib.Create("Enter layout", source, format)
					if err != nil {
						t.Fatal(err)
					}
					a.openNote(note)
					a.outline = true
					if mode == "blocks" && !a.enableBlocks() {
						t.Fatal(a.problem)
					}
					label := "Document editor"
					if s := a.blockEditor; s != nil {
						s.doc.Root.Walk(func(n *blocks.Node) {
							if headingLevel(n) > 0 && n.Plain() == target {
								label = "Block text " + n.ID
							}
						})
					}
					tt := ui.NewTester(a.view, 1280, 1000)
					click(t, tt, label)
					at := strings.Index(a.doc.Text().String(), target)
					a.doc.SetSelection(at, at)
					tt.Frame()
					outlineBefore, ok := tt.Find("Heading " + target)
					if !ok {
						t.Fatal("missing outline target")
					}
					headingY := func() float32 {
						t.Helper()
						box, ok := tt.Find(label)
						if !ok {
							t.Fatal("heading editor lost its identity")
						}
						for _, p := range a.doc.VisibleParagraphs() {
							start, end := a.doc.Text().ParagraphRange(p.Start)
							if a.doc.Text().Slice(start, end).String() == target {
								return box.Y + p.Top
							}
						}
						t.Fatal("heading left the visible editor")
						return 0
					}
					beforeY := headingY()
					previousY := beforeY
					checkOutline := func() {
						t.Helper()
						after, ok := tt.Find("Heading " + target)
						if !ok || after.Y != outlineBefore.Y {
							t.Fatalf("outline moved: before=%+v after=%+v", outlineBefore, after)
						}
						if got := currentHeadingTitles(a); len(got) != 2 || got[0] != "1. Section" || got[1] != "2. Next" {
							t.Fatal("Enter changed headings", got)
						}
						if !tt.Focused(label) {
							t.Fatal("Enter moved focus away from the heading")
						}
					}
					for range 3 {
						tt.Key(0, ui.KeyEnter)
						for range 4 {
							tt.Frame()
						}
						checkOutline()
						y := headingY()
						if y <= previousY+20 {
							t.Fatalf("heading did not move down: before=%v after=%v", previousY, y)
						}
						previousY = y
					}
					for range 3 {
						tt.Key(ui.Cmd, ui.KeyZ)
					}
					for range 4 {
						tt.Frame()
					}
					checkOutline()
					if y := headingY(); y != beforeY {
						t.Fatalf("undo did not restore position: %v != %v", y, beforeY)
					}
					for range 3 {
						tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
					}
					for range 4 {
						tt.Frame()
					}
					checkOutline()
					if !a.save() {
						t.Fatal(a.problem)
					}
					reopened, err := library.Open(a.lib.Dir)
					if err != nil {
						t.Fatal(err)
					}
					a.openNote(reopened.Get(note.ID))
					tt.Frame()
					if a.blockEditor == nil {
						t.Fatal("empty paragraphs were not preserved")
					}
					blankCount := 0
					a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
						if n.Kind == "p" && n.Plain() == "" {
							blankCount++
						}
					})
					if blankCount != 3 {
						t.Fatalf("saved %d blank paragraphs, want 3: %q", blankCount, a.current.Source)
					}
					// Typing in a saved blank paragraph must remain ordinary body text.
					for _, n := range a.blockEditor.visibleBlocks() {
						if n.Kind == "p" && n.Plain() == "" {
							click(t, tt, "Block text "+n.ID)
							tt.Type("Introduction")
							break
						}
					}
					if len(currentHeadingTitles(a)) != 2 || !strings.Contains(a.current.Source, "Introduction") {
						t.Fatal("typing into the blank paragraph failed")
					}
				})
			}
		}
	}
}

func TestHeadingEnterOtherPositions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		at, end int
		mods    ui.Modifiers
		want    string
	}{
		{"middle", 2, 2, 0, "He\nading"},
		{"end", 7, 7, 0, "Heading\n"},
		{"selection", 0, 2, 0, "\nading"},
		{"soft break", 0, 0, ui.Shift, "\nHeading"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			a.createNote("## Heading", "Other Enter")
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, "Document editor")
			a.doc.SetSelection(tc.at, tc.end)
			tt.Frame()
			tt.Key(tc.mods, ui.KeyEnter)
			if got := a.doc.Text().String(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEmptyHeadingsDoNotOccupyOutline(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		const source = "##\n\n## \n\n##\n\n##\n\n## 1. Section\n\nBody\n\n## 2. Next"
		a.createNote(source, "Existing empty headings")
		if structured {
			a.enableBlocks()
		}
		a.outline = true
		tt := ui.NewTester(a.view, 1280, 1000)
		for _, mode := range []string{"Write", "Read", "Source"} {
			a.changeMode(mode)
			tt.Frame()
			count := len(a.headings)
			if a.structured() {
				count = len(a.structuredHeadings)
			}
			if count != 2 {
				t.Fatalf("empty headings occupy the outline in %s: %d entries", mode, count)
			}
			heading, ok := tt.Find("Heading 1. Section")
			caption, _ := tt.Find("ON THIS PAGE")
			if !ok || heading.Y-caption.Y > 40 {
				t.Fatalf("empty outline gap remains: heading=%+v caption=%+v", heading, caption)
			}
			if !a.save() || a.current.Source != source {
				t.Fatal("hiding empty outline items rewrote the source")
			}
		}
	}
}
