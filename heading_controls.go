package main

import (
	"fmt"
	"strings"

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
	richTheme := a.docTheme.Rich(c.Theme(), font)
	// EditorBase paints its scrollbar over the full text width. The styled
	// editor's internal padding also participates in native hit testing, IME,
	// selection and wrapping; negative margins keep the text aligned as before.
	richTheme.Padding = [2]float32{documentScrollGutter, 1}
	placeholder := ""
	style := doc.Text().ParagraphStyleAt(0)
	// The native placeholder ignores paragraph indentation and top spacing.
	// Once a block format is chosen, its marker and caret provide the prompt.
	if style.Heading == 0 && style.List == richtext.NoList && !style.Quote && !style.CodeBlock && style.Indent == 0 && style.Align == ui.Start {
		placeholder = a.tr("Start writing…")
	}
	// Borrow the page's left padding for controls, keeping the text stationary.
	ui.Box(c).Grow(1).MinHeight(0).Margin(0, -documentScrollGutter, 0, -headingGutterWidth).Padding(0, 0, 0, headingGutterWidth-documentScrollGutter).Children(func() {
		ed := richtext.Editor(c.Key(a.current.ID), doc, richtext.EditorOptions{
			Label: a.tr("Document editor"), Placeholder: placeholder, Font: font,
			OnLink: a.openLink, OnLinkCommand: func() { a.linkURL = doc.Formatting().Link; a.linkOpen = true },
			Theme: richTheme,
			OnKey: func(key ui.Key, mods ui.Modifiers) bool {
				if isBlockDeleteKey(key, mods) && exitEmptyRichBlock(doc) {
					return true
				}
				redoMods, redoKey, err := parseKey(a.keyFor("Redo", "Cmd+Shift+Z"))
				if err == nil && key == redoKey && mods == redoMods {
					redoRichText(doc)
					return true
				}
				return false
			},
			OnCommand: func(command string) bool {
				if command == "redo" {
					redoRichText(doc)
					return true
				}
				return false
			},
		}).Bind(&a.richEditor).Fill().MinHeight(0).
			Border(0, ui.Transparent).Background(ui.Transparent).Radius(0).FocusRing(false)
		if a.modeCycleFocus {
			ed.Focus()
			a.modeCycleFocus = false
		}
		a.hoverDocumentLink(c, ed, doc, font, richTheme, documentScrollGutter)
		if a.bodyFocus {
			doc.SetSelection(0, 0)
			if a.bodyInput != "" {
				doc.InsertText(a.bodyInput)
			}
			ed.Focus()
			a.bodyFocus = false
			a.bodyInput = ""
		}
		ed.ObserveInput(func(event ui.InputEvent) { a.observeCodeFence(doc, event) })
		a.bodyIndentShortcuts(ed, doc)
		a.headingStartEnter(ed, doc)
		ed.OnPaste(func(text string) bool { a.queueInlineCode(doc, text); return false })
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

// A numeric prefix inside an existing heading is authored title text, not a
// request to replace that heading with a list. Keep other inline shortcuts on.
func headingNumberPrefix(doc *richtext.Document) bool {
	anchor, caret := doc.Selection()
	text := doc.Text()
	if anchor != caret || text.ParagraphStyleAt(caret).Heading == 0 {
		return false
	}
	start, _ := text.ParagraphRange(caret)
	prefix := text.Slice(start, caret).String()
	return len(prefix) >= 2 && (strings.HasSuffix(prefix, ".") || strings.HasSuffix(prefix, ")")) && strings.Trim(prefix[:len(prefix)-1], "0123456789") == ""
}

type headingNumberInput struct {
	doc     *richtext.Document
	before  richtext.Text
	caret   int
	version uint64
}

func headingSpaceEvent(event ui.InputEvent) bool {
	return event.Kind == ui.InputText && event.Text == " " ||
		event.Kind == ui.InputKeyDown && event.Key == ui.KeySpace && event.Mods&^ui.Shift == 0
}

func (a *app) observeHeadingNumberInput(doc *richtext.Document, event ui.InputEvent) {
	// Flush before the next event as well as before rendering: native events can
	// arrive in a batch without a frame between the heading and list shortcuts.
	a.applyHeadingNumberInput()
	// macOS can send committed text directly to the native text client without
	// an InputText event. Capture the prefix on keydown before that conversion.
	if !headingSpaceEvent(event) || !headingNumberPrefix(doc) {
		return
	}
	_, caret := doc.Selection()
	doc.BreakUndoCoalescing()
	a.headingNumberInput = &headingNumberInput{doc, doc.Text(), caret, doc.Version()}
}

func (a *app) applyHeadingNumberInput() {
	p := a.headingNumberInput
	if p == nil || p.doc.Version() == p.version {
		return
	}
	a.headingNumberInput = nil
	style := p.doc.Formatting().Paragraph
	if style.Heading != 0 || style.List != richtext.Numbers {
		return
	}
	// The native editor records the space and its Markdown conversion as two
	// steps. Replace those with a literal space, so undo/redo cannot resurrect
	// the unwanted list conversion. Breaking coalescing kept earlier typing out.
	p.doc.Undo()
	p.doc.Undo()
	if !p.doc.Text().Equal(p.before) {
		p.doc.Redo()
		p.doc.Redo()
		return
	}
	p.doc.SetSelection(p.caret, p.caret)
	p.doc.InsertText(" ")
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
	if s == nil || !s.restoreRich || a.current == nil {
		return
	}
	// Replacing the block editor must retain edits that have not reached autosave.
	a.captureBlocks()
	a.flushSource()
	if !document.RichSafe(a.current.Source) {
		return
	}
	a.doc = richtext.NewDocument(a.richText(a.current.Source))
	a.richHeadingLevels = richHeadingLevels(a.doc.Text())
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
