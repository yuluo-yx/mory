package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestExternalRemovalMenuKeepsOriginal(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(t.TempDir(), "Original.md")
	if err := os.WriteFile(path, []byte("Original words"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.openExternal(path)
	n := a.current
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Document actions")
	if tt.HasText("Move to Trash") {
		t.Fatal("external file offers library deletion")
	}
	click(t, tt, "Remove from list")
	if a.current != nil || a.lib.Get(n.ID) != nil || len(a.lib.Query("Trash", "", "", "")) != 0 {
		t.Fatal("external file entered trash or stayed open")
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "Original words" {
		t.Fatal("external file was modified", err)
	}
}

func TestTrashGroupedActionsAndRetentionUI(t *testing.T) {
	for _, list := range []bool{false, true} {
		a := testApp(t)
		a.createNote("parent", "Parent")
		parent := a.current
		a.createChildNote(parent)
		child := a.current
		a.trash(parent)
		a.home("Trash")
		a.lib.Settings.List = list
		tt := ui.NewTester(a.view, 1280, 900)
		if len(a.lib.Query("Trash", "", "", "")) != 1 || !tt.HasText("30 days remaining") {
			t.Fatal("group or retention label missing")
		}
		click(t, tt, "Actions Parent")
		if !tt.HasText("In the Trash") || tt.HasText("Rename") {
			t.Fatal("trash actions opened rename dialog")
		}
		click(t, tt, "Restore note")
		if parent.Deleted || child.Deleted {
			t.Fatal("restore left child in trash")
		}
		a.trash(parent)
		tt.Frame()
		click(t, tt, "Note Parent")
		click(t, tt, "Delete permanently")
		if a.lib.Get(parent.ID) != nil || a.lib.Get(child.ID) != nil {
			t.Fatal("permanent deletion left child behind")
		}
	}
}

func TestTrashFallbackAndExpiryUI(t *testing.T) {
	a := testApp(t)
	a.createNote("", "Orphan")
	n := a.current
	n.Parent = strings.Repeat("a", 26)
	a.trash(n)
	if !a.restoreNote(n) || n.Parent != "" || a.status != "Restored to My documents because the original location no longer exists" {
		t.Fatal("fallback feedback missing")
	}
	a.trash(n)
	n.DeletedAt = time.Now().Add(-library.TrashRetention)
	if err := a.lib.Save(n); err != nil {
		t.Fatal(err)
	}
	if a.noteSummary(n) != "Will be permanently deleted soon" {
		t.Fatal("expiry text missing")
	}
	a.home("Trash")
	if a.lib.Get(n.ID) != nil {
		t.Fatal("entering Trash did not prune expired note")
	}
}

func TestTrashActionsNeverExposeDocumentDetails(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, list := range []bool{false, true} {
			a := testApp(t)
			a.lib.Settings.Language = language
			a.createNote("Retained content", "Test1")
			n := a.current
			a.trash(n)
			a.home("Trash")
			a.lib.Settings.List = list
			tt := ui.NewTester(a.view, 1280, 900)
			if _, ok := tt.AccessByRole(ui.RoleButton, a.tr("Actions ")+n.Title); !ok {
				t.Fatal("trash actions are hidden from accessibility")
			}
			for _, entry := range []string{"Actions ", "Note "} {
				click(t, tt, a.tr(entry)+n.Title)
				if !tt.HasText(a.tr("In the Trash")) || a.renameOpen || a.moveOpen {
					t.Fatal("trash entry exposed live document actions")
				}
				for _, label := range []string{"Note name", "Document icon", "Rename", "Move to Trash", "Choose cover…", "History…"} {
					if _, ok := tt.Access(a.tr(label)); ok {
						t.Fatalf("trash exposes %q", label)
					}
				}
				for _, label := range []string{"Restore note", "Delete permanently", "Cancel"} {
					if _, ok := tt.AccessByRole(ui.RoleButton, a.tr(label)); !ok {
						t.Fatalf("trash action %q missing", label)
					}
				}
				click(t, tt, a.tr("Cancel"))
			}
			if err := tt.RightClick(a.tr("Note ") + n.Title); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{"Rename…", "Move…", "Duplicate", "Favorite", "Pin to top", "Move to Trash", "New sub-note"} {
				if err := tt.ChooseMenuItem(a.tr(label)); err == nil {
					t.Fatalf("trash context menu exposes %q", label)
				}
			}
			if err := tt.ChooseMenuItem(a.tr("Restore")); err != nil {
				t.Fatal(err)
			}
			if n.Deleted {
				t.Fatal("context-menu restore failed")
			}
		}
	}
}

func TestNoteActionDialogTracksRefreshedLibrary(t *testing.T) {
	for _, dialog := range []string{"details", "move"} {
		a := testApp(t)
		a.createNote("Shared", "Shared")
		n := a.current
		a.home("All notes")
		a.target = n
		a.renameOpen, a.moveOpen = dialog == "details", dialog == "move"
		tt := ui.NewTester(a.view, 1280, 900)
		other, err := library.Open(a.lib.Dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := other.TrashTree(other.Get(n.ID), true); err != nil {
			t.Fatal(err)
		}
		fresh, err := library.Open(a.lib.Dir)
		if err != nil {
			t.Fatal(err)
		}
		a.lib = fresh
		tt.Frame()
		if a.renameOpen || a.moveOpen || !a.deleteOpen || tt.HasText("Document details") || tt.HasText("Move document") {
			t.Fatal("stale dialog kept live document controls")
		}
		// A card captured before refresh must route by the current record.
		a.openNoteActions(n)
		tt.Frame()
		if !a.deleteOpen || a.renameOpen {
			t.Fatal("stale card reopened metadata editing")
		}
		if err := fresh.Purge(fresh.Get(n.ID)); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.target != nil || a.deleteOpen || a.renameOpen {
			t.Fatal("purged target retained a dialog")
		}
	}
}

func TestTrashDoesNotReorderDocuments(t *testing.T) {
	a := testApp(t)
	a.createNote("One", "One")
	one := a.current
	a.createNote("Two", "Two")
	two := a.current
	a.trash(one)
	a.home("All notes")
	a.reorderNote(one.ID, two.ID)
	if a.order == "Manual" || one.Order != 0 || two.Order != 0 {
		t.Fatal("trashed source participated in reordering")
	}
	a.reorderNote(two.ID, one.ID)
	a.home("Trash")
	a.reorderNote(two.ID, one.ID)
	if a.order == "Manual" || one.Order != 0 || two.Order != 0 {
		t.Fatal("trash changed document order")
	}
}
