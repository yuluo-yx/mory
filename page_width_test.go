package main

import (
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
	if n.Width != "Custom" || n.PageWidth != 820 {
		t.Fatal("custom width did not start from the standard width")
	}
	slider, ok := tt.Find("Custom page width")
	if !ok || slider.W < 100 {
		t.Fatalf("custom width slider is not usable: %+v", slider)
	}
	tt.Press(slider.X+slider.W/4, slider.Y+slider.H/2)
	tt.Move(slider.X+slider.W*0.7, slider.Y+slider.H/2)
	tt.Release(slider.X+slider.W*0.7, slider.Y+slider.H/2)
	if n.PageWidth <= 1600 || n.PageWidth >= maxPageWidth {
		t.Fatalf("drag did not update the preferred width: %d", n.PageWidth)
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
	if n.PageWidth != 1600 {
		t.Fatalf("numeric width was not applied: %d", n.PageWidth)
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
	if n.PageWidth != 1600 {
		t.Fatal("switching presets lost the custom width")
	}
	stored, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if saved := stored.Get(n.ID); saved.Width != "Custom" || saved.PageWidth != 1600 {
		t.Fatalf("custom width was not persisted: %+v", saved)
	}
	duplicate, err := a.lib.Duplicate(n)
	if err != nil || duplicate.Width != "Custom" || duplicate.PageWidth != 1600 {
		t.Fatalf("duplicate lost custom width: %+v, %v", duplicate, err)
	}
	a.renameOpen = false
	a.settingsOpen, a.settingsTab = true, "Editor"
	tt.Frame()
	if info, ok := tt.AccessByRole(ui.RoleSlider, "Custom page width"); !ok || info.Now != 1600 {
		t.Fatalf("editor preferences do not share the document width: %+v", info)
	}
	if n.Source != "# Heading\n\nBody" {
		t.Fatal("changing page width changed the document source")
	}
}

func TestCustomPageWidthBoundsAndWindowResize(t *testing.T) {
	for _, tc := range []struct{ value, want int }{{0, 820}, {-1, minPageWidth}, {200, minPageWidth}, {1600, 1600}, {10000, maxPageWidth}} {
		if got := documentPageWidth(&library.Note{Width: "Custom", PageWidth: tc.value}); got != float32(tc.want) {
			t.Fatalf("width %d: got %v, want %d", tc.value, got, tc.want)
		}
	}
	a := testApp(t)
	a.createNote("# Heading\n\nBody", "Large display")
	a.current.Width, a.current.PageWidth = "Custom", maxPageWidth
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
	if a.current.PageWidth != maxPageWidth {
		t.Fatal("window resizing overwrote the preferred width")
	}
}
