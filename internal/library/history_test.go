package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func historyFixture(t testing.TB, l *Library, n *Note, at time.Time, source string, restore bool) string {
	t.Helper()
	copy := *n
	copy.Source, copy.Updated = source, at
	b, err := json.Marshal(copy)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(l.Dir, "history", n.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := at.UTC().Format(historyTimeForm)
	if restore {
		name += "-restore"
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAutosaveCoalescingSurvivesReopen(t *testing.T) {
	l, _ := Open(t.TempDir())
	n, _ := l.Create("Draft", "original", "markdown")
	now := time.Now().Add(-time.Hour)
	for i := range 120 {
		n.Source = fmt.Sprintf("edit %d", i)
		if err := l.save(n, false, now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	h, err := l.History(n.ID)
	if err != nil || len(h) != 1 || h[0].Note.Source != "original" {
		t.Fatalf("rapid saves did not retain the initial checkpoint: %+v, %v", h, err)
	}
	reopened, _ := Open(l.Dir)
	n = reopened.Get(n.ID)
	if n.Source != "edit 119" {
		t.Fatal("autosave stopped writing current content")
	}
	n.Source = "after reopen"
	if err := reopened.save(n, false, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	h, _ = reopened.History(n.ID)
	if len(h) != 1 {
		t.Fatal("reopening reset the checkpoint interval")
	}
	n.Source = "next interval"
	if err := reopened.save(n, false, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := reopened.save(n, false, now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	h, _ = reopened.History(n.ID)
	if len(h) != 2 || h[0].Note.Source != "after reopen" {
		t.Fatal("interval boundary or unchanged-content suppression failed", h)
	}
	// A backwards clock must not produce a new snapshot on every keystroke.
	n.Source = "clock moved backwards"
	if err := reopened.save(n, false, now); err != nil {
		t.Fatal(err)
	}
	h, _ = reopened.History(n.ID)
	if len(h) != 2 {
		t.Fatal("clock rollback created duplicate checkpoints")
	}
}

func TestLegacyHistoryRetentionAndIsolation(t *testing.T) {
	l, _ := Open(t.TempDir())
	n, _ := l.Create("Draft", "current", "markdown")
	other, _ := l.Create("Other", "independent", "markdown")
	now := time.Now().UTC()
	for i := range 424 {
		historyFixture(t, l, n, now.Add(-time.Duration(i)*time.Second), fmt.Sprintf("legacy %d", i), false)
	}
	otherPath := historyFixture(t, l, other, now.Add(-90*24*time.Hour), "other old version", false)
	historyFixture(t, l, n, now.Add(-90*24*time.Hour), "expired", false)
	unknown := filepath.Join(l.Dir, "history", n.ID, "readme.txt")
	mustWrite(t, unknown, []byte("Keep unrelated files"))
	if err := l.Recover(n); err != nil {
		t.Fatal(err)
	}
	h, err := l.History(n.ID)
	if err != nil || len(h) != 2 || h[0].Note.Source != "legacy 0" || h[1].Note.Source != "legacy 300" {
		t.Fatalf("legacy coalescing: %+v, %v", h, err)
	}
	for _, path := range []string{otherPath, unknown, l.recoveryPath(n), filepath.Join(l.Dir, "notes", n.ID+".json")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("retention touched unrelated data", path, err)
		}
	}
	if err := l.pruneHistory(n.ID, now.Add(120*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	files, _ := l.historyFiles(n.ID)
	if len(files) != 1 {
		t.Fatal("expired documents must keep their latest checkpoint")
	}
}

func TestHistoryCountAndStorageLimits(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int64
		want int
	}{
		{"count", 0, 3},
		{"bytes", 600 << 10, 1},
		{"oversized latest", 2 << 20, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _ := Open(t.TempDir())
			n, _ := l.Create("Draft", "current", "markdown")
			l.Settings.History = HistorySettings{MaxVersions: 3, MaxMiB: 1}
			now := time.Now()
			for i := range 6 {
				path := historyFixture(t, l, n, now.Add(-time.Duration(i)*time.Hour), fmt.Sprint(i), false)
				if tc.size > 0 {
					// Sparse fixtures exercise byte accounting without allocating large documents.
					if err := os.Truncate(path, tc.size); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := l.pruneHistory(n.ID, now); err != nil {
				t.Fatal(err)
			}
			files, _ := l.historyFiles(n.ID)
			if len(files) != tc.want || files[0].name != now.UTC().Format(historyTimeForm)+".json" {
				t.Fatalf("retention lost the latest checkpoint or exceeded the limit: %+v", files)
			}
		})
	}
}

func TestHistoryDisabledAndRestoreCheckpoints(t *testing.T) {
	l, _ := Open(t.TempDir())
	n, _ := l.Create("Draft", "original", "markdown")
	n.Source = "working copy"
	if err := l.Save(n); err != nil {
		t.Fatal(err)
	}
	l.Settings.History.Disabled = true
	n.Source = "latest working copy"
	if err := l.Save(n); err != nil {
		t.Fatal(err)
	}
	h, _ := l.History(n.ID)
	if len(h) != 1 || h[0].Note.Source != "original" {
		t.Fatal("disabling history deleted or added ordinary snapshots")
	}
	n.Source = "original"
	if err := l.SaveWithHistory(n); err != nil {
		t.Fatal(err)
	}
	h, _ = l.History(n.ID)
	if len(h) != 2 || h[0].Note.Source != "latest working copy" {
		t.Fatal("restore did not preserve the previous working copy", h)
	}
	l.Settings.History.Disabled = false
	n.Source = "latest working copy"
	if err := l.SaveWithHistory(n); err != nil {
		t.Fatal(err)
	}
	h, _ = l.History(n.ID)
	if len(h) != 3 || h[0].Note.Source != "original" {
		t.Fatal("retention coalesced explicit restore checkpoints", h)
	}
}

func TestHistoryPolicyValidationAndPersistence(t *testing.T) {
	l, _ := Open(t.TempDir())
	if l.HistoryPolicy() != DefaultHistorySettings() {
		t.Fatal("legacy settings did not use bounded defaults")
	}
	l.Settings.History = HistorySettings{IntervalMinutes: -1, RetentionDays: 366, MaxVersions: 1001, MaxMiB: 1025}
	if l.HistoryPolicy() != DefaultHistorySettings() {
		t.Fatal("invalid settings did not fall back to defaults")
	}
	policy := HistorySettings{Disabled: true, IntervalMinutes: 10, RetentionDays: 7, MaxVersions: 20, MaxMiB: 16}
	l.Settings.History = policy
	if err := l.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(l.Dir)
	if err != nil || reopened.HistoryPolicy() != policy {
		t.Fatal("history settings did not persist", err)
	}
	policy.Disabled = false
	l.Settings.History = policy
	if err := l.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	if err := reopened.ReloadSettings(); err != nil || reopened.HistoryPolicy() != policy {
		t.Fatal("history settings did not reload", err)
	}
	// A custom interval affects actual saves, not just the settings display.
	n, _ := l.Create("Draft", "original", "markdown")
	now := time.Now().Add(-time.Hour)
	for _, minutes := range []int{0, 5, 10} {
		n.Source = fmt.Sprint(minutes)
		if err := l.save(n, false, now.Add(time.Duration(minutes)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	h, _ := l.History(n.ID)
	if len(h) != 2 || h[0].Note.Source != "5" {
		t.Fatal("custom interval was ignored", h)
	}
}

func TestRestoreCheckpointSurvivesClockRollbackAndTightLimit(t *testing.T) {
	l, _ := Open(t.TempDir())
	n, _ := l.Create("Draft", "current", "markdown")
	l.Settings.History.MaxVersions = 1
	now := time.Now()
	historyFixture(t, l, n, now.Add(time.Hour), "future timestamp", false)
	n.Source = "restored"
	if err := l.save(n, true, now); err != nil {
		t.Fatal(err)
	}
	h, err := l.History(n.ID)
	if err != nil || len(h) != 1 || h[0].Note.Source != "current" {
		t.Fatal("clock rollback pruned the pre-restore working version", h, err)
	}
}

func TestPurgeRemovesOnlyOwnedCopies(t *testing.T) {
	l, _ := Open(t.TempDir())
	n, _ := l.Create("Deleted", "original", "markdown")
	other, _ := l.Create("Other", "keep", "markdown")
	n.Source = "changed"
	if err := l.Save(n); err != nil {
		t.Fatal(err)
	}
	for _, note := range []*Note{n, other} {
		if err := l.Recover(note); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(l.Dir, "recovery", n.ID+".json"), []byte("legacy recovery"))
	asset := filepath.Join(l.Dir, "assets", "shared.txt")
	mustWrite(t, asset, []byte("shared asset"))
	otherHistory := historyFixture(t, l, other, time.Now(), "other version", false)
	if err := l.SetDeleted(n, true); err != nil {
		t.Fatal(err)
	}
	if err := l.Purge(n); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir, "history", n.ID)); !os.IsNotExist(err) {
		t.Fatal("purge retained the history directory", err)
	}
	entries, _ := os.ReadDir(filepath.Join(l.Dir, "recovery"))
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), other.ID) {
		t.Fatal("purge retained owned recoveries or removed another note's recovery")
	}
	for _, path := range []string{asset, otherHistory} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("purge removed shared or unrelated data", err)
		}
	}
	if err := l.Purge(&Note{ID: "../invalid", Deleted: true}); err == nil {
		t.Fatal("invalid purge ID accepted")
	}
}

func TestHistoryFailureLeavesCurrentDocumentIntact(t *testing.T) {
	l, _ := Open(t.TempDir())
	n, _ := l.Create("Draft", "original", "markdown")
	// A file at the history-directory path makes checkpoint creation fail.
	mustWrite(t, filepath.Join(l.Dir, "history", n.ID), []byte("blocked"))
	n.Source = "unsaved edit"
	if err := l.Save(n); err == nil {
		t.Fatal("failed checkpoint was ignored")
	}
	reopened, _ := Open(l.Dir)
	if reopened.Get(n.ID).Source != "original" {
		t.Fatal("checkpoint failure overwrote the original")
	}
	if _, err := l.History(n.ID); err == nil {
		t.Fatal("unreadable history was ignored")
	}
	// An external conflict must not generate history before ownership validation.
	path := filepath.Join(t.TempDir(), "external.md")
	mustWrite(t, path, []byte("external original"))
	external, _ := l.OpenExternal(path)
	mustWrite(t, path, []byte("changed elsewhere"))
	external.Source = "local edit"
	if err := l.Save(external); !errors.Is(err, ErrConflict) {
		t.Fatal("external conflict was ignored", err)
	}
	h, err := l.History(external.ID)
	if err != nil || len(h) != 0 {
		t.Fatal("rejected external save created history", h, err)
	}
}

func BenchmarkAutosaveWithBoundedHistory(b *testing.B) {
	l, err := Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	n, err := l.Create("Draft", "current", "markdown")
	if err != nil {
		b.Fatal(err)
	}
	now := time.Now()
	for i := range 100 {
		historyFixture(b, l, n, now.Add(-time.Duration(i)*5*time.Minute), fmt.Sprint(i), false)
	}
	i := 0
	for b.Loop() {
		i++
		n.Source = fmt.Sprintf("Autosaved edit %d", i)
		if err := l.save(n, false, now); err != nil {
			b.Fatal(err)
		}
	}
}
