package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/library"
)

func pasteMenu(t *testing.T, a *app) *mygo.MenuItem {
	t.Helper()
	for _, menu := range a.buildMenu(&mygo.MenuItem{Label: "Updates"}).Items() {
		for _, item := range menu.Submenu {
			if item.Label == a.tr("Paste") {
				if item.Click == nil || item.Role != "" {
					t.Fatal("native paste bypasses attachment handling")
				}
				return item
			}
		}
	}
	t.Fatal("paste menu is missing")
	return nil
}

func TestImagePasteEntrypoints(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, entry := range []string{"shortcut", "menu"} {
			for _, kind := range []string{"screenshot", "image-and-html", "files"} {
				t.Run(entry+"/"+kind+"/"+map[bool]string{false: "rich", true: "blocks"}[structured], func(t *testing.T) {
					a := testApp(t)
					a.createNote("Before after", "Image paste")
					if structured {
						a.enableBlocks()
					}
					tt := ui.NewTester(a.view, 1280, 900)
					label := "Document editor"
					if structured {
						label = "Block text " + a.blockEditor.active
					}
					click(t, tt, label)
					a.doc.SetSelection(7, 7)
					tt.Frame()
					png := pngBytes(t)
					data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, png)))
					wantImages := 1
					if kind == "image-and-html" {
						data = transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, png), transfer.Bytes(transfer.HTML, []byte(`<img src="https://invalid.example/image.png">`))))
					} else if kind == "files" {
						first, second := filepath.Join(t.TempDir(), "First image.png"), filepath.Join(t.TempDir(), "Second image.png")
						for _, path := range []string{first, second} {
							if err := os.WriteFile(path, png, 0o600); err != nil {
								t.Fatal(err)
							}
						}
						files, err := transfer.FileData(first, second)
						if err != nil {
							t.Fatal(err)
						}
						// Finder can also offer a thumbnail: preserve both original files.
						data = transfer.New(append(files.Items(), transfer.NewItem(transfer.Bytes(transfer.PNG, png)))...)
						wantImages = 2
					}
					tt.SetClipboardData(data)
					if entry == "shortcut" {
						tt.Key(ui.Cmd, ui.KeyV)
					} else {
						pasteMenu(t, a).Click(nil, nil)
						tt.Frame()
					}
					if a.blockEditor == nil {
						t.Fatal("paste did not insert an image")
					}
					images := 0
					for _, n := range a.blockEditor.doc.Root.Children {
						if n.Kind != "img" {
							continue
						}
						images++
						stored, err := os.ReadFile(filepath.Join(a.assetRoot(), n.Attr("href")))
						if err != nil || !bytes.Equal(stored, png) {
							t.Fatal("image was not saved as a local asset", err)
						}
					}
					if images != wantImages || !strings.HasPrefix(a.current.Source, "Before\n") || !strings.Contains(a.current.Source, "after") || strings.Contains(a.current.Source, "invalid.example") {
						t.Fatalf("paste changed content or lost images: %d, %s", images, a.current.Source)
					}
					if !a.save() {
						t.Fatal(a.problem)
					}
					stored, err := library.Open(a.lib.Dir)
					if err != nil || stored.Get(a.current.ID).Source != a.current.Source {
						t.Fatal("pasted image was lost after reopening", err)
					}
				})
			}
		}
	}
}

func TestNativePasteKeepsTextBehavior(t *testing.T) {
	a := testApp(t)
	a.createNote("Before", "Text paste")
	tt := ui.NewTester(a.view, 1280, 900)
	previous := nativePaste
	fallbacks := 0
	nativePaste = func() { fallbacks++; tt.Command("paste"); tt.Frame() }
	t.Cleanup(func() { nativePaste = previous })
	click(t, tt, "Document editor")
	a.doc.SetSelection(0, a.doc.Text().Len())
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte("<b>Rich text</b>")))))
	pasteMenu(t, a).Click(nil, nil)
	if fallbacks != 1 || a.doc.Formatting().Bold != richtext.FormatOn || !strings.Contains(a.current.Source, "**Rich text**") {
		t.Fatal("native rich-text paste lost formatting", a.current.Source)
	}
	tt.Command("undo")
	tt.Frame()
	if a.doc.Text().String() != "Before" {
		t.Fatal("native paste did not keep undo")
	}
	click(t, tt, "Document title")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.SetClipboard("Pasted title")
	pasteMenu(t, a).Click(nil, nil)
	if a.current.Title != "Pasted title" || fallbacks != 2 {
		t.Fatal("title paste was routed into the body")
	}
	for _, mode := range []string{"Source", "Read"} {
		a.changeMode(mode)
		tt.Frame()
		tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, pngBytes(t)))))
		before := a.current.Source
		pasteMenu(t, a).Click(nil, nil)
		if a.current.Source != before {
			t.Fatal("image paste modified a non-writing view")
		}
	}
}

func TestNativePasteScopeAndFileFailure(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 900)
	previous := nativePaste
	fallbacks := 0
	nativePaste = func() { fallbacks++ }
	t.Cleanup(func() { nativePaste = previous })
	pasteMenu(t, a).Click(nil, nil)
	if fallbacks != 1 {
		t.Fatal("library home did not retain native paste")
	}
	a.createNote("```go\ncode\n```", "Code paste")
	tt.Frame()
	click(t, tt, "Code editor")
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, pngBytes(t)))))
	pasteMenu(t, a).Click(nil, nil)
	if fallbacks != 2 || firstBlock(a, "img") != nil {
		t.Fatal("image paste replaced code editing")
	}
	a.createNote("Body", "Missing image")
	tt.Frame()
	click(t, tt, "Document editor")
	data, err := transfer.FileData(filepath.Join(t.TempDir(), "missing.png"))
	if err != nil {
		t.Fatal(err)
	}
	tt.SetClipboardData(data)
	a.lib.Settings.Keys["Paste"] = "Cmd+Shift+V"
	menu := pasteMenu(t, a)
	if menu.Accelerator != "Cmd+Shift+V" {
		t.Fatal("paste menu ignored the configured shortcut")
	}
	menu.Click(nil, nil)
	if a.problem == "" || fallbacks != 2 || a.current.Source != "Body" {
		t.Fatal("failed attachment paste was silent or changed the document")
	}
}
