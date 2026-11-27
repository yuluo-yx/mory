package main

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/yuluo-yx/mory/internal/library"
)

// Library folders are logical groups, not physical storage directories. Show
// their hierarchy in the header; external documents retain their actual path.
// Resolving this label never saves a draft or renames a file.
func (a *app) documentLocation(n *library.Note) string {
	if n.External != "" {
		return filepath.ToSlash(n.External)
	}
	name := n.FileName
	if name == "" {
		name = a.displayTitle(n.Title) + ".md"
		if n.Format == "xml" {
			name = a.displayTitle(n.Title) + ".xml"
		}
	}
	ancestors := []string{}
	root := n
	seen := map[string]bool{n.ID: true}
	for parent := a.lib.Get(n.Parent); parent != nil && !parent.Deleted && !seen[parent.ID]; parent = a.lib.Get(parent.Parent) {
		seen[parent.ID] = true
		ancestors = append(ancestors, a.displayTitle(parent.Title))
		root = parent
	}
	slices.Reverse(ancestors)
	parts := []string{}
	if folder := strings.Trim(root.Folder, "/"); folder != "" {
		parts = append(parts, folder)
	}
	parts = append(parts, ancestors...)
	parts = append(parts, name)
	return "/" + strings.Join(parts, "/")
}
