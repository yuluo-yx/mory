package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/doctheme"
	"github.com/yuluo-yx/mory/internal/document"
	"github.com/yuluo-yx/mory/internal/library"
	"github.com/yuluo-yx/mory/internal/plugins"
)

type app struct {
	associationStatus                                                               string
	searchFocus                                                                     bool
	settingsTab                                                                     string
	shortcutDraft                                                                   map[string]string
	quickDraft                                                                      string
	selected                                                                        map[string]bool
	collapsedFolders                                                                map[string]bool
	collapsedNotes                                                                  map[string]bool
	selecting, batchMove, folderOpen, emptyTrashOpen, historyOpen, recoveryOpen     bool
	folderFrom, folderName                                                          string
	revisions                                                                       []library.Revision
	recoveries                                                                      []library.Revision
	lastDiskCheck                                                                   time.Time
	quick                                                                           bool
	assetBitmaps                                                                    map[string]*ui.Bitmap
	docTheme                                                                        *doctheme.Theme
	themeName                                                                       string
	themeStamp                                                                      time.Time
	themeChecked                                                                    time.Time
	cssOpen                                                                         bool
	cssEditor                                                                       *stylesheetEditor
	cssText, cssName                                                                string
	services                                                                        ui.Services
	textFocused                                                                     bool
	reminderSources                                                                 map[string]string
	reminders                                                                       map[string][]reminder
	notified                                                                        map[string]bool
	filePreviews                                                                    map[string]*ui.Bitmap
	lib                                                                             *library.Library
	win                                                                             *mygo.Window
	current                                                                         *library.Note
	blockEditor                                                                     *blockSession
	documentColor                                                                   ui.Color
	doc                                                                             *richtext.Document
	code                                                                            *editor.State
	richVersion, codeVersion                                                        uint64
	mode, section, query, folder, order                                             string
	dirty                                                                           bool
	dirtySince                                                                      time.Time
	status, problem                                                                 string
	outline, focus                                                                  bool
	preview                                                                         markdown.Options
	previewText                                                                     string
	previewSource                                                                   string
	richSource                                                                      string
	richPending, codePending                                                        bool
	statsSource                                                                     string
	words, characters                                                               int
	headingSource                                                                   string
	headings                                                                        []document.Heading
	structuredHeadingVersion                                                        uint64
	structuredHeadings                                                              []structuredHeading
	slashPending                                                                    bool
	slashOffset                                                                     int
	plugins                                                                         []*plugins.Plugin
	showToolbar                                                                     bool
	settingsOpen, pluginsOpen, insertOpen, linkOpen, helpOpen, findOpen, exportOpen bool
	linkURL, findText, replaceText                                                  string
	renameOpen, moveOpen, deleteOpen, iconPickerOpen                                bool
	renameTitle, moveFolder, moveParent                                             string
	target                                                                          *library.Note
	insertQuery                                                                     string
}

type structuredHeading struct {
	id    string
	title string
	level int
}

const (
	lazyRichDocumentBytes = 256 << 10
	largeDocumentBytes    = 512 << 10
)

func largeDocumentBlocks(n *library.Note) bool {
	if n == nil || len(n.Source) <= largeDocumentBytes {
		return false
	}
	// The library may normalize imported Markdown to a format that looks rich
	// safe even when its content contains thousands of layout-heavy rows. The
	// byte threshold is therefore the reliable guard for the first frame.
	return true
}

func (a *app) ensureRichDocument() {
	if a.current == nil {
		return
	}
	if a.doc != nil && a.richSource == a.current.Source {
		return
	}
	a.doc = richtext.NewDocument(a.richText(a.current.Source))
	a.richPending = false
	a.richSource = a.current.Source
	a.richVersion = a.doc.Version()
	if a.current.Format == "markdown" {
		a.doc.SetSelection(0, 0)
	}
}

func newApp(dir string) (*app, error) {
	l, err := library.Open(dir)
	if err != nil {
		return nil, err
	}
	a := &app{lib: l, section: "All notes", order: "Modified", mode: "Write", status: "All changes saved", outline: true, showToolbar: true, selected: map[string]bool{}, collapsedFolders: map[string]bool{}, collapsedNotes: map[string]bool{}}
	a.recoveries, err = l.Recoveries()
	if err != nil {
		return nil, err
	}
	a.recoveryOpen = len(a.recoveries) > 0
	a.reloadPlugins()
	return a, nil
}

