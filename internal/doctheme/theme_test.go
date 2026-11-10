package doctheme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBuiltinsAndCSSCascade(t *testing.T) {
	for _, name := range []string{"Craft", "Feishu", "Notion", "Obsidian"} {
		theme := Parse(Builtin(name))
		if theme.Value(".page", "font-size") == "" {
			t.Fatal(name, "has no body style")
		}
		font := theme.Font(".page", ui.Font{Size: 17})
		if font.Size < 10 || font.Size > 30 {
			t.Fatal(font)
		}
		rich := theme.Rich(ui.LightTheme(), font)
		if rich.Headings[0].Size <= 1 {
			t.Fatal("heading scale", name)
		}
		if theme.Named("color", "red", ui.Transparent) == ui.Transparent {
			t.Fatal("missing palette")
		}
	}
	source := `/* comment */ :root{--accent:hsl(120,100%,25%);--nested:var(--accent)} body{color:var(--nested);font-size:18px;line-height:1.6;font-family:"Georgia",serif} h1,h2{font-size:2em;font-weight:600} a{color:var(--missing,#123)} p{margin-bottom:0.5rem} .unknown{bogus:1} @media print{p{margin-bottom:99px}} .page{max-width:50rem}`
	theme := Parse(source)
	if theme.Color("body", "color", ui.Transparent).G < 120 {
		t.Fatal("variables/hsl")
	}
	if theme.Length("p", "margin-bottom", 18, 18, 0) != 8 {
		t.Fatal("ignored at rule leaked")
	}
	if theme.Length("h1", "font-size", 18, 18, 0) != 36 || theme.Length("h2", "font-size", 18, 18, 0) != 36 {
		t.Fatal("selector group")
	}
	if len(theme.Warnings) != 2 {
		t.Fatal(theme.Warnings)
	}
	font := theme.Font("body", ui.Font{Size: 16})
	if font.Family != "Georgia" || font.LineHeight != 1.6 {
		t.Fatal(font)
	}
	if theme.Color("a", "color", ui.Transparent) != ui.Hex("#112233") {
		t.Fatal("fallback")
	}
	if theme.Length(".page", "max-width", 18, 18, 0) != 800 {
		t.Fatal("rem")
	}
}

func TestColorLengthAndInvalidStyles(t *testing.T) {
	for _, value := range []string{"#abc", "#abcd", "#123456", "#12345678", "rgb(100%,0%,0%)", "rgba(12,34,56,.4)", "rgb(12 34 56 / 40%)", "hsl(0,100%,50%)", "hsl(60,100%,50%)", "hsl(120,100%,50%)", "hsl(180,100%,50%)", "hsl(240,100%,50%)", "hsla(300,100%,50%,.5)", "inherit", "transparent", "white", "black"} {
		if _, ok := Color(value, ui.Hex("#444444")); !ok {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"", "#xxz", "#12", "rgb(1,2)", "rgb(x,2,3)", "unknown(1,2,3)", "rgb(1,2,3", "a"} {
		if _, ok := Color(value, ui.Transparent); ok {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"2px", "1em", "2rem", "50%", "1.5"} {
		if _, ok := length(value, 20, 18); !ok {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"bad", "NaN", "Inf"} {
		if _, ok := length(value, 20, 18); ok {
			t.Fatal(value)
		}
	}
	theme := Parse(`:root{--a:var(--b);--b:var(--a)} .page{color:var(--a);font-size:broken;background-color:invalid} .title{font-weight:700} h9{line-height:20px} b{color:#abc} td{border-color:#eee} input[type=checkbox]{width:16px} .token.comment{color:#abc} @import 'x';`)
	if len(theme.Warnings) < 4 || theme.Value(".page", "font-size") != "" {
		t.Fatal(theme.Warnings)
	}
	if theme.resolve("var(--x", 0) != "" {
		t.Fatal("malformed variable")
	}
	if !supported("h9", "color") || supported("bogus", "color") || supported(".token.comment", "font-size") {
		t.Fatal("selector validation")
	}
	if validValue("padding", "") || validValue("font-size", "1furlong") {
		t.Fatal("bad length")
	}
	if theme.Named("color", "missing", ui.Hex("#333333")) != ui.Hex("#333333") {
		t.Fatal("named fallback")
	}
	for _, family := range []string{"system-ui", "-apple-system", "sans-serif", "ui-monospace"} {
		css := Parse(`.page{font-family:` + family + `;line-height:24px}`)
		if css.Font(".page", ui.Font{Size: 16}).LineHeight != 1.5 {
			t.Fatal("line height")
		}
	}
	_ = Parse("p{broken")
}

func TestThemeFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, "../x"); err == nil {
		t.Fatal("unsafe name")
	}
	if theme, err := Load(dir, "Missing"); err != nil || !strings.Contains(theme.Source, "Craft") {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "custom.css"), []byte(".page{color:#123456}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "craft.css"), []byte(".page{font-size:19px}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(Names(dir)) != 6 {
		t.Fatal(Names(dir))
	}
	theme, err := Load(dir, "Custom")
	if err != nil || theme.Color(".page", "color", ui.Transparent) != ui.Hex("#123456") {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "bad.css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, "bad"); err == nil {
		t.Fatal("directory accepted")
	}
}
