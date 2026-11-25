package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestFolderDeletionMenusAndRecovery(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, more := range []bool{false, true} {
			t.Run(language+map[bool]string{false: "/context", true: "/more"}[more], func(t *testing.T) {
				a := testApp(t)
				a.lib.Settings.Language = language
				a.folder = "Work"
				a.createNote("Parent body", "Parent")
				parent := a.current
				a.createChildNote(parent)
				child := a.current
				a.doc.InsertText("Pending child edits")
				a.collapsedFolders["Work/Empty"] = true
				tt := ui.NewTester(a.view, 1000, 720)
				openDelete := func() {
					t.Helper()
					label := a.tr("Folder ") + "Work"
					if more {
						row, ok := tt.Find(label)
						if !ok {
							t.Fatal("folder row is missing")
						}
						tt.Move(row.X+50, row.Y+row.H/2)
						tt.Press(row.X+row.W-15, row.Y+row.H/2)
						tt.Release(row.X+row.W-15, row.Y+row.H/2)
					} else if err := tt.RightClick(label); err != nil {
						t.Fatal(err)
					}
					if err := tt.ChooseMenuItem(a.tr("Delete folder…")); err != nil {
						t.Fatal(err)
					}
				}
				openDelete()
				if !a.folderDeleteOpen || a.folderDeleteTarget != "Work" || parent.Deleted || child.Deleted {
					t.Fatal("opening folder deletion changed documents or targeted another folder")
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
					saveImage(t, tt, filepath.Join(dir, "folder-delete-"+language+".png"))
				}
				click(t, tt, a.tr("Cancel"))
				if a.folderDeleteOpen || a.current != child || parent.Deleted {
					t.Fatal("canceling changed the folder or current note")
				}
				openDelete()
				click(t, tt, a.tr("Delete folder"))
				if a.problem != "" || a.folderDeleteOpen || a.current != nil || a.folder != "" || len(a.collapsedFolders) != 0 || tt.HasText(a.tr("Folder ")+"Work") {
					t.Fatalf("folder deletion left stale UI: problem=%q current=%v folder=%q", a.problem, a.current, a.folder)
				}
				reopened, err := library.Open(a.lib.Dir)
				if err != nil {
					t.Fatal(err)
				}
				if !reopened.Get(parent.ID).Deleted || reopened.Get(child.ID).Source != "Pending child edits" || !reopened.Get(child.ID).Deleted {
					t.Fatal("deletion lost pending edits or left descendants active")
				}
				click(t, tt, a.tr("Trash"))
				if err := tt.RightClick(a.tr("Trashed folder ") + "Work"); err != nil {
					t.Fatal(err)
				}
				if err := tt.ChooseMenuItem(a.tr("Restore")); err != nil {
					t.Fatal(err)
				}
				if parent.Deleted || child.Deleted || parent.Folder != "Work" || child.Parent != parent.ID {
					t.Fatal("folder notes could not be restored with their hierarchy")
				}
			})
		}
	}
}

func TestFolderDeletionFailureKeepsDialogAndDocuments(t *testing.T) {
	for _, conflict := range []string{"save", "settings"} {
		t.Run(conflict, func(t *testing.T) {
			a := testApp(t)
			a.folder = "Work"
			a.createNote("Original", "Draft")
			n := a.current
			other, err := library.Open(a.lib.Dir)
			if err != nil {
				t.Fatal(err)
			}
			if conflict == "save" {
				other.Get(n.ID).Title = "Changed elsewhere"
				if err := other.Save(other.Get(n.ID)); err != nil {
					t.Fatal(err)
				}
				a.doc.InsertText("Pending ")
			} else if err := other.RenameFolder("", "Other"); err != nil {
				t.Fatal(err)
			}
			a.folderDeleteTarget, a.folderDeleteOpen = "Work", true
			tt := ui.NewTester(a.view, 1000, 720)
			click(t, tt, "Delete folder")
			if a.problem == "" || !a.folderDeleteOpen || a.current != n || n.Deleted || !slices.Contains(a.lib.Folders(), "Work") {
				t.Fatal("failed deletion hid an error or discarded the current document")
			}
		})
	}
}

func TestFolderDeletionKeepsUnrelatedCurrentDocument(t *testing.T) {
	a := testApp(t)
	if err := a.lib.RenameFolder("", "Work"); err != nil {
		t.Fatal(err)
	}
	a.folder = "Workshop"
	a.createNote("Unrelated", "Other document")
	n := a.current
	a.doc.InsertText("Pending ")
	a.folderDeleteTarget, a.folderDeleteOpen = "Work", true
	tt := ui.NewTester(a.view, 1000, 720)
	click(t, tt, "Delete folder")
	if a.problem != "" || a.folderDeleteOpen || a.current != n || n.Deleted || n.Source != "Pending Unrelated" || a.folder != "Workshop" {
		t.Fatal("deleting an empty folder changed an unrelated editor")
	}
}

func TestNestedFolderDeletionShowsLeafAndRestoresExternalNotes(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, list := range []bool{false, true} {
			t.Run(language+map[bool]string{false: "/grid", true: "/list"}[list], func(t *testing.T) {
				a := testApp(t)
				a.lib.Settings.Language, a.lib.Settings.List = language, list
				folder := "01-Technology/Development-and-Cloud/kueue/test"
				path := filepath.Join(t.TempDir(), "Empty.md")
				writeTest(t, path, "")
				a.openExternal(path)
				n := a.current
				if err := a.lib.Move(n, folder, ""); err != nil {
					t.Fatal(err)
				}
				a.home("All notes")
				a.folder = folder
				tt := ui.NewTester(a.view, 1280, 820)
				if err := tt.RightClick(a.tr("Folder ") + folder); err != nil {
					t.Fatal(err)
				}
				if err := tt.ChooseMenuItem(a.tr("Delete folder…")); err != nil {
					t.Fatal(err)
				}
				if !tt.HasText(a.trf("Delete folder “%s”?", "test")) || !tt.HasText(a.trf("Location: %s", "01-Technology/Development-and-Cloud/kueue")) || tt.HasText(a.trf("Delete folder “%s”?", folder)) {
					t.Fatal("folder confirmation did not distinguish the leaf name and location")
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
					saveImage(t, tt, filepath.Join(dir, "nested-folder-delete-"+language+".png"))
				}
				click(t, tt, a.tr("Delete folder"))
				click(t, tt, a.tr("Trash"))
				if !tt.HasText(a.tr("Trashed folder ")+"test") || tt.HasText(a.tr("Trash is empty")) {
					t.Fatal("deleted external folder is missing from Trash")
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && !list {
					saveImage(t, tt, filepath.Join(dir, "nested-folder-trash-"+language+".png"))
				}
				click(t, tt, a.tr("Actions ")+"test")
				click(t, tt, a.tr("Restore folder"))
				if a.problem != "" || n.Deleted || n.Folder != folder || n.External == "" || len(a.lib.Query("Trash", "", "", "")) != 0 {
					t.Fatal("folder restoration lost its external document", a.problem)
				}
				if data, err := os.ReadFile(path); err != nil || len(data) != 0 {
					t.Fatal("external original changed", err)
				}
			})
		}
	}
}
