package main

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
)

type numberedHeading struct {
	level int
	title string
}

// Only an explicit decimal heading prefix opts a section into numbering.
func headingNumbers(title string) []int {
	word := ""
	// Accept other whitespace (including imported non-breaking spaces).
	if fields := strings.Fields(title); len(fields) > 0 {
		word = fields[0]
	}
	if !strings.Contains(word, ".") {
		return nil
	}
	parts := strings.Split(strings.TrimSuffix(word, "."), ".")
	numbers := make([]int, len(parts))
	for i, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return nil
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n == int(^uint(0)>>1) {
			return nil
		}
		numbers[i] = n
	}
	return numbers
}

func nextHeadingNumber(previous []numberedHeading, level int) string {
	for i := len(previous) - 1; i >= 0; i-- {
		h := previous[i]
		if h.level > level {
			continue
		}
		numbers := headingNumbers(h.title)
		if len(numbers) == 0 {
			return ""
		}
		if h.level == level {
			numbers[len(numbers)-1]++
		} else {
			for depth := h.level; depth < level; depth++ {
				numbers = append(numbers, 1)
			}
		}
		parts := make([]string, len(numbers))
		for i, n := range numbers {
			parts[i] = strconv.Itoa(n)
		}
		prefix := strings.Join(parts, ".")
		if len(parts) == 1 {
			prefix += "."
		}
		return prefix + " "
	}
	return ""
}

// Read live editor fields as well as unopened blocks, including folded sections.
func (a *app) precedingHeadings(doc *richtext.Document, start int) []numberedHeading {
	var headings []numberedHeading
	appendText := func(text richtext.Text, stop int) {
		for at, p := range text.Paragraphs() {
			if at >= stop {
				break
			}
			if p.Style.Heading > 0 {
				headings = append(headings, numberedHeading{p.Style.Heading, pText(p)})
			}
		}
	}
	if s := a.blockEditor; s != nil {
		found := false
		s.doc.Root.Walk(func(n *blocks.Node) {
			if found {
				return
			}
			if f := s.fields[n.ID]; f != nil && f.doc != nil {
				if f.doc == doc {
					found = true
					return
				}
				appendText(f.doc.Text(), f.doc.Text().Len()+1)
			} else if level := headingLevel(n); level > 0 {
				headings = append(headings, numberedHeading{level, n.Plain()})
			}
		})
	}
	appendText(doc.Text(), start)
	return headings
}

type automaticHeadingInput struct {
	doc          *richtext.Document
	start, level int
	version      uint64
	prefix       string
}

func (a *app) observeAutomaticHeading(doc *richtext.Document, event ui.InputEvent) {
	a.applyAutomaticHeading()
	if event.Kind != ui.InputText || event.Text != " " {
		return
	}
	anchor, caret := doc.Selection()
	text := doc.Text()
	start, end := text.ParagraphRange(caret)
	if anchor != caret || caret != end || text.ParagraphStyleAt(caret).CodeBlock {
		return
	}
	marks := text.Slice(start, caret).String()
	if len(marks) == 0 || len(marks) > 6 || strings.Trim(marks, "#") != "" {
		return
	}
	prefix := nextHeadingNumber(a.precedingHeadings(doc, start), len(marks))
	if prefix != "" {
		a.automaticHeadingInput = &automaticHeadingInput{doc, start, len(marks), doc.Version(), prefix}
	}
}

func (a *app) applyAutomaticHeading() {
	p := a.automaticHeadingInput
	if p == nil || p.doc.Version() == p.version {
		return
	}
	a.automaticHeadingInput = nil
	text := p.doc.Text()
	start, end := text.ParagraphRange(p.start)
	if start != p.start || start != end || text.ParagraphStyleAt(start).Heading != p.level {
		return
	}
	p.doc.InsertText(p.prefix)
}

func (a *app) numberEmptyHeading(doc *richtext.Document, level int) {
	if level == 0 {
		return
	}
	anchor, caret := doc.Selection()
	start, end := doc.Text().ParagraphRange(caret)
	if anchor != caret || start != end {
		return
	}
	if prefix := nextHeadingNumber(a.precedingHeadings(doc, start), level); prefix != "" {
		doc.InsertText(prefix)
	}
}