func (a *app) reloadPlugins() {
	ps, errs := plugins.Load(filepath.Join(a.lib.Dir, "plugins"))
	a.plugins = ps
	if len(errs) > 0 {
		a.problem = fmt.Sprint(errs)
	}
}

func (a *app) fail(err error) bool {
	if err != nil {
		a.problem = err.Error()
		a.status = "Changes not saved"
		return true
	}
	return false
}

func (a *app) markDirty() {
	if !a.dirty {
		a.dirtySince = time.Now()
	}
	a.dirty = true
	a.status = "Saving…"
}

// capture observes document versions rather than exporting the whole rich text
// on every frame. Opening or viewing a note never normalizes its source.
func (a *app) capture() {
	if a.current == nil {
		return
	}
	if a.structured() {
		a.captureBlocks()
		return
	}
	if a.mode == "Write" && a.doc != nil && a.doc.Version() != a.richVersion {
		a.richVersion = a.doc.Version()
		// Large documents serialize at the save boundary, not on the typing frame.
		if a.doc.Text().Len() > 200000 {
			a.richPending = true
			a.markDirty()
			return
		}
		a.current.Source = a.doc.Text().Markdown()
		a.richSource = a.current.Source
		a.markDirty()
		anchor, caret := a.doc.Selection()
		if anchor == caret && caret > 0 {
			runes := []rune(a.doc.Text().String())
			if caret <= len(runes) && runes[caret-1] == '/' && (caret == 1 || runes[caret-2] == '\n') {
				a.slashPending = true
				a.slashOffset = caret - 1
				a.insertOpen = true
				a.insertQuery = ""
			}
		}
	} else if (a.mode == "Source" || a.mode == "Split") && a.code != nil && a.code.Buffer.Version() != a.codeVersion {
		a.codeVersion = a.code.Buffer.Version()
		if a.mode == "Source" && a.code.Buffer.Snapshot().Len() > 200000 {
			a.codePending = true
			a.markDirty()
			return
		}
		a.current.Source = a.code.Buffer.String()
		a.markDirty()
	}
}

func (a *app) save() bool {
	a.capture()
	a.flushSource()
	if a.current == nil || !a.dirty {
		return true
	}
	a.current.Updated = time.Now()
	if a.fail(a.lib.Save(a.current)) {
		if err := a.lib.Recover(a.current); err != nil {
			a.problem += "; recovery: " + err.Error()
		}
		return false
	}
	a.dirty = false
	a.status = "All changes saved"
	return true
}

func (a *app) openNote(n *library.Note) {
	if n == nil {
		return
	}
	if !a.save() {
		return
	}
	a.current = n
	a.settingsOpen, a.helpOpen = false, false
	if a.lib.Changed(n) {
		if a.fail(a.lib.Reload(n.ID)) {
			return
		}
	}
	a.readMetadata()
	n.Opened = time.Now()
	a.mode = "Write"
	a.loadEditors()
	a.fail(a.lib.Save(n))
	a.previewSource = "\x00"
}

func (a *app) loadEditors() {
	if a.current == nil {
		return
	}
	n := a.current
	a.blockEditor = nil
	a.structuredHeadingVersion = 0
	a.structuredHeadings = nil
	largeDocument := largeDocumentBlocks(n)
	if largeDocument || n.Format == "xml" || !document.RichSafe(n.Source) {
		// Large documents use the block editor from the first frame. This keeps
		// the normal Write experience while avoiding a second whole-document
		// rich-text tree and allows large tables to use their compact renderer.
		session, err := newBlockSession(n.Source, n.Format)
		if !a.fail(err) {
			a.blockEditor = session
		}
	}
	a.richPending = false
	a.codePending = false
	if a.blockEditor != nil && (largeDocument || len(n.Source) > lazyRichDocumentBytes) {
		// Large structured documents are edited through block fields. Building a
		// second rich-text tree here duplicates the whole document before the
		// first frame and delays the window for no user-visible benefit.
		a.doc = nil
		a.richSource = ""
		a.richVersion = 0
	} else {
		a.ensureRichDocument()
	}
	b := editor.NewBuffer(n.Source)
	name := "note.md"
	if n.Format == "xml" {
		name = "note.xml"
	}
	b.SetLanguage(editor.LanguageFor(name))
	a.code = &editor.State{Buffer: b, SoftWrap: true}
	a.codeVersion = b.Version()
	a.preview.LoadImage = markdown.FileImages(a.assetRoot())
	a.preview.OnLink = a.openLink
	a.preview.Extensions = parser.Default | parser.ExtDollarMath

}

