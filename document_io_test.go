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

func TestMissingExternalDismissAndRetry(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		t.Run(language, func(t *testing.T) {
			a := testApp(t)
			a.lib.Settings.Language = language
			path := filepath.Join(t.TempDir(), "Original.md")
			writeTest(t, path, "Cached content")
			a.openExternal(path)
			n := a.current
			a.createNote("Unrelated editor buffer", "Other")
			if err := os.Rename(path, path+".moved"); err != nil {
				t.Fatal(err)
			}
			a.openNote(n)
			tt := ui.NewTester(a.view, 1000, 720)
			if !tt.HasText(a.tr("Original file is missing")) || a.doc.Text().String() != "Cached content" {
				t.Fatal("missing file did not show its own cached content and recovery actions")
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, "missing-file-"+language+".png"))
			}
			click(t, tt, a.tr("Dismiss"))
			for range 3 {
				a.lastDiskCheck = time.Time{}
				a.backgroundTick()
				tt.Frame()
				if a.problem != "" {
					t.Fatal("background refresh reopened the dismissed error", a.problem)
				}
			}
			// Explicitly reopening the entry should show the missing-file actions.
			a.openNote(n)
			tt.Frame()
			click(t, tt, a.tr("Retry"))
			if a.problem == "" || a.missingExternal != n.ID {
				t.Fatal("retry hid the missing original")
			}
			writeTest(t, path, "Restored content")
			click(t, tt, a.tr("Retry"))
			if a.problem != "" || a.missingExternal != "" || a.current.Source != "Restored content" || a.doc.Text().String() != "Restored content" {
				t.Fatal("retry did not reload the restored file", a.problem)
			}
		})
	}
}

func TestMissingExternalCloseAndRemove(t *testing.T) {
	for _, action := range []string{"Close document", "Remove from list"} {
		t.Run(action, func(t *testing.T) {
			a := testApp(t)
			path := filepath.Join(t.TempDir(), "Original.md")
			writeTest(t, path, "Cached content")
			a.openExternal(path)
			n := a.current
			if err := os.Rename(path, path+".moved"); err != nil {
				t.Fatal(err)
			}
			a.lastDiskCheck = time.Time{}
			a.refreshDisk()
			tt := ui.NewTester(a.view, 1000, 720)
			click(t, tt, action)
			if a.current != nil || a.problem != "" || !tt.HasText("Search notes") {
				t.Fatal("missing file trapped the user in the editor", a.problem)
			}
			if (a.lib.Get(n.ID) == nil) != (action == "Remove from list") {
				t.Fatal("close and remove did not respect the chosen action")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("missing original was recreated", err)
			}
			if data, err := os.ReadFile(path + ".moved"); err != nil || string(data) != "Cached content" {
				t.Fatal("moved original was changed", err)
			}
		})
	}
}

func TestFailedAutosaveCanCloseAndRecoverImmediately(t *testing.T) {
	for _, mode := range []string{"Write", "Source"} {
		t.Run(mode, func(t *testing.T) {
			a := testApp(t)
			path := filepath.Join(t.TempDir(), "Original.md")
			writeTest(t, path, "Original content")
			a.openExternal(path)
			n := a.current
			a.changeMode(mode)
			if mode == "Write" {
				a.doc.InsertText("Pending ")
			} else {
				a.code.Buffer.Set("Pending Original content")
			}
			if err := os.Rename(path, path+".moved"); err != nil {
				t.Fatal(err)
			}
			a.capture()
			a.dirtySince = time.Now().Add(-time.Second)
			a.backgroundTick()
			if !a.saveFailed || !a.dirty {
				t.Fatal("failed autosave did not preserve dirty state")
			}
			tt := ui.NewTester(a.view, 1000, 720)
			click(t, tt, "Dismiss")
			for range 3 {
				a.lastDiskCheck = time.Time{}
				a.backgroundTick()
				tt.Frame()
				if a.problem != "" {
					t.Fatal("autosave reopened a dismissed error")
				}
			}
			// Navigation exposes the retry/close actions without discarding edits.
			a.home("All notes")
			tt.Frame()
			if a.current != n {
				t.Fatal("navigation discarded the dirty document")
			}
			click(t, tt, "Close document")
			if a.current != nil || a.dirty || a.saveFailed || a.problem != "" || !a.recoveryOpen {
				t.Fatal("close did not preserve edits and return to the library", a.problem)
			}
			if a.lib.Get(n.ID).Source != "Original content" {
				t.Fatal("closed entry treats the unsaved buffer as saved")
			}
			click(t, tt, "Recover as a copy")
			if a.current == nil || a.current.ID == n.ID || a.current.External != "" || a.current.Source != "Pending Original content" || a.recoveryOpen {
				t.Fatal("recovery did not open an independent copy with all pending edits")
			}
			other, err := library.Open(a.lib.Dir)
			if err != nil || other.Get(a.current.ID).Source != "Pending Original content" {
				t.Fatal("recovered content was not durable", err)
			}
		})
	}
}

