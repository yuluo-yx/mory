package library

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// FolderContains matches a workspace folder and its slash-separated subfolders.
// The empty library root is never a deletable folder.
func FolderContains(folder, candidate string) bool {
	return folder != "" && (candidate == folder || strings.HasPrefix(candidate, folder+"/"))
}

// TrashFolder removes a folder subtree and trashes its live document trees.
// Existing trash groups stay intact; external registrations are only forgotten.
func (l *Library) TrashFolder(folder string) error {
	if folder == "" {
		return fmt.Errorf("folder name is empty")
	}
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := checkVersion(filepath.Join(l.Dir, "settings.json"), l.settingsVersion); err != nil {
		return err
	}

	affected := map[string]bool{}
	children := map[string][]*Note{}
	for _, n := range l.Notes {
		children[n.Parent] = append(children[n.Parent], n)
	}
	var include func(*Note)
	include = func(n *Note) {
		if affected[n.ID] {
			return
		}
		affected[n.ID] = true
		for _, child := range children[n.ID] {
			include(child)
		}
	}
	for _, n := range l.Notes {
		if !n.Deleted && FolderContains(folder, n.Folder) {
			include(n)
		}
	}
	type removal struct {
		note *Note
		root string
	}
	var removals []removal
	seen := map[string]bool{}
	var visit func(*Note, string)
	visit = func(n *Note, root string) {
		if seen[n.ID] || !affected[n.ID] {
			return
		}
		seen[n.ID] = true
		if !n.Deleted {
			if root == "" && n.External == "" {
				root = n.ID
			}
			removals = append(removals, removal{n, root})
		}
		for _, child := range children[n.ID] {
			visit(child, root)
		}
	}
	for _, n := range l.Notes {
		if !affected[n.Parent] {
			visit(n, "")
		}
	}
	// A manually edited parent cycle still gets one visible, recoverable group.
	for _, n := range l.Notes {
		visit(n, "")
	}
	// Preflight the entire folder before moving anything, including registrations.
	for _, item := range removals {
		if item.note.External != "" {
			err = checkVersion(l.notePath(item.note), l.versions[item.note.ID])
		} else {
			err = l.checkTrashMove(item.note, true)
		}
		if err != nil {
			return err
		}
	}
	now := time.Now()
	for _, item := range removals {
		if item.note.External != "" {
			err = l.forget(item.note)
		} else {
			next := *item.note
			next.TrashRoot = item.root
			err = l.setDeleted(item.note, next, true, now)
		}
		if err != nil {
			return err
		}
	}
	folders := l.Settings.Folders
	l.Settings.Folders = slices.DeleteFunc(slices.Clone(folders), func(candidate string) bool {
		return FolderContains(folder, candidate)
	})
	if err := l.saveSettings(); err != nil {
		l.Settings.Folders = folders
		return err
	}
	return nil
}
