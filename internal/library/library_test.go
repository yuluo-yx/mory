package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustLibrary(t *testing.T) *Library {
	t.Helper()
	l, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func mustNote(t *testing.T, l *Library, title string) *Note {
	t.Helper()
	n, err := l.Create(title, "# Unicode\n\n世界 🌱\n", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLifecycle(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "First")
	n.Pinned = true
	n.Starred = true
	n.Folder = "Work"
	n.Opened = time.Now()
	if err := l.Save(n); err != nil {
		t.Fatal(err)
	}
	copy, err := l.Duplicate(n)
	if err != nil {
		t.Fatal(err)
	}
	if copy.ID == n.ID || copy.Source != n.Source || copy.Folder != "Work" {
		t.Fatalf("duplicate: %+v", copy)
	}
	l.Settings.Appearance = "Dark"
	if err = l.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	l2, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if l2.Get(n.ID).Source != n.Source || l2.Settings.Appearance != "Dark" {
		t.Fatal("reopen lost data")
	}
	if err = l.SetDeleted(n, true); err != nil {
		t.Fatal(err)
	}
	if len(l.Query("Trash", "", "", "")) != 1 {
		t.Fatal("trash")
	}
	if err = l.SetDeleted(n, false); err != nil {
		t.Fatal(err)
	}
	if len(l.Folders()) != 1 {
		t.Fatal("folders")
	}
	if err = l.Purge(n); err == nil {
		t.Fatal("purged active note")
	}
	if err = l.SetDeleted(n, true); err != nil {
		t.Fatal(err)
	}
	if err = l.Purge(n); err != nil {
		t.Fatal(err)
	}
	if l.Get(n.ID) != nil {
		t.Fatal("purge kept note")
	}
	if l.Get("missing") != nil {
		t.Fatal("missing ID")
	}
}

func TestConflictAndDeletionRecovery(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Draft")
	other, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	otherNote := other.Get(n.ID)
	otherNote.Source = "External change"
	if err = other.Save(otherNote); err != nil {
		t.Fatal(err)
	}
	n.Source = "Local edit"
	if err = l.Save(n); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict: %v", err)
	}
	if err = l.SetDeleted(n, true); !errors.Is(err, ErrConflict) || n.Deleted {
		t.Fatalf("failed trash changed state: %v", err)
	}
	n.Deleted = true
	if err = l.Purge(n); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	n.Deleted = false
	copy, err := l.Duplicate(n)
	if err != nil {
		t.Fatal(err)
	}
	if copy.Source != "Local edit" {
		t.Fatal("recovery lost local edits")
	}
	if err = os.Remove(filepath.Join(l.Dir, "notes", copy.ID+".json")); err != nil {
		t.Fatal(err)
	}
	if err = l.Save(copy); !errors.Is(err, ErrConflict) {
		t.Fatal("deleted file overwritten")
	}
}

func TestQuery(t *testing.T) {
	l := mustLibrary(t)
	a := mustNote(t, l, "Alpha")
	b := mustNote(t, l, "Beta")
	b.Starred = true
	b.Opened = time.Now()
	a.Pinned = true
	a.Folder = "Home"
	for _, tc := range []struct {
		section, query, folder, order string
		count                         int
	}{{"All notes", "世界", "", "", 2}, {"All notes", "missing", "", "", 0}, {"Favorites", "", "", "", 1}, {"Recent", "", "", "", 1}, {"All notes", "", "Home", "Title", 1}, {"Trash", "", "", "", 0}} {
		if got := len(l.Query(tc.section, tc.query, tc.folder, tc.order)); got != tc.count {
			t.Fatalf("%+v: %d", tc, got)
		}
	}
	if l.Query("All notes", "", "", "Modified")[0] != a {
		t.Fatal("pin ordering")
	}
	a.Pinned = false
	if l.Query("All notes", "", "", "Created")[0] != b {
		t.Fatal("created ordering")
	}
	if l.Query("All notes", "", "", "Title")[0] != a {
		t.Fatal("title ordering")
	}
	a.Updated = b.Updated
	a.Created = b.Created
	a.Title = b.Title
	a.Opened = b.Opened
	for _, order := range []string{"Title", "Modified", "Created"} {
		if len(l.Query("All notes", "", "", order)) != 2 {
			t.Fatal(order)
		}
	}
	if len(l.Query("Recent", "", "", "")) != 2 {
		t.Fatal("recent")
	}
}

func TestInvalidFilesAndSettings(t *testing.T) {
	for _, tc := range []struct{ name, content string }{{"broken.json", "{"}, {"bad.json", `{"id":"../escape","format":"markdown"}`}, {strings.Repeat("a", 26) + ".json", `{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaa","format":"unknown"}`}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "notes", tc.name), []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(dir); err == nil {
				t.Fatal("invalid file accepted")
			}
		})
	}
	l := mustLibrary(t)
	if err := l.Save(&Note{ID: "../escape"}); err == nil {
		t.Fatal("invalid ID")
	}
	if err := l.Save(&Note{ID: strings.Repeat("a", 26), Format: "bad"}); err == nil {
		t.Fatal("invalid format")
	}
	path := filepath.Join(l.Dir, "settings.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(l.Dir); err == nil {
		t.Fatal("bad settings accepted")
	}
	if err := os.WriteFile(path, []byte(`{"fontSize":100}`), 0o644); err != nil {
		t.Fatal(err)
	}
	l2, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if l2.Settings.FontSize != 17 {
		t.Fatal("size not normalized")
	}
	if err = l.SaveSettings(); !errors.Is(err, ErrConflict) {
		t.Fatal("settings conflict missed")
	}
	if _, err = Open(path); err == nil {
		t.Fatal("file treated as directory")
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := AtomicWrite(path, []byte("original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatal("permissions changed")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary file leaked")
	}
	if err := AtomicWrite(filepath.Join(dir, "missing", "note"), nil); err == nil {
		t.Fatal("missing parent accepted")
	}
	if err := AtomicWrite(dir, []byte("bad")); err == nil {
		t.Fatal("replaced directory")
	}
}

