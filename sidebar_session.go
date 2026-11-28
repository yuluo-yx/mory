package main

import (
	"strings"
	"time"

	"github.com/yuluo-yx/mory/internal/library"
)

// Rebuild startup navigation from persisted document activity, not every folder
// expanded in the previous session. Opening the library is read-only here.
func (a *app) restoreSidebarFolders() {
	for _, folder := range a.lib.Folders() {
		a.collapsedFolders[folder] = true
	}
	var latest *library.Note
	for _, note := range a.lib.Notes {
		if note.Deleted || note.Opened.IsZero() {
			continue
		}
		activity := maxNoteActivity(note)
		if latest == nil || activity.After(maxNoteActivity(latest)) || activity.Equal(maxNoteActivity(latest)) && note.ID < latest.ID {
			latest = note
		}
	}
	a.revealNoteInSidebar(latest)
}

// A document can be edited in an older window after another note was opened.
// Updated counts only for notes with Opened set, so imports alone do not expand
// folders on the next launch.
func maxNoteActivity(note *library.Note) time.Time {
	if note.Updated.After(note.Opened) {
		return note.Updated
	}
	return note.Opened
}

// Reveal the actual parent chain and folder prefixes, without closing folders
// the user expanded manually in this session. A sibling prefix is not an ancestor.
func (a *app) revealNoteInSidebar(note *library.Note) {
	if note == nil || note.Deleted {
		return
	}
	root := note
	seen := map[string]bool{note.ID: true}
	for parent := a.lib.Get(note.Parent); parent != nil && !parent.Deleted && !seen[parent.ID]; parent = a.lib.Get(parent.Parent) {
		seen[parent.ID] = true
		a.collapsedNotes[parent.ID] = false
		root = parent
	}
	for _, folder := range a.lib.Folders() {
		if folder == root.Folder || strings.HasPrefix(root.Folder, folder+"/") {
			a.collapsedFolders[folder] = false
		}
	}
}
