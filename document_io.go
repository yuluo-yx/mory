package main

import (
	"errors"
	"io/fs"
	"os"
	"time"

	"github.com/yuluo-yx/mory/internal/library"
)

// Only a missing external original gets the cached-document recovery actions.
// A missing library record or attachment is a different error.
func (a *app) noteMissingExternal(n *library.Note, err error) bool {
	var pathError *os.PathError
	if n == nil || n.External == "" || !errors.Is(err, fs.ErrNotExist) || !errors.As(err, &pathError) || pathError.Path != n.External {
		return false
	}
	a.missingExternal = n.ID
	return true
}

// Explicit retries resume I/O; background frames never reopen a dismissed
// failure dialog. Capture first so retry cannot reload over pending input.
func (a *app) retryDocumentIO() {
	a.capture()
	a.flushSource()
	a.problem = ""
	if a.current == nil {
		return
	}
	if a.dirty {
		a.save()
		return
	}
	a.saveFailed, a.missingExternal = false, ""
	a.lastDiskCheck = time.Time{}
	a.refreshDisk()
}

// Closing is an explicit escape from failed saves. Do not discard the buffer
// unless both its recovery snapshot and the recovery list were read successfully.
func (a *app) closeFailedDocument() {
	a.capture()
	a.flushSource()
	var recoveries []library.Revision
	if a.current != nil && a.dirty {
		if a.fail(a.lib.Recover(a.current)) {
			a.saveFailed, a.missingExternal = true, ""
			return
		}
		var err error
		recoveries, err = a.lib.Recoveries()
		if a.fail(err) {
			a.saveFailed, a.missingExternal = true, ""
			return
		}
	}
	if a.current != nil {
		// Recover() saved the edited buffer separately. Restore the registered
		// snapshot so reopening this entry never treats unsaved text as saved.
		fresh, err := library.Open(a.lib.Dir)
		if a.fail(err) {
			a.saveFailed, a.missingExternal = true, ""
			return
		}
		a.lib = fresh
	}
	if len(recoveries) > 0 {
		a.recoveries, a.recoveryOpen = recoveries, true
	}
	a.current, a.dirty = nil, false
	a.saveFailed, a.missingExternal, a.problem = false, "", ""
	a.renameOpen, a.moveOpen, a.deleteOpen, a.folderDeleteOpen = false, false, false, false
	a.target = nil
	a.home("All notes")
	if a.recoveryOpen {
		a.status = "Edits saved in recovery"
	}
}
