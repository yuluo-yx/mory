package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
)

// File-level title attributes travel with DocxXML exports. Reading them does
// not normalize the original XML; only an explicit metadata edit rewrites it.
func (a *app) readMetadata() {
	n := a.current
	if n == nil || n.Format != "xml" {
		return
	}
	d, err := blocks.Parse(n.Source, "xml")
	if err != nil {
		return
	}
	for _, b := range d.Root.Children {
		if b.Kind == "title" {
			n.Title = b.Plain()
			n.Icon = b.Attr("icon")
			n.Cover = b.Attr("cover")
			x, _ := strconv.ParseFloat(b.Attr("cover-x"), 32)
			y, _ := strconv.ParseFloat(b.Attr("cover-y"), 32)
			n.CoverX, n.CoverY = float32(x), float32(y)
			return
		}
	}
}

func (a *app) syncMetadata() {
	n := a.current
	if n == nil {
		return
	}
	if n.Format == "xml" && a.blockEditor != nil {
		d := a.blockEditor.doc
		set := func(b *blocks.Node) {
			if b.Plain() != n.Title {
				b.Children = []*blocks.Node{blocks.Text(n.Title)}
			}
			b.Set("icon", n.Icon)
			b.Set("cover", n.Cover)
			b.Set("cover-x", fmt.Sprint(n.CoverX))
			b.Set("cover-y", fmt.Sprint(n.CoverY))
		}
		var title *blocks.Node
		for _, b := range d.Root.Children {
			if b.Kind == "title" {
				title = b
				break
			}
		}
		if title == nil {
			title = blocks.New("title")
			set(title)
			a.fail(d.Splice(d.Root.ID, 0, 0, title))
		} else {
			a.fail(d.Transform(title.ID, set))
		}
		a.captureBlocks()
	}
	a.markDirty()
}

func (a *app) documentHeader(c *ui.Context) {
	n := a.current
	if n.Cover != "" {
		bitmap := a.loadAssetBitmap(n.Cover)
		if bitmap != nil {
			handle := ui.Box(c).FillWidth().Height(128).Radius(8).Clip().Label(a.tr("Document cover"))
			handle.Draw(func(p *ui.Painter, r ui.Rect) {
				bw, bh := bitmap.Size()
				w, h := float32(bw), float32(bh)
				scale := max(r.W/w, r.H/h)
				dw, dh := w*scale, h*scale
				p.Image(bitmap, ui.Rect{X: r.X + (r.W-dw)*(n.CoverX+1)/2, Y: r.Y + (r.H-dh)*(n.CoverY+1)/2, W: dw, H: dh}, ui.Cover)
			})
			if dx, dy, ok := handle.Dragged(); ok {
				n.CoverX = max(-1, min(1, n.CoverX+dx/150))
				n.CoverY = max(-1, min(1, n.CoverY+dy/150))
				a.syncMetadata()
			}
		}
	}
	ui.Row(c).Gap(8).Padding(0, 0, 8).Children(func() {
		if n.Icon != "" {
			ui.Text(c, n.Icon).FontSize(30)
		}
		if ui.TextInput(c, &n.Title).Label(a.tr("Document title")).Font("-apple-system, BlinkMacSystemFont, 'SF Pro Display', sans-serif").FontSize(28).Bold().LetterSpacing(-0.4).Grow(1).Background(ui.Transparent).Border(0, ui.Transparent).Changed() {
			a.syncMetadata()
		}
	})
	if !n.HideInfo {
		info := a.tr("Modified ") + a.dateText(n.Updated, true)
		if n.ShowCreated {
			info = a.tr("Created ") + a.dateText(n.Created, false) + " · " + info
		}
		ui.Text(c, info).FontSize(10).TextColor(c.Theme().TextMuted).Padding(0, 0, 10)
	}
}

func (a *app) loadAssetBitmap(path string) *ui.Bitmap {
	key := a.assetRoot() + "/" + path
	if bitmap, ok := a.assetBitmaps[key]; ok {
		return bitmap
	}
	root, err := os.OpenRoot(a.assetRoot())
	if err != nil {
		return nil
	}
	defer root.Close()
	b, err := root.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return nil
	}
	bitmap, _ := ui.DecodeBitmap(b)
	if a.assetBitmaps == nil {
		a.assetBitmaps = map[string]*ui.Bitmap{}
	}
	a.assetBitmaps[key] = bitmap
	return bitmap
}

func (a *app) documentControls(c *ui.Context) {
	n := a.target
	if n == nil {
		return
	}
	ui.Row(c).Gap(10).Children(func() {
		ui.Text(c, a.tr("Icon"))
		if ui.TextInput(c, &n.Icon).Label(a.tr("Document icon")).Width(100).Changed() {
			if n == a.current {
				a.syncMetadata()
			} else {
				a.fail(a.lib.Save(n))
			}
		}
		width := n.Width
		if width == "" {
			width = "Standard"
		}
		if a.selectText(c, &width, []string{"Standard", "Wide", "Full"}).Label(a.tr("Page width")).Width(130).Changed() {
			a.mutate(n, func() { n.Width = width })
		}
	})
	ui.Row(c).Gap(10).Children(func() {
		show := !n.HideInfo
		if ui.Checkbox(c, &show, a.tr("Document information")).Changed() {
			a.mutate(n, func() { n.HideInfo = !show })
		}
		if ui.Checkbox(c, &n.ShowCreated, a.tr("Created date")).Changed() {
			a.mutate(n, func() {})
		}
	})
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, a.tr("Choose cover…")).Clicked() {
			a.chooseCover(n.ID)
		}
		if n.Cover != "" && ui.Button(c, a.tr("Remove cover")).Clicked() {
			n.Cover = ""
			if n == a.current {
				a.syncMetadata()
			} else {
				a.fail(a.lib.Save(n))
			}
		}
		if ui.Button(c, a.tr("History…")).Clicked() {
			var err error
			a.revisions, err = a.lib.History(n.ID)
			if !a.fail(err) {
				if a.current != n {
					a.openNote(n)
				}
				a.historyOpen = true
				a.renameOpen = false
			}
		}
	})
}

func (a *app) chooseCover(id string) {
	if a.win == nil {
		return
	}
	options := mygo.OpenDialogOptions{Parent: a.win, Title: a.tr("Choose document cover"), Filters: []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "webp"}}}}
	go func() {
		paths, err := openDialog(options)
		a.win.Update(func() {
			if a.fail(err) || len(paths) == 0 {
				return
			}
			path := paths[0]
			source, err := document.CopyAssets("![Cover](<"+filepath.Base(path)+">)", "markdown", filepath.Dir(path), a.assetRoot())
			if a.fail(err) {
				return
			}
			start, end := strings.Index(source, "("), strings.LastIndex(source, ")")
			if n := a.lib.Get(id); n != nil && start >= 0 && end > start {
				n.Cover = source[start+1 : end]
				if n == a.current {
					a.syncMetadata()
				} else {
					a.fail(a.lib.Save(n))
				}
			}
		})
	}()
}
