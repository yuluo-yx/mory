package main

import (
	"bytes"
	"fmt"
	"image/png"
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
			for _, width := range []string{"Standard", "Wide", "Full", "Custom"} {
				a.lib.Settings.PageWidthMode = width
				a.lib.Settings.PageWidth = 1600
				pageWidth := float32(820)
				if width == "Wide" {
					pageWidth = 1020
				} else if width == "Custom" {
					pageWidth = 1600
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
					if !ok || !coverOK || math.Abs(float64(title.X+title.W/2-want)) > 1 || math.Abs(float64(banner.X-240)) > 1 || math.Abs(float64(banner.W-2320)) > 1 {
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

func TestSidebarAndOutlineToggleIndependently(t *testing.T) {
	for _, side := range []string{"Left", "Right"} {
		for _, mode := range []string{"Write", "Source", "Read"} {
			t.Run(side+"/"+mode, func(t *testing.T) {
				a := testApp(t)
				a.createNote("# Heading\n\nDocument body", "Independent navigation")
				a.lib.Settings.OutlineSide = side
				a.changeMode(mode)
				tt := ui.NewTester(a.view, 1280, 820)
				assertPanels := func(sidebar, outline bool) {
					t.Helper()
					tt.Frame()
					_, hasSidebar := tt.Find("New folder")
					_, hasOutline := tt.Find("ON THIS PAGE")
					if hasSidebar != sidebar || hasOutline != outline {
						t.Fatalf("navigation visibility: sidebar=%t outline=%t; want %t/%t", hasSidebar, hasOutline, sidebar, outline)
					}
				}
				assertPanels(true, true)
				click(t, tt, "Toggle sidebar")
				assertPanels(false, true)
				if a.focus {
					t.Fatal("collapsing the sidebar entered focus mode")
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && mode == "Write" {
					saveImage(t, tt, filepath.Join(dir, "independent-outline-"+side+".png"))
				}
				click(t, tt, "Hide outline")
				assertPanels(false, false)
				click(t, tt, "Toggle sidebar")
				assertPanels(true, false)
				click(t, tt, "Toggle outline")
				assertPanels(true, true)
				click(t, tt, "Hide outline")
				assertPanels(true, false)
				click(t, tt, "Toggle outline")
				click(t, tt, "Toggle sidebar")
				// Focus mode is a temporary override, preserving both panel preferences.
				click(t, tt, "Toggle focus mode")
				assertPanels(false, false)
				click(t, tt, "Toggle focus mode")
				assertPanels(false, true)
				if a.outlineSide() != side || a.current.Source != "# Heading\n\nDocument body" {
					t.Fatal("navigation toggles changed the outline position or document")
				}
				// The home header must also restore an independently collapsed sidebar.
				click(t, tt, "Back to library")
				assertPanels(false, false)
				click(t, tt, "Toggle sidebar")
				assertPanels(true, false)
			})
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
	source := "## 1\\. **Physical infrastructure and software facilities for a large computing cluster**\n\nIntroduction.\n\n### 1\\.1 IDC facilities\n\n1. Power distribution and battery backup for continuous operation.\n2. Cooling systems for high density equipment, including air cooling, liquid cooling and redundant systems that keep the facility operating continuously.\n   - Monitor coolant temperature.\n   - Maintain backup capacity.\n\nClosing paragraph.\n\n- Inspect the equipment.\n- Record maintenance and operational changes for the next shift.\n"
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
		for _, mode := range []string{"Read", "Write"} {
			a.changeMode(mode)
			if mode == "Write" && !a.enableBlocks() {
				t.Fatal(a.problem)
			}
			tt.Frame()
			saveImage(t, tt, filepath.Join(dir, "list-layout-"+strings.ToLower(mode)+".png"))
		}
		if a.current.Source != source {
			t.Fatal("list layout changed the source")
		}
	}
}

func TestCoverGalleryPersistenceAndPortability(t *testing.T) {
	a := testApp(t)
	a.createNote("Keep the authored content.", "Covers")
	a.lib.Settings.Language = "zh-CN"
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, a.tr("Choose cover…"))
	if !tt.HasText(a.tr("Built-in covers")) || !tt.HasText(a.tr("Mountains")) {
		t.Fatal("cover gallery is missing its categories")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "cover-gallery.png"))
	}
	click(t, tt, a.tr("Cover: ")+a.tr("Morning sky"))
	if a.coverPickerOpen || a.current.Cover == "" || !a.save() {
		t.Fatal("choosing a cover did not save it", a.problem)
	}
	first := a.current.Cover
	data, err := os.ReadFile(filepath.Join(a.lib.Dir, first))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != 1600 || config.Height != 480 {
		t.Fatal("preset is not a portable full-size PNG", config, err)
	}
	stored, err := library.Open(a.lib.Dir)
	if err != nil || stored.Get(a.current.ID).Cover != first {
		t.Fatal("cover did not persist", err)
	}
	page, err := renderExport(a.current, a.lib.Dir)
	if err != nil || !strings.Contains(page, "data:image/png;base64,") {
		t.Fatal("export lost built-in artwork", err)
	}
	click(t, tt, a.tr("Choose cover…"))
	click(t, tt, a.tr("Random cover"))
	if a.current.Cover == "" || a.problem != "" {
		t.Fatal("random cover failed")
	}
	click(t, tt, a.tr("Upload"))
	if !tt.HasText(a.tr("Choose local image…")) {
		t.Fatal("local upload is unavailable")
	}
	click(t, tt, a.tr("Remove cover"))
	if a.current.Cover != "" || a.coverPickerOpen || a.current.Source != "Keep the authored content." {
		t.Fatal("cover removal changed document content")
	}
	// Every built-in design must produce visible, nonuniform image content.
	for _, preset := range coverPresets {
		pixels := coverPixels(preset, 100, 32)
		if pixels.RGBAAt(0, 0) == pixels.RGBAAt(99, 31) {
			t.Fatal("flat or invalid preset", preset.name)
		}
	}
}

func TestCoverAndTitleScrollOutOfEditor(t *testing.T) {
	for _, mode := range []string{"Write", "Blocks", "Read", "Source", "Split"} {
		t.Run(mode, func(t *testing.T) {
			a := testApp(t)
			var source strings.Builder
			for i := range 60 {
				fmt.Fprintf(&source, "## Section %d\n\nParagraph %d, with enough text to keep a long document scrolling.\n\n", i, i)
			}
			a.createNote(source.String(), "A long document with a cover")
			a.coverPickerID = a.current.ID
			a.applyCoverPreset(15)
			if mode == "Blocks" {
				a.enableBlocks()
			} else {
				a.changeMode(mode)
			}
			tt := ui.NewTester(a.view, 1280, 900)
			for range 4 {
				tt.Frame()
			}
			cover, ok := tt.Find("Document cover")
			if !ok || cover.X != 240 || cover.W != 1040 || cover.Y-a.pageViewport.Y > 1 {
				t.Fatal("cover does not span the document area below the toolbar", cover, a.pageViewport)
			}
			label := "Document editor"
			if mode == "Read" {
				label = "Document preview"
			} else if mode == "Blocks" {
				label = "Document blocks"
			} else if mode == "Source" || mode == "Split" {
				label = "Source editor"
			}
			ed, ok := tt.Find(label)
			if !ok {
				t.Fatal("missing editor", label)
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "cover-top-"+mode+".png"))
			}
			tt.Scroll(ed.X+ed.W/2, min(ed.Y+ed.H-30, a.pageViewport.Y+a.pageHeaderSpace()+60), 0, 700)
			for range 6 {
				tt.Frame()
			}
			if a.pageHeaderOffset < a.pageHeaderSpace()-1 {
				t.Fatal("cover and title stayed fixed while body scrolled", a.pageHeaderOffset, a.pageHeaderSpace())
			}
			ed, _ = tt.Find(label)
			if ed.Y > a.pageViewport.Y+5 || ed.H < a.pageViewport.H-24 {
				t.Fatal("editor did not reclaim header space", ed, a.pageViewport)
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "cover-scrolled-"+mode+".png"))
			}
			tt.Scroll(ed.X+ed.W/2, ed.Y+ed.H/2, 0, -100000)
			for range 6 {
				tt.Frame()
			}
			if a.pageHeaderOffset > 1 {
				t.Fatal("scrolling to the start did not restore the cover", a.pageHeaderOffset)
			}
			if !a.save() || a.current.Source != source.String() {
				t.Fatal("scrolling changed authored content", a.problem)
			}
		})
	}
}
