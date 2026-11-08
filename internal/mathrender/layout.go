package mathrender

// Native KaTeX layout rules are adapted from MIT-licensed rendering code
// (Copyright 2026 s1dashu), replacing GPUI shaping/painting with MyGo primitives.
import (
	"fmt"
	"html"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	xhtml "golang.org/x/net/html"
)

type item struct {
	x, y, w, h float32
	glyphs     []ui.Glyph
	svg        *ui.SVG
	color      ui.Color
}
type Formula struct {
	Width, Ascent, Depth float32
	items                []item
	Tag                  *Formula
}

func (f *Formula) Draw(p *ui.Painter, r ui.Rect) {
	for _, it := range f.items {
		if it.glyphs != nil {
			p.Glyphs(it.glyphs, r.X+it.x, r.Y+f.Ascent+it.y, it.color)
		} else if it.svg != nil {
			p.Icon(it.svg, ui.Rect{X: r.X + it.x, Y: r.Y + f.Ascent + it.y, W: it.w, H: it.h}, it.color)
		} else {
			p.Fill(ui.Rect{X: r.X + it.x, Y: r.Y + f.Ascent + it.y, W: it.w, H: it.h}, it.color, 0)
		}
	}
	if f.Tag != nil {
		f.Tag.Draw(p, ui.Rect{X: r.X + max(f.Width+24, r.W-f.Tag.Width), Y: r.Y, W: f.Tag.Width, H: r.H})
	}
}

type element struct {
	name, text   string
	classes      []string
	style, attrs map[string]string
	kids         []*element
}

