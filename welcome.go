package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/yuluo-yx/mory/internal/library"
)

//go:embed resources/welcome.md
var welcome string

//go:embed resources/welcome.zh-CN.md
var welcomeChinese string

const welcomeImageURL = "https://raw.githubusercontent.com/yuluo-yx/mory/main/resources/icon.png"

// welcomeNote reuses an untouched current guide. Edited or older guides remain
// user documents; requesting the latest guide never overwrites their contents.
func (a *app) welcomeNote() (*library.Note, error) {
	return a.welcomeNoteFor("Welcome to Mory", welcome)
}

func (a *app) chineseWelcomeNote() (*library.Note, error) {
	return a.welcomeNoteFor("欢迎使用 Mory", welcomeChinese)
}

func (a *app) ensureWelcomeNotes() error {
	if err := a.ensureWelcomeAsset(); err != nil {
		return err
	}
	if len(a.lib.Notes) != 0 {
		return nil
	}
	if _, err := a.welcomeNote(); err != nil {
		return err
	}
	_, err := a.chineseWelcomeNote()
	return err
}

func (a *app) welcomeNoteFor(title, source string) (*library.Note, error) {
	if err := a.ensureWelcomeAsset(); err != nil {
		return nil, err
	}
	for _, n := range a.lib.Notes {
		if !n.Deleted && n.External == "" && n.Source == source {
			return n, nil
		}
	}
	return a.lib.Create(title, source, "markdown")
}

func (a *app) ensureWelcomeAsset() error {
	imagePath := filepath.Join(a.lib.Dir, "assets", "mory-welcome.png")
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		if err := library.AtomicWrite(imagePath, iconPNG); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
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
