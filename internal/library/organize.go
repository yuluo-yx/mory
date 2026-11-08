package library

import (
	"fmt"
	"slices"
	"strings"
)

// Descendants is cycle safe even for a manually edited library index.
func (l *Library) Descendants(id string) []*Note {
	var out []*Note
	seen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		if n := l.Get(id); n != nil {
			out = append(out, n)
		}
		for _, n := range l.Notes {
			if n.Parent == id {
				visit(n.ID)
			}
		}
	}
	visit(id)
	return out
}

func (l *Library) Move(n *Note, folder, parent string) error {
	for _, child := range l.Descendants(n.ID) {
		if child.ID == parent {
			return fmt.Errorf("a note cannot contain itself")
		}
	}
	if parent != "" && (l.Get(parent) == nil || l.Get(parent).Deleted) {
		return fmt.Errorf("parent no longer exists")
	}
	oldFolder, oldParent := n.Folder, n.Parent
	n.Folder, n.Parent = strings.TrimSpace(folder), parent
	if err := l.Save(n); err != nil {
		n.Folder, n.Parent = oldFolder, oldParent
		return err
	}
	return nil
}

// TrashTree records the deletion group, so restoring a parent does not restore
// descendants that were already independently in the trash.
func (l *Library) TrashTree(n *Note, deleted bool) error {
	notes := l.Descendants(n.ID)
	if !deleted {
		notes = []*Note{n}
		for _, child := range l.Notes {
			if child.ID != n.ID && child.TrashRoot == n.ID {
				notes = append(notes, child)
			}
		}
	}
	for _, item := range notes {
		if item.Deleted == deleted {
			continue
		}
		old := item.TrashRoot
		if deleted {
			item.TrashRoot = n.ID
		} else {
			item.TrashRoot = ""
		}
		if err := l.SetDeleted(item, deleted); err != nil {
			item.TrashRoot = old
			return err
		}
	}
	return nil
}

func (l *Library) EmptyTrash() error {
	for _, n := range slices.Clone(l.Notes) {
		if n.Deleted {
			if err := l.Purge(n); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *Library) RenameFolder(from, to string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("folder name is empty")
	}
	if from != to && slices.Contains(l.Folders(), to) {
		return fmt.Errorf("folder already exists")
	}
	for _, n := range l.Notes {
		if n.Folder == from && from != "" {
			if err := l.Move(n, to, n.Parent); err != nil {
				return err
			}
		}
	}
	l.Settings.Folders = slices.DeleteFunc(l.Settings.Folders, func(s string) bool { return s == from })
	if !slices.Contains(l.Settings.Folders, to) {
		l.Settings.Folders = append(l.Settings.Folders, to)
	}
	return l.SaveSettings()
}

func (l *Library) DeleteFolder(folder string) error {
	for _, n := range l.Notes {
		if n.Folder == folder {
			if err := l.Move(n, "", n.Parent); err != nil {
				return err
			}
		}
	}
	l.Settings.Folders = slices.DeleteFunc(l.Settings.Folders, func(s string) bool { return s == folder })
	return l.SaveSettings()
}

func (l *Library) Reorder(ids []string) error {
	for i, id := range ids {
		if n := l.Get(id); n != nil {
			n.Order = int64(i + 1)
			if err := l.Save(n); err != nil {
				return err
			}
		}
	}
	return nil
}
