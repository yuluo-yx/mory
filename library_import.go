package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuluo-yx/mory/internal/library"
)

type importMetadata struct {
	Name     string                         `json:"name"`
	Folder   string                         `json:"folder"`
	Parent   string                         `json:"parent"`
	Created  int64                          `json:"created"`
	Modified int64                          `json:"modified"`
	Opened   int64                          `json:"opened"`
	Order    int64                          `json:"order"`
	Pinned   bool                           `json:"pinned"`
	Starred  int64                          `json:"starred"`
	Markdown bool                           `json:"markdown"`
	Width    string                         `json:"width"`
	Info     struct{ Hidden, Created bool } `json:"info"`
}

// Import an indexed DocxXML folder in two passes so document references and
// parent relationships point at the newly imported notes, never old IDs.
func (a *app) importIndexedLibrary(root string) {
	b, err := os.ReadFile(filepath.Join(root, "index.json"))
	if a.fail(err) {
		return
	}
	var index map[string]importMetadata
	if a.fail(json.Unmarshal(b, &index)) {
		return
	}
	files, err := os.ReadDir(filepath.Join(root, "docs"))
	if a.fail(err) {
		return
	}
	mapping := map[string]string{}
	var imported []*library.Note
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if ext != ".xml" && ext != ".md" && ext != ".markdown" {
			continue
		}
		old := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		a.importFileFrom(filepath.Join(root, "docs", file.Name()), root)
		if a.problem != "" {
			return
		}
		n := a.current
		meta := index[old]
		mapping[old] = n.ID
		if meta.Name != "" {
			n.Title = meta.Name
		}
		n.Folder, n.Pinned, n.Starred, n.Order = meta.Folder, meta.Pinned, meta.Starred > 0, meta.Order
		if meta.Created > 0 {
			n.Created = time.UnixMilli(meta.Created)
		}
		if meta.Modified > 0 {
			n.Updated = time.UnixMilli(meta.Modified)
		}
		if meta.Opened > 0 {
			n.Opened = time.UnixMilli(meta.Opened)
		}
		n.Width = map[string]string{"standard": "Standard", "wide": "Wide", "full": "Full"}[meta.Width]
		n.HideInfo, n.ShowCreated = meta.Info.Hidden, meta.Info.Created
		n.Parent = meta.Parent
		if a.fail(a.lib.Save(n)) {
			return
		}
		imported = append(imported, n)
	}
	for _, n := range imported {
		n.Parent = mapping[n.Parent]
		for old, newID := range mapping {
			for _, attribute := range []string{"doc-id", "document-id"} {
				n.Source = strings.ReplaceAll(n.Source, attribute+`="`+old+`"`, attribute+`="`+newID+`"`)
			}
		}
		if a.fail(a.lib.Save(n)) {
			return
		}
	}
	if a.current != nil {
		a.loadEditors()
	}
	a.status = fmt.Sprintf("Imported %d documents and their metadata", len(imported))
}
