package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
)

const headingGutterWidth float32 = 42

func headingLevel(n *blocks.Node) int {
	if len(n.Kind) == 2 && n.Kind[0] == 'h' && n.Kind[1] >= '1' && n.Kind[1] <= '9' {
		return int(n.Kind[1] - '0')
	}
	return 0
}

func (s *blockSession) headingFolded(n *blocks.Node) bool {
	if folded, ok := s.foldedHeadings[n.ID]; ok {
		return folded
	}
	return n.Attr("folded") == "true"
}

func (s *blockSession) visibleBlocks() []*blocks.Node {
	if s.visibleCache != nil && s.visibleCacheVersion == s.doc.Version && s.visibleCacheFold == s.foldVersion {
		return s.visibleCache
	}
	var visible []*blocks.Node
	hiddenLevel := 0
	for _, n := range blocks.Visible(s.doc.Root.Children) {
		if n.Kind == "title" {
			continue
		}
		level := headingLevel(n)
		if hiddenLevel > 0 {
			if level > 0 && level <= hiddenLevel {
				hiddenLevel = 0
			} else {
				continue
			}
		}
		visible = append(visible, n)
		if level > 0 && s.headingFolded(n) {
			hiddenLevel = level
		}
	}
	s.visibleCache = visible
	s.visibleCacheVersion = s.doc.Version
	s.visibleCacheFold = s.foldVersion
	return visible
}

// Outline and find navigation reveal the enclosing sections of their target.
func (s *blockSession) revealHeadingParents(target string) {
	var parents []*blocks.Node
	changed := false
	for _, n := range blocks.Visible(s.doc.Root.Children) {
		level := headingLevel(n)
		if level > 0 {
			for len(parents) > 0 && headingLevel(parents[len(parents)-1]) >= level {
				parents = parents[:len(parents)-1]
			}
		}
		if n.ID == target {
			for _, parent := range parents {
				if s.headingFolded(parent) {
					if s.foldedHeadings == nil {
						s.foldedHeadings = make(map[string]bool)
					}
					folded, ok := s.foldedHeadings[parent.ID]
					if !ok || folded {
						s.foldedHeadings[parent.ID] = false
						changed = true
					}
				}
			}
			if changed {
				s.foldVersion++
			}
			return
		}
		if level > 0 {
			parents = append(parents, n)
		}
	}
}

func (a *app) headingToggle(c *ui.Context, level int, title string, folded bool) ui.Element {
	label, icon := "Collapse section ", iconChevronDown()
	if folded {
		label, icon = "Expand section ", iconChevronRight()
	}
	button := ghostButton(c, "").Label(a.tr(label) + title).Tooltip(a.tr(label) + title).
		KeepFocus().Width(36).Height(24).Padding(2).Radius(4)
	button.Children(func() {
		ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).Size(9, 9).TextColor(c.Theme().TextMuted)
			ui.Text(c, fmt.Sprintf("H%d", level)).FontSize(11).TextColor(c.Theme().TextMuted)
		})
	})
	return button
}

func (a *app) richDocumentEditor(c *ui.Context) {
	font := a.writingFont(".page")
	doc := a.doc
	// Borrow the page's left padding for controls, keeping the text stationary.
	ui.Box(c).Grow(1).MinHeight(0).Margin(0, 0, 0, -headingGutterWidth).Padding(0, 0, 0, headingGutterWidth).Children(func() {
		ed := richtext.EditorBase(c.Key(a.current.ID), doc, richtext.EditorOptions{
			Label: a.tr("Document editor"), Placeholder: a.tr("Start writing…"), Font: font,
			OnLink: a.openLink, OnLinkCommand: func() { a.linkURL = doc.Formatting().Link; a.linkOpen = true },
			Theme: a.docTheme.Rich(c.Theme(), font),
		}).Fill()
		a.textFocused = ed.Focused()
		_, y, over := ed.PointerPosition()
		text := doc.Text()
		for _, box := range doc.VisibleParagraphs() {
			level := text.ParagraphStyleAt(box.Start).Heading
			if level == 0 || box.Top < 0 || box.Top+24 > ed.LayoutBounds().H {
				continue
			}
			_, end := text.ParagraphRange(box.Start)
			title := text.Slice(box.Start, end).String()
			top := box.Top + max(0, min(box.Height, 40)-24)/2
			button := a.headingToggle(c.Key(box.Start), level, title, false).Absolute().Left(0).Top(top)
			if !(over && y >= box.Top && y < box.Top+box.Height) && !button.Hovered() && !button.Focused() {
				button.Opacity(0)
			}
			if button.Clicked() {
				a.foldRichHeading(box.Start)
				c.AnimationFrame()
				break
			}
		}
	})
}

func (a *app) foldRichHeading(start int) {
	ordinal := 0
	for at, paragraph := range a.doc.Text().Paragraphs() {
		if at >= start {
			break
		}
		if paragraph.Style.Heading > 0 {
			ordinal++
		}
	}
	if !a.enableBlocks() {
		return
	}
	s := a.blockEditor
	s.foldPresentation = true
	// a.doc now belongs to a block; returning to plain editing must rebuild it.
	a.richSource = "\x00"
	for _, n := range s.doc.Root.Children {
		if headingLevel(n) == 0 {
			continue
		}
		if ordinal == 0 {
			a.toggleHeading(n)
			s.focus, s.focusAt = n.ID, 0
			return
		}
		ordinal--
	}
}

func (a *app) toggleHeading(n *blocks.Node) {
	s := a.blockEditor
	if s.foldedHeadings == nil {
		s.foldedHeadings = make(map[string]bool)
	}
	s.foldedHeadings[n.ID] = !s.headingFolded(n)
	s.foldVersion++
	s.split = splitScrollState{}
	// Folding is presentation state: never remove or serialize hidden content.
	if !s.foldedHeadings[n.ID] && s.foldPresentation {
		ordinal := 0
		for _, child := range s.doc.Root.Children {
			if headingLevel(child) > 0 {
				if s.headingFolded(child) {
					return
				}
				if child.ID == n.ID {
					s.restoreHeading = ordinal
				}
				ordinal++
			}
		}
		s.restoreRich = true
	}
}

func (a *app) restoreUnfoldedEditor() {
	s := a.blockEditor
	if s == nil || !s.restoreRich || a.current == nil || !document.RichSafe(a.current.Source) {
		return
	}
	a.doc = richtext.NewDocument(a.richText(a.current.Source))
	a.richSource, a.richVersion = a.current.Source, a.doc.Version()
	a.blockEditor = nil
	ordinal := 0
	for at, paragraph := range a.doc.Text().Paragraphs() {
		if paragraph.Style.Heading > 0 {
			if ordinal == s.restoreHeading {
				a.doc.SetSelection(at, at)
				break
			}
			ordinal++
		}
	}
}
