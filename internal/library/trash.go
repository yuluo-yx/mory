package library

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const TrashRetention = 30 * 24 * time.Hour

func (l *Library) notePath(n *Note) string {
	dir := "notes"
	if n.Deleted {
		dir = "trash"
	}
	return filepath.Join(l.Dir, dir, n.ID+".json")
}

// loadNotes also completes interrupted moves. Each file carries its own state,
// original location and deletion group, so there is no separate index to lose.
// Legacy deletions receive a full retention period at migration time.
func (l *Library) loadNotes() error {
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	type record struct {
		note *Note
		path string
		data []byte
	}
	var records []record
	for _, dir := range []string{"notes", "trash"} {
		entries, err := os.ReadDir(filepath.Join(l.Dir, dir))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			path := filepath.Join(l.Dir, dir, entry.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var n Note
			if err := json.Unmarshal(b, &n); err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if !validID(n.ID) || entry.Name() != n.ID+".json" {
				return fmt.Errorf("invalid note ID in %s", path)
			}
			if n.Format != "markdown" && n.Format != "xml" {
				return fmt.Errorf("unsupported note format in %s", path)
			}
			records = append(records, record{&n, path, b})
		}
	}
	// Validate duplicates before changing any file. Only byte-identical copies
	// left between link and unlink can be reconciled automatically.
	seen := map[string][]byte{}
	for _, r := range records {
		if previous, ok := seen[r.note.ID]; ok && !bytes.Equal(previous, r.data) {
			return fmt.Errorf("conflicting copies of note %s: %w", r.note.ID, ErrConflict)
		}
		seen[r.note.ID] = r.data
	}
	now := time.Now()
	for _, r := range records {
		n := r.note
		if l.Get(n.ID) != nil {
			continue
		}
		target := l.notePath(n)
		if r.path != target {
			if err := moveNoteFile(r.path, target); err != nil {
				return err
			}
		}
		// An interrupted move may have left the same file in both directories.
		other := *n
		other.Deleted = !n.Deleted
		if b, err := os.ReadFile(l.notePath(&other)); err == nil && bytes.Equal(b, r.data) {
			if err := os.Remove(l.notePath(&other)); err != nil {
				return err
			}
		}
		if n.Deleted && n.DeletedAt.IsZero() {
			n.DeletedAt = now
			r.data, err = json.MarshalIndent(n, "", "  ")
			if err != nil {
				return err
			}
			if err := AtomicWrite(target, r.data); err != nil {
				return err
			}
		}
		l.Notes = append(l.Notes, n)
		l.versions[n.ID] = sha256.Sum256(r.data)
	}
	return nil
}

// Both paths belong to this library. Link/unlink never overwrites a competing
// destination. Unsupported or cross-device links fail with the source intact.
func moveNoteFile(from, to string) error {
	if err := os.Link(from, to); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
		a, aerr := os.ReadFile(from)
		b, berr := os.ReadFile(to)
		if aerr != nil || berr != nil || !bytes.Equal(a, b) {
			return fmt.Errorf("destination already exists: %s: %w", to, ErrConflict)
		}
	}
	return os.Remove(from)
}

// SetDeleted moves one record. Tree operations below call this under one lock.
// External files are only forgotten; they never enter either trash.
func (l *Library) SetDeleted(n *Note, deleted bool) error {
	if n == nil || !validID(n.ID) {
		return errors.New("invalid note ID")
	}
	if deleted && n.External != "" && !n.Deleted {
		return l.Forget(n)
	}
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	next := *n
	if deleted {
		next.TrashRoot = n.ID
	}
	return l.setDeleted(n, next, deleted, time.Now())
}

func (l *Library) setDeleted(n *Note, next Note, deleted bool, now time.Time) error {
	if n.Deleted == deleted {
		return nil
	}
	if err := l.checkTrashMove(n, deleted); err != nil {
		return err
	}
	from := l.notePath(n)
	next.Deleted = deleted
	if deleted {
		next.DeletedAt = now
	} else {
		next.DeletedAt = time.Time{}
		next.TrashRoot = ""
	}
	to := l.notePath(&next)
	original, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(&next, "", "  ")
	if err != nil {
		return err
	}
	// Persist the intended state first. A crash before the move is completed by
	// loadNotes; failed moves are rolled back before returning to the caller.
	if err := AtomicWrite(from, data); err != nil {
		return err
	}
	if err := os.Link(from, to); err != nil {
		return errors.Join(err, AtomicWrite(from, original))
	}
	if err := os.Remove(from); err != nil {
		return errors.Join(err, os.Remove(to), AtomicWrite(from, original))
	}
	*n = next
	l.versions[n.ID] = sha256.Sum256(data)
	return nil
}

