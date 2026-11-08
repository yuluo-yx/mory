package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExternalHistoryAndConflictRecovery(t *testing.T) {
	l, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Original.md")
	mustWrite(t, path, []byte("original"))
	n, err := l.OpenExternal(path)
	if err != nil {
		t.Fatal(err)
	}
	id := n.ID
	n.Source = "edited"
	if err = l.Save(n); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "edited" {
		t.Fatal("external file was not saved")
	}
	history, err := l.History(id)
	if err != nil || len(history) != 1 || history[0].Note.Source != "original" {
		t.Fatalf("history: %+v %v", history, err)
	}
	mustWrite(t, path, []byte("other editor"))
	n.Source = "unsaved local"
	if !l.Changed(n) {
		t.Fatal("external change not detected")
	}
	if err = l.Save(n); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = l.Recover(n); err != nil {
		t.Fatal(err)
	}
	recovery, err := l.Recoveries()
	if err != nil || len(recovery) != 1 || recovery[0].Note.Source != "unsaved local" {
		t.Fatal(recovery, err)
	}
	if err = l.Reload(id); err != nil || n.Source != "other editor" {
		t.Fatal(err, n.Source)
	}
	again, err := l.OpenExternal(path)
	if err != nil || again.ID != id {
		t.Fatal(err)
	}
	if err = l.TrashTree(n, true); err != nil {
		t.Fatal(err)
	}
	if err = l.Purge(n); err != nil {
		t.Fatal(err)
	}
	if b, err = os.ReadFile(path); err != nil || string(b) != "other editor" {
		t.Fatal("purge touched original file")
	}
	if err = l.Reload(id); err == nil {
		t.Fatal("missing note accepted")
	}
}

func mustWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTreeFoldersAndOrder(t *testing.T) {
	l, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := l.Create("Parent", "p", "markdown")
	child, _ := l.Create("Child", "c", "markdown")
	grand, _ := l.Create("Grandchild", "g", "markdown")
	if err = l.RenameFolder("", "Work"); err != nil {
		t.Fatal(err)
	}
	if err = l.Move(child, "Work", parent.ID); err != nil {
		t.Fatal(err)
	}
	if err = l.Move(grand, "Work", child.ID); err != nil {
		t.Fatal(err)
	}
	if err = l.Move(parent, "", grand.ID); err == nil {
		t.Fatal("cycle allowed")
	}
	if err = l.Move(parent, "", "missing"); err == nil {
		t.Fatal("missing parent allowed")
	}
	if len(l.Descendants(parent.ID)) != 3 {
		t.Fatal("descendants")
	}
	if err = l.SetDeleted(grand, true); err != nil {
		t.Fatal(err)
	}
	if err = l.TrashTree(parent, true); err != nil {
		t.Fatal(err)
	}
	if !child.Deleted {
		t.Fatal("child remained visible")
	}
	if err = l.TrashTree(parent, false); err != nil {
		t.Fatal(err)
	}
	if child.Deleted || !grand.Deleted {
		t.Fatal("restore revived independent trash")
	}
	if err = l.RenameFolder("Work", "Writing"); err != nil {
		t.Fatal(err)
	}
	if child.Folder != "Writing" {
		t.Fatal("folder contents not moved")
	}
	if err = l.RenameFolder("", "Writing"); err == nil {
		t.Fatal("duplicate folder")
	}
	if err = l.RenameFolder("Writing", ""); err == nil {
		t.Fatal("empty folder")
	}
	if err = l.Reorder([]string{child.ID, parent.ID}); err != nil {
		t.Fatal(err)
	}
	if l.Query("All notes", "", "", "Manual")[0] != child {
		t.Fatal("manual order")
	}
	if err = l.DeleteFolder("Writing"); err != nil || child.Folder != "" || len(l.Folders()) != 0 {
		t.Fatal(err, l.Folders())
	}
	if err = l.EmptyTrash(); err != nil || l.Get(grand.ID) != nil {
		t.Fatal(err)
	}
	copy, err := l.Duplicate(parent)
	if err != nil || copy.ID == parent.ID {
		t.Fatal(err)
	}
	if err = l.Reload(parent.ID); err != nil {
		t.Fatal(err)
	}
	if l.Changed(parent) {
		t.Fatal("clean note changed")
	}
}

func TestHistoryIOErrors(t *testing.T) {
	l, _ := Open(t.TempDir())
	n, _ := l.Create("Note", "one", "markdown")
	if _, err := l.History("../invalid"); err == nil {
		t.Fatal("invalid id")
	}
	if err := l.Recover(&Note{ID: "bad"}); err == nil {
		t.Fatal("invalid recovery")
	}
	if h, err := l.History(n.ID); err != nil || len(h) != 0 {
		t.Fatal(err)
	}
	if _, err := l.OpenExternal(filepath.Join(l.Dir, "missing")); err == nil {
		t.Fatal("missing external file")
	}
	path := filepath.Join(l.Dir, "notes", n.ID+".json")
	mustWrite(t, path, []byte("bad json"))
	if err := l.Reload(n.ID); err == nil {
		t.Fatal("corrupt note accepted")
	}
	if !l.Changed(n) {
		t.Fatal("changed note")
	}
	other, _ := Open(t.TempDir())
	item, _ := other.Create("Note", "x", "xml")
	if err := os.Mkdir(filepath.Join(other.Dir, "history", item.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(other.Dir, "history", item.ID, "bad.json"), []byte("bad"))
	if _, err := other.History(item.ID); err == nil {
		t.Fatal("corrupt history accepted")
	}
	mustWrite(t, filepath.Join(other.Dir, "recovery", "bad.json"), []byte("bad"))
	if _, err := other.Recoveries(); err == nil {
		t.Fatal("corrupt recovery accepted")
	}
	missing := &Library{Dir: filepath.Join(t.TempDir(), "missing")}
	if _, err := missing.Recoveries(); err == nil {
		t.Fatal("missing recovery dir")
	}
	third, _ := Open(t.TempDir())
	p, _ := third.Create("P", "", "markdown")
	c, _ := third.Create("C", "", "markdown")
	c.Parent = p.ID
	third.Save(c)
	b, _ := json.Marshal(map[string]string{"changed": "outside"})
	mustWrite(t, filepath.Join(third.Dir, "notes", c.ID+".json"), b)
	if err := third.Move(c, "folder", ""); err == nil || c.Parent != p.ID {
		t.Fatal("failed move did not rollback")
	}
	if err := third.TrashTree(p, true); err == nil {
		t.Fatal("trash conflict ignored")
	}
	if err := third.Reorder([]string{c.ID}); err == nil {
		t.Fatal("reorder conflict ignored")
	}
	if err := third.RenameFolder("", "New"); err != nil {
		t.Fatal(err)
	}
}