func (a *app) changeMode(mode string) {
	if a.current == nil || mode == a.mode {
		return
	}
	a.capture()
	a.flushSource()
	if mode == "Write" || mode == "Read" {
		if largeDocumentBlocks(a.current) || a.current.Format == "xml" || !document.RichSafe(a.current.Source) {
			if a.blockEditor == nil || a.blockEditor.doc.Source() != a.current.Source {
				session, err := newBlockSession(a.current.Source, a.current.Format)
				if a.fail(err) {
					return
				}
				a.blockEditor = session
				a.structuredHeadingVersion = 0
				a.structuredHeadings = nil
			}
		} else {
			a.blockEditor = nil
		}
		if a.blockEditor == nil {
			a.ensureRichDocument()
		}
	} else if mode == "Source" || mode == "Split" {
		if a.code.Buffer.String() != a.current.Source {
			a.code.Buffer.Set(a.current.Source)
		}
		a.codeVersion = a.code.Buffer.Version()
		if mode == "Split" && a.blockEditor != nil {
			// Re-entering split view follows the source, not a stale read-mode position.
			a.blockEditor.split = splitScrollState{}
		}
	}
	a.mode = mode
}

func (a *app) markdownSource() string {
	if a.current == nil {
		return ""
	}
	source := a.current.Source
	if a.current.Format == "markdown" {
		return source
	}
	if a.previewSource != source {
		_, md, err := document.FromXML(source)
		a.previewSource = source
		if err != nil {
			a.previewText = "XML preview unavailable: " + err.Error()
		} else {
			a.previewText = md
		}
	}
	return a.previewText
}

func (a *app) createNote(source, title string) {
	if !a.save() {
		return
	}
	n, err := a.lib.Create(title, source, "markdown")
	if a.fail(err) {
		return
	}
	n.Folder = a.folder
	a.openNote(n)
}

func (a *app) createChildNote(parent *library.Note) {
	if !a.save() {
		return
	}
	n, err := a.lib.Create("Untitled", "", "markdown")
	if a.fail(err) {
		return
	}
	n.Folder = parent.Folder
	n.Parent = parent.ID
	a.openNote(n)
}

func (a *app) home(section string) {
	if !a.save() {
		return
	}
	a.current = nil
	a.settingsOpen, a.helpOpen = false, false
	a.section = section
	a.folder = ""
	a.query = ""
}

func (a *app) importFile(path string) {
	a.importFileFrom(path, filepath.Dir(path))
}

func (a *app) importFileFrom(path, root string) {
	if !a.save() {
		return
	}
	info, err := os.Stat(path)
	if a.fail(err) {
		return
	}
	if info.Size() > 32<<20 {
		a.fail(fmt.Errorf("document exceeds 32 MiB: %s", path))
		return
	}
	b, err := os.ReadFile(path)
	if a.fail(err) {
		return
	}
	format := "markdown"
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if strings.EqualFold(filepath.Ext(path), ".xml") {
		format = "xml"
	}
	if a.fail(document.Validate(string(b), format)) {
		return
	}
	if format == "xml" {
		if t, _, _ := document.FromXML(string(b)); t != "" {
			title = t
		}
	}
	source, err := document.RewriteAssets(string(b), format, func(asset string) (string, error) {
		absolute := filepath.Join(filepath.Dir(path), filepath.FromSlash(asset))
		if filepath.IsAbs(asset) {
			absolute = asset
		}
		rel, err := filepath.Rel(root, absolute)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("attachment is outside the imported folder: %s", asset)
		}
		return filepath.ToSlash(rel), nil
	})
	if a.fail(err) {
		return
	}
	source, err = document.CopyAssets(source, format, root, a.lib.Dir)
	if a.fail(err) {
		return
	}
	n, err := a.lib.Create(title, source, format)
	if a.fail(err) {
		return
	}
	a.openNote(n)
	a.status = "Imported a copy · original file unchanged"
}

