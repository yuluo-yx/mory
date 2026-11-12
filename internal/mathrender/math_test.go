package mathrender

import (
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestKaTeXNative(t *testing.T) {
	for _, source := range []string{`\frac{a}{b}+\sqrt{x}`, `\sum_{i=0}^{n}i^2`, `\begin{pmatrix}a&b\\c&d\end{pmatrix}`, `\int_0^1 x\,dx=\frac12`, `\overbrace{x+y}^{n}\quad\boxed{E=mc^2}`} {
		html, err := HTML(source, true, "html")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(html, "katex-html") {
			t.Fatal(html)
		}
		f, err := Layout(source, true, 20, ui.Hex("#292927"), 0)
		if err != nil {
			t.Fatal(err)
		}
		if f.Width <= 0 || f.Ascent <= 0 || len(f.items) == 0 {
			t.Fatalf("empty formula for %s: %+v", source, f)
		}
	}
	if _, err := HTML(`\frac{a`, false, "html"); err == nil {
		t.Fatal("invalid formula accepted")
	}
}

func TestEquationLayoutCorpus(t *testing.T) {
	corpus := []string{
		`a^2+b^2=c^2\tag{1}`, `\left(\frac{\sqrt[3]{x}}{1+x}\right)^2`,
		`\displaystyle\lim_{x\to\infty}\frac{1}{x}=0`, `\underbrace{a+b}_{x}+\overline{z}+\underline{w}`,
		`\begin{aligned}a&=b+c\\d&=e-f\end{aligned}`, `\begin{array}{|c:c|}\hline a&b\\\hdashline c&d\\\hline\end{array}`,
		`\begin{cases}x^2&x>0\\0&x\le0\end{cases}`, `\frac{\dbinom{n}{k}}{\tbinom{n}{k}}`,
		`\overrightarrow{AB}\quad\widehat{xyz}\quad\widetilde{xyz}\quad\vec{x}`,
		`\color{red}{x}+\textcolor{#123456}{y}+\colorbox{yellow}{text}`, `\fcolorbox{blue}{white}{boxed}`,
		`\cancel{x}+\bcancel{y}+\xcancel{z}+\sout{w}`, `\xrightarrow[below]{above}\xleftarrow{a}`,
		`\mathbb{R}\mathcal{F}\mathfrak{g}\mathscr{L}\mathrm{roman}\mathsf{sans}\mathtt{mono}`,
		`\textbf{bold}\textit{italic}\textsf{sans}\texttt{type}\mathbf{x}\boldsymbol{\alpha}`,
		`\Huge x\large y\small z\scriptsize w`, `\phantom{x}\hphantom{y}\vphantom{z}\smash{a}`,
		`\rule{2em}{.1em}\kern1em\raisebox{.5em}{up}\llap{x}\rlap{y}`, `\not=\ne\neq\implies\iff`,
		`\overset{!}{=}\underset{n}{\sum}\prod_{i=1}^n x_i`, `x\\[1em]y`,
	}
	for i, source := range corpus {
		t.Run(source, func(t *testing.T) {
			f, err := Layout(source, true, 20, ui.Hex("#123456"), i+1)
			if err != nil {
				t.Fatal(err)
			}
			if f.Width < 0 || math.IsNaN(float64(f.Width)) || f.Ascent+f.Depth <= 0 {
				t.Fatal("invalid dimensions", f)
			}
			cached, err := Layout(source, true, 20, ui.Hex("#123456"), i+1)
			if err != nil || cached != f {
				t.Fatal("layout cache")
			}
			tester := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) { f.Draw(p, r) }) }, 800, 200)
			if tester.Image() == nil {
				t.Fatal("native drawing")
			}
			if markup, err := HTML(source, true, "mathml"); err != nil || !strings.Contains(markup, "<math") {
				t.Fatal("MathML export", err)
			}
		})
	}
	if _, err := Layout(`\undefinedCommand`, false, 16, ui.Transparent, 0); err == nil {
		t.Fatal("invalid expression")
	}
	if _, err := HTML(strings.Repeat("x", 65537), false, "html"); err == nil {
		t.Fatal("unbounded equation")
	}
}

func TestMathCSSGeometry(t *testing.T) {
	width := float32(200)
	for _, tc := range []struct {
		value string
		want  float32
	}{{"2em", 40}, {"12px", 12}, {"25%", 50}, {"3", 3}} {
		got, ok := length(tc.value, 20, &width)
		if !ok || got != tc.want {
			t.Fatal(tc, got)
		}
	}
	if _, ok := length("25%", 20, nil); ok {
		t.Fatal("percentage without containing block")
	}
	if cssColor("#ff0000", ui.Transparent) != ui.Hex("#ff0000") || cssColor("blue", ui.Transparent) != ui.Hex("#0000ff") {
		t.Fatal("CSS colors")
	}
	root, err := parseDOM(`<span class="katex-html"><span class="base" style="width:100px;height:20px;border:2px dashed red;padding:1px 2px 3px 4px;background-color:yellow;vertical-align:2px">x</span><span class="newline"></span><span class="base" style="margin-left:5%;width:50%;color:blue">y</span></span>`)
	if err != nil {
		t.Fatal(err)
	}
	e := layoutEngine{}
	f := e.root(root, true, 20, ui.Hex("#222222"))
	if f.Width < 20 {
		t.Fatal("CSS geometry")
	}
	if empty := e.root(&element{}, false, 20, ui.Transparent); empty.Width != 0 {
		t.Fatal("empty math tree")
	}
}