func TestCloseFailedDocumentRefusesToLoseRecovery(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(t.TempDir(), "Original.md")
	writeTest(t, path, "Original")
	a.openExternal(path)
	n := a.current
	a.doc.InsertText("Pending ")
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	if a.save() {
		t.Fatal("missing original unexpectedly saved")
	}
	if err := os.Rename(filepath.Join(a.lib.Dir, "recovery"), filepath.Join(a.lib.Dir, "recovery-blocked")); err != nil {
		t.Fatal(err)
	}
	writeTest(t, filepath.Join(a.lib.Dir, "recovery"), "not a directory")
	tt := ui.NewTester(a.view, 1000, 720)
	click(t, tt, "Close document")
	if a.current != n || !a.dirty || a.problem == "" || n.Source != "Pending Original" || !a.saveFailed {
		t.Fatal("recovery failure discarded unsaved edits")
	}
	if tt.HasText("Original file is missing") || !tt.HasText("Something needs your attention") || !strings.Contains(a.problem, "recovery") {
		t.Fatal("missing-file explanation hid the failed recovery write")
	}
}

func TestConflictingSaveCanCloseWithoutOverwritingOtherWriter(t *testing.T) {
	a := testApp(t)
	a.createNote("Original", "Draft")
	n := a.current
	other, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Get(n.ID).Source = "Other writer"
	if err := other.Save(other.Get(n.ID)); err != nil {
		t.Fatal(err)
	}
	a.doc.InsertText("Pending ")
	if a.save() {
		t.Fatal("conflicting save succeeded")
	}
	tt := ui.NewTester(a.view, 1000, 720)
	click(t, tt, "Close document")
	if a.current != nil || a.problem != "" || a.lib.Get(n.ID).Source != "Other writer" || !a.recoveryOpen {
		t.Fatal("close changed the competing version or hid recovery")
	}
	click(t, tt, "Recover as a copy")
	if a.current == nil || a.current.ID == n.ID || a.current.Source != "Pending Original" || a.lib.Get(n.ID).Source != "Other writer" {
		t.Fatal("recovery did not preserve both versions")
	}
}

func TestRetryFailedSaveResumesAutosave(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(t.TempDir(), "Original.md")
	writeTest(t, path, "Original")
	a.openExternal(path)
	a.doc.InsertText("Pending ")
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	if a.save() {
		t.Fatal("save to missing original succeeded")
	}
	if err := os.Rename(path+".moved", path); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1000, 720)
	click(t, tt, "Retry")
	if a.problem != "" || a.saveFailed || a.dirty || a.missingExternal != "" {
		t.Fatal("successful retry did not resume normal saving", a.problem)
	}
	a.doc.InsertText("More ")
	a.capture()
	a.dirtySince = time.Now().Add(-time.Second)
	a.backgroundTick()
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "More") || a.dirty {
		t.Fatal("autosave did not resume", string(data), err)
	}
}
