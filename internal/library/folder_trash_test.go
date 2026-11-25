package library

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestTrashFolderPreservesRecoverableTrees(t *testing.T) {
	l := mustLibrary(t)
	l.Settings.Folders = []string{"Work", "Work/Nested", "Work/Empty", "Workshop", "Other"}
	if err := l.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	parent := mustNote(t, l, "Parent")
	child := mustNote(t, l, "Child")
	grandchild := mustNote(t, l, "Grandchild")
	nested := mustNote(t, l, "Nested")
	independent := mustNote(t, l, "Already deleted")
	sibling := mustNote(t, l, "Sibling")
	outside := mustNote(t, l, "Outside")
	for _, item := range []struct {
		note           *Note
		folder, parent string
	}{
		{parent, "Work", ""}, {child, "", parent.ID}, {grandchild, "Other", child.ID},
		{nested, "Work/Nested", ""}, {independent, "Work", parent.ID},
		{sibling, "Workshop", ""}, {outside, "Other", ""},
	} {
		if err := l.Move(item.note, item.folder, item.parent); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.TrashTree(independent, true); err != nil {
		t.Fatal(err)
	}
	deletedAt := independent.DeletedAt
	// Disk order is arbitrary: children must still belong to their live root.
	slices.Reverse(l.Notes)
	if err := l.TrashFolder("Work"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l.Folders(), []string{"Other", "Workshop"}) {
		t.Fatal("folder subtree remains", l.Folders())
	}
	if !independent.DeletedAt.Equal(deletedAt) || independent.TrashRoot != independent.ID {
		t.Fatal("existing trash group changed")
	}
	l, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []*Note{parent, child, grandchild, nested, independent} {
		saved := l.Get(n.ID)
		if !saved.Deleted || saved.Source != n.Source || saved.DeletedAt.IsZero() {
			t.Fatalf("note was not preserved in Trash: %+v", saved)
		}
	}
	if len(l.Query("Trash", "", "", "")) != 3 || l.Get(sibling.ID).Deleted || l.Get(outside.ID).Deleted {
		t.Fatal("folder deletion affected unrelated notes or flattened groups")
	}
	if moved, err := l.RestoreTree(l.Get(parent.ID)); err != nil || !moved {
		t.Fatal("deleted-folder restore did not fall back to the library root", moved, err)
	}
	if l.Get(parent.ID).Folder != "" || l.Get(child.ID).Parent != parent.ID || l.Get(grandchild.ID).Parent != child.ID || !l.Get(independent.ID).Deleted {
		t.Fatal("restoring lost hierarchy or revived an independent deletion")
	}
}

func TestTrashFolderForgetsExternalRegistrationsOnly(t *testing.T) {
	l := mustLibrary(t)
	path := filepath.Join(t.TempDir(), "external.md")
	mustWrite(t, path, []byte("original content"))
	external, err := l.OpenExternal(path)
	if err != nil {
		t.Fatal(err)
	}
	child := mustNote(t, l, "Local child")
	if err := l.Move(external, "Work", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Move(child, "", external.ID); err != nil {
		t.Fatal(err)
	}
	// Forgetting a missing external original must also work.
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashFolder("Work"); err != nil {
		t.Fatal(err)
	}
	if l.Get(external.ID) != nil || !child.Deleted || child.TrashRoot != child.ID || len(l.Folders()) != 0 {
		t.Fatal("external registration or its child remained active")
	}
	if data, err := os.ReadFile(path + ".moved"); err != nil || string(data) != "original content" {
		t.Fatal("external original changed", err)
	}
	if moved, err := l.RestoreTree(child); err != nil || !moved || child.Parent != "" {
		t.Fatal("external child could not be restored", moved, err)
	}
}

func TestTrashFolderPreflightsAllRecords(t *testing.T) {
	for _, conflict := range []string{"settings", "note", "destination", "external"} {
		t.Run(conflict, func(t *testing.T) {
			l := mustLibrary(t)
			if err := l.RenameFolder("", "Work"); err != nil {
				t.Fatal(err)
			}
			first := mustNote(t, l, "First")
			last := mustNote(t, l, "Last")
			for _, n := range []*Note{first, last} {
				if err := l.Move(n, "Work", ""); err != nil {
					t.Fatal(err)
				}
			}
			switch conflict {
			case "settings":
				other := reopenLibrary(t, l)
				if err := other.RenameFolder("", "Other"); err != nil {
					t.Fatal(err)
				}
			case "note":
				other := reopenLibrary(t, l)
				other.Get(last.ID).Title = "Changed elsewhere"
				if err := other.Save(other.Get(last.ID)); err != nil {
					t.Fatal(err)
				}
			case "destination":
				next := *last
				next.Deleted = true
				mustWrite(t, l.notePath(&next), []byte("occupied"))
			case "external":
				path := filepath.Join(t.TempDir(), "external.md")
				mustWrite(t, path, []byte("original"))
				n, err := l.OpenExternal(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := l.Move(n, "Work", ""); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, l.notePath(n), []byte("changed registration"))
			}
			if err := l.TrashFolder("Work"); !errors.Is(err, ErrConflict) {
				t.Fatal("known conflict was ignored", err)
			}
			if first.Deleted || last.Deleted || !slices.Contains(l.Folders(), "Work") {
				t.Fatal("known conflict left a partially removed folder")
			}
		})
	}
}

func reopenLibrary(t *testing.T, l *Library) *Library {
	t.Helper()
	other, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return other
}

func TestTrashFolderEmptyAndInvalidTargets(t *testing.T) {
	l := mustLibrary(t)
	root := mustNote(t, l, "Root")
	if err := l.TrashFolder(""); err == nil || root.Deleted {
		t.Fatal("library root was accepted as a folder")
	}
	for _, folder := range []string{"Work", "Work/Empty", "Workshop"} {
		if err := l.RenameFolder("", folder); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := l.TrashFolder("Work"); err != nil {
			t.Fatal(err)
		}
	}
	l = reopenLibrary(t, l)
	if !slices.Equal(l.Folders(), []string{"Workshop"}) || l.Get(root.ID).Deleted {
		t.Fatal("empty folder deletion was not persisted or included a prefix sibling")
	}
	if err := os.Remove(filepath.Join(l.Dir, ".write.lock")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(l.Dir, ".write.lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashFolder("Workshop"); err == nil {
		t.Fatal("lock failure was ignored")
	}
}
