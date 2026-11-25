package library

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
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
	if len(l.Query("Trash", "", "", "")) != 2 || l.Get(sibling.ID).Deleted || l.Get(outside.ID).Deleted {
		t.Fatal("folder deletion affected unrelated notes or flattened groups")
	}
	group := folderTrashEntry(t, l, "Work")
	if found := l.Query("Trash", "Child", "", ""); len(found) != 1 || found[0].ID != group.ID {
		t.Fatal("child search did not find its folder")
	}
	if moved, err := l.RestoreTree(group); err != nil || moved {
		t.Fatal("folder restore did not preserve the original location", moved, err)
	}
	if l.Get(parent.ID).Folder != "Work" || l.Get(child.ID).Parent != parent.ID || l.Get(grandchild.ID).Parent != child.ID || !l.Get(independent.ID).Deleted {
		t.Fatal("restoring lost hierarchy or revived an independent deletion")
	}
	if l.Get(group.ID) != nil || !slices.Contains(l.Folders(), "Work/Empty") || l.Get(nested.ID).Deleted {
		t.Fatal("folder restore omitted an empty subfolder or retained its trash marker")
	}
}

func folderTrashEntry(t *testing.T, l *Library, folder string) *Note {
	t.Helper()
	for _, n := range l.Query("Trash", "", "", "") {
		if n.FolderTrash && n.Folder == folder {
			return n
		}
	}
	t.Fatalf("folder %q is missing from Trash", folder)
	return nil
}

func TestTrashFolderPreservesExternalRegistrations(t *testing.T) {
	l := mustLibrary(t)
	path := filepath.Join(t.TempDir(), "external.md")
	mustWrite(t, path, []byte("original content"))
	external, err := l.OpenExternal(path)
	if err != nil {
		t.Fatal(err)
	}
	originalPath := external.External
	child := mustNote(t, l, "Local child")
	if err := l.Move(external, "Work", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Move(child, "", external.ID); err != nil {
		t.Fatal(err)
	}
	// A missing original must not prevent preserving its cached registration.
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashFolder("Work"); err != nil {
		t.Fatal(err)
	}
	group := folderTrashEntry(t, l, "Work")
	if l.Get(external.ID) == nil || !external.Deleted || !child.Deleted || child.TrashRoot != group.ID || external.TrashRoot != group.ID || len(l.Folders()) != 0 {
		t.Fatal("external registration or its child remained active")
	}
	if data, err := os.ReadFile(path + ".moved"); err != nil || string(data) != "original content" {
		t.Fatal("external original changed", err)
	}
	if moved, err := l.RestoreTree(group); err != nil || moved || child.Parent != external.ID || external.Deleted || external.Folder != "Work" {
		t.Fatal("external file tree could not be restored", moved, err)
	}
	if external.External != originalPath || external.Source != "original content" {
		t.Fatal("external reference or cached source changed")
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
	group := folderTrashEntry(t, l, "Work")
	if len(l.Query("Trash", "", "", "")) != 1 {
		t.Fatal("retry duplicated the folder trash entry")
	}
	if moved, err := l.RestoreTree(group); err != nil || moved || !slices.Contains(l.Folders(), "Work/Empty") {
		t.Fatal("empty folder tree could not be restored", moved, err)
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

func TestFolderTrashPurgeAndRetentionKeepExternalOriginals(t *testing.T) {
	for _, action := range []string{"purge", "empty", "expire"} {
		t.Run(action, func(t *testing.T) {
			l := mustLibrary(t)
			path := filepath.Join(t.TempDir(), "original.md")
			mustWrite(t, path, []byte("External content"))
			external, err := l.OpenExternal(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Move(external, "Parent/test", ""); err != nil {
				t.Fatal(err)
			}
			if err := l.TrashFolder("Parent/test"); err != nil {
				t.Fatal(err)
			}
			group := folderTrashEntry(t, l, "Parent/test")
			switch action {
			case "purge":
				err = l.Purge(group)
			case "empty":
				err = l.EmptyTrash()
			case "expire":
				err = l.PruneTrash(group.DeletedAt.Add(TrashRetention + time.Second))
			}
			if err != nil || l.Get(group.ID) != nil || l.Get(external.ID) != nil {
				t.Fatal("folder group was not fully removed", err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "External content" {
				t.Fatal("permanent deletion touched the external original", err)
			}
		})
	}
}

func TestFolderRestorePreflightsSettingsAndDocuments(t *testing.T) {
	for _, conflict := range []string{"settings", "destination"} {
		t.Run(conflict, func(t *testing.T) {
			l := mustLibrary(t)
			n := mustNote(t, l, "Child")
			if err := l.Move(n, "Parent/test", ""); err != nil {
				t.Fatal(err)
			}
			if err := l.TrashFolder("Parent/test"); err != nil {
				t.Fatal(err)
			}
			l = reopenLibrary(t, l)
			n = l.Get(n.ID)
			group := folderTrashEntry(t, l, "Parent/test")
			if conflict == "settings" {
				other := reopenLibrary(t, l)
				if err := other.RenameFolder("", "Other"); err != nil {
					t.Fatal(err)
				}
			} else {
				next := *n
				next.Deleted = false
				mustWrite(t, l.notePath(&next), []byte("Occupied"))
			}
			if _, err := l.RestoreTree(group); !errors.Is(err, ErrConflict) {
				t.Fatal("folder restoration ignored a conflict", err)
			}
			if !n.Deleted || l.Get(group.ID) == nil || slices.Contains(l.Folders(), "Parent/test") {
				t.Fatal("conflicting restore partially changed the folder")
			}
		})
	}
}

func TestFolderTrashRestoresAlongsideRecreatedFolder(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Original")
	if err := l.Move(n, "Parent/test", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashFolder("Parent/test"); err != nil {
		t.Fatal(err)
	}
	group := folderTrashEntry(t, l, "Parent/test")
	newNote := mustNote(t, l, "New")
	if err := l.Move(newNote, "Parent/test", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := l.RestoreTree(group); err != nil {
		t.Fatal(err)
	}
	if n.Deleted || newNote.Deleted || len(l.Query("All notes", "", "Parent/test", "")) != 2 {
		t.Fatal("restoring the old folder replaced new documents")
	}
}
