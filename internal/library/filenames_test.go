package library

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func TestImportedFilenamesAndCollisionLifecycle(t *testing.T) {
	for _, name := range []string{"Original name 世界.md", "Guide.MDX", "Notes.markdown", "Data.XML", " leading space.md"} {
		t.Run(name, func(t *testing.T) {
			l := mustLibrary(t)
			format := "markdown"
			if strings.EqualFold(filepath.Ext(name), ".xml") {
				format = "xml"
			}
			n, err := l.CreateFile("Different display title", "Original body", format, name)
			if err != nil || n.FileName != name {
				t.Fatal("original filename changed", n, err)
			}
			if err := l.TrashTree(n, true); err != nil {
				t.Fatal(err)
			}
			copy, err := l.CreateFile("Same display title", "Second body", format, name)
			want := strings.TrimSuffix(name, filepath.Ext(name)) + " (2)" + filepath.Ext(name)
			if err != nil || copy.FileName != want {
				t.Fatal("trash did not reserve the original name", copy, err)
			}
			if _, err := l.RestoreTree(n); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(l.Dir)
			if err != nil || len(reopened.Notes) != 2 || reopened.Get(n.ID).Source != "Original body" || reopened.Get(copy.ID).Source != "Second body" {
				t.Fatal("restore or reopen lost documents", err)
			}
			if reopened.Get(n.ID).FileName != name || reopened.Get(copy.ID).FileName != want {
				t.Fatal("reopen changed filenames")
			}
		})
	}
}

