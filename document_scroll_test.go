package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDocumentScrollbarClearOfText(t *testing.T) {
	for _, appearance := range []string{"Light", "Dark"} {
		t.Run(appearance, func(t *testing.T) {
			a := testApp(t)
			a.lib.Settings.Appearance = appearance
			a.outline = false
			a.createNote(strings.Repeat("平台原生，体验好，跨平台能力为 0；\n\n", 90), "Scroll layout")
			tt := ui.NewTester(a.view, 1280, 820)
			click(t, tt, "Document editor")
			a.doc.SetSelection(0, 0)
			tt.Frame()
			title, _ := tt.Find("Document title")
			ed, _ := tt.Find("Document editor")
			caret, ok := a.doc.CaretRect()
			if !ok || math.Abs(float64(ed.X+caret.X-title.X)) > 1 || math.Abs(float64(ed.W-2*documentScrollGutter-title.W)) > 1 {
				t.Fatalf("scroll gutter changed text alignment: title=%+v editor=%+v caret=%+v", title, ed, caret)
			}
			// Right-aligned line endings exercise the exact edge where an overlay
			// scrollbar used to paint over punctuation and intercept caret clicks.
			a.doc.SetAlign(ui.End)
			_, end := a.doc.Text().ParagraphRange(0)
			a.doc.SetSelection(end, end)
			tt.Frame()
			caret, ok = a.doc.CaretRect()
			style := documentScrollbar(ui.LightTheme()).Resolve(ui.LightTheme())
			trackX := ed.W - style.TrackWidth
			if !ok || caret.X+caret.W > trackX-4 || math.Abs(float64(caret.X-(ed.W-documentScrollGutter))) > 2 {
				t.Fatalf("line end overlaps scroll track: editor=%+v caret=%+v track=%v", ed, caret, trackX)
			}
			tt.ClickAt(ed.X+caret.X-1, ed.Y+caret.Y+caret.H/2)
			_, selected := a.doc.Selection()
			if selected != end {
				t.Fatalf("line-end click did not place caret: %d != %d", selected, end)
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "document-scroll-"+appearance+".png"))
			}
			a.doc.SetAlign(ui.Start)
			a.doc.SetSelection(0, 0)
			tt.Frame()
			beforeScroll := a.doc.Text()
			x := ed.X + ed.W - style.TrackWidth/2
			tt.Move(x, ed.Y+6)
			tt.Press(x, ed.Y+6)
			tt.Move(x, ed.Y+ed.H-10)
			tt.Release(x, ed.Y+ed.H-10)
			paragraphs := a.doc.VisibleParagraphs()
			if len(paragraphs) == 0 || paragraphs[0].Index == 0 {
				t.Fatal("scrollbar drag no longer scrolls")
			}
			if !a.doc.Text().Equal(beforeScroll) {
				t.Fatal("scrollbar drag changed document content")
			}
		})
	}
}

func TestStructuredScrollGutterAcrossModes(t *testing.T) {
	for _, width := range []int{1000, 1600} {
		for _, mode := range []string{"Write", "Read", "Split"} {
			t.Run(fmt.Sprint(width)+"/"+mode, func(t *testing.T) {
				a := testApp(t)
				a.outline = false
				a.createNote(strings.Repeat("<p>平台原生，体验好，跨平台能力为 0；</p>", 60), "Scroll blocks")
				a.changeMode(mode)
				tt := ui.NewTester(a.view, width, 820)
				for range 3 {
					tt.Frame()
				}
				list, ok := tt.Find("Document blocks")
				if !ok {
					t.Fatal("missing document scroll container")
				}
				field, ok := tt.Find("Block text " + firstBlock(a, "p").ID)
				if !ok || list.X+list.W-(field.X+field.W) < documentScrollGutter-1 {
					t.Fatalf("block text reaches scroll track: list=%+v field=%+v", list, field)
				}
				tt.Move(list.X+list.W-5, list.Y+8)
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && width == 1000 {
					saveImage(t, tt, filepath.Join(dir, "document-scroll-"+mode+".png"))
				}
			})
		}
	}
}
