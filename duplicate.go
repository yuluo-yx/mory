package main

import (
	"path/filepath"

	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
	"github.com/yuluo-yx/mory/internal/library"
)

// A copied external document becomes a library note. Its relative resources
// must move with the content instead of pointing into a different asset root.
func (a *app) duplicateWithAssets(note *library.Note) (*library.Note, error) {
	copy := *note
	if note.External != "" {
		root := filepath.Dir(note.External)
		source, err := document.CopyAssets(note.Source, note.Format, root, a.lib.Dir)
		if err != nil {
			return nil, err
		}
		copy.Source = source
		if note.Cover != "" {
			image := blocks.New("img")
			image.Set("href", note.Cover)
			markup, err := document.CopyAssets(image.XML(), "xml", root, a.lib.Dir)
			if err != nil {
				return nil, err
			}
			parsed, err := blocks.ParseXML(markup)
			if err != nil {
				return nil, err
			}
			copy.Cover = parsed.Elements()[0].Attr("href")
		}
	}
	return a.lib.Duplicate(&copy)
}
