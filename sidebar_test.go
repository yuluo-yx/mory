package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestSidebarHoverKeepsLayoutStable(t *testing.T) {
	for _, appearance := range []string{"Light", "Dark"} {
		t.Run(appearance, func(t *testing.T) {
			a := testApp(t)
			a.lib.Settings.Appearance = appearance
			a.lib.Settings.Folders = []string{"Development-and-Cloud", "Work"}
			a.createNote("Parent body", "Long parent document title for navigation")
			parent := a.current
			parent.Pinned, parent.Starred = true, true
			a.createChildNote(parent)
			a.current.Title = "Child document"
			a.createNote("Sibling body", "Another long document title for navigation")
			a.home("All notes")
			a.settingsOpen, a.settingsTab = true, "Keyboard shortcuts"
			tt := ui.NewTester(a.view, 1000, 820)
			labels := []string{
				"Folder Development-and-Cloud", "Development-and-Cloud", "Folder Work", "Work",
				"Open " + parent.Title, parent.Title, "Open Child document", "Child document",
				"Open Another long document title for navigation", "Another long document title for navigation",
				"Global quick note", "Shortcut Settings", "Save shortcuts",
			}
			bounds := make(map[string]ui.Rect, len(labels))
			for _, label := range labels {
				rect, ok := tt.Find(label)
				if !ok {
					t.Fatalf("missing baseline element %q", label)
				}
				bounds[label] = rect
			}
			rows := []string{"Folder Development-and-Cloud", "Folder Work", "Open " + parent.Title, "Open Child document", "Open Another long document title for navigation"}
			for _, row := range rows {
				rect := bounds[row]
				for _, offset := range []float32{50, rect.W - 14, 50} {
					tt.Move(rect.X+offset, rect.Y+rect.H/2)
					for frame := range 3 {
						tt.Frame()
						for _, label := range labels {
							got, ok := tt.Find(label)
							if !ok || got != bounds[label] {
								t.Fatalf("hovering %q at offset %.1f, frame %d moved %q: got %+v, want %+v", row, offset, frame, label, got, bounds[label])
							}
						}
					}
				}
			}
			tt.Move(800, 100)
			if _, ok := tt.Find("Actions " + parent.Title); ok {
				t.Fatal("document actions remain visible after leaving the sidebar")
			}
			for _, row := range rows[2:] {
				rect := bounds[row]
				tt.Move(rect.X+50, rect.Y+rect.H/2)
				if _, ok := tt.Find("New sub-note"); !ok {
					t.Fatalf("hovered row %q has no child action", row)
				}
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "sidebar-hover-"+appearance+".png"))
			}
		})
	}
}

