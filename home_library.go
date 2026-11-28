package main

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/yuluo-yx/mory/internal/library"
)

type homeFolder struct {
	path    string
	count   int
	preview *library.Note
	updated time.Time
}

// Folder names represent logical paths. Only existing groups form navigation
// levels, so imported paths do not invent empty intermediate folders.
func parentHomeFolder(folder string, folders []string) string {
	parent := ""
	for _, candidate := range folders {
		if len(candidate) > len(parent) && strings.HasPrefix(folder, candidate+"/") {
			parent = candidate
		}
	}
	return parent
}

func (a *app) homeContents() ([]homeFolder, []*library.Note) {
	notes := a.lib.Query(a.section, a.query, a.folder, a.order)
	// Search, favorites, recents and trash retain their flat result semantics.
	if a.query != "" || a.section != "" && a.section != "All notes" {
		return nil, notes
	}
	var visible []*library.Note
	for _, n := range notes {
		parent := a.lib.Get(n.Parent)
		if n.Folder == a.folder && (parent == nil || parent.Deleted) {
			visible = append(visible, n)
		}
	}
	paths := a.lib.Folders()
	var folders []homeFolder
	for _, path := range paths {
		if parentHomeFolder(path, paths) != a.folder {
			continue
		}
		item := homeFolder{path: path}
		for _, n := range a.lib.Notes {
			if n.Deleted || n.Folder != path && !strings.HasPrefix(n.Folder, path+"/") {
				continue
			}
			item.count++
			if item.preview == nil || n.Updated.After(item.updated) || n.Updated.Equal(item.updated) && n.ID < item.preview.ID {
				item.preview, item.updated = n, n.Updated
			}
		}
		folders = append(folders, item)
	}
	sort.SliceStable(folders, func(i, j int) bool {
		if a.order == "Modified" && !folders[i].updated.Equal(folders[j].updated) {
			return folders[i].updated.After(folders[j].updated)
		}
		return strings.ToLower(folders[i].path) < strings.ToLower(folders[j].path)
	})
	return folders, visible
}

func (a *app) openHomeFolder(folder string) {
	if !a.save() {
		return
	}
	a.home("All notes")
	a.endSelection()
	a.folder = folder
	if folder != "" {
		a.collapsedFolders[folder] = false
	}
}

// Keep paper cards close to a readable portrait size instead of stretching four
// columns across every window. The same geometry is used by cards and previews.
func homeGridGeometry(available float32) (columns int, height float32) {
	columns = max(1, int(math.Round(float64((available+24)/264))))
	width := max(160, (available-float32(columns-1)*24)/float32(columns))
	return columns, max(290, min(370, width*1.32))
}
