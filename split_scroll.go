package main

import (
	"math"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// splitScrollState maps source lines to rendered blocks instead of equating
// scrollbar percentages: a table or image can be much taller than its source.
// It belongs to the parsed session, so source edits rebuild its mapping.
type splitScrollState struct {
	scroll         ui.ScrollState
	viewport       ui.Rect
	rows           map[int]ui.Rect
	lines          []int
	lastSource     float64
	lastPreview    float32
	geometryScroll float32
	initialized    bool
	driven         bool
	pending        bool
	target         int
	fraction       float64
}

// splitSourceLines locates the original spelling retained by the lossless
// parser. Searching forward distinguishes repeated paragraphs and skips raw
// whitespace, title metadata, and folded blocks that the preview does not show.
func splitSourceLines(source, format string, nodes []*blocks.Node) []int {
	lines := make([]int, 0, len(nodes)+1)
	cursor, line := 0, 0
	for _, n := range nodes {
		raw := n.Markdown()
		if format == "xml" {
			raw = n.XML()
		}
		at := strings.Index(source[cursor:], raw)
		if at < 0 || raw == "" {
			lines = append(lines, line)
			continue
		}
		start := cursor + at
		line += strings.Count(source[cursor:start], "\n")
		lines = append(lines, line)
		line += strings.Count(raw, "\n")
		cursor = start + len(raw)
	}
	return append(lines, line+strings.Count(source[cursor:], "\n")+1)
}

func (s *splitScrollState) sync(c *ui.Context, code *editor.State, list *ui.ListState, source, format string, nodes []*blocks.Node) {
	if len(nodes) == 0 {
		return
	}
	if s.rows == nil {
		s.rows = make(map[int]ui.Rect)
		s.lines = splitSourceLines(source, format, nodes)
	}
	y := code.ScrollY()
	sourceMoved := !s.initialized || math.Abs(y-s.lastSource) > 0.5
	if sourceMoved {
		s.initialized = true
		s.target = max(0, sort.Search(len(nodes), func(i int) bool { return code.LineTop(s.lines[i]) > y })-1)
		top, end := code.LineTop(s.lines[s.target]), code.LineTop(s.lines[s.target+1])
		s.fraction = 0
		if end > top {
			s.fraction = max(0, min(1, (y-top)/(end-top)))
		}
		s.pending = true
	}
	if s.pending {
		if r, ok := s.rows[s.target]; ok && s.viewport.H > 0 {
			s.scroll.Y = max(0, s.geometryScroll+r.Y-s.viewport.Y+float32(s.fraction)*(r.H+7))
			if y <= 0.5 {
				s.scroll.Y = 0
			}
			s.pending = false
		} else {
			// The virtual list must measure a distant block before its fractional
			// offset is known. Complete alignment in the following frame.
			list.ScrollTo(s.target, ui.Start)
		}
		s.driven = true
		c.AnimationFrame()
	} else if math.Abs(float64(s.scroll.Y-s.lastPreview)) > 0.5 {
		// Wheel input updates the scroll state before the list lays out its rows.
		// Wait for matching geometry instead of translating the previous frame.
		if math.Abs(float64(s.scroll.Y-s.geometryScroll)) > 0.5 {
			s.lastSource = code.ScrollY()
			c.AnimationFrame()
			return
		}
		first, last := list.Visible()
		for i := first; i <= last && i < len(nodes); i++ {
			r, ok := s.rows[i]
			if !ok || r.Y+r.H+7 <= s.viewport.Y {
				continue
			}
			fraction := max(0, min(1, float64(s.viewport.Y-r.Y)/float64(r.H+7)))
			top, end := code.LineTop(s.lines[i]), code.LineTop(s.lines[i+1])
			code.SetScrollY(top + fraction*(end-top))
			if s.scroll.Y <= 0.5 {
				code.SetScrollY(0)
			}
			c.AnimationFrame()
			break
		}
	}
	s.lastSource, s.lastPreview = code.ScrollY(), s.scroll.Y
}
