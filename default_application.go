package main

import "github.com/yuluo-yx/mory/internal/fileassoc"

var setMarkdownDefault = fileassoc.SetMarkdownDefault

func (a *app) makeMarkdownDefault() {
	a.associationStatus = ""
	if !a.fail(setMarkdownDefault()) {
		a.associationStatus = a.tr("Mory is now the default Markdown application.")
	}
}
