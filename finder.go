package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/yuluo-yx/mory/internal/library"
)

// Keep Finder requests injectable so tests never open the user's desktop.
var showItemInFolder = mygo.Shell.ShowItemInFolder
var openFolderPath = mygo.Shell.OpenPath

func (a *app) copyDocumentPath(n *library.Note) {
	path, err := a.lib.DocumentPath(n.ID)
	if !a.fail(err) {
		a.services.WriteClipboard(path)
	}
}

func (a *app) showDocumentInFinder(n *library.Note) {
	path, err := a.lib.DocumentPath(n.ID)
	if a.fail(err) || a.fail(checkFinderPath(path, false)) {
		return
	}
	// Opening a file with OpenPath would launch its default editor, possibly
	// Mory itself. Finder selection uses the dedicated native shell operation.
	showItemInFolder(path)
}

func (a *app) openStorageInFinder() {
	// Sidebar folders are metadata, including empty and imported folders. They
	// have no matching on-disk directory; the menu names the storage location.
	path, err := filepath.Abs(filepath.Join(a.lib.Dir, "notes"))
	if a.fail(err) || a.fail(checkFinderPath(path, true)) {
		return
	}
	a.fail(openFolderPath(path))
}

func checkFinderPath(path string, directory bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if directory && !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	if !directory && !info.Mode().IsRegular() {
		return fmt.Errorf("not a document file: %s", path)
	}
	return nil
}
