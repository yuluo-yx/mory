package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

const pendingMarker = "Edited 世界 🧪 "

func pendingBlockEdit(t *testing.T) (*app, string) {
	t.Helper()
	a := testApp(t)
	// Cross the real document-size threshold without constructing thousands
	// of unnecessary nodes in every persistence scenario.
	paragraph := strings.Repeat("Paragraph text. ", 40) + "**bold**.\n\n"
	source := "Start\n\n" + strings.Repeat(paragraph, largeDocumentBytes/len(paragraph)+1)
	a.createNote(source, "Large block edit")
	f := a.blockField(a.blockEditor.doc.Root.Children[0], ui.Hex("#303030"))
	f.doc.SetSelection(0, 0)
	f.doc.InsertText(pendingMarker)
	a.capture()
	if !a.blockPending || !a.dirty || a.current.Source != source {
		t.Fatal("typing serialized the full document or failed to mark it dirty")
	}
	return a, source
}

func TestPendingBlocksAtSourceBoundaries(t *testing.T) {
	for _, action := range []string{"save", "Source", "Split", "Read", "home", "background", "unfold"} {
		t.Run(action, func(t *testing.T) {
			a, source := pendingBlockEdit(t)
			note := a.current
			switch action {
			case "save":
				if !a.save() {
					t.Fatal(a.problem)
				}
			case "home":
				a.home("All notes")
			case "background":
				a.dirtySince = time.Now().Add(-time.Second)
				a.backgroundTick()
			case "unfold":
				a.blockEditor.restoreRich = true
				a.restoreUnfoldedEditor()
				if a.blockEditor != nil || !strings.HasPrefix(a.doc.Text().String(), pendingMarker+"Start") {
					t.Fatal("restoring the rich editor lost deferred edits")
				}
			default:
				a.changeMode(action)
			}
			// The edited paragraph may normalize its separator; untouched blocks
			// must retain every source byte, not just the visible prefix.
			if a.blockPending || !strings.HasPrefix(note.Source, pendingMarker+"Start\n\n") || !strings.HasSuffix(note.Source, strings.TrimPrefix(source, "Start\n\n")) {
				t.Fatal("boundary lost pending text or changed untouched blocks")
			}
			if action == "Source" || action == "Split" {
				if a.code.Buffer.String() != note.Source {
					t.Fatal("source editor is stale")
				}
			}
			if action == "save" || action == "home" || action == "background" {
				stored, err := library.Open(a.lib.Dir)
				if err != nil {
					t.Fatal(err)
				}
				if stored.Get(note.ID).Source != note.Source {
					t.Fatal("persisted document is stale")
				}
			}
		})
	}
}

func TestPendingBlockUndoRedo(t *testing.T) {
	a, source := pendingBlockEdit(t)
	a.blockUndo(false)
	a.flushSource()
	if a.current.Source != source {
		t.Fatal("undo changed original bytes")
	}
	a.blockUndo(true)
	if !a.blockPending {
		t.Fatal("redo did not mark the block source pending")
	}
	if !a.save() || !strings.HasPrefix(a.current.Source, pendingMarker+"Start\n\n") || !strings.HasSuffix(a.current.Source, strings.TrimPrefix(source, "Start\n\n")) {
		t.Fatal("redo was not persisted")
	}
}

func TestPendingBlockConflictRecovery(t *testing.T) {
	a, _ := pendingBlockEdit(t)
	other, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	remote := other.Get(a.current.ID)
	remote.Source = "External change"
	if err := other.Save(remote); err != nil {
		t.Fatal(err)
	}
	a.lastDiskCheck = time.Time{}
	a.refreshDisk()
	if !a.dirty || a.blockPending {
		t.Fatal("conflict did not flush and retain the local edit")
	}
	revisions, err := a.lib.Recoveries()
	if err != nil || len(revisions) == 0 {
		t.Fatal("missing recovery", err)
	}
	found := false
	for _, revision := range revisions {
		if strings.HasPrefix(revision.Note.Source, pendingMarker+"Start") {
			found = true
		}
	}
	if !found {
		t.Fatal("recovery omitted deferred edits")
	}
	if a.save() {
		t.Fatal("conflicting save overwrote external data")
	}
}

func TestPendingBlockFailedSaveKeepsSource(t *testing.T) {
	a, _ := pendingBlockEdit(t)
	// A missing external target forces the real save failure/recovery path.
	a.current.External = t.TempDir() + "/missing.md"
	if a.save() || !a.dirty || !strings.HasPrefix(a.current.Source, pendingMarker) {
		t.Fatal("failed save lost pending content")
	}
	if _, err := os.Stat(a.current.External); err == nil {
		t.Fatal("missing target unexpectedly created")
	}
}
