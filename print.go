package main

import (
	"github.com/egoist/mygo"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
	"github.com/yuluo-yx/mory/internal/library"
	"github.com/yuluo-yx/mory/internal/mathrender"
)

var printPage = func(w *mygo.Window) { w.Page().Print() }

func renderExport(note *library.Note, root string) (string, error) {
	d, err := blocks.Parse(note.Source, note.Format)
	if err != nil {
		return "", err
	}
	page, err := d.HTML(blocks.ExportOptions{Title: note.Title, Icon: note.Icon, Cover: note.Cover, Math: func(source string, display bool) (string, error) { return mathrender.HTML(source, display, "mathml") }})
	if err != nil {
		return "", err
	}
	return document.EmbedImages(page, root)
}

func (a *app) printDocument() {
	if a.current == nil || a.win == nil {
		return
	}
	a.capture()
	a.flushSource()
	page, err := renderExport(a.current, a.assetRoot())
	if a.fail(err) {
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{Title: "Print — " + a.current.Title, Width: 850, Height: 1000})
	w.Page().OnDidFinishLoad(func() { printPage(w) })
	w.Page().LoadHTML(page, "")
}
