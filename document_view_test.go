package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/document"
	"github.com/yuluo-yx/mory/internal/library"
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
			left += 240
		} else if outline == "Right" {
			right -= 240
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

func TestNewTitlePlaceholderAndPersistence(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		t.Run(language, func(t *testing.T) {
			a := testApp(t)
			tt := ui.NewTester(a.view, 1280, 820)
			click(t, tt, "Create note")
			a.lib.Settings.Language = language
			tt.Frame()
			note := a.current
			field, ok := tt.Access(a.tr("Document title"))
			if !ok || field.Value != "" || field.Placeholder != a.tr("Untitled") || note.Title != "" {
				t.Fatalf("new note has a literal title instead of a placeholder: %+v", field)
			}
			if !tt.HasText(a.tr("Open ") + a.tr("Untitled")) {
				t.Fatal("unnamed note has no sidebar label")
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "title-placeholder-"+language+".png"))
			}
			click(t, tt, a.tr("Document title"))
			tt.Type("Project notes")
			if note.Title != "Project notes" || !a.save() {
				t.Fatal("typing into the title appended to placeholder content")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyBackspace)
			if note.Title != "" || !a.save() {
				t.Fatal("clearing the title did not preserve an empty value")
			}
			a.home("All notes")
			tt.Frame()
			click(t, tt, a.tr("Note ")+a.tr("Untitled"))
			field, ok = tt.Access(a.tr("Document title"))
			if !ok || field.Value != "" || field.Placeholder != a.tr("Untitled") {
				t.Fatal("reopening replaced an empty title with the display label")
			}
			reopened, err := library.Open(a.lib.Dir)
			if err != nil || reopened.Get(note.ID).Title != "" || document.Filename(note.Title) != "Untitled" {
				t.Fatal("empty titles were not saved or have no safe export filename", err)
			}
			// An authored title equal to the placeholder must remain real text.
			a.createNote("Content", "Untitled")
			tt.Frame()
			field, _ = tt.Access(a.tr("Document title"))
			if field.Value != "Untitled" || a.displayTitle("Untitled") != "Untitled" || a.displayTitle("  ") != a.tr("Untitled") {
				t.Fatal("a real title was translated or treated as placeholder content")
			}
		})
	}
}

func TestChildAndCopiedNotesKeepEmptyTitles(t *testing.T) {
	a := testApp(t)
	a.createNote("Parent content", "Parent")
	parent := a.current
	a.createChildNote(parent)
	if a.current.Parent != parent.ID || a.current.Title != "" {
		t.Fatal("child note starts with a literal default title")
	}
	copy, err := a.lib.Duplicate(a.current)
	if err != nil || copy.Title != "" {
		t.Fatal("duplicating an unnamed note manufactured a title", err)
	}
}

func TestOutlineRenderedTitlesAndWrapping(t *testing.T) {
	a := testApp(t)
	const title = "1. Physical infrastructure and software facilities for a large computing cluster"
	source := "## 1\\. **Physical infrastructure and software facilities for a large computing cluster**\n\nIntroduction.\n\n### 1\\.1 IDC facilities\n\n1. Power distribution and battery backup for continuous operation.\n2. Cooling systems for high density equipment.\n\nClosing paragraph.\n"
	a.createNote(source, "Infrastructure")
	a.outline = true
	a.lib.Settings.OutlineSide = "Right"
	tt := ui.NewTester(a.view, 1280, 1000)
	for _, size := range []float32{12, 14, 22} {
		a.lib.Settings.OutlineFontSize = size
		tt.Frame()
		parent, ok := tt.Find("Heading " + title)
		child, childOK := tt.Find("Heading 1.1 IDC facilities")
		if !ok || !childOK || parent.H < size*2 || child.Y < parent.Y+parent.H || parent.X+parent.W > 1280 {
			t.Fatalf("outline does not wrap within its column at %v: parent=%+v child=%+v", size, parent, child)
		}
	}
	click(t, tt, "Heading 1.1 IDC facilities")
	_, caret := a.doc.Selection()
	if a.doc.Text().ParagraphStyleAt(caret).Heading != 3 {
		t.Fatal("escaped heading did not navigate to the matching paragraph")
	}
	if a.current.Source != source {
		t.Fatal("outline presentation rewrote the source")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		a.lib.Settings.OutlineFontSize = 14
		for _, appearance := range []string{"Light", "Dark"} {
			a.lib.Settings.Appearance = appearance
			tt.Frame()
			saveImage(t, tt, filepath.Join(dir, "typography-"+strings.ToLower(appearance)+".png"))
		}
	}
}
