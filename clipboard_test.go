package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/document"
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
					if kind == "files" {
						a.blockUndo(false)
						if strings.Contains(a.current.Source, "![") {
							t.Fatal("one undo did not remove the complete file paste", a.current.Source)
						}
						a.blockUndo(true)
						if strings.Count(a.current.Source, "![") != wantImages {
							t.Fatal("redo lost pasted files")
						}
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

// A valid ancillary PNG chunk exercises the compressed-file limit without
// requiring a huge decoded bitmap in every regression run.
func largeAttachmentPNG(t *testing.T) []byte {
	png := pngBytes(t)
	chunk := append([]byte("tEXtComment\x00"), bytes.Repeat([]byte("x"), 33<<20)...)
	var out bytes.Buffer
	out.Write(png[:len(png)-12])
	binary.Write(&out, binary.BigEndian, uint32(len(chunk)-4))
	out.Write(chunk)
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	out.Write(png[len(png)-12:])
	return out.Bytes()
}

func TestLargeImageAttachmentEntrypoints(t *testing.T) {
	data := largeAttachmentPNG(t)
	if path := os.Getenv("MORY_TEST_ATTACHMENT"); path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := decodeImage(data, "image.png", "image/png"); err != nil {
		t.Fatal("image fixture does not decode", err)
	}
	path := filepath.Join(t.TempDir(), "Large #100% image.png")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"file", "clipboard-file", "clipboard-image", "cover"} {
		t.Run(entry, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Existing text", "Large image")
			switch entry {
			case "file":
				a.attachFile(path)
			case "clipboard-file":
				files, err := transfer.FileData(path)
				if err != nil {
					t.Fatal(err)
				}
				a.pasteObjects(files)
			case "clipboard-image":
				a.pasteObjects(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, data))))
			case "cover":
				a.setCoverAsset(a.current.ID, data, filepath.Base(path))
			}
			if a.problem != "" || !a.save() {
				t.Fatal("large valid image was rejected", a.problem)
			}
			asset := a.current.Cover
			if entry != "cover" {
				n := firstBlock(a, "img")
				if n == nil {
					t.Fatal("image block was not inserted")
				}
				asset = n.Attr("href")
				a.blockUndo(false)
				if a.current.Source != "Existing text" {
					t.Fatal("undo damaged existing text", a.current.Source)
				}
				a.blockUndo(true)
				a.save()
			}
			stored, err := os.ReadFile(filepath.Join(a.lib.Dir, asset))
			if err != nil || !bytes.Equal(stored, data) {
				t.Fatal("image bytes were changed", err)
			}
			opened, err := library.Open(a.lib.Dir)
			if err != nil || opened.Get(a.current.ID).Source != a.current.Source || opened.Get(a.current.ID).Cover != a.current.Cover {
				t.Fatal("image did not survive reopening", err)
			}
		})
	}
}

func TestAttachmentErrorPresentation(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, failure := range []*document.AssetError{
			{Name: "large.png", Reason: "too-large", Size: 129 << 20},
			{Name: "folder", Reason: "not-file"},
			{Name: "missing.png", Reason: "read", Err: os.ErrNotExist},
			{Name: "private.png", Reason: "read", Err: os.ErrPermission},
			{Name: "offline.png", Reason: "read", Err: fmt.Errorf("offline")},
			{Name: "full.png", Reason: "write", Err: syscall.ENOSPC},
			{Name: "readonly.png", Reason: "write", Err: os.ErrPermission},
		} {
			a := testApp(t)
			a.lib.Settings.Language = language
			a.createNote("Keep these edits", "Attachment error")
			a.markDirty()
			a.fail(fmt.Errorf("resource import: %w", failure))
			// Wrapped backend errors still retain the typed attachment reason.
			if a.attachmentProblem != failure {
				t.Fatal("attachment classification was lost")
			}
			tt := ui.NewTester(a.view, 1280, 900)
			if !tt.HasText(a.tr("Could not add attachment")) || tt.HasText(a.tr("Your edits are still in memory. Export a copy before closing.")) || a.status == "Changes not saved" {
				t.Fatal("attachment failure was reported as lost document edits")
			}
			message := a.attachmentProblemText()
			if !strings.Contains(message, failure.Name) || strings.Contains(message, "%!") || language == "zh-CN" && strings.Contains(message, "Could not") {
				t.Fatal("attachment diagnostic is not localized", message)
			}
			if failure.Reason == "too-large" && (!strings.Contains(message, "129.0") || !strings.Contains(message, "128")) {
				t.Fatal("size diagnostic omitted actual size or limit", message)
			}
			click(t, tt, a.tr("Dismiss"))
			if a.problem != "" || a.current.Source != "Keep these edits" || !a.save() {
				t.Fatal("dismissal did not leave the document usable")
			}
			a.problem = "Other failure"
			if a.attachmentProblemText() != "" {
				t.Fatal("stale attachment error affected an unrelated failure")
			}
		}
	}
}
