// Package doctheme reads document CSS independently from the application's UI.
package doctheme

import (
	"embed"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

//go:embed assets/*.css
var builtins embed.FS

type Theme struct {
	Rules     map[string]map[string]string
	Variables map[string]string
	Warnings  []string
	Source    string
}

func Builtin(name string) string {
	b, _ := builtins.ReadFile("assets/" + strings.ToLower(name) + ".css")
	return string(b)
}

func Load(dir, name string) (*Theme, error) {
	if filepath.Base(name) != name {
		return nil, fmt.Errorf("invalid theme name")
	}
	b, err := os.ReadFile(filepath.Join(dir, strings.ToLower(name)+".css"))
	if os.IsNotExist(err) {
		source := Builtin(name)
		if source == "" {
			source = Builtin("Craft")
		}
		return Parse(source), nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(string(b)), nil
}

func Names(dir string) []string {
	out := []string{"Craft", "Feishu", "Notion", "Obsidian"}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".css") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".css")
		found := false
		for _, v := range out {
			if strings.EqualFold(v, name) {
				found = true
			}
		}
		if !found {
			out = append(out, name)
		}
	}
	sort.Strings(out[4:])
	return out
}

func Parse(source string) *Theme {
	t := &Theme{Rules: map[string]map[string]string{}, Variables: map[string]string{}, Source: source}
	p := css.NewParser(parse.NewInput(strings.NewReader(source)), false)
	selectors := []string{}
	skip := 0
	for {
		grammar, _, data := p.Next()
		if grammar == css.ErrorGrammar {
			if p.Err() != io.EOF {
				t.Warnings = append(t.Warnings, p.Err().Error())
			}
			break
		}
		var value strings.Builder
		for _, v := range p.Values() {
			value.Write(v.Data)
		}
		switch grammar {
		case css.BeginAtRuleGrammar:
			skip++
			t.Warnings = append(t.Warnings, "Unsupported rule: "+string(data))
		case css.EndAtRuleGrammar:
			skip = max(0, skip-1)
		case css.AtRuleGrammar:
			t.Warnings = append(t.Warnings, "Unsupported rule: "+string(data))
		case css.BeginRulesetGrammar:
			if skip == 0 {
				selectors = strings.Split(value.String(), ",")
			}
		case css.EndRulesetGrammar:
			selectors = nil
		case css.DeclarationGrammar, css.CustomPropertyGrammar:
			if skip > 0 {
				continue
			}
			key, val := strings.TrimSpace(string(data)), strings.TrimSpace(value.String())
			for _, s := range selectors {
				s = canonical(strings.TrimSpace(s))
				if s == ":root" && strings.HasPrefix(key, "--") {
					t.Variables[key] = val
					continue
				}
				if !supported(s, key) {
					t.Warnings = append(t.Warnings, s+" { "+key+": "+val+" }")
					continue
				}
				if t.Rules[s] == nil {
					t.Rules[s] = map[string]string{}
				}
				t.Rules[s][key] = val
			}
		}
	}
	for selector, props := range t.Rules {
		for key, val := range props {
			resolved := t.resolve(val, 0)
			if !validValue(key, resolved) {
				t.Warnings = append(t.Warnings, "Invalid value: "+selector+" "+key+": "+val)
				delete(props, key)
			}
		}
	}
	return t
}

func canonical(s string) string {
	switch s {
	case "body":
		return ".page"
	case "b":
		return "strong"
	case "td":
		return "table"
	case "input[type=checkbox]":
		return ".task"
	}
	return s
}

func supported(s, key string) bool {
	if len(s) == 2 && s[0] == 'h' && s[1] >= '1' && s[1] <= '9' {
		s = "heading"
	}
	if strings.HasPrefix(s, ".token.") {
		return key == "color"
	}
	allowed := map[string]string{
		".page":       "background-color color font-family font-size line-height max-width caret-color",
		"::selection": "background-color", "p": "margin-bottom", ".title": "font-family font-size line-height font-weight color",
		"heading": "font-family font-size line-height font-weight color margin-top margin-bottom",
		"strong":  "font-weight color", "a": "color text-decoration text-decoration-color", "a:hover": "color",
		"code": "color background-color border-color border-radius padding-left", "pre": "font-family font-size line-height color background-color border-color border-radius",
		".line-numbers": "display color background-color", "blockquote": "border-left border-left-width border-left-color padding-left color font-style",
		"ul": "padding-left", "ul li::marker": "color", "ol li::marker": "color", "li::marker": "color",
		".task": "width border-radius border-color", ".task:checked": "background-color color", ".task-done": "color text-decoration",
		"hr": "border-top border-top-style border-top-width border-top-color", "table": "font-size line-height border-color border-radius", "th": "background-color",
		".callout": "border-radius border-width padding",
	}
	return strings.Contains(" "+allowed[s]+" ", " "+key+" ")
}

