package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDocumentPagePlacement(t *testing.T) {
	a := testApp(t)
	a.createNote("# Page heading\n\nThe document stays readable on a wide screen.", "Page placement")
	cover, err := os.ReadFile("resources/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.lib.Dir, "assets", "cover.png"), cover, 0o600); err != nil {
		t.Fatal(err)
	}
	a.current.Cover = "assets/cover.png"
	tt := ui.NewTester(a.view, 2560, 1200)
	for _, outline := range []string{"Left", "Right", "Hidden"} {
		a.outline = outline != "Hidden"
		a.lib.Settings.OutlineSide = outline
		left, right := float32(240), float32(2560)
		if outline == "Left" {
			left += 200
		} else if outline == "Right" {
			right -= 200
		}
		for _, mode := range []string{"Write", "Source", "Read"} {
			a.changeMode(mode)
			for _, width := range []string{"Standard", "Wide", "Full"} {
				a.current.Width = width
				pageWidth := float32(820)
				if width == "Wide" {
					pageWidth = 1020
				} else if width == "Full" {
					pageWidth = right - left
				}
				for _, position := range []string{"Left", "Center", "Right"} {
					a.lib.Settings.PagePosition = position
					tt.Frame()
					title, ok := tt.Find("Document title")
					banner, coverOK := tt.Find("Document cover")
					want := left + pageWidth/2
					if position == "Center" {
						want = (left + right) / 2
					} else if position == "Right" {
						want = right - pageWidth/2
					}
					if !ok || !coverOK || math.Abs(float64(title.X+title.W/2-want)) > 1 || math.Abs(float64(banner.X+banner.W/2-want)) > 1 {
						t.Fatalf("page alignment %s/%s/%s/%s: title=%+v cover=%+v expected center=%v", outline, mode, width, position, title, banner, want)
					}
					if title.X < left || title.X+title.W > right {
						t.Fatal("document overlaps sidebar or outline")
					}
					if outline != "Hidden" {
						heading, _ := tt.Find("Heading Page heading")
						caption, _ := tt.Find("ON THIS PAGE")
						gap := heading.X - (title.X + title.W)
						if outline == "Left" {
							gap = title.X - (heading.X + heading.W)
						}
						if gap < 0 || gap > 64 || math.Abs(float64(caption.Y-title.Y)) > 32 {
							t.Fatalf("outline is not beside the document title (%s/%s/%s): gap=%v caption=%+v title=%+v", mode, width, position, gap, caption, title)
						}
					}
					if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && outline == "Left" && mode == "Write" && width == "Standard" {
						saveImage(t, tt, filepath.Join(dir, "page-"+position+".png"))
					}
				}
			}
		}
	}
}

func TestPagePlacementPreference(t *testing.T) {
	a := testApp(t)
	if a.pagePosition() != "Center" {
		t.Fatal("existing libraries must default to centered pages")
	}
	a.createNote("# Heading\n\nBody", "Position preference")
	original := a.current.Source
	a.settingsOpen, a.settingsTab = true, "Editor"
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Document position")
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if a.pagePosition() != "Right" || a.current.Source != original {
		t.Fatal("page position was not applied independently of document contents")
	}
	b, err := newApp(a.lib.Dir)
	if err != nil || b.pagePosition() != "Right" {
		t.Fatalf("page position did not persist: %v", err)
	}
	previous := appWindows
	appWindows = []*app{a, b}
	t.Cleanup(func() { appWindows = previous })
	a.setPagePosition("Left")
	if b.pagePosition() != "Left" {
		t.Fatal("sibling window did not receive the position preference")
	}
	a.setPagePosition("Center")
	if b.pagePosition() != "Center" {
		t.Fatal("center preference did not propagate")
	}
}
