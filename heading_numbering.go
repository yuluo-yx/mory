package main

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

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
	appendText := func(text richtext.Text, stop, modelLevel int) {
		for at, p := range text.Paragraphs() {
			if at >= stop {
				break
			}
			if p.Style.Heading > 0 {
				level := p.Style.Heading
				// Rich text renders H7-H9 using H6 metrics; numbering must
				// still use the original level retained by the block model.
				if level == 6 && modelLevel > 6 {
					level = modelLevel
				}
				headings = append(headings, numberedHeading{level, pText(p)})
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
				appendText(f.doc.Text(), f.doc.Text().Len()+1, headingLevel(n))
			} else if level := headingLevel(n); level > 0 {
				headings = append(headings, numberedHeading{level, n.Plain()})
			}
		})
	}
	appendText(doc.Text(), start, 0)
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
	if !headingSpaceEvent(event) {
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

// Converting an existing paragraph also joins its preceding numbered section.
func (a *app) numberHeading(doc *richtext.Document, level int) {
	if level == 0 {
		return
	}
	anchor, caret := doc.Selection()
	start, end := doc.Text().ParagraphRange(caret)
	if anchor != caret || len(headingNumbers(doc.Text().Slice(start, end).String())) > 0 {
		return
	}
	if prefix := nextHeadingNumber(a.precedingHeadings(doc, start), level); prefix != "" {
		doc.SetSelection(start, start)
		doc.InsertText(prefix)
		doc.SetSelection(anchor+len(prefix), caret+len(prefix))
	}
}

func richHeadingLevels(text richtext.Text) []int {
	var levels []int
	for _, p := range text.Paragraphs() {
		if p.Style.Heading > 0 {
			levels = append(levels, p.Style.Heading)
		}
	}
	return levels
}

type headingPosition struct {
	id    string
	level int
}

func blockHeadingPositions(doc *blocks.Document) []headingPosition {
	var positions []headingPosition
	doc.Root.Walk(func(n *blocks.Node) {
		if level := headingLevel(n); level > 0 {
			positions = append(positions, headingPosition{n.ID, level})
		}
	})
	return positions
}

// Numbered runs are opt-in: an unnumbered heading starts a separate section.
// Derive the root from the shallowest remaining heading so deleting or moving
// a parent also repairs the depth of its surviving children.
func reorderedHeadingPrefixes(headings []numberedHeading) []string {
	prefixes := make([]string, len(headings))
	for start := 0; start < len(headings); {
		if len(headingNumbers(headings[start].title)) == 0 {
			start++
			continue
		}
		end, root := start, headings[start].level
		for end < len(headings) && len(headingNumbers(headings[end].title)) > 0 {
			root = min(root, headings[end].level)
			end++
		}
		counts := make([]int, 10)
		for i := start; i < end; i++ {
			level := headings[i].level
			for parent := root; parent < level; parent++ {
				if counts[parent] == 0 {
					counts[parent] = 1
				}
			}
			counts[level]++
			clear(counts[level+1:])
			parts := make([]string, 0, level-root+1)
			for _, n := range counts[root : level+1] {
				parts = append(parts, strconv.Itoa(n))
			}
			prefixes[i] = strings.Join(parts, ".")
			if level == root {
				prefixes[i] += "."
			}
		}
		start = end
	}
	return prefixes
}

type headingPrefixEdit struct {
	start, end int
	prefix     string
}

func headingPrefixChanges(text richtext.Text, prefixes []string) []headingPrefixEdit {
	var edits []headingPrefixEdit
	i := 0
	for at, p := range text.Paragraphs() {
		if p.Style.Heading == 0 {
			continue
		}
		prefix := prefixes[i]
		i++
		if prefix == "" {
			continue
		}
		old := strings.Fields(pText(p))[0]
		if old != prefix {
			title := pText(p)
			leading := len([]rune(title)) - len([]rune(strings.TrimLeftFunc(title, unicode.IsSpace)))
			start := at + leading
			edits = append(edits, headingPrefixEdit{start, start + len([]rune(old)), prefix})
		}
	}
	return edits
}

func replaceHeadingPrefixes(text richtext.Text, edits []headingPrefixEdit, anchor, caret int) (richtext.Text, int, int) {
	// Reverse offsets keep edits valid and preserve all title formatting.
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		adjust := func(pos int) int {
			if pos <= e.start {
				return pos
			}
			if pos < e.end {
				return e.start + min(pos-e.start, len(e.prefix))
			}
			return pos + len(e.prefix) - (e.end - e.start)
		}
		anchor, caret = adjust(anchor), adjust(caret)
		text = text.ReplaceString(e.start, e.end, e.prefix, text.AttributesAt(e.start))
	}
	return text, anchor, caret
}

