package typography

import (
	"bytes"
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBundledFontAndFallback(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, family := range ui.FontFamilies() {
		if family.Name == "Luo" && family.Registered {
			found = true
		}
	}
	if !found {
		t.Fatal("Luo is not registered as an application font")
	}
	// Draw at a fixed baseline to compare the actual selected glyphs rather
	// than the line metrics of the first font in each cascade.
	render := func(text, family string, weight int) *image.RGBA {
		glyphs := ui.Shape(text, ui.Font{Family: family, Size: 24, Weight: weight})
		return ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.Hex("#ffffff")).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Glyphs(glyphs, r.X+8, r.Y+40, ui.Hex("#111111"))
			})
		}, 440, 70).Image()
	}
	for _, weight := range []int{400, 700} {
		for _, family := range []string{Text, Display, Mono} {
			// Core Text can adjust punctuation spacing in a mixed cascade;
			// compare its glyph separately from the surrounding Han run.
			for _, sample := range []string{"中文字体落文", "，", "。"} {
				got := render(sample, family, weight)
				want := render(sample, "Luo", weight)
				if !bytes.Equal(got.Pix, want.Pix) {
					t.Fatalf("CJK did not use Luo: text=%q family=%q weight=%d", sample, family, weight)
				}
			}
		}
		for _, sample := range []struct{ text, family, fallback string }{
			{"Mory 0123 iW", Text, "-apple-system"},
			{"Mory 0123 iW", Display, "-apple-system"},
			{"func main() 0123", Mono, "monospace"},
			{"🌱", Text, "-apple-system"},
			{"體龍𠮷", Text, "-apple-system"},
		} {
			got := render(sample.text, sample.family, weight)
			want := render(sample.text, sample.fallback, weight)
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("fallback changed for %q: family=%q weight=%d", sample.text, sample.family, weight)
			}
		}
	}
	if bytes.Equal(render("中文落文", Text, 400).Pix, render("中文落文", "-apple-system", 400).Pix) {
		t.Fatal("Luo silently fell back to the system Chinese font")
	}
}