func (a *app) duplicate(n *library.Note) {
	if !a.save() {
		return
	}
	copy, err := a.duplicateWithAssets(n)
	if !a.fail(err) {
		a.openNote(copy)
	}
}

func (a *app) trash(n *library.Note) {
	if !a.save() {
		return
	}
	if a.fail(a.lib.TrashTree(n, true)) {
		return
	}
	if a.current != nil && a.current.Deleted {
		a.current = nil
	}
	a.status = "Moved to Trash"
}

func (a *app) runPlugin(p *plugins.Plugin) {
	if a.current == nil {
		return
	}
	a.capture()
	a.flushSource()
	if a.current.Format != "markdown" {
		a.problem = "Plugins transform Markdown documents. Export this XML document as Markdown and import that copy first."
		return
	}
	out, err := p.Run(context.Background(), plugins.Input{Title: a.current.Title, Markdown: a.current.Source})
	if a.fail(err) {
		return
	}
	if out.Markdown != nil && *out.Markdown != a.current.Source {
		if a.mode == "Write" && document.RichSafe(*out.Markdown) {
			length := a.doc.Text().Len()
			a.doc.Edit(func(e *richtext.Edit) { e.Replace(0, length, richtext.ParseMarkdown(*out.Markdown)) })
			a.capture()
		} else {
			a.changeMode("Source")
			a.code.Buffer.Set(*out.Markdown)
			a.capture()
		}
		a.save()
	}
	if out.Message != "" {
		a.status = out.Message
	}
}

func (a *app) mutate(n *library.Note, fn func()) {
	if !a.save() {
		return
	}
	fn()
	a.fail(a.lib.Save(n))
}

func (a *app) insert(markdownText string) {
	if a.current == nil {
		return
	}
	if a.blockEditor != nil || !document.RichSafe(markdownText) {
		root, err := blocks.ParseMarkdown(markdownText)
		if a.fail(err) {
			return
		}
		if a.slashPending && a.blockEditor != nil {
			if field := a.blockEditor.fields[a.blockEditor.active]; field != nil && field.doc != nil {
				field.doc.SetSelection(0, field.doc.Text().Len())
				field.doc.InsertText("")
				a.captureBlocks()
			}
			a.slashPending = false
		}
		a.insertNodes(root.Children...)
		return
	}
	if a.slashPending && a.mode == "Write" {
		a.doc.SetSelection(a.slashOffset, a.slashOffset+1)
		a.doc.InsertText("")
		a.slashPending = false
		a.capture()
	}
	if a.mode == "Write" && document.RichSafe(markdownText) {
		a.doc.Insert(a.richText(markdownText))
	} else {
		a.changeMode("Source")
		a.code.Insert(markdownText)
	}
	a.capture()
	a.insertOpen = false
}

// Local image attachments are decoded once when a rich document is loaded.
func (a *app) richText(source string) richtext.Text {
	text := richtext.ParseMarkdown(source)
	if !strings.Contains(source, "![") {
		return text
	}
	root, err := os.OpenRoot(a.assetRoot())
	if err != nil {
		return text
	}
	defer root.Close()
	var paragraphs []richtext.Paragraph
	for _, p := range text.Paragraphs() {
		for i := range p.Runs {
			attr := &p.Runs[i].Attributes
			if attr.Attachment == nil || attr.Attachment.URL == "" {
				continue
			}
			b, err := root.ReadFile(attr.Attachment.URL)
			if err != nil {
				continue
			}
			bitmap, err := ui.DecodeBitmap(b)
			if err != nil {
				continue
			}
			copy := *attr.Attachment
			copy.Image = bitmap
			copy.Width = 480
			attr.Attachment = &copy
		}
		paragraphs = append(paragraphs, p)
	}
	return richtext.TextOf(paragraphs...)
}

// flushSource commits pending editor snapshots before any operation that consumes
// source bytes. The editor keeps the authoritative text while the debounce runs.
func (a *app) flushSource() {
	if a.current == nil {
		return
	}
	if a.richPending {
		a.current.Source = a.doc.Text().Markdown()
		a.richSource = a.current.Source
		a.richPending = false
	}
	if a.codePending {
		a.current.Source = a.code.Buffer.String()
		a.codePending = false
	}
}
