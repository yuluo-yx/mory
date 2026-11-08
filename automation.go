//go:build e2e

package main

import (
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
	"github.com/yuluo-yx/mory/internal/document"
	"os"
	"path/filepath"
)

func init() {
	if dir := os.Getenv("MORY_E2E_FIXTURES"); dir != "" {
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
		if path := os.Getenv("MORY_E2E_PDF"); path != "" {
			a.exportPDF(path, document.HTML("Native PDF 世界", "# Export verification\n\nHello 世界.\n\n| A | B |\n| --- | --- |\n| one | two |\n"))
		}
	}
}
