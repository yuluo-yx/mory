package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestCustomPageWidthControlsAndPersistence(t *testing.T) {
	a := testApp(t)
	a.createNote("# Heading\n\nBody", "Custom width")
	n := a.current
	tt := ui.NewTester(a.view, 1280, 820)
	openWidths := func() {
		t.Helper()
		control, ok := tt.AccessByRole(ui.RolePopUpButton, "Page width")
		if !ok {
			t.Fatal("page width selector is missing")
		}
		r := control.Bounds
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	}
	click(t, tt, "Document actions")
	openWidths()
	click(t, tt, "Custom")
	if a.pageWidthMode() != "Custom" || a.lib.Settings.PageWidth != 820 {
		t.Fatal("custom width did not start from the standard width")
	}
	slider, ok := tt.Find("Custom page width")
	if !ok || slider.W < 100 {
		t.Fatalf("custom width slider is not usable: %+v", slider)
	}
	tt.Press(slider.X+slider.W/4, slider.Y+slider.H/2)
	tt.Move(slider.X+slider.W*0.7, slider.Y+slider.H/2)
	tt.Release(slider.X+slider.W*0.7, slider.Y+slider.H/2)
	if a.lib.Settings.PageWidth <= 1600 || a.lib.Settings.PageWidth >= maxPageWidth {
		t.Fatalf("drag did not update the preferred width: %d", a.lib.Settings.PageWidth)
	}
	// NumberInput labels its group; its text field is before the step buttons.
	input, ok := tt.Find("Page width in pixels")
	if !ok {
		t.Fatal("numeric width input is missing")
	}
	tt.ClickAt(input.X+12, input.Y+input.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("1600")
	tt.Key(0, ui.KeyEnter)
	if a.lib.Settings.PageWidth != 1600 {
		t.Fatalf("numeric width was not applied: %d", a.lib.Settings.PageWidth)
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		a.lib.Settings.Language = "zh-CN"
		tt.Frame()
		saveImage(t, tt, filepath.Join(dir, "custom-page-width.png"))
		a.lib.Settings.Language = "en"
		tt.Frame()
	}
	openWidths()
	click(t, tt, "Full")
	if _, visible := tt.Find("Custom page width"); visible {
		t.Fatal("full width still shows a fixed width slider")
	}
	openWidths()
	click(t, tt, "Custom")
	if a.lib.Settings.PageWidth != 1600 {
		t.Fatal("switching presets lost the custom width")
	}
	stored, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if saved := stored.Settings; saved.PageWidthMode != "Custom" || saved.PageWidth != 1600 {
		t.Fatalf("global custom width was not persisted: %+v", saved)
	}
	duplicate, err := a.lib.Duplicate(n)
	if err != nil || duplicate.Width != "" || duplicate.PageWidth != 0 {
		t.Fatalf("global width leaked into document metadata: %+v, %v", duplicate, err)
	}
	a.renameOpen = false
	a.settingsOpen, a.settingsTab = true, "Editor"
	tt.Frame()
	if info, ok := tt.AccessByRole(ui.RoleSlider, "Custom page width"); !ok || info.Now != 1600 {
		t.Fatalf("editor preferences do not share the document width: %+v", info)
	}
	if n.Source != "# Heading\n\nBody" || n.Width != "" || n.PageWidth != 0 {
		t.Fatal("changing global width changed the document")
	}
}

func TestCustomPageWidthBoundsAndWindowResize(t *testing.T) {
	a := testApp(t)
	for _, tc := range []struct{ value, want int }{{0, 820}, {-1, minPageWidth}, {200, minPageWidth}, {1600, 1600}, {10000, maxPageWidth}} {
		a.lib.Settings.PageWidthMode, a.lib.Settings.PageWidth = "Custom", tc.value
		if got := a.documentPageWidth(); got != float32(tc.want) {
			t.Fatalf("width %d: got %v, want %d", tc.value, got, tc.want)
		}
	}
	a.createNote("# Heading\n\nBody", "Large display")
	a.lib.Settings.PageWidthMode, a.lib.Settings.PageWidth = "Custom", maxPageWidth
	tt := ui.NewTester(a.view, 1000, 820)
	for _, width := range []int{1000, 1920, 5120} {
		tt.SetSize(width, 820)
		title, ok := tt.Find("Document title")
		if !ok || title.X < 240 || title.X+title.W > float32(width) || title.W > maxPageWidth {
			t.Fatalf("page exceeds available space at %d: %+v", width, title)
		}
		if width == 5120 && title.W < 3000 {
			t.Fatalf("custom width did not expand on a large display: %+v", title)
		}
	}
	if a.lib.Settings.PageWidth != maxPageWidth {
		t.Fatal("window resizing overwrote the preferred width")
	}
}

func TestPageWidthStaysUniformAcrossDocuments(t *testing.T) {
	a := testApp(t)
	var notes []*library.Note
	for _, fixture := range []struct{ width, source, format string }{
		{"Standard", "# Heading\n\nPlain body", "markdown"},
		{"Wide", "> [!NOTE]\n> Structured body", "markdown"},
		{"Full", "<title>XML title</title><p>XML body</p>", "xml"},
		{"Custom", "", "markdown"},
	} {
		n, err := a.lib.Create("Uniform width", fixture.source, fixture.format)
		if err != nil {
			t.Fatal(err)
		}
		n.Width, n.PageWidth = fixture.width, 1600
		if err := a.lib.Save(n); err != nil {
			t.Fatal(err)
		}
		notes = append(notes, n)
	}
	tt := ui.NewTester(a.view, 2560, 1200)
	for _, preset := range []struct {
		mode  string
		width int
		want  float32
	}{{"", 0, 820}, {"Wide", 0, 1020}, {"Custom", 1400, 1400}, {"Full", 0, 2080}} {
		a.setPageWidth(preset.mode, preset.width)
		for _, n := range notes {
			originalSource, originalWidth := n.Source, n.Width
			a.openNote(n)
			for _, mode := range []string{"Write", "Source", "Read"} {
				a.changeMode(mode)
				tt.Frame()
				title, ok := tt.Find("Document title")
				// Every page has 40 logical pixels of padding on either side.
				if !ok || math.Abs(float64(title.W-(preset.want-80))) > 1 {
					t.Fatalf("document width changed with content or legacy settings: preset=%s legacy=%s mode=%s title=%+v", preset.mode, originalWidth, mode, title)
				}
			}
			if n.Width != originalWidth || n.PageWidth != 1600 || n.Source != originalSource {
				t.Fatal("global width overwrote the original document")
			}
		}
	}
	a.setPageWidth("Custom", 1400)
	a.createNote("New body", "New note")
	tt.Frame()
	if title, ok := tt.Find("Document title"); !ok || math.Abs(float64(title.W-1320)) > 1 {
		t.Fatal("new note did not inherit the global width", title)
	}
}

func TestGlobalPageWidthSettingsWithoutDocument(t *testing.T) {
	a := testApp(t)
	a.settingsOpen, a.settingsTab = true, "Editor"
	tt := ui.NewTester(a.view, 1280, 820)
	openWidths := func() {
		t.Helper()
		control, ok := tt.AccessByRole(ui.RolePopUpButton, "Page width")
		if !ok {
			t.Fatal("page width selector is missing")
		}
		r := control.Bounds
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	}
	if a.current != nil || !tt.HasText("Applies to all documents.") {
		t.Fatal("width scope is missing in library-level preferences")
	}
	openWidths()
	click(t, tt, "Wide")
	if a.documentPageWidth() != 1020 {
		t.Fatal("global width cannot be changed without an open note")
	}
	openWidths()
	click(t, tt, "Custom")
	if a.lib.Settings.PageWidth != 1020 {
		t.Fatal("custom width did not inherit the selected preset")
	}
	stored, err := newApp(a.lib.Dir)
	if err != nil || stored.documentPageWidth() != 1020 || stored.pageWidthMode() != "Custom" {
		t.Fatal("global width was not restored at startup", err)
	}
	// Full width has no fixed size to inherit, so use the standard custom default.
	a.setPageWidth("Full", 0)
	tt.Frame()
	openWidths()
	click(t, tt, "Custom")
	if a.lib.Settings.PageWidth != 820 {
		t.Fatal("custom width inherited an unbounded size")
	}
	a.lib.Settings.Language = "zh-CN"
	tt.Frame()
	if !tt.HasText("应用于所有文档。") {
		t.Fatal("global scope has no Chinese translation")
	}
}

func TestPageWidthSyncAndFailedSaveRollback(t *testing.T) {
	a := testApp(t)
	a.createNote("Preserve source", "First window")
	n := a.current
	n.Width, n.PageWidth = "Wide", 1000
	if err := a.lib.Save(n); err != nil {
		t.Fatal(err)
	}
	b, err := newApp(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	previous := appWindows
	appWindows = []*app{a, b}
	t.Cleanup(func() { appWindows = previous })
	a.setPageWidth("Custom", 1500)
	if a.documentPageWidth() != 1500 || b.documentPageWidth() != 1500 {
		t.Fatal("global width did not reach sibling windows")
	}
	b.setPageWidth("Wide", 1500)
	if a.documentPageWidth() != 1020 || a.lib.Settings.PageWidth != 1500 {
		t.Fatal("another window lost the global width or custom preference")
	}
	if revisions, err := a.lib.History(n.ID); err != nil || len(revisions) != 0 {
		t.Fatal("width preference created document history", err)
	}
	if err := os.WriteFile(filepath.Join(a.lib.Dir, "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a.setPageWidth("Custom", 1700)
	if a.pageWidthMode() != "Wide" || a.lib.Settings.PageWidth != 1500 || b.documentPageWidth() != 1020 || a.problem == "" {
		t.Fatal("failed write changed a window's width")
	}
	if n.Source != "Preserve source" || n.Width != "Wide" || n.PageWidth != 1000 {
		t.Fatal("global preference rewrote legacy document metadata")
	}
	a.lib.Settings.PageWidthMode = "unknown"
	if a.pageWidthMode() != "Standard" || a.documentPageWidth() != 820 {
		t.Fatal("invalid preferences have no consistent fallback")
	}
}