func (e *element) has(c string) bool { return slices.Contains(e.classes, c) }
func (e *element) any(cs ...string) bool {
	for _, c := range cs {
		if e.has(c) {
			return true
		}
	}
	return false
}
func (e *element) find(c string) *element {
	if e.has(c) {
		return e
	}
	for _, k := range e.kids {
		if hit := k.find(c); hit != nil {
			return hit
		}
	}
	return nil
}
func parseDOM(source string) (*element, error) {
	root, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return nil, err
	}
	var convert func(*xhtml.Node) *element
	convert = func(n *xhtml.Node) *element {
		e := &element{name: n.Data, attrs: map[string]string{}, style: map[string]string{}}
		if n.Type == xhtml.TextNode {
			e.name = "#text"
			e.text = n.Data
		}
		for _, a := range n.Attr {
			e.attrs[a.Key] = a.Val
			if a.Key == "class" {
				e.classes = strings.Fields(a.Val)
			}
			if a.Key == "style" {
				for _, decl := range strings.Split(a.Val, ";") {
					k, v, ok := strings.Cut(decl, ":")
					if ok {
						e.style[strings.TrimSpace(k)] = strings.TrimSpace(v)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == xhtml.ElementNode || c.Type == xhtml.TextNode {
				e.kids = append(e.kids, convert(c))
			}
		}
		return e
	}
	return convert(root), nil
}

type box struct {
	w, h, d    float32
	items      []item
	lines, pct bool
}

func (b *box) append(other box) {
	for _, it := range other.items {
		it.x += b.w
		b.items = append(b.items, it)
	}
	b.w += other.w
	b.h = max(b.h, other.h)
	b.d = max(b.d, other.d)
	b.lines = b.lines || other.lines
	b.pct = b.pct || other.pct
}

type context struct {
	size         float32
	family       string
	bold, italic bool
	color        ui.Color
	align        float32
	width        *float32
}
type layoutEngine struct{ number int }

var layoutCache struct {
	sync.Mutex
	values map[string]*Formula
}

func Layout(source string, display bool, size float32, color ui.Color, number int) (*Formula, error) {
	key := fmt.Sprintf("%s:%t:%g:%v:%d", source, display, size, color, number)
	layoutCache.Lock()
	defer layoutCache.Unlock()
	if hit := layoutCache.values[key]; hit != nil {
		return hit, nil
	}
	html, err := HTML(source, display, "html")
	if err != nil {
		return nil, err
	}
	root, err := parseDOM(html)
	if err != nil {
		return nil, err
	}
	e := layoutEngine{number: number}
	formula := e.root(root, display, size, color)
	if layoutCache.values == nil || len(layoutCache.values) > 512 {
		layoutCache.values = map[string]*Formula{}
	}
	layoutCache.values[key] = formula
	return formula, nil
}

func length(v string, size float32, width *float32) (float32, bool) {
	v = strings.TrimSpace(v)
	scale := float32(1)
	if strings.HasSuffix(v, "em") {
		scale = size
		v = strings.TrimSuffix(v, "em")
	} else if strings.HasSuffix(v, "px") {
		v = strings.TrimSuffix(v, "px")
	} else if strings.HasSuffix(v, "%") {
		if width == nil {
			return 0, false
		}
		scale = *width / 100
		v = strings.TrimSuffix(v, "%")
	}
	n, err := strconv.ParseFloat(v, 32)
	return float32(n) * scale, err == nil
}
func value(v string, c context) float32 { n, _ := length(v, c.size, c.width); return n }
func sides(v string) [4]string {
	parts := strings.Fields(v)
	switch len(parts) {
	case 1:
		return [4]string{parts[0], parts[0], parts[0], parts[0]}
	case 2:
		return [4]string{parts[0], parts[1], parts[0], parts[1]}
	case 3:
		return [4]string{parts[0], parts[1], parts[2], parts[1]}
	case 4:
		return [4]string{parts[0], parts[1], parts[2], parts[3]}
	}
	return [4]string{"0", "0", "0", "0"}
}
func cssColor(v string, fallback ui.Color) ui.Color {
	if strings.HasPrefix(v, "#") {
		return ui.Hex(v)
	}
	names := map[string]string{"black": "#000000", "white": "#ffffff", "red": "#ff0000", "green": "#008000", "blue": "#0000ff", "yellow": "#ffff00", "cyan": "#00ffff", "magenta": "#ff00ff", "gray": "#808080", "orange": "#ffa500", "purple": "#800080", "brown": "#a52a2a", "pink": "#ffc0cb", "lime": "#00ff00", "navy": "#000080", "teal": "#008080", "olive": "#808000", "maroon": "#800000", "silver": "#c0c0c0", "gold": "#ffd700", "violet": "#ee82ee"}
	if hex := names[strings.ToLower(v)]; hex != "" {
		return ui.Hex(hex)
	}
	return fallback
}

func (e *layoutEngine) root(root *element, display bool, size float32, color ui.Color) *Formula {
	c := context{size: size, family: "KaTeX_Main", color: color}
	asc, desc := float32(.9155)*size, float32(.2845)*size
	out := &Formula{Ascent: asc, Depth: desc}
	html := root.find("katex-html")
	if html == nil {
		return out
	}
	lines := []box{{}}
	for _, child := range html.kids {
		if child.has("newline") {
			lines = append(lines, box{})
		} else if child.has("tag") {
			b := e.element(child, c, html.classes, false)
			out.Tag = &Formula{Width: b.w, Ascent: b.h, Depth: b.d, items: b.items}
		} else {
			lines[len(lines)-1].append(e.element(child, c, html.classes, false))
		}
	}
	for _, line := range lines {
		out.Width = max(out.Width, line.w)
	}
	y, depth := float32(0), float32(0)
	for i, line := range lines {
		h, d := max(line.h, asc), max(line.d, desc)
		if i == 0 {
			out.Ascent = h
		} else {
			y += depth + h
		}
		depth = d
		dx := float32(0)
		if display {
			dx = (out.Width - line.w) / 2
		}
		for _, it := range line.items {
			it.x += dx
			it.y += y
			out.items = append(out.items, it)
		}
	}
	out.Depth = y + depth
	return out
}

func apply(n *element, parent []string, c context) context {
	if n.any("sizing", "fontsize-ensurer") {
		from, to := 0, 0
		for _, name := range n.classes {
			if strings.HasPrefix(name, "reset-size") {
				from, _ = strconv.Atoi(strings.TrimPrefix(name, "reset-size"))
			} else if strings.HasPrefix(name, "size") {
				to, _ = strconv.Atoi(strings.TrimPrefix(name, "size"))
			}
		}
		sizes := []float32{.5, .6, .7, .8, .9, 1, 1.2, 1.44, 1.728, 2.074, 2.488}
		if from >= 1 && from <= 11 && to >= 1 && to <= 11 {
			c.size *= sizes[to-1] / sizes[from-1]
		}
	}
	if n.has("textbf") {
		c.bold = true
	}
	if n.has("textit") {
		c.italic = true
	}
	for _, rule := range []struct {
		classes      []string
		family       string
		bold, italic int
	}{{[]string{"textrm"}, "KaTeX_Main", 0, 0}, {[]string{"textsf"}, "KaTeX_SansSerif", 0, 0}, {[]string{"texttt"}, "KaTeX_Typewriter", 0, 0}, {[]string{"mathnormal"}, "KaTeX_Math", 0, 1}, {[]string{"mathit"}, "KaTeX_Main", 0, 1}, {[]string{"mathrm"}, "", 0, -1}, {[]string{"mathbf"}, "KaTeX_Main", 1, 0}, {[]string{"boldsymbol"}, "KaTeX_Math", 1, 1}, {[]string{"amsrm", "mathbb", "textbb"}, "KaTeX_AMS", 0, 0}, {[]string{"mathcal"}, "KaTeX_Caligraphic", 0, 0}, {[]string{"mathfrak", "textfrak"}, "KaTeX_Fraktur", 0, 0}, {[]string{"mathboldfrak", "textboldfrak"}, "KaTeX_Fraktur", 1, 0}, {[]string{"mathtt"}, "KaTeX_Typewriter", 0, 0}, {[]string{"mathscr", "textscr"}, "KaTeX_Script", 0, 0}, {[]string{"mathsf", "textsf"}, "KaTeX_SansSerif", 0, 0}, {[]string{"mathboldsf", "textboldsf"}, "KaTeX_SansSerif", 1, 0}, {[]string{"mathsfit", "mathitsf", "textitsf"}, "KaTeX_SansSerif", 0, 1}, {[]string{"mainrm"}, "KaTeX_Main", 0, -1}} {
		if n.any(rule.classes...) {
			if rule.family != "" {
				c.family = rule.family
			}
			if rule.bold != 0 {
				c.bold = rule.bold > 0
			}
			if rule.italic != 0 {
				c.italic = rule.italic > 0
			}
		}
	}
	if n.has("delimsizing") {
		for i := 1; i <= 4; i++ {
			if n.has(fmt.Sprint("size", i)) {
				c.family = fmt.Sprint("KaTeX_Size", i)
			}
		}
	}
	if slices.Contains(parent, "delim-size1") {
		c.family = "KaTeX_Size1"
	}
	if slices.Contains(parent, "delim-size4") {
		c.family = "KaTeX_Size4"
	}
	if n.has("op-symbol") {
		if n.has("small-op") {
			c.family = "KaTeX_Size1"
		} else if n.has("large-op") {
			c.family = "KaTeX_Size2"
		}
	}
	c.color = cssColor(n.style["color"], c.color)
	if n.any("msupsub", "svg-align", "cd-label-left") {
		c.align = 0
	} else if n.any("x-arrow", "mover", "munder") {
		c.align = .5
	} else if n.has("cd-label-right") {
		c.align = 1
	}
	if n.has("vlist-t") {
		for _, cl := range parent {
			switch cl {
			case "mfrac", "op-limits", "accent", "col-align-c":
				c.align = .5
			case "col-align-l":
				c.align = 0
			case "col-align-r":
				c.align = 1
			}
		}
	}
	return c
}
func (e *layoutEngine) text(s string, c context, b *box) {
	if s == "" {
		return
	}
	weight := 400
	if c.bold {
		weight = 700
	}
	font := ui.Font{Family: c.family, Size: c.size, Weight: weight, Italic: c.italic}
	glyphs := ui.Shape(s, font)
	metrics := font.Metrics()
	half := (1.2*c.size - metrics.Ascent - metrics.Descent) / 2
	w := float32(0)
	for _, g := range glyphs {
		w = max(w, g.X+g.Advance)
	}
	b.items = append(b.items, item{x: b.w, glyphs: glyphs, color: c.color})
	b.w += w
	b.h = max(b.h, metrics.Ascent+half)
	b.d = max(b.d, metrics.Descent+half)
	b.lines = true
}

var inlineBlocks = []string{"base", "strut", "mspace", "nulldelimiter", "frac-line", "rule", "overline-line", "underline-line", "hline", "hdashline", "fix", "vertical-separator", "arraycolsep", "cd-vert-arrow", "overlay", "stretchy", "vbox", "hbox", "thinbox"}
var fullWidth = []string{"frac-line", "overline-line", "underline-line", "hline", "hdashline", "stretchy", "hide-tail", "hbox"}
var absolute = []string{"halfarrow-left", "halfarrow-right", "brace-left", "brace-center", "brace-right"}

func (e *layoutEngine) element(n *element, pc context, parent []string, atomic bool) box {
	if n.name == "#text" {
		b := box{}
		e.text(n.text, pc, &b)
		return b
	}
	c := apply(n, parent, pc)
	if n.name == "svg" || n.any("vlist-s", "mathml", "katex-mathml") {
		return box{}
	}
	if n.has("vlist-t") {
		return e.vlist(n, c)
	}
	ib, block := atomic || n.any(inlineBlocks...), n.has("overlay")
	ml, mr, pl, pr := float32(0), float32(0), float32(0), float32(0)
	if n.has("root") && slices.Contains(parent, "sqrt") {
		ml = .2777778 * c.size
		mr = -.5555556 * c.size
	}
	if n.has("angl") {
		mr = .03889 * c.size
	}
	if n.has("cancel-lap") {
		ml = -.2 * c.size
		mr = ml
	}
	if v := n.style["margin"]; v != "" {
		s := sides(v)
		ml = value(s[3], c)
		mr = value(s[1], c)
	}
	if v := n.style["margin-left"]; v != "" {
		ml = value(v, c)
	}
	if v := n.style["margin-right"]; v != "" {
		mr = value(v, c)
	}
	for _, rule := range []struct {
		name string
		l, r float32
	}{{"x-arrow-pad", .5, .5}, {"cd-arrow-pad", .27778, .55556}, {"boxpad", .3, .3}, {"cancel-pad", .2, .2}, {"anglpad", .03889, .03889}} {
		if n.has(rule.name) {
			pl = rule.l * c.size
			pr = rule.r * c.size
		}
	}
	if v := n.style["padding-left"]; v != "" {
		pl = value(v, c)
	}
	if v := n.style["padding-right"]; v != "" {
		pr = value(v, c)
	}
	bw := [4]float32{}
	bs := [4]int{}
	borderBox := false
	if n.any("frac-line", "overline-line", "underline-line", "hline") {
		bs[2] = 1
	}
	if n.has("hdashline") {
		bs[2] = 2
	}
	if n.has("rule") {
		bs = [4]int{1, 1, 1, 1}
	}
	if n.any("fbox", "fcolorbox") {
		bw = [4]float32{.04 * c.size, .04 * c.size, .04 * c.size, .04 * c.size}
		bs = [4]int{1, 1, 1, 1}
		borderBox = true
	}
	if n.has("sout") {
		bw[2] = .08 * c.size
		bs[2] = 1
	}
	if n.has("angl") {
		bw[0] = .049 * c.size
		bw[1] = bw[0]
		bs[0] = 1
		bs[1] = 1
		borderBox = true
	}
	borderStyle := func(s string) int {
		if s == "solid" {
			return 1
		}
		if s == "dashed" {
			return 2
		}
		return 0
	}
	if v := n.style["border-style"]; v != "" {
		for i, s := range sides(v) {
			bs[i] = borderStyle(s)
		}
	}
	if v := n.style["border-width"]; v != "" {
		for i, s := range sides(v) {
			bw[i] = value(s, c)
		}
	}
	for i, side := range []string{"top", "right", "bottom", "left"} {
		if v := n.style["border-"+side+"-width"]; v != "" {
			bw[i] = value(v, c)
		}
		if v := n.style["border-"+side+"-style"]; v != "" {
			bs[i] = borderStyle(v)
		}
		if bs[i] == 0 || bw[i] <= 0 {
			bw[i] = 0
		} else {
			bw[i] = float32(math.Max(1, math.Floor(float64(bw[i]*2))) / 2)
		}
	}
	lap := n.any("llap", "rlap", "clap")
	flow := box{}
	if n.has("eqn-num") {
		e.number++
		e.text(fmt.Sprintf("(%d)", e.number), c, &flow)
	}
	for _, child := range n.kids {
		if child.name == "svg" || child.any(absolute...) || lap && child.has("inner") {
			continue
		}
		hadLines := flow.lines
		flow.append(e.element(child, c, n.classes, false))
		if child.has("overlay") {
			flow.lines = hadLines
		}
	}
	contentLines := flow.lines
	if lap {
		for _, child := range n.kids {
			if !child.has("inner") {
				continue
			}
			b := e.element(child, c, n.classes, false)
			dx := float32(0)
			if n.has("llap") {
				dx = -b.w
			} else if n.has("clap") {
				dx = -b.w / 2
			}
			for _, it := range b.items {
				it.x += flow.w + dx
				flow.items = append(flow.items, it)
			}
			flow.h = max(flow.h, b.h)
			flow.d = max(flow.d, b.d)
		}
	}
	pct, cw := flow.pct, flow.w
	if ib || block {
		hpad := pl + pr + bw[1] + bw[3]
		w, known := length(n.style["width"], c.size, c.width)
		if strings.HasSuffix(n.style["width"], "%") && c.width == nil {
			pct = true
		}
		if !known && n.any(fullWidth...) {
			if c.width != nil {
				w = *c.width
				if !borderBox {
					w -= hpad
				}
				known = true
			} else {
				pct = true
			}
		}
		if n.has("thinbox") {
			w = 0
			known = true
		}
		if n.has("nulldelimiter") {
			w = .12 * c.size
			known = true
		}
		if known {
			cw = w
			if borderBox {
				cw -= hpad
			}
		} else if pct && n.any(fullWidth...) {
			cw = 0
		}
		minimum := value(n.style["min-width"], c)
		if n.has("vertical-separator") {
			minimum = max(1, minimum)
		}
		cw = max(cw, minimum)
	}
	if n.has("accent-body") && !n.has("accent-full") || lap || n.has("mtr-glue") && ib {
		cw = 0
	}
	top, bottom := -flow.h, flow.d
	if (ib || block) && !contentLines {
		height, known := length(n.style["height"], c.size, nil)
		if !known {
			height = flow.h
		}
		if n.any("frac-line", "overline-line", "underline-line", "hline", "hdashline", "rule") {
			height = max(height, 1)
		}
		total := height + bw[0] + bw[2]
		if borderBox {
			total = max(height, bw[0]+bw[2])
		}
		top = -total
		bottom = 0
	}
	x0, x1 := ml, ml+bw[3]+pl+cw+pr+bw[1]
	contentX := ml + bw[3] + pl
	out := box{}
	if v := n.style["background-color"]; v != "" {
		out.items = append(out.items, item{x: x0, y: top, w: x1 - x0, h: bottom - top, color: cssColor(v, ui.Transparent)})
	}
	for _, it := range flow.items {
		it.x += contentX
		out.items = append(out.items, it)
	}
	e.borders(&out, [4]float32{x0, top, x1, bottom}, bw, bs, cssColor(n.style["border-color"], c.color))
	pad := [4]float32{ml + bw[3], top + bw[0], x1 - bw[1], bottom - bw[2]}
	ownH, hasH := length(n.style["height"], c.size, nil)
	for _, child := range n.kids {
		if child.name == "svg" {
			e.svg(child, c, pad, ownH, hasH, &out)
		} else if child.any(absolute...) {
			pw := pad[2] - pad[0]
			fraction, at := float32(.251), float32(1)
			switch {
			case child.has("halfarrow-left"):
				fraction = .502
				at = 0
			case child.has("halfarrow-right"):
				fraction = .502
			case child.has("brace-left"):
				at = 0
			case child.has("brace-center"):
				fraction = .5
				at = -1
			}
			w := pw * fraction
			x := pad[0] + at*(pw-w)
			if at < 0 {
				x = pad[0] + pw*.25
			}
			cc := apply(child, n.classes, c)
			h, ok := length(child.style["height"], cc.size, nil)
			rect := [4]float32{x, pad[1], x + w, pad[1] + h}
			for _, svg := range child.kids {
				if svg.name == "svg" {
					e.svg(svg, cc, rect, h, ok, &out)
				}
			}
		}
	}
	v := value(n.style["vertical-align"], c)
	dx := value(n.style["left"], c)
	if n.style["left"] == "" {
		dx = -value(n.style["right"], c)
	}
	dy := value(n.style["top"], c)
	if n.style["top"] == "" {
		dy = -value(n.style["bottom"], c)
	}
	for i := range out.items {
		out.items[i].x += dx
		out.items[i].y += dy - v
	}
	out.w = x1 + mr
	out.h = -top + v
	out.d = bottom - v
	out.lines = ib || block || contentLines || ml != 0 || mr != 0 || pl != 0 || pr != 0
	out.pct = pct
	return out
}

func (e *layoutEngine) borders(out *box, r [4]float32, width [4]float32, style [4]int, color ui.Color) {
	x0, y0, x1, y1 := r[0], r[1], r[2], r[3]
	rects := [][4]float32{{x0, y0, x1 - x0, width[0]}, {x1 - width[1], y0, width[1], y1 - y0}, {x0, y1 - width[2], x1 - x0, width[2]}, {x0, y0, width[3], y1 - y0}}
	for i, r := range rects {
		if width[i] <= 0 || r[2] <= 0 || r[3] <= 0 {
			continue
		}
		if style[i] == 2 {
			horizontal := i%2 == 0
			total := r[3]
			if horizontal {
				total = r[2]
			}
			dash := max(width[i]*3, 2)
			for pos := float32(0); pos < total; pos += dash * 2 {
				piece := r
				length := min(dash, total-pos)
				if horizontal {
					piece[0] += pos
					piece[2] = length
				} else {
					piece[1] += pos
					piece[3] = length
				}
				out.items = append(out.items, item{x: piece[0], y: piece[1], w: piece[2], h: piece[3], color: color})
			}
		} else {
			out.items = append(out.items, item{x: r[0], y: r[1], w: r[2], h: r[3], color: color})
		}
	}
}

func (e *layoutEngine) svg(n *element, c context, rect [4]float32, height float32, hasHeight bool, out *box) {
	w := rect[2] - rect[0]
	if v, ok := length(n.style["width"], c.size, &w); ok {
		w = v
	}
	h := height
	if !hasHeight {
		h = rect[3] - rect[1]
		if v, ok := length(n.attrs["height"], c.size, nil); ok {
			h = v
		}
	}
	if w <= 0 || h <= 0 {
		return
	}
	var body strings.Builder
	for _, child := range n.kids {
		if child.name == "#text" {
			continue
		}
		body.WriteString("<" + child.name)
		for key, v := range child.attrs {
			if strings.HasSuffix(v, "em") {
				v = fmt.Sprint(value(v, c))
			}
			body.WriteString(" " + key + `="` + html.EscapeString(v) + `"`)
		}
		if child.name == "line" {
			body.WriteString(` stroke="currentColor"`)
		}
		body.WriteString("/>")
	}
	view := n.attrs["viewBox"]
	if view == "" {
		view = n.attrs["viewbox"]
	}
	if view == "" {
		view = fmt.Sprintf("0 0 %g %g", w, h)
	}
	aspect := n.attrs["preserveAspectRatio"]
	if aspect == "" {
		aspect = n.attrs["preserveaspectratio"]
	}
	if aspect == "" {
		aspect = "xMidYMid meet"
	}
	svg, err := ui.ParseSVG([]byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%g" height="%g" viewBox="%s" preserveAspectRatio="%s" fill="currentColor">%s</svg>`, w, h, view, aspect, body.String())))
	if err != nil {
		return
	}
	out.items = append(out.items, item{x: rect[0], y: rect[1], w: w, h: h, svg: svg, color: c.color})
}

func (e *layoutEngine) vlist(n *element, c context) box {
	var rows []*element
	for _, r := range n.kids {
		if r.has("vlist-r") {
			rows = append(rows, r)
		}
	}
	out := box{lines: true}
	if len(rows) == 0 {
		return out
	}
	cell := rows[0].find("vlist")
	if cell == nil {
		return out
	}
	out.h = value(cell.style["height"], c)
	if len(rows) > 1 {
		if second := rows[1].find("vlist"); second != nil {
			out.d = value(second.style["height"], c)
		}
	}
	type entry struct {
		ml, mr, y float32
		c         context
		b         box
		n         *element
	}
	entries := []entry{}
	lay := func(n *element, c context) (float32, box) {
		strut := float32(0)
		b := box{}
		for _, child := range n.kids {
			if child.has("pstrut") {
				strut = value(child.style["height"], c)
			} else {
				b.append(e.element(child, c, n.classes, true))
			}
		}
		return strut, b
	}
	for _, wrapper := range cell.kids {
		if wrapper.name == "#text" {
			continue
		}
		wc := apply(wrapper, cell.classes, c)
		strut, b := lay(wrapper, wc)
		entries = append(entries, entry{ml: value(wrapper.style["margin-left"], wc), mr: value(wrapper.style["margin-right"], wc), y: value(wrapper.style["top"], wc) + strut, c: wc, b: b, n: wrapper})
	}
	width := float32(0)
	for _, entry := range entries {
		width = max(width, entry.ml+entry.b.w+entry.mr)
	}
	for _, entry := range entries {
		if entry.b.pct {
			w := width - entry.ml - entry.mr
			entry.c.width = &w
			_, entry.b = lay(entry.n, entry.c)
		}
		x := entry.ml + entry.c.align*(width-entry.ml-entry.mr-entry.b.w)
		for _, it := range entry.b.items {
			it.x += x
			it.y += entry.y
			out.items = append(out.items, it)
		}
	}
	out.w = width
	return out
}
