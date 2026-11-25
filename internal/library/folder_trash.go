package library

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// FolderName is the leaf label shown in the sidebar; storage retains the path.
func FolderName(folder string) string {
	return folder[strings.LastIndex(folder, "/")+1:]
}

// FolderContains matches a workspace folder and its slash-separated subfolders.
// The empty library root is never a deletable folder.
func FolderContains(folder, candidate string) bool {
	return folder != "" && (candidate == folder || strings.HasPrefix(candidate, folder+"/"))
}

// TrashFolder removes a folder subtree and trashes its live document trees.
// A dedicated trash record preserves empty folders and groups every document,
// including external registrations. External original files are never changed.
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
	var removals []*Note
	for _, n := range l.Notes {
		if affected[n.ID] && !n.Deleted {
			removals = append(removals, n)
		}
	}
	// Preflight the entire folder before moving anything, including registrations.
	for _, n := range removals {
		if err := l.checkTrashMove(n, true); err != nil {
			return err
		}
	}
	folders := []string{folder}
	for _, name := range l.Folders() {
		if name != folder && FolderContains(folder, name) {
			folders = append(folders, name)
		}
	}
	if len(removals) == 0 && len(folders) == 1 && !slices.Contains(l.Folders(), folder) {
		return nil
	}
	now := time.Now()
	root := &Note{
		ID: strings.ToLower(rand.Text()), Title: FolderName(folder), Folder: folder,
		Format: "markdown", FolderTrash: true, TrashFolders: folders,
		Deleted: true, DeletedAt: now, Created: now, Updated: now,
	}
	root.TrashRoot = root.ID
	data, err := marshalNote(root)
	if err != nil {
		return err
	}
	// Persist the group first so an interrupted move always has a visible entry.
	if err := checkVersion(l.notePath(root), [32]byte{}); err != nil {
		return err
	}
	if err := AtomicWrite(l.notePath(root), data); err != nil {
		return err
	}
	l.Notes = append(l.Notes, root)
	l.versions[root.ID] = sha256.Sum256(data)
	for _, n := range removals {
		next := *n
		next.TrashRoot = root.ID
		if err := l.setDeleted(n, next, true, now); err != nil {
			return err
		}
	}
	previous := l.Settings.Folders
	l.Settings.Folders = slices.DeleteFunc(slices.Clone(previous), func(candidate string) bool {
		return FolderContains(folder, candidate)
	})
	if err := l.saveSettings(); err != nil {
		l.Settings.Folders = previous
		return err
	}
	return nil
}

// restoreFolderNames runs under the same lock as RestoreTree. Folder names are
// restored before their documents; retries merge safely with an existing folder.
func (l *Library) restoreFolderNames(n *Note) error {
	previous := l.Settings.Folders
	folders := slices.Clone(previous)
	for _, folder := range append([]string{n.Folder}, n.TrashFolders...) {
		if folder != "" && !slices.Contains(folders, folder) {
			folders = append(folders, folder)
		}
	}
	l.Settings.Folders = folders
	if err := l.saveSettings(); err != nil {
		l.Settings.Folders = previous
		return err
	}
	return nil
}
