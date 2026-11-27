package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestDocumentLocation(t *testing.T) {
	a := testApp(t)
	parent, err := a.lib.Create("Parent document", "Parent body", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	parent.Folder = "test/test1"
	child, err := a.lib.CreateFile("Display title", "Exact source", "markdown", "Original 文件.MDX")
	if err != nil {
		t.Fatal(err)
	}
	child.Parent = parent.ID
	for _, tc := range []struct {
		note *library.Note
		want string
	}{
		{parent, "/test/test1/Parent document.md"},
		{child, "/test/test1/Parent document/Original 文件.MDX"},
		{&library.Note{ID: "root", FileName: "test.md"}, "/test.md"},
		{&library.Note{ID: "xml", Title: "Draft", Format: "xml"}, "/Draft.xml"},
		{&library.Note{ID: "draft", Title: "Draft"}, "/Draft.md"},
		{&library.Note{ID: "missing", Parent: "absent", Folder: "Fallback", FileName: "Child.md"}, "/Fallback/Child.md"},
		{&library.Note{ID: "external", External: "/Users/test/Notes/External file.md", Folder: "Group"}, "/Users/test/Notes/External file.md"},
	} {
		if got := a.documentLocation(tc.note); got != tc.want {
			t.Fatalf("location = %q, want %q", got, tc.want)
		}
	}
	// Corrupt parent links must not hang the render loop.
	parent.Parent = child.ID
	if got := a.documentLocation(child); got != "/test/test1/Parent document/Original 文件.MDX" {
		t.Fatal("cycle changed location", got)
	}
	parent.Deleted = true
	if got := a.documentLocation(child); got != "/Original 文件.MDX" {
		t.Fatal("deleted parent still included", got)
	}
}

func TestDocumentHeaderActionsStayAtWindowEdge(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, hidden := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/hidden=%t", language, hidden), func(t *testing.T) {
				a := testApp(t)
				a.lib.Settings.Language = language
				a.sidebarHidden = hidden
				a.createNote("Body stays unchanged", "Long document title with enough text to wrap before this fix")
				a.current.Folder = strings.Repeat("Folder with Unicode 世界/", 8) + "test1"
				a.save()
				before := a.current.Source
				tt := ui.NewTester(a.view, 1280, 820)
				for _, width := range []int{1000, 1280, 2560, 3840} {
					tt.SetSize(width, 820)
					for _, bar := range []ui.TitleBar{{Height: 48, Left: 78}, {}} {
						tt.SetTitleBar(bar)
						for _, mode := range []string{"Write", "Source", "Split", "Read"} {
							a.changeMode(mode)
							tt.Frame()
							header, _ := tt.Find(a.tr("Document header"))
							location, ok := tt.Find(a.tr("Document location"))
							actions, _ := tt.Find(a.tr("Document header actions"))
							last, _ := tt.Find(a.tr("Document actions"))
							if !ok || math.Abs(float64(last.X+last.W-float32(width-20))) > 1 || math.Abs(float64(actions.X+actions.W-float32(width-20))) > 1 {
								t.Fatal("actions did not follow window right edge", width, mode, actions, last)
							}
							if location.X+location.W > actions.X-9 || location.H > 24 || location.W <= 0 || location.Y < header.Y || location.Y+location.H > header.Y+header.H {
								t.Fatal("long location overlaps actions or wraps", width, mode, location, actions)
							}
							if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && mode == "Write" && bar.Height == 0 && language == "zh-CN" && (width == 1280 || width == 2560) {
								saveImage(t, tt, filepath.Join(dir, fmt.Sprintf("header-%d-sidebar-hidden-%t.png", width, hidden)))
							}
						}
					}
				}
				if a.current.Source != before {
					t.Fatal("header layout modified document content")
				}
			})
		}
	}
}

func TestDocumentLocationTracksMoveRenameAndActions(t *testing.T) {
	a := testApp(t)
	a.createNote("Preserved body", "Original")
	a.current.Folder = "test/test1"
	a.save()
	tt := ui.NewTester(a.view, 1280, 820)
	if info, ok := tt.Access("Document location"); !ok || info.Value != "/test/test1/Original.md" {
		t.Fatal("header omitted folder path or file extension")
	}
	a.current.Title = "Renamed"
	a.markDirty()
	a.save()
	tt.Frame()
	if info, ok := tt.Access("Document location"); !ok || info.Value != "/test/test1/Renamed.md" {
		t.Fatal("header kept stale filename after rename")
	}
	if err := a.lib.Move(a.current, "Moved", ""); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if info, ok := tt.Access("Document location"); !ok || info.Value != "/Moved/Renamed.md" {
		t.Fatal("header kept stale folder after move")
	}
	click(t, tt, "Toggle favorite")
	if !a.current.Starred {
		t.Fatal("right-aligned favorite button did not work")
	}
	click(t, tt, "Document actions")
	click(t, tt, "Cancel")
	for _, mode := range []string{"Source", "Read", "Split", "Write"} {
		click(t, tt, mode)
		if a.mode != mode || a.current.Source != "Preserved body" {
			t.Fatal("header mode button changed document content", mode)
		}
	}
}
