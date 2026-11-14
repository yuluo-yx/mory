package main

import (
	"time"

	"github.com/yuluo-yx/mory/internal/library"
)

// Route every document-actions entry point through the current library state.
// Deleted records never expose metadata editing, even through a stale card.
func (a *app) openNoteActions(n *library.Note) {
	if n == nil {
		return
	}
	n = a.lib.Get(n.ID)
	if n == nil {
		return
	}
	a.target = n
	a.renameOpen, a.deleteOpen = !n.Deleted, n.Deleted
	if !a.batchMove {
		a.moveOpen = false
	}
	if !n.Deleted {
		a.renameTitle = n.Title
	}
}

// A refresh can replace Note pointers while a dialog is open. Reconcile the
// target before building controls so a deleted record cannot retain live actions.
func (a *app) syncNoteActionState() {
	if a.target == nil {
		return
	}
	n := a.lib.Get(a.target.ID)
	if n == nil {
		a.target = nil
		a.renameOpen, a.deleteOpen = false, false
		if !a.batchMove {
			a.moveOpen = false
		}
		return
	}
	a.target = n
	if n.Deleted && (a.renameOpen || a.moveOpen && !a.batchMove) {
		a.renameOpen, a.moveOpen, a.deleteOpen = false, false, true
	}
	if !n.Deleted {
		a.deleteOpen = false
	}
}

func deleteLabel(n *library.Note) string {
	if n != nil && n.External != "" {
		return "Remove from list"
	}
	return "Move to Trash"
}

func (a *app) restoreNote(n *library.Note) bool {
	moved, err := a.lib.RestoreTree(n)
	if a.fail(err) {
		return false
	}
	a.status = "Restored"
	if moved {
		a.status = "Restored to My documents because the original location no longer exists"
	}
	return true
}

// Retention describes the whole deletion group shown by each Trash card.
func (a *app) noteSummary(n *library.Note) string {
	if !n.Deleted {
		return a.dateText(n.Updated, false) + "  ·  " + formatLabel(n)
	}
	left := time.Until(n.DeletedAt.Add(library.TrashRetention))
	if left < 24*time.Hour {
		return a.tr("Will be permanently deleted soon")
	}
	days := int((left + 24*time.Hour - 1) / (24 * time.Hour))
	return a.trf("%d days remaining", days)
}
