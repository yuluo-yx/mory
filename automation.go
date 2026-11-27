//go:build e2e

package main

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
	"github.com/yuluo-yx/mory/internal/document"
)

// Native tests must neither inherit nor overwrite the user's window geometry.
type e2eWindowStore struct{ states map[string]mygo.WindowState }

func (s *e2eWindowStore) LoadWindowStates() (map[string]mygo.WindowState, error) {
	return s.states, nil
}

func (s *e2eWindowStore) SaveWindowStates(states map[string]mygo.WindowState) error {
	s.states = states
	return nil
}

func init() {
	mygo.App.SetWindowStateStore(&e2eWindowStore{})
	if dir := os.Getenv("MORY_E2E_FIXTURES"); dir != "" {
		// Capture the exact shell boundary while leaving the user's Finder alone.
		showItemInFolder = func(path string) {
			if err := os.WriteFile(filepath.Join(dir, "finder-selection.txt"), []byte(path), 0o600); err != nil {
				panic(err)
			}
		}
		openFolderPath = func(path string) error {
			return os.WriteFile(filepath.Join(dir, "finder-folder.txt"), []byte(path), 0o600)
		}
		openExternal = func(destination string) error {
			file, err := os.OpenFile(filepath.Join(dir, "opened-links.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			_, err = file.WriteString(destination + "\n")
			closeErr := file.Close()
			if err != nil {
				return err
			}
			return closeErr
		}
		// Exercise the native print renderer without leaving an interactive
		// printer-selection sheet open on the CI desktop.
		printPage = func(w *mygo.Window) {
			go func() {
				b, err := w.Page().PrintToPDF(mygo.PDFOptions{PageSize: mygo.PageA4, Background: true})
				if err == nil {
					_ = os.WriteFile(filepath.Join(dir, "print.pdf"), b, 0o644)
				}
				w.Destroy()
			}()
		}
		openDialog = func(options mygo.OpenDialogOptions) ([]string, error) {
			if options.Title == "Insert image or attachment" || options.Title == "Choose document cover" {
				return []string{filepath.Join(dir, "image.png")}, nil
			}
			if options.Title == "Import document theme" {
				return []string{filepath.Join(dir, "Personal.css")}, nil
			}
			if options.Title == "Open local documents" && options.Directory {
				return []string{filepath.Join(dir, "collection")}, nil
			}
			return []string{filepath.Join(dir, "Fixture import.md")}, nil
		}
		saveDialog = func(options mygo.SaveDialogOptions) (string, error) {
			return filepath.Join(dir, options.DefaultPath), nil
		}
	}
	nativeReadyHook = func(a *app) {
		if os.Getenv("MORY_E2E_FIXTURES") != "" {
			// Native clipboard tests replace the system pasteboard temporarily.
			if data, err := mygo.Clipboard.Read(); err == nil {
				mygo.App.OnWillQuit(func(*mygo.QuitEvent) { _ = mygo.Clipboard.Write(data) })
			}
		}
		width, _ := strconv.Atoi(os.Getenv("MORY_E2E_WIDTH"))
		height, _ := strconv.Atoi(os.Getenv("MORY_E2E_HEIGHT"))
		if width > 0 && height > 0 {
			a.win.SetContentSize(width, height)
		}
		if path := os.Getenv("MORY_E2E_PDF"); path != "" {
			a.exportPDF(path, document.HTML("Native PDF 世界", "# Export verification\n\nHello 世界.\n\n| A | B |\n| --- | --- |\n| one | two |\n"))
		}
	}
}