func BenchmarkSearch1000Notes(b *testing.B) {
	l := &Library{}
	for i := 0; i < 1000; i++ {
		l.Notes = append(l.Notes, &Note{Title: "Writing", Source: strings.Repeat("local writing ", 100), Updated: time.Now()})
	}
	b.ResetTimer()
	for b.Loop() {
		l.Query("All notes", "writing", "", "Modified")
	}
}

func TestConcurrentWriters(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Concurrent")
	a, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	a.Get(n.ID).Source = "writer A"
	b.Get(n.ID).Source = "writer B"
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, writer := range []*Library{a, b} {
		wg.Go(func() { <-start; results <- writer.Save(writer.Get(n.ID)) })
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
}

func TestFilesystemFailures(t *testing.T) {
	l := mustLibrary(t)
	if err := os.Mkdir(filepath.Join(l.Dir, "notes", "directory.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l.Dir, "notes", "readme.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(l.Dir); err != nil {
		t.Fatal(err)
	}
	if validID(strings.Repeat("1", 26)) {
		t.Fatal("invalid ID accepted")
	}
	if err := os.Remove(filepath.Join(l.Dir, ".write.lock")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(l.Dir, ".write.lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Create("Blocked", "text", "markdown"); err == nil {
		t.Fatal("lock failure ignored")
	}
	if err := l.SaveSettings(); err == nil {
		t.Fatal("settings lock failure ignored")
	}
	if _, err := l.Duplicate(&Note{}); err == nil {
		t.Fatal("duplicate failure ignored")
	}
	if err := l.Purge(&Note{Deleted: true}); err == nil {
		t.Fatal("purge lock failure ignored")
	}
}

func TestSettingsReloadPreservesNoteVersions(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, err := l.Create("Draft", "Original", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Settings.PagePosition = "Right"
	other.Settings.OutlineFontSize = 19
	if err := other.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err != nil {
		t.Fatal(err)
	}
	if l.Settings.PagePosition != "Right" || l.Settings.OutlineFontSize != 19 {
		t.Fatal("stale preferences")
	}
	n.Source = "Edited after reload"
	if err := l.Save(n); err != nil {
		t.Fatal("reload lost note version", err)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"Docusaurus"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err != nil || l.Settings.Keys == nil || l.Settings.DisabledPlugins == nil {
		t.Fatal("legacy defaults", err)
	}
	if err := os.WriteFile(path, []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err == nil || l.Settings.Theme != "Docusaurus" {
		t.Fatal("invalid settings replaced live preferences")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err == nil {
		t.Fatal("missing settings accepted")
	}
}