func TestSidebarActionsKeepTheirTargets(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Folders = []string{"Work"}
	a.createNote("Parent body", "Parent")
	parent := a.current
	a.createChildNote(parent)
	a.current.Title = "Child"
	a.home("All notes")
	tt := ui.NewTester(a.view, 1000, 620)
	parentRow, _ := tt.Find("Open Parent")
	tt.Move(parentRow.X+50, parentRow.Y+parentRow.H/2)
	tt.Press(parentRow.X+11, parentRow.Y+parentRow.H/2)
	tt.Release(parentRow.X+11, parentRow.Y+parentRow.H/2)
	if !a.collapsedNotes[parent.ID] || tt.HasText("Open Child") || a.current != nil {
		t.Fatal("collapsing a note selected it or left its child visible")
	}
	tt.Press(parentRow.X+11, parentRow.Y+parentRow.H/2)
	tt.Release(parentRow.X+11, parentRow.Y+parentRow.H/2)
	if a.collapsedNotes[parent.ID] || !tt.HasText("Open Child") {
		t.Fatal("expanding a note did not restore its child")
	}
	tt.Move(parentRow.X+50, parentRow.Y+parentRow.H/2)
	click(t, tt, "Actions Parent")
	if !tt.HasText("Rename") || a.current != nil {
		t.Fatal("opening document actions selected the document or lost the menu")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Move(parentRow.X+50, parentRow.Y+parentRow.H/2)
	click(t, tt, "New sub-note")
	if a.current == nil || a.current.Parent != parent.ID {
		t.Fatal("hover action created a child under the wrong document")
	}
	a.home("All notes")
	tt.Frame()
	folderRow, _ := tt.Find("Folder Work")
	tt.Move(folderRow.X+50, folderRow.Y+folderRow.H/2)
	tt.Press(folderRow.X+folderRow.W-38, folderRow.Y+folderRow.H/2)
	tt.Release(folderRow.X+folderRow.W-38, folderRow.Y+folderRow.H/2)
	if a.current == nil || a.current.Folder != "Work" {
		t.Fatal("hover action did not create a note in its folder")
	}
}

func TestSidebarHoverPreservesScrollPosition(t *testing.T) {
	a := testApp(t)
	for index := range 30 {
		a.createNote("Body", fmt.Sprintf("Document %02d with a long title", index))
	}
	a.home("All notes")
	tt := ui.NewTester(a.view, 1000, 620)
	tt.Scroll(100, 250, 0, -240)
	before := make(map[string]ui.Rect)
	for _, note := range a.lib.Notes {
		label := "Open " + note.Title
		if rect, ok := tt.Find(label); ok && rect.Y > 200 && rect.Y+rect.H < 500 {
			before[label] = rect
		}
	}
	if len(before) < 3 {
		t.Fatalf("not enough visible rows after scrolling: %v", before)
	}
	for label, rect := range before {
		tt.Move(rect.X+50, rect.Y+rect.H/2)
		for other, want := range before {
			if got, ok := tt.Find(other); !ok || got != want {
				t.Fatalf("hovering scrolled row %q moved %q: got %+v, want %+v", label, other, got, want)
			}
		}
	}
}

func TestFolderNavigationClearsSettingsAndSearch(t *testing.T) {
	a := testApp(t)
	a.folder = "Work"
	a.createNote("Original", "Draft")
	note := a.current
	a.doc.SetSelection(a.doc.Text().Len(), a.doc.Text().Len())
	a.doc.InsertText(" pending")
	a.settingsOpen, a.settingsTab = true, "Keyboard shortcuts"
	a.query = "unrelated search"
	a.shortcutDraft = map[string]string{"New note": "Cmd+Alt+9"}
	tt := ui.NewTester(a.view, 1000, 620)
	click(t, tt, "Folder Work")
	if a.current != nil || a.settingsOpen || a.helpOpen || a.query != "" || a.folder != "Work" || a.shortcutDraft != nil {
		t.Fatalf("folder navigation left stale state: current=%v settings=%t help=%t query=%q folder=%q draft=%v", a.current, a.settingsOpen, a.helpOpen, a.query, a.folder, a.shortcutDraft)
	}
	reopened, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Get(note.ID).Source != "Original pending" || !tt.HasText("Note Draft") {
		t.Fatal("folder navigation lost pending edits or the folder's documents")
	}
}

func TestRenamedActiveFolderKeepsItsContentsAndCollapseState(t *testing.T) {
	a := testApp(t)
	a.folder = "Work"
	a.createNote("Body", "Folder note")
	note := a.current
	a.home("All notes")
	a.folder = "Work"
	a.collapsedFolders["Work"] = true
	a.folderFrom, a.folderName, a.folderOpen = "Work", "  Renamed  ", true
	tt := ui.NewTester(a.view, 1000, 620)
	click(t, tt, "Save folder")
	if a.folder != "Renamed" || !a.collapsedFolders["Renamed"] || !tt.HasText("Note Folder note") || a.folderOpen {
		t.Fatalf("renaming lost active folder state: folder=%q collapsed=%v", a.folder, a.collapsedFolders)
	}
	if _, exists := a.collapsedFolders["Work"]; exists {
		t.Fatal("renaming retained the old collapse key")
	}
	a.folderFrom, a.folderName, a.folderOpen = "Renamed", "Renamed", true
	tt.Frame()
	click(t, tt, "Save folder")
	if !a.collapsedFolders["Renamed"] {
		t.Fatal("saving an unchanged name expanded its folder")
	}
	reopened, err := library.Open(a.lib.Dir)
	if err != nil || reopened.Get(note.ID).Folder != "Renamed" {
		t.Fatal("renamed folder was not persisted", err)
	}
}
