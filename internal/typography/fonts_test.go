package typography

import (
	"bytes"
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/doctheme"
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
			{"Mory ABC xyz 0123456789 iW", Text, "Seravek"},
			{"Mory ABC xyz 0123456789 iW", Display, "Seravek"},
			{"func main() 0123", Mono, "monospace"},
			// Core Text chooses fallback metrics from the leading Latin face.
			{"🌱", Text, "Seravek"},
			{"體龍𠮷", Text, "Seravek"},
		} {
			got := render(sample.text, sample.family, weight)
			want := render(sample.text, sample.fallback, weight)
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("fallback changed for %q: family=%q weight=%d", sample.text, sample.family, weight)
			}
		}
		for _, name := range doctheme.Names("") {
			theme := doctheme.Parse(doctheme.Builtin(name))
			body := theme.Font(".page", ui.Font{Family: Text})
			for _, selector := range []string{".page", ".title", "h1", "h2", "h3", "h4", "h5", "h6"} {
				family := theme.Font(selector, body).Family
				for _, sample := range []struct{ text, family string }{
					{"Mory 0123456789", "Seravek"},
					{"中文落文", "Luo"},
				} {
					if !bytes.Equal(render(sample.text, family, weight).Pix, render(sample.text, sample.family, weight).Pix) {
						t.Fatalf("unexpected theme glyphs: theme=%q selector=%q text=%q weight=%d", name, selector, sample.text, weight)
					}
				}
			}
		}
	}
	if bytes.Equal(render("中文落文", Text, 400).Pix, render("中文落文", "-apple-system", 400).Pix) {
		t.Fatal("Luo silently fell back to the system Chinese font")
	}
	if bytes.Equal(render("Mory 0123456789", Text, 400).Pix, render("Mory 0123456789", "-apple-system", 400).Pix) {
		t.Fatal("Seravek silently fell back to the system Latin font")
	}
}
