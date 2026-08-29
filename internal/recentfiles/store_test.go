package recentfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreKeepsExistingSupportedDocumentsAndWorkspacesMostRecentFirst(t *testing.T) {
	root := t.TempDir()
	store := New(filepath.Join(root, "state", "recent-files.json"))
	first := filepath.Join(root, "first.md")
	second := filepath.Join(root, "second.txt")
	unsupported := filepath.Join(root, "image.png")
	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first, second, unsupported} {
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{first, unsupported, workspace, second, first} {
		if err := store.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0] != first || entries[1] != second || entries[2] != workspace {
		t.Fatalf("entries = %#v", entries)
	}
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	entries, err = store.List()
	if err != nil || len(entries) != 2 || entries[0] != second || entries[1] != workspace {
		t.Fatalf("pruned entries = %#v, error = %v", entries, err)
	}
}

func TestStoreBoundsAndClearsEntries(t *testing.T) {
	root := t.TempDir()
	store := New(filepath.Join(root, "recent-files.json"))
	for index := 0; index < maximumEntries+2; index++ {
		path := filepath.Join(root, string(rune('a'+index))+".md")
		if err := os.WriteFile(path, []byte("note"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := store.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := store.List()
	if err != nil || len(entries) != maximumEntries {
		t.Fatalf("entries = %d, error = %v", len(entries), err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	entries, err = store.List()
	if err != nil || len(entries) != 0 {
		t.Fatalf("cleared entries = %#v, error = %v", entries, err)
	}
}
