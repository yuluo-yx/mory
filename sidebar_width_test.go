package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func sidebarDivider(t *testing.T, tt *ui.Tester, a *app) ui.Rect {
	t.Helper()
	info, ok := tt.AccessByRole(ui.RoleSplitter, a.tr("Resize sidebar"))
	if !ok {
		t.Fatal("sidebar divider is missing")
	}
	return info.Bounds
}

func dragSidebar(t *testing.T, tt *ui.Tester, a *app, dx float32) {
	t.Helper()
	r := sidebarDivider(t, tt, a)
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x+dx, y)
	tt.Release(x+dx, y)
	tt.Frame()
}

func TestSidebarResizeCursorDragAndPersistence(t *testing.T) {
	for _, appearance := range []string{"Light", "Dark"} {
		t.Run(appearance, func(t *testing.T) {
			a := testApp(t)
			a.lib.Settings.Appearance = appearance
			a.createNote("Unchanged source", "A long document title that benefits from a wider sidebar")
			n := a.current
			tt := ui.NewTester(a.view, 1200, 820)
			r := sidebarDivider(t, tt, a)
			for _, offset := range []float32{1, 4, 7} {
				tt.Move(r.X+offset, r.Y+300)
				if tt.Cursor() != ui.CursorResizeEW {
					t.Fatal("divider does not show the horizontal resize cursor")
				}
			}
			x, y := r.X+r.W/2, r.Y+300
			tt.Press(x, y)
			tt.Move(x+120, y)
			if a.visibleSidebarWidth(1200) != 360 || a.lib.Settings.SidebarWidth != 0 {
				t.Fatal("drag did not preview live or wrote preferences before release", a.visibleSidebarWidth(1200))
			}
			tt.Release(x+120, y)
			tt.Frame()
			if a.lib.Settings.SidebarWidth != 360 || a.problem != "" {
				t.Fatal("release did not save sidebar width", a.lib.Settings.SidebarWidth, a.problem)
			}
			row, ok := tt.Find("Open " + n.Title)
			if !ok || row.W != 335 {
				t.Fatal("document tree did not follow sidebar width", row)
			}
			stored, err := newApp(a.lib.Dir)
			if err != nil || stored.preferredSidebarWidth() != 360 {
				t.Fatal("sidebar width did not survive reopening", err)
			}
			if n.Source != "Unchanged source" {
				t.Fatal("resizing changed document content")
			}
			if history, err := a.lib.History(n.ID); err != nil || len(history) != 0 {
				t.Fatal("resizing created document history", err)
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "sidebar-resize-"+appearance+".png"))
			}
		})
	}
}

func TestSidebarResizeBoundsAndWindowResize(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1200, 820)
	dragSidebar(t, tt, a, 900)
	if a.preferredSidebarWidth() != maxSidebarWidth {
		t.Fatal("maximum sidebar width was not enforced")
	}
	tt.SetSize(800, 620)
	if r := sidebarDivider(t, tt, a); r.X+4 != 320 || a.lib.Settings.SidebarWidth != maxSidebarWidth {
		t.Fatal("small window did not preserve the preferred width", r)
	}
	r := sidebarDivider(t, tt, a)
	tt.ClickAt(r.X+4, r.Y+300)
	if a.lib.Settings.SidebarWidth != maxSidebarWidth {
		t.Fatal("clicking without dragging overwrote the preferred width")
	}
	tt.SetSize(1200, 820)
	if r := sidebarDivider(t, tt, a); r.X+4 != maxSidebarWidth {
		t.Fatal("wider window did not restore the preferred width", r)
	}
	dragSidebar(t, tt, a, -900)
	if a.preferredSidebarWidth() != minSidebarWidth {
		t.Fatal("minimum sidebar width was not enforced")
	}
	// Reversing an overshoot must wait until the pointer returns to the bound.
	r = sidebarDivider(t, tt, a)
	x, y := r.X+4, r.Y+300
	tt.Press(x, y)
	tt.Move(x-100, y)
	tt.Move(x-50, y)
	if a.visibleSidebarWidth(1200) != minSidebarWidth {
		t.Fatal("overshoot made the divider jump")
	}
	tt.Move(x+60, y)
	tt.Release(x+60, y)
	tt.Frame()
	if a.preferredSidebarWidth() != 260 {
		t.Fatal("drag did not resume after returning inside bounds", a.preferredSidebarWidth())
	}
}

func TestSidebarResizeKeyboardResetAndVisibility(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Language = "zh-CN"
	a.setSidebarWidth(340)
	tt := ui.NewTester(a.view, 1200, 820)
	r := sidebarDivider(t, tt, a)
	tt.ClickAt(r.X+4, r.Y+300)
	tt.Key(0, ui.KeyRight)
	if a.preferredSidebarWidth() != 350 {
		t.Fatal("keyboard resize failed", a.preferredSidebarWidth())
	}
	tt.Key(0, ui.KeyLeft)
	if a.preferredSidebarWidth() != 340 {
		t.Fatal("keyboard resize failed")
	}
	r = sidebarDivider(t, tt, a)
	tt.ClickAt(r.X+4, r.Y+300)
	tt.ClickAt(r.X+4, r.Y+300)
	if a.preferredSidebarWidth() != defaultSidebarWidth {
		t.Fatal("double-click did not restore the default width", a.preferredSidebarWidth())
	}
	a.sidebarHidden = true
	tt.Frame()
	if _, ok := tt.Find(a.tr("Resize sidebar")); ok {
		t.Fatal("hidden sidebar left a resize target")
	}
	a.settingsOpen = true
	tt.Frame()
	dragSidebar(t, tt, a, 60)
	if a.preferredSidebarWidth() != 300 {
		t.Fatal("settings sidebar is not resizable")
	}
}

func TestSidebarWidthSyncAndFailedSaveRollback(t *testing.T) {
	a := testApp(t)
	b, err := newApp(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	previous := appWindows
	appWindows = []*app{a, b}
	t.Cleanup(func() { appWindows = previous })
	tt := ui.NewTester(a.view, 1200, 820)
	dragSidebar(t, tt, a, 80)
	if b.preferredSidebarWidth() != 320 {
		t.Fatal("sidebar width did not reach the other window")
	}
	other, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Settings.FontSize = 20
	if err := other.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	dragSidebar(t, tt, a, 60)
	if a.problem == "" || a.preferredSidebarWidth() != 320 || b.preferredSidebarWidth() != 320 {
		t.Fatal("failed preference save did not roll back the width")
	}
}

func TestSidebarResizeCanceledGestureKeepsSavedWidth(t *testing.T) {
	a := testApp(t)
	a.setSidebarWidth(300)
	tt := ui.NewTester(a.view, 1200, 820)
	r := sidebarDivider(t, tt, a)
	x, y := r.X+4, r.Y+300
	tt.Press(x, y)
	tt.Move(x+100, y)
	if a.visibleSidebarWidth(1200) != 400 {
		t.Fatal("drag preview is missing")
	}
	tt.CancelPress()
	tt.Frame()
	if a.sidebarDragging || a.visibleSidebarWidth(1200) != 300 || a.lib.Settings.SidebarWidth != 300 {
		t.Fatal("canceled drag changed the saved sidebar width")
	}
}