func validValue(key, val string) bool {
	if strings.HasSuffix(key, "color") {
		_, ok := Color(val, ui.Color{})
		return ok
	}
	if strings.Contains(key, "size") || strings.Contains(key, "width") || strings.Contains(key, "height") || strings.Contains(key, "radius") || strings.HasPrefix(key, "margin") || strings.HasPrefix(key, "padding") {
		parts := strings.Fields(val)
		if len(parts) == 0 {
			return false
		}
		for _, part := range parts {
			if _, ok := length(part, 16, 16); !ok {
				return false
			}
		}
		return true
	}
	return val != ""
}

func (t *Theme) resolve(value string, depth int) string {
	if depth > 16 {
		return ""
	}
	for {
		start := strings.Index(value, "var(")
		if start < 0 {
			return value
		}
		level, end := 1, start+4
		for ; end < len(value) && level > 0; end++ {
			if value[end] == '(' {
				level++
			}
			if value[end] == ')' {
				level--
			}
		}
		if level != 0 {
			return ""
		}
		name, fallback, _ := strings.Cut(value[start+4:end-1], ",")
		replacement, ok := t.Variables[strings.TrimSpace(name)]
		if !ok {
			replacement = strings.TrimSpace(fallback)
		}
		value = value[:start] + t.resolve(replacement, depth+1) + value[end:]
	}
}

func (t *Theme) Value(selector, key string) string {
	return t.resolve(t.Rules[canonical(selector)][key], 0)
}
func (t *Theme) Color(selector, key string, fallback ui.Color) ui.Color {
	v, _ := Color(t.Value(selector, key), fallback)
	return v
}
func (t *Theme) Named(kind, name string, fallback ui.Color) ui.Color {
	v, ok := t.Variables["--"+kind+"-"+name]
	if !ok {
		return fallback
	}
	c, _ := Color(t.resolve(v, 0), fallback)
	return c
}
func (t *Theme) Length(selector, key string, base, font, fallback float32) float32 {
	v, ok := length(t.Value(selector, key), base, font)
	if !ok {
		return fallback
	}
	return v
}

func length(value string, base, font float32) (float32, bool) {
	scale := float32(1)
	switch {
	case strings.HasSuffix(value, "rem"):
		value = strings.TrimSuffix(value, "rem")
		scale = 16
	case strings.HasSuffix(value, "em"):
		value = strings.TrimSuffix(value, "em")
		scale = font
	case strings.HasSuffix(value, "px"):
		value = strings.TrimSuffix(value, "px")
	case strings.HasSuffix(value, "%"):
		value = strings.TrimSuffix(value, "%")
		scale = base / 100
	}
	f, err := strconv.ParseFloat(value, 32)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return float32(f) * scale, true
}

func Color(value string, fallback ui.Color) (ui.Color, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "inherit":
		return fallback, true
	case "transparent":
		return ui.Transparent, true
	case "white":
		return ui.RGB(255, 255, 255), true
	case "black":
		return ui.RGB(0, 0, 0), true
	}
	if strings.HasPrefix(value, "#") {
		s := value[1:]
		if len(s) == 3 || len(s) == 4 {
			var b strings.Builder
			for _, r := range s {
				b.WriteRune(r)
				b.WriteRune(r)
			}
			s = b.String()
		}
		if len(s) != 6 && len(s) != 8 {
			return fallback, false
		}
		if _, err := strconv.ParseUint(s, 16, 32); err != nil {
			return fallback, false
		}
		return ui.Hex("#" + s), true
	}
	name, args, ok := strings.Cut(value, "(")
	if !ok || !strings.HasSuffix(args, ")") {
		return fallback, false
	}
	parts := strings.Fields(strings.NewReplacer(",", " ", "/", " ").Replace(strings.TrimSuffix(args, ")")))
	if len(parts) < 3 || len(parts) > 4 {
		return fallback, false
	}
	numbers := make([]float64, len(parts))
	for i, p := range parts {
		n, err := strconv.ParseFloat(strings.TrimSuffix(p, "%"), 64)
		if err != nil {
			return fallback, false
		}
		if strings.HasSuffix(p, "%") {
			if strings.HasPrefix(name, "rgb") && i < 3 {
				n = n * 255 / 100
			} else {
				n /= 100
			}
		}
		numbers[i] = n
	}
	r, g, b := numbers[0], numbers[1], numbers[2]
	if name == "hsl" || name == "hsla" {
		h := math.Mod(r, 360) / 360
		s, l := g, b
		chroma := (1 - math.Abs(2*l-1)) * s
		x := chroma * (1 - math.Abs(math.Mod(h*6, 2)-1))
		m := l - chroma/2
		switch int(h * 6) {
		case 0:
			r, g, b = chroma, x, 0
		case 1:
			r, g, b = x, chroma, 0
		case 2:
			r, g, b = 0, chroma, x
		case 3:
			r, g, b = 0, x, chroma
		case 4:
			r, g, b = x, 0, chroma
		default:
			r, g, b = chroma, 0, x
		}
		r, g, b = (r+m)*255, (g+m)*255, (b+m)*255
	} else if name != "rgb" && name != "rgba" {
		return fallback, false
	}
	alpha := float32(1)
	if len(numbers) == 4 {
		alpha = float32(numbers[3])
	}
	return ui.RGB(uint8(max(0, min(255, r))), uint8(max(0, min(255, g))), uint8(max(0, min(255, b)))).Alpha(max(0, min(1, alpha))), true
}

