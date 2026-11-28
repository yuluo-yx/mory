package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestSidebarStartupCollapsesFoldersWithoutOpenedDocuments(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Folders = []string{"Empty", "Work", "Work/Project", "Personal"}
	if err := a.lib.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	n, err := a.lib.Create("Imported only", "Exact source", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	n.Folder = "Work/Project"
	if err := a.lib.Save(n); err != nil {
		t.Fatal(err)
	}
	reopened, err := newApp(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(reopened.view, 1280, 820)
	for _, folder := range a.lib.Folders() {
		info, ok := tt.Access("Folder " + folder)
		if !ok || info.Expanded || !reopened.collapsedFolders[folder] {
			t.Fatal("startup expanded a folder without document activity", folder, info)
		}
	}
	if _, ok := tt.Find("Open Imported only"); ok || reopened.current != nil {
		t.Fatal("startup opened an imported document")
	}
}

func TestSidebarRestartRevealsOnlyLastDocumentPath(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Folders = []string{"Work", "Work/Project", "Work/Project-other", "Work/Other", "Empty", "Personal"}
	if err := a.lib.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	a.folder = "Personal"
	a.createNote("Personal source", "Personal note")
	personal := a.current
	a.folder = "Work/Project"
	a.createNote("Parent source", "Parent")
	parent := a.current
	a.createChildNote(parent)
	a.current.Title = "Child"
	a.doc.InsertText("Pending child body")
	a.save()
	child := a.current
	a.collapsedNotes[parent.ID] = true
	a.collapsedFolders["Work/Project"] = true
	// Reopening by search/home must reveal the folder and parent path.
	a.openNote(child)
	if a.collapsedFolders["Work/Project"] || a.collapsedNotes[parent.ID] {
		t.Fatal("opening a child left its navigation path hidden")
	}
	a.home("All notes")
	before := map[string][]byte{}
	for _, n := range []*library.Note{personal, parent, child} {
		path := filepath.Join(a.lib.Dir, "notes", n.FileName)
		before[path], _ = os.ReadFile(path)
	}
	reopened, err := newApp(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(reopened.view, 1280, 820)
	for _, folder := range reopened.lib.Folders() {
		want := folder == "Work" || folder == "Work/Project"
		info, ok := tt.Access("Folder " + folder)
		if !ok || info.Expanded != want {
			t.Fatal("restart restored unrelated or missed ancestor folder", folder, info)
		}
	}
	if !tt.HasText("Open Child") || tt.HasText("Open Personal note") || reopened.current != nil {
		t.Fatal("startup did not reveal the child in its folder")
	}
	for path, expected := range before {
		actual, err := os.ReadFile(path)
		if err != nil || string(actual) != string(expected) {
			t.Fatal("restoring sidebar modified a document", err)
		}
	}
	// Manual expansion is session-local. Opening another note leaves it alone;
	// restarting closes it again and follows the newly used document instead.
	reopened.collapsedFolders["Empty"] = false
	reopened.openNote(reopened.lib.Get(personal.ID))
	if reopened.collapsedFolders["Empty"] || reopened.collapsedFolders["Personal"] {
		t.Fatal("navigation collapsed a folder the user opened")
	}
	last, err := newApp(a.lib.Dir)
	if err != nil || !last.collapsedFolders["Empty"] || !last.collapsedFolders["Work/Project"] || last.collapsedFolders["Personal"] {
		t.Fatal("second restart kept stale folder expansion", err)
	}
}

func TestSidebarActivityUsesEditsAndCurrentHierarchy(t *testing.T) {
	a := testApp(t)
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a.lib.Settings.Folders = []string{"Old", "Moved", "Recent", "Root", "Deleted", "Never opened"}
	old := &library.Note{ID: "old", Folder: "Moved", Opened: stamp, Updated: stamp.Add(3 * time.Hour)}
	recent := &library.Note{ID: "recent", Folder: "Recent", Opened: stamp.Add(time.Hour)}
	deleted := &library.Note{ID: "deleted", Folder: "Deleted", Opened: stamp.Add(5 * time.Hour), Deleted: true}
	imported := &library.Note{ID: "imported", Folder: "Never opened", Updated: stamp.Add(6 * time.Hour)}
	a.lib.Notes = []*library.Note{old, recent, deleted, imported}
	a.restoreSidebarFolders()
	if a.collapsedFolders["Moved"] || !a.collapsedFolders["Recent"] || !a.collapsedFolders["Deleted"] || !a.collapsedFolders["Never opened"] {
		t.Fatal("startup activity ignored edits or included trash/imports", a.collapsedFolders)
	}
	// A root-level document has no folder path to expand.
	old.Folder = ""
	a.restoreSidebarFolders()
	for folder, collapsed := range a.collapsedFolders {
		if !collapsed {
			t.Fatal("root document expanded a folder", folder)
		}
	}
	// Missing/deleted parents fall back to the note's own folder, and corrupt
	// cycles must terminate instead of hanging application startup.
	old.Folder, old.Parent = "Root", "missing"
	a.restoreSidebarFolders()
	if a.collapsedFolders["Root"] {
		t.Fatal("orphan path was hidden")
	}
	old.Parent = deleted.ID
	a.restoreSidebarFolders()
	if a.collapsedFolders["Root"] || !a.collapsedFolders["Deleted"] {
		t.Fatal("deleted parent affected the visible path")
	}
	old.Parent, recent.Parent = recent.ID, old.ID
	a.restoreSidebarFolders()
	if a.collapsedFolders["Recent"] {
		t.Fatal("cycle-safe ancestor traversal lost the root folder")
	}
}
