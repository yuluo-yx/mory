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

const documentCoverHeight, documentCoverBottomGap float32 = 180, 16

func (a *app) documentHeader(c *ui.Context) {
	t := c.Theme()
	n := a.current
	if n == nil {
		return
	}

	// 1. Full-width Cover Banner (Notion / Craft style)
	if n.Cover != "" {
		bitmap := a.loadAssetBitmap(n.Cover)
		if bitmap != nil {
			handle := ui.Box(c).FillWidth().Height(documentCoverHeight).Radius(10).Clip().Label(a.tr("Document cover")).Margin(0, 0, documentCoverBottomGap)
			handle.Draw(func(p *ui.Painter, r ui.Rect) {
				bw, bh := bitmap.Size()
				w, h := float32(bw), float32(bh)
				if w <= 0 || h <= 0 {
					return
				}
				scale := max(r.W/w, r.H/h)
				dw, dh := w*scale, h*scale
				p.Clip(r, 10, func() {
					p.Image(bitmap, ui.Rect{X: r.X + (r.W-dw)*(n.CoverX+1)/2, Y: r.Y + (r.H-dh)*(n.CoverY+1)/2, W: dw, H: dh}, ui.Cover)
				})
			})
			if dx, dy, ok := handle.Dragged(); ok {
				n.CoverX = max(-1, min(1, n.CoverX+dx/150))
				n.CoverY = max(-1, min(1, n.CoverY+dy/150))
				a.syncMetadata()
			}
			handle.Children(func() {
				if handle.Hovered() {
					ui.Row(c).Fill().Padding(10, 12).Justify(ui.End).AlignItems(ui.Start).Gap(6).Children(func() {
						btnChange := ghostButton(c, "").Label(a.tr("Choose cover…")).Tooltip(a.tr("Change cover")).Padding(5, 10).Radius(6).Background(ui.Hex("#000000").Alpha(0.65))
						btnChange.Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, iconImage()).Width(13).Height(13).TextColor(ui.Hex("#ffffff"))
								ui.Text(c, a.tr("Change cover")).FontSize(12).TextColor(ui.Hex("#ffffff"))
							})
						})
						if btnChange.Clicked() {
							a.chooseCover(n.ID)
						}

						btnRemove := ghostButton(c, "").Label(a.tr("Remove cover")).Tooltip(a.tr("Remove cover")).Padding(5, 8).Radius(6).Background(ui.Hex("#000000").Alpha(0.65))
						btnRemove.Children(func() {
							ui.Icon(c, iconTrash()).Width(13).Height(13).TextColor(ui.Hex("#ffffff"))
						})
						if btnRemove.Clicked() {
							n.Cover = ""
							a.syncMetadata()
						}
					})
				} else {
					// Invisible accessibility bridges when not hovered
					ui.Row(c).Height(1).Width(1).Opacity(0).Background(ui.Transparent).Border(0, ui.Transparent).Padding(0).Children(func() {
						if b := ui.Button(c, "").Label(a.tr("Choose cover…")).Height(1).Width(1).Background(ui.Transparent).Border(0, ui.Transparent).Padding(0); b.Clicked() {
							a.chooseCover(n.ID)
						}
						if b := ui.Button(c, "").Label(a.tr("Remove cover")).Height(1).Width(1).Background(ui.Transparent).Border(0, ui.Transparent).Padding(0); b.Clicked() {
							n.Cover = ""
							a.syncMetadata()
						}
					})
				}
			})
		}
	}

	// 2. Hover / Action Bar above title (Add icon / Add cover)
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(2, 0, 8).Children(func() {
		if n.Icon == "" {
			btnAddIcon := ghostButton(c, "").Label(a.tr("Add icon")).Padding(4, 8).Radius(6)
			btnAddIcon.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconSmile()).Width(14).Height(14).TextColor(t.TextMuted)
					ui.Text(c, a.tr("Add icon")).FontSize(12).TextColor(t.TextMuted)
				})
			})
			if btnAddIcon.Clicked() {
				n.Icon = "📝"
				a.syncMetadata()
			}
		}
		if n.Cover == "" {
			btnAddCover := ghostButton(c, "").Label(a.tr("Choose cover…")).Tooltip(a.tr("Add cover")).Padding(4, 8).Radius(6)
			btnAddCover.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconImage()).Width(14).Height(14).TextColor(t.TextMuted)
					ui.Text(c, a.tr("Add cover")).FontSize(12).TextColor(t.TextMuted)
				})
			})
			if btnAddCover.Clicked() {
				a.chooseCover(n.ID)
			}
		}
	})

	// 3. Document Icon & Title Header
	ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(0, 0, 6).Children(func() {
		if n.Icon != "" {
			iconBtn := ghostButton(c, "").Label(a.tr("Change icon")).Padding(4, 6).Radius(8)
			if iconBtn.Hovered() {
				iconBtn.Background(t.SurfaceHover)
			}
			iconBtn.Children(func() {
				ui.Text(c, n.Icon).FontSize(34)
			})
			if iconBtn.Clicked() {
				a.iconPickerOpen = !a.iconPickerOpen
			}
			ui.Popover(c, iconBtn, &a.iconPickerOpen, func() {
				ui.Column(c).Width(240).Padding(12).Gap(10).Radius(10).Background(t.Background).Border(1, t.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.12)).Children(func() {
					ui.Text(c, a.tr("Choose icon")).FontSize(12).Bold().TextColor(t.TextMuted)
					ui.Grid(c).Columns(6).Gap(4).Children(func() {
						for _, emoji := range []string{"📝", "🏡", "💡", "🚀", "📖", "🎨", "🌿", "☕️", "📌", "🎯", "📊", "💻", "✨", "🔥", "🛠️", "📚", "🌱", "📘"} {
							emojiBtn := ghostButton(c, emoji).Padding(4, 6).Radius(6).FontSize(18)
							if emojiBtn.Hovered() {
								emojiBtn.Background(t.SurfaceHover)
							}
							if emojiBtn.Clicked() {
								n.Icon = emoji
								a.syncMetadata()
								a.iconPickerOpen = false
							}
						}
					})
					ui.Box(c).Height(1).FillWidth().Background(t.Border)
					if ghostButton(c, a.tr("Remove icon")).FillWidth().Padding(4, 8).Radius(6).TextColor(ui.Hex("#ff3b30")).Clicked() {
						n.Icon = ""
						a.syncMetadata()
						a.iconPickerOpen = false
					}
				})
			})
		}
		if ui.TextInput(c, &n.Title).Label(a.tr("Document title")).Placeholder(a.tr("Untitled")).PlaceholderColor(t.TextMuted).Font("-apple-system, BlinkMacSystemFont, 'SF Pro Display', sans-serif").FontSize(32).Bold().LetterSpacing(-0.5).Grow(1).Background(ui.Transparent).Border(0, ui.Transparent).Changed() {
			a.syncMetadata()
		}
	})

	// 4. Metadata Info Line
	if !n.HideInfo {
		info := a.tr("Modified ") + a.dateText(n.Updated, true)
		if n.ShowCreated {
			info = a.tr("Created ") + a.dateText(n.Created, false) + " · " + info
		}
		ui.Text(c, info).FontSize(11).TextColor(t.TextMuted).Padding(2, 0, 8)
	}
	ui.Box(c).Height(14)
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
	t := c.Theme()
	n := a.target
	if n == nil {
		return
	}

	// 1. Icon & Layout Appearance Card
	ui.Column(c).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border.Alpha(0.6)).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Icon")).FontSize(13).Bold()
			if ui.TextInput(c, &n.Icon).Label(a.tr("Document icon")).Width(90).Placeholder("Emoji…").Changed() {
				if n == a.current {
					a.syncMetadata()
				} else {
					a.fail(a.lib.Save(n))
				}
			}
			ui.Box(c).Grow(1)
			ui.Text(c, a.tr("Page width")).FontSize(13).TextColor(t.TextMuted)
			width := n.Width
			if width == "" {
				width = "Standard"
			}
			if a.selectText(c, &width, []string{"Standard", "Wide", "Full"}).Label(a.tr("Page width")).Width(120).Changed() {
				a.mutate(n, func() { n.Width = width })
			}
		})
	})

	// 2. Metadata & Attributes Card
	ui.Column(c).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border.Alpha(0.6)).Gap(8).Children(func() {
		show := !n.HideInfo
		if ui.Checkbox(c, &show, a.tr("Document information")).Changed() {
			a.mutate(n, func() { n.HideInfo = !show })
		}
		if ui.Checkbox(c, &n.ShowCreated, a.tr("Created date")).Changed() {
			a.mutate(n, func() {})
		}
	})

	// 3. Cover & History Actions
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, a.tr("Choose cover…")).Padding(5, 10).Radius(6).Clicked() {
			a.chooseCover(n.ID)
		}
		if n.Cover != "" && ui.Button(c, a.tr("Remove cover")).Padding(5, 10).Radius(6).Clicked() {
			n.Cover = ""
			if n == a.current {
				a.syncMetadata()
			} else {
				a.fail(a.lib.Save(n))
			}
		}
		ui.Box(c).Grow(1)
		if ui.Button(c, a.tr("History…")).Padding(5, 10).Radius(6).Clicked() {
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
