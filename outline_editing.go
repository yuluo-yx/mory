package main

import "strings"

type sourceListDraft struct {
	line, caret int
}

// A newly typed lone dash can become either a list marker or a Setext heading.
// Defer only this transient outline entry; the Markdown itself stays untouched.
func (a *app) trackSourceListDraft() {
	a.sourceListDraft = nil
	if a.mode != "Source" || a.current.Format != "markdown" || len(a.code.Selections()) != 1 {
		return
	}
	sel := a.code.Selection()
	text := a.code.Buffer.Snapshot()
	line := text.LineOf(sel.Head)
	if sel.Empty() && sel.Head == text.LineEnd(line) && strings.Trim(text.Line(line), " \t\r") == "-" {
		a.sourceListDraft = &sourceListDraft{line: text.LineStart(line), caret: sel.Head}
	}
}

func (a *app) outlineDraftLine() int {
	draft := a.sourceListDraft
	if draft == nil {
		return -1
	}
	if a.mode != "Source" || a.code == nil || !a.code.Selection().Empty() || a.code.Cursor() != draft.caret {
		a.sourceListDraft = nil
		return -1
	}
	return draft.line
}
