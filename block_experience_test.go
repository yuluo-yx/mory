package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/doctheme"
)

// Compare the editable hit region with the renderer, not a second copy of the
// sizing formula. The same text must stay reachable after resize and restyling.
func TestBlockTextFitsRenderedContent(t *testing.T) {
	for _, theme := range []string{"Craft", "Notion", "Feishu"} {
		for _, width := range []int{1000, 1440} {
			t.Run(fmt.Sprintf("%s/%d", theme, width), func(t *testing.T) {
				a, tt := xmlApp(t, `<h1>万卡集群从开始到运行起来模型需要什么？</h1><p>Follow-up paragraph.</p>`)
				a.docTheme = doctheme.Parse(doctheme.Builtin(theme))
				tt.SetSize(width, 900)
				tt.Frame()
				h := firstBlock(a, "h1")
				label := "Block text " + h.ID
				for _, size := range []int{width, 1600, width} {
					tt.SetSize(size, 900)
					tt.Frame()
					r, ok := tt.Find(label)
					boxes := a.blockEditor.fields[h.ID].doc.VisibleParagraphs()
					if !ok || len(boxes) != 1 || r.H < boxes[0].Height || r.H-boxes[0].Height > 20 {
						t.Fatalf("heading hit region does not fit its text: rect=%+v paragraphs=%+v", r, boxes)
					}
					p, _ := tt.Find("Block text " + firstBlock(a, "p").ID)
					if p.Y < r.Y+r.H || p.Y-r.Y-r.H > 12 {
						t.Fatalf("body overlaps heading or has a dead gap: heading=%+v body=%+v", r, p)
					}
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && theme == "Craft" && width == 1440 {
					r, _ := tt.Find(label)
					tt.Move(r.X+20, r.Y+10)
					saveImage(t, tt, filepath.Join(dir, "editing-heading.png"))
				}
				click(t, tt, label)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("Short")
				tt.Frame()
				r, _ := tt.Find(label)
				boxes := a.blockEditor.fields[h.ID].doc.VisibleParagraphs()
				if math.Abs(float64(r.H-boxes[0].Height)) > 20 {
					t.Fatal("shortening a heading left a stale hit region", r, boxes)
				}
			})
		}
	}
}

func TestHeadingActionsStaySeparateFromFolding(t *testing.T) {
	a, tt := xmlApp(t, `<h1>Section</h1><p>Keep this body</p><h1>Next</h1>`)
	heading := firstBlock(a, "h1")
	r, _ := tt.Find("Block text " + heading.ID)
	tt.Move(r.X+10, r.Y+10)
	handle, ok := tt.Find("Block actions h1")
	if !ok {
		t.Fatal("heading has no discoverable block action handle")
	}
	fold, ok := tt.Find("Collapse section Section")
	if !ok || handle.X+handle.W > fold.X && fold.X+fold.W > handle.X {
		t.Fatalf("heading actions overlap folding: handle=%+v fold=%+v", handle, fold)
	}
	if err := tt.RightClick("Block actions h1"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Delete block"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Continue ")
	a.capture()
	if strings.Contains(a.current.Source, "Section") || !strings.Contains(a.current.Source, "Keep this body") || !strings.Contains(a.current.Source, "Continue") {
		t.Fatal("deleting the heading lost its body or typing focus", a.current.Source)
	}
}

func TestHoverDoesNotPaintDocumentAsSelected(t *testing.T) {
	for _, appearance := range []string{"Light", "Dark"} {
		t.Run(appearance, func(t *testing.T) {
			a, tt := xmlApp(t, `<h1>Heading</h1><p>Body</p>`)
			a.lib.Settings.Appearance = appearance
			tt.Frame()
			label := "Block text " + firstBlock(a, "h1").ID
			r, _ := tt.Find(label)
			x, y := int(r.X+r.W-25), int(r.Y+8)
			tt.Move(0, 0)
			before := tt.Image().RGBAAt(x, y)
			tt.Move(r.X+30, r.Y+10)
			after := tt.Image().RGBAAt(x, y)
			if before != after || a.blockEditor.chosen.Len() != 0 {
				t.Fatalf("hover looks like block selection: before=%v after=%v", before, after)
			}
			click(t, tt, "Block actions h1")
			r, _ = tt.Find(label)
			selected := tt.Image().RGBAAt(int(r.X+r.W-25), int(r.Y+8))
			if selected == before || a.blockEditor.chosen.Len() != 1 || !tt.HasText("Delete selected blocks") {
				t.Fatal("explicit block selection is not distinguishable from hover")
			}
			click(t, tt, label)
			if a.blockEditor.chosen.Len() != 0 {
				t.Fatal("editing text retained a block selection")
			}
		})
	}
}

func TestMarkdownBoundaryDeletionAndUndo(t *testing.T) {
	for _, forward := range []bool{false, true} {
		t.Run(fmt.Sprint(forward), func(t *testing.T) {
			a := testApp(t)
			const source = "First\n\nSecond\n\n```go\nkeep\n```\n"
			a.createNote(source, "Boundary deletion")
			tt := ui.NewTester(a.view, 1280, 900)
			var target string
			for _, n := range a.blockEditor.visibleBlocks() {
				if n.Plain() == "Second" && !forward || n.Plain() == "First" && forward {
					target = "Block text " + n.ID
				}
			}
			click(t, tt, target)
			if forward {
				tt.Key(ui.Cmd, ui.KeyEnd)
				tt.Key(0, ui.KeyDelete)
			} else {
				tt.Key(ui.Cmd, ui.KeyHome)
				tt.Key(0, ui.KeyBackspace)
			}
			tt.Type(" joined ")
			a.capture()
			if !strings.Contains(a.current.Source, "First joined Second") || firstCodeBlock(t, a).Plain() != "keep\n" {
				t.Fatal("boundary deletion did not join the adjacent text", a.current.Source)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Key(ui.Cmd, ui.KeyZ)
			a.capture()
			if a.current.Source != source {
				t.Fatal("undo did not restore exact separators", a.current.Source)
			}
		})
	}
}

func TestEmptyParagraphAfterCodeCanBeRemoved(t *testing.T) {
	a := testApp(t)
	const source = "```go\nkeep\n```\n"
	a.createNote(source, "Empty paragraph")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Write below code")
	tt.Key(0, ui.KeyBackspace)
	if !tt.Focused("Code editor") || firstCodeBlock(t, a).Plain() != "keep\n" {
		t.Fatal("backspace did not return from empty paragraph to code")
	}
	for _, n := range a.blockEditor.visibleBlocks() {
		if n.Kind == "p" {
			t.Fatal("empty paragraph survived boundary deletion")
		}
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if !strings.Contains(a.current.Source, "keep") {
		t.Fatal("undo removed code")
	}
}

func TestModifiedArrowsStayInTextEditor(t *testing.T) {
	a, tt := xmlApp(t, `<p>Before</p><p>After</p>`)
	p := firstBlock(a, "p")
	label := "Block text " + p.ID
	click(t, tt, label)
	tt.Key(ui.Cmd, ui.KeyEnd)
	tt.Key(ui.Shift, ui.KeyRight)
	if !tt.Focused(label) {
		t.Fatal("extending a text selection moved into another block")
	}
}