// Preflight every group member before starting a move, so a known conflict in
// a child cannot leave only its parent moved. Each actual move rechecks as well.
func (l *Library) checkTrashMove(n *Note, deleted bool) error {
	if err := checkVersion(l.notePath(n), l.versions[n.ID]); err != nil {
		return err
	}
	if n.Deleted == deleted {
		return nil
	}
	next := *n
	next.Deleted = deleted
	if _, err := os.Lstat(l.notePath(&next)); !errors.Is(err, fs.ErrNotExist) {
		if err != nil {
			return err
		}
		return fmt.Errorf("destination already exists: %w", ErrConflict)
	}
	return nil
}

func (l *Library) trashGroup(n *Note) []*Note {
	group := []*Note{n}
	for _, child := range l.Notes {
		if child.ID != n.ID && child.Deleted && child.TrashRoot == n.ID {
			group = append(group, child)
		}
	}
	return group
}

// RestoreTree returns whether a missing original parent or folder required a
// move to the library root. Parents are restored before their children.
func (l *Library) RestoreTree(n *Note) (bool, error) {
	if n == nil || !validID(n.ID) {
		return false, errors.New("invalid note ID")
	}
	unlock, err := l.lock()
	if err != nil {
		return false, err
	}
	defer unlock()
	group := l.trashGroup(n)
	for _, item := range group {
		if err := l.checkTrashMove(item, false); err != nil {
			return false, err
		}
	}
	depth := func(item *Note) int {
		seen := map[string]bool{item.ID: true}
		d := 0
		for p := l.Get(item.Parent); p != nil && !seen[p.ID]; p = l.Get(p.Parent) {
			seen[p.ID] = true
			d++
		}
		return d
	}
	slices.SortStableFunc(group, func(a, b *Note) int { return depth(a) - depth(b) })
	folders := l.Folders()
	moved := false
	for _, item := range group {
		next := *item
		if next.Parent != "" {
			parent := l.Get(next.Parent)
			if parent == nil || parent.Deleted {
				next.Parent, next.Folder = "", ""
				moved = true
			} else if next.Folder != "" && !slices.Contains(folders, next.Folder) {
				next.Folder = parent.Folder
			}
		} else if next.Folder != "" && !slices.Contains(folders, next.Folder) {
			next.Folder = ""
			moved = true
		}
		if err := l.setDeleted(item, next, false, time.Now()); err != nil {
			return moved, err
		}
	}
	return moved, nil
}

// Purge permanently removes the selected deletion group, including history and
// recovery copies. External originals and shared assets remain untouched.
func (l *Library) Purge(n *Note) error {
	if n == nil || !validID(n.ID) {
		return errors.New("invalid note ID")
	}
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	group := l.trashGroup(n)
	for _, item := range group {
		if !item.Deleted {
			return errors.New("only trashed notes can be permanently deleted")
		}
		if err := checkVersion(l.notePath(item), l.versions[item.ID]); err != nil {
			return err
		}
	}
	// Keep the root as a visible retry target if deleting a child fails.
	slices.Reverse(group)
	for _, item := range group {
		if err := l.purgeOne(item); err != nil {
			return err
		}
	}
	return nil
}

// PruneTrash runs when the application lists Trash, not in a background daemon.
func (l *Library) PruneTrash(now time.Time) error {
	for _, n := range l.Query("Trash", "", "", "") {
		if !n.DeletedAt.IsZero() && !now.Before(n.DeletedAt.Add(TrashRetention)) {
			if err := l.Purge(n); err != nil {
				return err
			}
		}
	}
	return nil
}

// Forget removes only the library's registration of an external file. Children
// remain in the library, and no operation opens or modifies the external path.
func (l *Library) Forget(n *Note) error {
	if n == nil || !validID(n.ID) || n.External == "" {
		return errors.New("only external files can be removed from the list")
	}
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := checkVersion(l.notePath(n), l.versions[n.ID]); err != nil {
		return err
	}
	// Children retain their parent ID until restored or moved; root queries
	// expose missing-parent documents as top-level entries.
	if err := l.removeNoteCopies(n.ID); err != nil {
		return err
	}
	if err := os.Remove(l.notePath(n)); err != nil {
		return err
	}
	l.Notes = slices.DeleteFunc(l.Notes, func(item *Note) bool { return item.ID == n.ID })
	delete(l.versions, n.ID)
	return nil
}
