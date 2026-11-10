package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

var highlightedLines = regexp.MustCompile(`\{([0-9, -]+)\}`)

// Code highlighting is presentation only, including Docusaurus magic comments.
func docusaurusCodeHighlights(source, meta string, color ui.Color) []editor.LineBackground {
	lines := strings.Split(source, "\n")
	marked := map[int]bool{}
	if match := highlightedLines.FindStringSubmatch(meta); match != nil {
		for _, part := range strings.Split(match[1], ",") {
			first, last, ranged := strings.Cut(strings.TrimSpace(part), "-")
			a, err := strconv.Atoi(strings.TrimSpace(first))
			if err != nil || a < 1 {
				continue
			}
			b := a
			if ranged {
				b, err = strconv.Atoi(strings.TrimSpace(last))
				if err != nil {
					continue
				}
			}
			for i := a; i <= min(b, len(lines)); i++ {
				marked[i-1] = true
			}
		}
	}
	inside, next := false, false
	for i, line := range lines {
		s := strings.TrimSpace(line)
		comment := strings.HasPrefix(s, "//") || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "/*") || strings.HasPrefix(s, "<!--")
		if comment {
			switch {
			case strings.Contains(s, "highlight-start"):
				inside = true
				continue
			case strings.Contains(s, "highlight-end"):
				inside = false
				continue
			case strings.Contains(s, "highlight-next-line"):
				next = true
				continue
			}
		}
		if inside || next {
			marked[i] = true
		}
		next = false
	}
	var out []editor.LineBackground
	for i := range lines {
		if marked[i] {
			out = append(out, editor.LineBackground{First: i, Last: i, Color: color})
		}
	}
	return out
}
