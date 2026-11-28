package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"

	"github.com/egoist/mygo/ui"
)

type coverPreset struct {
	name, category string
	colors         []string
}

// Original procedural artwork ships as code and becomes an ordinary portable
// PNG asset when selected. No remote image service or preset URI is required.
var coverPresets = []coverPreset{
	{"Lavender mist", "Gradients", []string{"#dfc4e8", "#bdc5eb"}},
	{"Sea glass", "Gradients", []string{"#8de9cc", "#8bdadd"}},
	{"Warm sand", "Gradients", []string{"#d9bda5", "#cf9f8b"}},
	{"Morning sky", "Gradients", []string{"#a8cafd", "#b8e2f7"}},
	{"Iris", "Gradients", []string{"#7370d9", "#6852ac"}},
	{"Apricot", "Gradients", []string{"#ffd37d", "#f5a46f"}},
	{"Deep sea", "Gradients", []string{"#182c34", "#315662"}},
	{"Cloud", "Gradients", []string{"#faf9f7", "#e5e8ed"}},
	{"Rose glow", "Aurora", []string{"#f5e1eb", "#fac6df", "#e1d3f2"}},
	{"Sunrise glow", "Aurora", []string{"#ffd346", "#ffa68d", "#ed79b5"}},
	{"Ocean glow", "Aurora", []string{"#14546e", "#169eab", "#5e3ac4"}},
	{"Orchid glow", "Aurora", []string{"#b197ef", "#ecadf6", "#d382ed"}},
	{"Spring glow", "Aurora", []string{"#e0f3d5", "#f3ecae", "#a6dba7"}},
	{"Ember glow", "Aurora", []string{"#81351d", "#d95d18", "#5766af"}},
	{"Terracotta mountains", "Mountains", []string{"#fff0e2", "#f4b08e", "#d98161", "#ad533d", "#743326"}},
	{"Blue mountains", "Mountains", []string{"#dff1fc", "#92c9ee", "#609bca", "#3d71a2", "#203f70"}},
	{"Green mountains", "Mountains", []string{"#e4f4e4", "#a3d29c", "#70b576", "#448b53", "#285c38"}},
	{"Violet mountains", "Mountains", []string{"#eee5f7", "#c3b1de", "#9d87c0", "#75609c", "#49366f"}},
}

func coverPixels(preset coverPreset, width, height int) *image.RGBA {
	palette := make([]color.RGBA, len(preset.colors))
	for i, hex := range preset.colors {
		v := ui.Hex(hex)
		palette[i] = color.RGBA{R: v.R, G: v.G, B: v.B, A: 255}
	}
	mix := func(a, b color.RGBA, at float64) color.RGBA {
		at = max(0, min(1, at))
		return color.RGBA{R: uint8(float64(a.R)*(1-at) + float64(b.R)*at), G: uint8(float64(a.G)*(1-at) + float64(b.G)*at), B: uint8(float64(a.B)*(1-at) + float64(b.B)*at), A: 255}
	}
	im := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			u, v := float64(x)/float64(width), float64(y)/float64(height)
			c := mix(palette[0], palette[1], u*.85+v*.15)
			switch preset.category {
			case "Aurora":
				c = mix(c, palette[2], math.Pow(u, 1.5)*(.6+.4*math.Sin(v*3+u*2)))
			case "Mountains":
				c = palette[0]
				for layer := 1; layer < len(palette); layer++ {
					level := float64(layer)
					ridge := .28 + level*.135 + .105*math.Sin(u*9+level*1.6) + .04*math.Sin(u*20+level)
					if v >= ridge {
						c = palette[layer]
					}
				}
			}
			im.SetRGBA(x, y, c)
		}
	}
	return im
}

func encodeCover(preset coverPreset, width, height int) ([]byte, error) {
	var data bytes.Buffer
	err := png.Encode(&data, coverPixels(preset, width, height))
	return data.Bytes(), err
}

