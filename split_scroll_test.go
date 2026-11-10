package main

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

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
