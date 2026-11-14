package library

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrashMovesAndRestoresDeletionGroups(t *testing.T) {
	l := mustLibrary(t)
	parent := mustNote(t, l, "Parent")
	child := mustNote(t, l, "Child")
	independent := mustNote(t, l, "Independent")
	for _, n := range []*Note{child, independent} {
		if err := l.Move(n, "", parent.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.TrashTree(independent, true); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashTree(parent, true); err != nil {
		t.Fatal(err)
	}
	for _, n := range []*Note{parent, child, independent} {
		if _, err := os.Stat(filepath.Join(l.Dir, "notes", n.ID+".json")); !os.IsNotExist(err) {
			t.Fatal("active copy remains", err)
		}
		if _, err := os.Stat(l.notePath(n)); err != nil {
			t.Fatal(err)
		}
		if n.DeletedAt.IsZero() {
			t.Fatal("deletion time missing")
		}
	}
	if len(l.Query("Trash", "", "", "")) != 2 {
		t.Fatal("deletion groups were flattened")
	}
	if found := l.Query("Trash", "Child", "", ""); len(found) != 1 || found[0].ID != parent.ID {
		t.Fatal("child search did not find its group")
	}
	l, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if moved, err := l.RestoreTree(l.Get(parent.ID)); err != nil || moved {
		t.Fatal("restore failed", moved, err)
	}
	if l.Get(child.ID).Deleted || !l.Get(independent.ID).Deleted {
		t.Fatal("restore mixed separate deletions")
	}
	if err := l.TrashTree(l.Get(parent.ID), true); err != nil {
		t.Fatal(err)
	}
	if err := l.Purge(l.Get(parent.ID)); err != nil {
		t.Fatal(err)
	}
	if l.Get(parent.ID) != nil || l.Get(child.ID) != nil || l.Get(independent.ID) == nil {
		t.Fatal("purge did not respect deletion groups")
	}
	if moved, err := l.RestoreTree(l.Get(independent.ID)); err != nil || !moved || l.Get(independent.ID).Parent != "" {
		t.Fatal("missing-parent fallback failed", moved, err)
	}
}

func TestTrashRetentionAndMigration(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Legacy")
	n.Deleted = true
	n.Updated = time.Now().Add(-365 * 24 * time.Hour)
	b, _ := json.Marshal(n)
	mustWrite(t, filepath.Join(l.Dir, "notes", n.ID+".json"), b)
	start := time.Now()
	l, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	n = l.Get(n.ID)
	if n.DeletedAt.Before(start) || n.Source == "" {
		t.Fatal("legacy record did not get full retention")
	}
	if err := l.PruneTrash(n.DeletedAt.Add(TrashRetention - time.Second)); err != nil || l.Get(n.ID) == nil {
		t.Fatal("early expiry", err)
	}
	if err := l.PruneTrash(n.DeletedAt.Add(TrashRetention)); err != nil || l.Get(n.ID) != nil {
		t.Fatal("expiry failed", err)
	}
}

func TestInterruptedTrashMoves(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		for _, both := range []bool{false, true} {
			l := mustLibrary(t)
			n := mustNote(t, l, "Interrupted")
			from := filepath.Join(l.Dir, "notes", n.ID+".json")
			to := filepath.Join(l.Dir, "trash", n.ID+".json")
			if !deleted {
				from, to = to, from
				if err := os.Rename(to, from); err != nil {
					t.Fatal(err)
				}
			}
			n.Deleted = deleted
			if deleted {
				n.DeletedAt = time.Now()
			}
			b, _ := json.MarshalIndent(n, "", "  ")
			mustWrite(t, from, b)
			if both {
				if err := os.Link(from, to); err != nil {
					t.Fatal(err)
				}
			}
			opened, err := Open(l.Dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(opened.Notes) != 1 || opened.Get(n.ID).Deleted != deleted {
				t.Fatal("move state lost")
			}
			if _, err := os.Stat(from); !os.IsNotExist(err) {
				t.Fatal("old copy retained", err)
			}
			data, err := os.ReadFile(to)
			if err != nil || !bytes.Equal(data, b) {
				t.Fatal("content changed during recovery", err)
			}
		}
	}
}

func TestTrashConflictsNeverOverwrite(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Conflict")
	original, _ := os.ReadFile(l.notePath(n))
	target := filepath.Join(l.Dir, "trash", n.ID+".json")
	mustWrite(t, target, []byte("occupied"))
	if err := l.TrashTree(n, true); !errors.Is(err, ErrConflict) || n.Deleted {
		t.Fatal("target conflict ignored", err)
	}
	data, _ := os.ReadFile(l.notePath(n))
	if !bytes.Equal(data, original) {
		t.Fatal("source overwritten")
	}
	other := *n
	other.Deleted = true
	other.Source = "conflicting duplicate"
	b, _ := json.Marshal(other)
	mustWrite(t, target, b)
	if _, err := Open(l.Dir); !errors.Is(err, ErrConflict) {
		t.Fatal("conflicting copies accepted", err)
	}
	data, _ = os.ReadFile(target)
	if !bytes.Equal(data, b) {
		t.Fatal("conflicting copy overwritten")
	}
}

func TestTrashMultiInstanceAndMissingFolder(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Shared")
	if err := l.RenameFolder("", "Work"); err != nil {
		t.Fatal(err)
	}
	if err := l.Move(n, "Work", ""); err != nil {
		t.Fatal(err)
	}
	other, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.TrashTree(n, true); err != nil {
		t.Fatal(err)
	}
	if err := other.Save(other.Get(n.ID)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale save resurrected deleted file", err)
	}
	if err := other.Reload(n.ID); err != nil || !other.Get(n.ID).Deleted {
		t.Fatal("reload failed after move", err)
	}
	l.Settings.Folders = nil
	if err := l.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	if moved, err := l.RestoreTree(n); err != nil || !moved || n.Folder != "" {
		t.Fatal("missing-folder fallback", moved, err)
	}
	if err := other.Purge(other.Get(n.ID)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale purge removed restored document", err)
	}
}

func TestForgetMissingExternalFile(t *testing.T) {
	l := mustLibrary(t)
	path := filepath.Join(t.TempDir(), "external.md")
	mustWrite(t, path, []byte("outside"))
	n, err := l.OpenExternal(path)
	if err != nil {
		t.Fatal(err)
	}
	child := mustNote(t, l, "Child")
	if err := l.Move(child, "", n.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashTree(n, true); err != nil {
		t.Fatal(err)
	}
	if l.Get(n.ID) != nil || child.Deleted || len(l.Query("Trash", "", "", "")) != 0 {
		t.Fatal("forget deleted a library child or populated trash")
	}
	if err := l.Forget(child); err == nil {
		t.Fatal("forgot a library-owned document")
	}
}

func TestTrashMoveFailureRollsBackState(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Rollback")
	path := l.notePath(n)
	before, _ := os.ReadFile(path)
	// A missing destination directory allows preflight, then rejects linking.
	if err := os.Remove(filepath.Join(l.Dir, "trash")); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashTree(n, true); err == nil {
		t.Fatal("missing destination accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || n.Deleted || !n.DeletedAt.IsZero() || l.Changed(n) {
		t.Fatal("failed move changed source or metadata", err)
	}
}

func TestTrashGroupPreflightsChildDestinations(t *testing.T) {
	for _, restore := range []bool{false, true} {
		l := mustLibrary(t)
		parent := mustNote(t, l, "Parent")
		child := mustNote(t, l, "Child")
		if err := l.Move(child, "", parent.ID); err != nil {
			t.Fatal(err)
		}
		if restore {
			if err := l.TrashTree(parent, true); err != nil {
				t.Fatal(err)
			}
		}
		next := *child
		next.Deleted = !restore
		mustWrite(t, l.notePath(&next), []byte("occupied"))
		if err := l.TrashTree(parent, !restore); !errors.Is(err, ErrConflict) {
			t.Fatal("child conflict ignored", err)
		}
		if parent.Deleted != restore || child.Deleted != restore {
			t.Fatal("group partially moved despite known conflict")
		}
	}
}

func TestTrashFailureBoundaries(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Failure")
	for _, fn := range []func() error{
		func() error { return l.SetDeleted(nil, true) },
		func() error { return l.TrashTree(nil, true) },
		func() error { return l.TrashTree(nil, false) },
		func() error { return l.Forget(nil) },
	} {
		if err := fn(); err == nil {
			t.Fatal("invalid note accepted")
		}
	}
	if err := l.SetDeleted(n, false); err != nil {
		t.Fatal("active no-op failed", err)
	}
	if err := l.TrashTree(n, true); err != nil {
		t.Fatal(err)
	}
	n.DeletedAt = time.Now().Add(-TrashRetention)
	if err := l.Save(n); err != nil {
		t.Fatal(err)
	}
	// Owned-copy cleanup must fail visibly and retain the document for retry.
	mustWrite(t, filepath.Join(l.Dir, "history", n.ID), []byte("blocked history"))
	if err := l.PruneTrash(time.Now()); err == nil || l.Get(n.ID) == nil {
		t.Fatal("expiry failure dropped the note", err)
	}
	if err := l.EmptyTrash(); err == nil {
		t.Fatal("empty trash hid cleanup failure")
	}
	if _, err := os.Stat(l.notePath(n)); err != nil {
		t.Fatal("failed purge deleted document", err)
	}
	if err := os.Remove(filepath.Join(l.Dir, ".write.lock")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(l.Dir, ".write.lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, fn := range []func() error{
		func() error { return l.SetDeleted(n, false) },
		func() error { return l.TrashTree(n, true) },
		func() error { _, err := l.RestoreTree(n); return err },
		func() error { return l.Purge(n) },
		func() error { n.External = "missing.md"; return l.Forget(n) },
	} {
		if err := fn(); err == nil {
			t.Fatal("lock failure ignored")
		}
	}
	if _, err := Open(l.Dir); err == nil {
		t.Fatal("loading without a lock succeeded")
	}
}

func TestTrashExternalAndFolderFallback(t *testing.T) {
	l := mustLibrary(t)
	path := filepath.Join(t.TempDir(), "source.md")
	mustWrite(t, path, []byte("outside"))
	n, err := l.OpenExternal(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.SetDeleted(n, true); err != nil || l.Get(n.ID) != nil {
		t.Fatal("external single delete did not forget", err)
	}
	parent := mustNote(t, l, "Parent")
	child := mustNote(t, l, "Child")
	if err := l.Move(child, "Obsolete", parent.ID); err != nil {
		t.Fatal(err)
	}
	if err := l.TrashTree(parent, true); err != nil {
		t.Fatal(err)
	}
	if _, err := l.RestoreTree(parent); err != nil || child.Folder != "" {
		t.Fatal("child retained missing folder", err)
	}
}