func (a *app) openCoverPicker(id string) {
	a.coverPickerID, a.coverPickerTab, a.coverPickerOpen = id, "Built-in covers", true
	a.renameOpen = false
}

func (a *app) applyCoverPreset(index int) {
	preset := coverPresets[index]
	data, err := encodeCover(preset, 1600, 480)
	if a.fail(err) {
		return
	}
	a.setCoverAsset(a.coverPickerID, data, preset.name+".png")
}

func (a *app) coverGallery(c *ui.Context) {
	ui.Modal(c, &a.coverPickerOpen, func() {
		n := a.lib.Get(a.coverPickerID)
		if n == nil {
			a.coverPickerOpen = false
			return
		}
		t := c.Theme()
		ui.Column(c).Width(548).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.tr("Choose cover")).FontSize(19).Bold().Grow(1)
				if ghostButton(c, "×").Label(a.tr("Close cover picker")).Clicked() {
					a.coverPickerOpen = false
				}
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				for _, tab := range []string{"Built-in covers", "Upload"} {
					button := ghostButton(c, a.tr(tab)).Radius(6)
					if a.coverPickerTab == tab {
						button.Background(t.Accent.Alpha(.1)).TextColor(t.Accent)
					}
					if button.Clicked() {
						a.coverPickerTab = tab
					}
				}
				ui.Box(c).Grow(1)
				if ghostButton(c, a.tr("Random cover")).Clicked() {
					a.applyCoverPreset(rand.IntN(len(coverPresets)))
				}
				if n.Cover != "" && ghostButton(c, a.tr("Remove cover")).Clicked() {
					n.Cover, n.CoverX, n.CoverY = "", 0, 0
					if n == a.current {
						a.syncMetadata()
					} else {
						a.fail(a.lib.Save(n))
					}
					a.coverPickerOpen = false
				}
			})
			if a.coverPickerTab == "Upload" {
				ui.Column(c).Height(200).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Gap(14).Background(t.Surface).Radius(10).Children(func() {
					ui.Icon(c, iconImage()).Size(32, 32).TextColor(t.TextMuted)
					ui.Text(c, a.tr("Choose a local image up to 128 MiB.")).TextColor(t.TextMuted)
					if ui.PrimaryButton(c, a.tr("Choose local image…")).Clicked() {
						a.coverPickerOpen = false
						a.chooseCover(n.ID)
					}
				})
				return
			}
			_, windowHeight := c.Size()
			ui.Scroll(c).Height(min(470, max(220, windowHeight-220))).FillWidth().Gap(12).Children(func() {
				for _, category := range []string{"Gradients", "Aurora", "Mountains"} {
					ui.Text(c, a.tr(category)).FontSize(12).TextColor(t.TextMuted)
					ui.Grid(c).Columns(4).Gap(10).Children(func() {
						for i, preset := range coverPresets {
							if preset.category != category {
								continue
							}
							if a.coverThumbnails == nil {
								a.coverThumbnails = map[string]*ui.Bitmap{}
							}
							bitmap := a.coverThumbnails[preset.name]
							if bitmap == nil {
								data, _ := encodeCover(preset, 240, 100)
								bitmap, _ = ui.DecodeBitmap(data)
								a.coverThumbnails[preset.name] = bitmap
							}
							button := ui.Button(c, "").Label(a.tr("Cover: ")+a.tr(preset.name)).Tooltip(a.tr(preset.name)).Height(66).Padding(0).Radius(7).Clip().Border(1, t.Border)
							button.Draw(func(p *ui.Painter, r ui.Rect) {
								p.Clip(r, 7, func() { p.Image(bitmap, r, ui.Cover) })
							})
							if button.Hovered() || button.Focused() {
								button.Border(2, t.Accent)
							}
							if button.Clicked() {
								a.applyCoverPreset(i)
								a.coverPickerOpen = false
							}
						}
					})
				}
			})
		})
	})
}