func (t *Theme) Font(selector string, base ui.Font) ui.Font {
	if family := t.Value(selector, "font-family"); family != "" {
		family = strings.Trim(strings.TrimSpace(strings.Split(family, ",")[0]), "\"'")
		switch family {
		case "system-ui", "-apple-system", "sans-serif":
			family = "Helvetica Neue"
		case "ui-monospace":
			family = "monospace"
		}
		base.Family = family
	}
	base.Size = t.Length(selector, "font-size", base.Size, base.Size, base.Size)
	if line := t.Value(selector, "line-height"); line != "" {
		height, ok := length(line, base.Size, base.Size)
		if ok {
			if strings.ContainsAny(line, "pxem") {
				height /= base.Size
			}
			base.LineHeight = height
		}
	}
	return base
}

func (t *Theme) Rich(base *ui.Theme, font ui.Font) *richtext.Theme {
	r := richtext.DefaultTheme(base)
	r.Text = t.Color(".page", "color", base.Text)
	r.Caret = t.Color(".page", "caret-color", base.Accent)
	r.Selection = t.Color("::selection", "background-color", base.Selection)
	r.Link = t.Color("a", "color", base.Accent)
	r.Code = t.Color("code", "color", r.Text)
	r.CodeBackground = t.Color("code", "background-color", r.CodeBackground)
	r.CodeFont = t.Font("pre", font).Family
	r.CodeBlockBackground = t.Color("pre", "background-color", r.CodeBlockBackground)
	r.CodeBlockRadius = t.Length("pre", "border-radius", font.Size, font.Size, 5) / font.Size
	r.ParagraphSpacing = t.Length("p", "margin-bottom", font.Size, font.Size, 8) / font.Size
	r.Indent = t.Length("ul", "padding-left", font.Size, font.Size, 24) / font.Size
	r.Marker = t.Color("li::marker", "color", r.Text)
	r.Checkbox = t.Color(".task:checked", "background-color", base.Accent)
	r.Checkmark = t.Color(".task:checked", "color", ui.RGB(255, 255, 255))
	r.Muted = t.Color(".task-done", "color", base.TextMuted)
	r.QuoteBar = t.Color("blockquote", "border-left-color", base.Border)
	r.QuoteText = t.Color("blockquote", "color", base.TextMuted)
	r.QuoteBarWidth = t.Length("blockquote", "border-left-width", font.Size, font.Size, 3)
	for i := range r.Headings {
		s := fmt.Sprintf("h%d", i+1)
		h := &r.Headings[i]
		h.Size = t.Length(s, "font-size", font.Size, font.Size, h.Size*font.Size) / font.Size
		h.Color = t.Color(s, "color", r.Text)
		h.Top = t.Length(s, "margin-top", font.Size, font.Size, 8) / font.Size
		h.Bottom = t.Length(s, "margin-bottom", font.Size, font.Size, 5) / font.Size
		if weight, err := strconv.Atoi(t.Value(s, "font-weight")); err == nil {
			h.Weight = weight
		}
	}
	if base.Dark {
		r.Text = base.Text
		r.Link = ui.Hex("#8db2ff")
		r.QuoteText = base.TextMuted
		r.Code = base.Text
		r.CodeBackground = base.Surface
		r.CodeBlockBackground = base.Surface
		for i := range r.Headings {
			r.Headings[i].Color = base.Text
		}
	}
	return r
}
