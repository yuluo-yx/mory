package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestExternalAttachmentsSurviveLibraryCopy(t *testing.T) {
	a := testApp(t)
	folder := t.TempDir()
	path := filepath.Join(folder, "External.md")
	writeTest(t, path, "External document")
	a.openExternal(path)
	a.insertAsset(pngBytes(t), "Picture.png")
	image := firstBlock(a, "img")
	if image == nil {
		t.Fatal("external image insertion")
	}
	asset := image.Attr("href")
	if _, err := os.Stat(filepath.Join(a.assetRoot(), asset)); err != nil {
		t.Fatal("asset not stored beside external file", err)
	}
	a.current.Cover = asset
	a.markDirty()
	if !a.save() {
		t.Fatal(a.problem)
	}
	original := a.current
	copy, err := a.duplicateWithAssets(original)
	if err != nil {
		t.Fatal(err)
	}
	if copy.External != "" {
		t.Fatal("copy still edits the original")
	}
	a.openNote(copy)
	page, err := renderExport(copy, a.assetRoot())
	if err != nil || !strings.Contains(page, "data:image/png") {
		t.Fatal("copied document has broken images", err)
	}
	if a.loadAssetBitmap(copy.Cover) == nil {
		t.Fatal("copied cover is missing")
	}
	d, err := blocks.Parse(copy.Source, copy.Format)
	if err != nil || !strings.Contains(d.Root.XML(), "assets/") {
		t.Fatal("asset references not portable", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("original removed")
	}
	original.Source = "![Missing](missing.png)"
	if _, err := a.duplicateWithAssets(original); err == nil {
		t.Fatal("missing asset silently ignored")
	}
	original.Source = "Text"
	original.Cover = "missing.png"
	if _, err := a.duplicateWithAssets(original); err == nil {
		t.Fatal("missing cover silently ignored")
	}
}