func TestFilenameRenameHistoryAndOtherWindow(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "First")
	id, source := n.ID, n.Source
	other, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	n.Title = "Renamed 世界"
	n.Source = "Updated body"
	if err := l.Save(n); err != nil || n.FileName != "Renamed 世界.md" {
		t.Fatal("title rename failed", n.FileName, err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir, "notes", "First.md")); !os.IsNotExist(err) {
		t.Fatal("old filename remains", err)
	}
	if !other.Changed(other.Get(id)) {
		t.Fatal("other window missed the rename")
	}
	if err := other.Save(other.Get(id)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale writer recreated the old name", err)
	}
	if err := other.Reload(id); err != nil || other.Get(id).FileName != n.FileName || other.Get(id).Source != n.Source {
		t.Fatal("other window could not find renamed note", err)
	}
	history, err := l.History(id)
	if err != nil || len(history) != 1 || history[0].Note.Source != source {
		t.Fatal("rename broke history", history, err)
	}
	n.Title = "RENAMED 世界"
	if err := l.Save(n); err != nil {
		t.Fatal("case-only rename failed", err)
	}
	entries, err := os.ReadDir(filepath.Join(l.Dir, "notes"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "RENAMED 世界.md" {
		t.Fatal("case-only rename lost the only entry", entries, err)
	}
	if _, err := Open(l.Dir); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyReadableNameMigration(t *testing.T) {
	dir := t.TempDir()
	for _, folder := range []string{"notes", "trash"} {
		if err := os.MkdirAll(filepath.Join(dir, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Round(0)
	parent := &Note{ID: strings.Repeat("a", 26), Title: "Original 世界", Source: "Exact\r\nbody  \n", Format: "markdown", Created: now, Updated: now}
	child := *parent
	child.ID, child.Parent, child.Folder = strings.Repeat("b", 26), parent.ID, "Work/Nested"
	child.Source, child.Deleted, child.DeletedAt = "mory:"+parent.ID, true, now
	for _, n := range []*Note{parent, &child} {
		folder := "notes"
		if n.Deleted {
			folder = "trash"
		}
		data, err := marshalNote(n)
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(dir, folder, n.ID+".md"), data)
	}
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if l.Get(parent.ID).FileName != "Original 世界.md" || l.Get(child.ID).FileName != "Original 世界 (2).md" {
		t.Fatal("legacy names were not made readable", l.Get(parent.ID), l.Get(child.ID))
	}
	for _, want := range []*Note{parent, &child} {
		got := l.Get(want.ID)
		if got.Source != want.Source || got.Parent != want.Parent || got.Folder != want.Folder || !got.Updated.Equal(want.Updated) || got.Deleted != want.Deleted {
			t.Fatal("migration changed document identity or content", got)
		}
		before, err := os.ReadFile(l.notePath(got))
		if err != nil {
			t.Fatal(err)
		}
		reopened, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(reopened.notePath(reopened.Get(want.ID)))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("migration is not idempotent", err)
		}
	}
}

func TestFilenameSafetyAndConcurrentCreation(t *testing.T) {
	l := mustLibrary(t)
	for _, name := range []string{"../escape.md", "/outside.md", "a\\b.md", "bad\x00.md", "unknown.txt", strings.Repeat("a", 253) + ".md"} {
		if _, err := l.CreateFile("Title", "Body", "markdown", name); err == nil {
			t.Fatal("invalid basename accepted", name)
		}
	}
	n := mustNote(t, l, "../"+strings.Repeat("世界", 150))
	if !validDocumentName(n.FileName) || !utf8.ValidString(n.FileName) || !strings.HasPrefix(l.notePath(n), filepath.Join(l.Dir, "notes")+string(filepath.Separator)) {
		t.Fatal("unsafe or truncated generated filename", n.FileName)
	}
	for _, title := range []string{"Report", "report", "café", "cafe\u0301"} {
		mustNote(t, l, title)
	}
	if l.Notes[2].FileName != "report (2).md" || l.Notes[4].FileName != "cafe\u0301 (2).md" {
		t.Fatal("case or Unicode collision not detected")
	}
	other, err := Open(l.Dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, writer := range []*Library{l, other} {
		wg.Go(func() {
			_, err := writer.CreateFile("Concurrent", "Body", "markdown", "Concurrent.md")
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"Concurrent.md", "Concurrent (2).md"} {
		if _, err := os.Stat(filepath.Join(l.Dir, "notes", name)); err != nil {
			t.Fatal("concurrent create lost a document", err)
		}
	}
	occupied := filepath.Join(l.Dir, "notes", "Occupied.md")
	mustWrite(t, occupied, []byte("Keep original"))
	if err := writeNewRecord(occupied, []byte("Replacement")); !errors.Is(err, ErrConflict) {
		t.Fatal("initial write overwrote existing file", err)
	}
	data, _ := os.ReadFile(occupied)
	if string(data) != "Keep original" {
		t.Fatal("occupied file changed")
	}
}

func TestInterruptedFilenameMoveAndConflict(t *testing.T) {
	for _, both := range []bool{false, true} {
		l := mustLibrary(t)
		n := mustNote(t, l, "Before")
		from := l.notePath(n)
		n.Title, n.FileName = "After", "After.md"
		data, _ := marshalNote(n)
		mustWrite(t, from, data)
		if both {
			mustWrite(t, l.notePath(n), data)
		}
		reopened, err := Open(l.Dir)
		if err != nil || reopened.Get(n.ID).FileName != "After.md" {
			t.Fatal("interrupted rename did not resume", err)
		}
		if _, err := os.Stat(from); !os.IsNotExist(err) {
			t.Fatal("old name remains after recovery", err)
		}
		actual, _ := os.ReadFile(reopened.notePath(reopened.Get(n.ID)))
		if !bytes.Equal(actual, data) {
			t.Fatal("rename recovery changed bytes")
		}
	}
	l := mustLibrary(t)
	n := mustNote(t, l, "Before")
	from := l.notePath(n)
	n.FileName = "Occupied.md"
	data, _ := marshalNote(n)
	mustWrite(t, from, data)
	other := mustNote(t, l, "Other")
	otherPath := l.notePath(other)
	if err := os.Rename(otherPath, l.notePath(n)); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(l.Dir); !errors.Is(err, ErrConflict) {
		t.Fatal("interrupted rename overwrote another note", err)
	}
	actual, _ := os.ReadFile(from)
	if !bytes.Equal(actual, data) {
		t.Fatal("failed rename changed original")
	}
}
