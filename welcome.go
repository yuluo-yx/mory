package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/yuluo-yx/mory/internal/library"
)

//go:embed resources/welcome.md
var welcome string

// welcomeNote reuses an untouched current guide. Edited or older guides remain
// user documents; requesting the latest guide never overwrites their contents.
func (a *app) welcomeNote() (*library.Note, error) {
	imagePath := filepath.Join(a.lib.Dir, "assets", "mory-welcome.png")
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		if err := library.AtomicWrite(imagePath, iconPNG); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	for _, n := range a.lib.Notes {
		if !n.Deleted && n.External == "" && n.Source == welcome {
			return n, nil
		}
	}
	return a.lib.Create("Welcome to Mory", welcome, "markdown")
}

func (a *app) openWelcome() {
	if !a.save() {
		return
	}
	n, err := a.welcomeNote()
	if !a.fail(err) {
		a.openNote(n)
	}
}