func (a *app) renumberRichHeadings() {
	doc := a.doc
	levels := richHeadingLevels(doc.Text())
	changed := !slices.Equal(levels, a.richHeadingLevels)
	a.richHeadingLevels = levels
	// Undo restores the original snapshot, including intentionally imported
	// numbering. Do not turn that replay into a fresh edit or discard redo.
	if !changed || doc.CanRedo() || !doc.CanUndo() {
		return
	}
	var headings []numberedHeading
	for _, p := range doc.Text().Paragraphs() {
		if p.Style.Heading > 0 {
			headings = append(headings, numberedHeading{p.Style.Heading, pText(p)})
		}
	}
	edits := headingPrefixChanges(doc.Text(), reorderedHeadingPrefixes(headings))
	if len(edits) == 0 {
		return
	}
	anchor, caret := doc.Selection()
	text, anchor, caret := replaceHeadingPrefixes(doc.Text(), edits, anchor, caret)
	// The rich-text API has no amend operation. Replay the user's last edit
	// together with the derived numbering as a single undoable transaction.
	doc.Undo()
	doc.Edit(func(e *richtext.Edit) {
		e.Replace(0, e.Text().Len(), text)
		// Replacement inherits boundary paragraph styles, including empty tails.
		for at, p := range text.Paragraphs() {
			e.FormatParagraphs(at, at, func(style *richtext.ParagraphStyle) { *style = p.Style })
		}
		e.Select(anchor, caret)
	})
}

func (a *app) renumberBlockHeadings() {
	s := a.blockEditor
	positions := blockHeadingPositions(s.doc)
	changed := !slices.Equal(positions, s.headingPositions)
	s.headingPositions = positions
	if !changed || s.numberingReplay {
		return
	}
	var nodes []*blocks.Node
	var headings []numberedHeading
	s.doc.Root.Walk(func(n *blocks.Node) {
		if level := headingLevel(n); level > 0 {
			nodes = append(nodes, n)
			headings = append(headings, numberedHeading{level, n.Plain()})
		}
	})
	prefixes := reorderedHeadingPrefixes(headings)
	before := s.doc.Version
	err := s.doc.Transaction(func() error {
		for i, n := range nodes {
			if prefixes[i] == "" || strings.Fields(n.Plain())[0] == prefixes[i] {
				continue
			}
			f := a.blockField(n, a.documentColor)
			edits := headingPrefixChanges(f.doc.Text(), prefixes[i:i+1])
			anchor, caret := f.doc.Selection()
			text, anchor, caret := replaceHeadingPrefixes(f.doc.Text(), edits, anchor, caret)
			parent, at := s.doc.Parent(n.ID)
			if err := s.doc.Splice(parent.ID, at, 1, blocks.FromRich(text, n)...); err != nil {
				return err
			}
			f.doc.SetText(text)
			f.doc.SetSelection(anchor, caret)
			f.version = f.doc.Version()
			if s.focus == n.ID {
				s.focusAt = caret
			}
		}
		return nil
	})
	if a.fail(err) {
		return
	}
	if s.doc.Version != before {
		s.doc.Coalesce()
		s.lastTextVersion = s.doc.Version
	}
}
