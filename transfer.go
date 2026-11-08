package main

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
	"github.com/yuluo-yx/mory/internal/library"
)

// Dialog functions are replaceable in the e2e build so fixture selection does
// not depend on the user's files or macOS accessibility permissions.
var openDialog = mygo.Dialog.Open
var saveDialog = mygo.Dialog.Save

// Native dialogs run outside the draw callback. All resulting state changes are
// marshalled back through Window.Update, keeping the UI and its library confined.
func (a *app) chooseImport() {
	if a.win == nil {
		return
	}
	go func() {
		paths, err := openDialog(mygo.OpenDialogOptions{Parent: a.win, Title: "Import documents", Multiple: true, Filters: []mygo.FileFilter{{Name: "Documents", Extensions: []string{"md", "markdown", "xml"}}}})
		a.win.Update(func() {
			if a.fail(err) {
				return
			}
			for _, path := range paths {
				a.importFile(path)
			}
		})
	}()
}

func (a *app) chooseExport(format string) {
	if a.current == nil || a.win == nil {
		return
	}
	a.capture()
	a.flushSource()
	note := *a.current
	root := a.assetRoot()
	doc, err := blocks.Parse(note.Source, note.Format)
	if a.fail(err) {
		return
	}
	md := note.Source
	if note.Format == "xml" {
		md = doc.Root.Markdown()
	}
	ext := map[string]string{"Markdown": "md", "DocxXML": "xml", "HTML": "html", "PDF": "pdf"}[format]
	go func() {
		path, err := saveDialog(mygo.SaveDialogOptions{Parent: a.win, Title: "Export " + format, DefaultPath: document.Filename(note.Title) + "." + ext, Filters: []mygo.FileFilter{{Name: format, Extensions: []string{ext}}}})
		if err != nil || path == "" {
			a.win.Update(func() { a.fail(err) })
			return
		}
		if filepath.Ext(path) == "" {
			path += "." + ext
		}
		if format == "PDF" {
			page, err := renderExport(&note, root)
			a.win.Update(func() {
				if !a.fail(err) {
					a.exportPDF(path, page)
				}
			})
			return
		}
		source := md
		switch format {
		case "DocxXML":
			if note.Format == "xml" {
				source = note.Source
			} else {
				source = blocks.New("title", blocks.Text(note.Title)).XML() + "\n" + doc.Root.XML()
			}
		case "HTML":
			source, err = renderExport(&note, root)
		}
		if format == "HTML" {
			// renderExport already embeds the local images.
		} else {
			assetFormat := "markdown"
			if format == "DocxXML" {
				assetFormat = "xml"
			}
			source, err = document.CopyAssets(source, assetFormat, root, filepath.Dir(path))
		}
		if err == nil {
			err = library.AtomicWrite(path, []byte(source))
		}
		a.win.Update(func() {
			if !a.fail(err) {
				a.status = "Exported " + filepath.Base(path)
			}
		})
	}()
}

func (a *app) reveal(path string) {
	if a.win == nil {
		return
	}
	go func() { err := mygo.Shell.OpenPath(path); a.win.Update(func() { a.fail(err) }) }()
}

func (a *app) openLink(raw string) {
	if strings.HasPrefix(raw, "mory-inline:") && a.blockEditor != nil {
		if n := a.blockEditor.doc.Find(strings.TrimPrefix(raw, "mory-inline:")); n != nil {
			if n.Kind == "latex" {
				a.blockEditor.formulaID = n.ID
				a.blockEditor.formulaText = n.Plain()
				a.blockEditor.formulaOpen = true
			} else {
				a.editBlockProperties(n)
			}
		}
		return
	}
	if strings.HasPrefix(raw, "mory:") {
		if n := a.lib.Get(strings.TrimPrefix(raw, "mory:")); n != nil {
			a.openNote(n)
		}
		return
	}
	if strings.HasPrefix(raw, "#") {
		a.preview.Anchor = raw
		a.preview.AnchorVersion++
		return
	}
	if a.win == nil {
		return
	}
	if ui.SafeToOpen(raw) {
		go func() { err := mygo.Shell.OpenExternal(raw); a.win.Update(func() { a.fail(err) }) }()
		return
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return
	}
	root, err := os.OpenRoot(a.assetRoot())
	if a.fail(err) {
		return
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(u.Path))
	if a.fail(err) {
		return
	}
	f.Close()
	a.reveal(filepath.Join(a.assetRoot(), filepath.FromSlash(u.Path)))
}

func (a *app) chooseAttachment() {
	if a.win == nil || a.current == nil {
		return
	}
	id := a.current.ID
	go func() {
		paths, err := openDialog(mygo.OpenDialogOptions{Parent: a.win, Title: "Insert image or attachment"})
		if err != nil || len(paths) == 0 {
			a.win.Update(func() { a.fail(err) })
			return
		}
		path := paths[0]
		info, err := os.Stat(path)
		if err == nil && info.Size() > 32<<20 {
			err = fmt.Errorf("attachment exceeds 32 MiB")
		}
		if err != nil {
			a.win.Update(func() { a.fail(err) })
			return
		}
		b, err := os.ReadFile(path)
		if err != nil {
			a.win.Update(func() { a.fail(err) })
			return
		}
		name := strings.ToLower(rand.Text()) + strings.ToLower(filepath.Ext(path))
		dest := filepath.Join(a.lib.Dir, "assets", name)
		err = library.AtomicWrite(dest, b)
		a.win.Update(func() {
			if a.fail(err) {
				return
			}
			if a.current == nil || a.current.ID != id {
				a.status = "Attachment copied to library assets"
				return
			}
			prefix := ""
			switch strings.ToLower(filepath.Ext(path)) {
			case ".png", ".jpg", ".jpeg", ".gif", ".webp":
				prefix = "!"
			}
			label := strings.NewReplacer("[", "\\[", "]", "\\]").Replace(filepath.Base(path))
			a.insert("\n" + prefix + "[" + label + "](assets/" + name + ")\n")
		})
	}()
}

// Only PDF export uses a temporary system WebKit window. The editing surface,
// library, previews, menus and plugin host stay native. CSP disables active content.
func (a *app) exportPDF(path, html string) {
	w := mygo.NewWindow(mygo.WindowOptions{Title: "Mory PDF export", Hidden: true, Width: 850, Height: 1100})
	ready := make(chan error, 1)
	w.Page().OnDidFinishLoad(func() {
		select {
		case ready <- nil:
		default:
		}
	})
	w.Page().OnDidFailLoad(func(e *mygo.LoadError) {
		select {
		case ready <- fmt.Errorf("PDF document failed to load: %v", e):
		default:
		}
	})
	w.Page().LoadHTML(html, "")
	go func() {
		var err error
		select {
		case err = <-ready:
		case <-time.After(20 * time.Second):
			err = fmt.Errorf("PDF rendering timed out")
		}
		if err == nil {
			var b []byte
			b, err = w.Page().PrintToPDF(mygo.PDFOptions{PageSize: mygo.PageA4, Background: true})
			if err == nil {
				err = library.AtomicWrite(path, b)
			}
		}
		w.Destroy()
		a.win.Update(func() {
			if !a.fail(err) {
				a.status = "Exported " + filepath.Base(path)
			}
		})
	}()
}
