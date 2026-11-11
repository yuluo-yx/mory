package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func TestLargeStructuredDocumentDefersRichTree(t *testing.T) {
	a := testApp(t)
	rows := strings.Repeat("| value | value |\n", 32000)
	n, err := a.lib.Create("large", "| a | b |\n| --- | --- |\n"+rows, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	a.current = n
	a.loadEditors()
	if a.blockEditor == nil {
		t.Fatal("large structured document did not create the virtualized block editor")
	}
	if a.doc != nil {
		t.Fatal("large structured document eagerly built a duplicate rich-text tree")
	}
	if a.mode != "Write" {
		t.Fatalf("large structured document opened in %q mode, want Write", a.mode)
	}
}

func TestLargeRichSafeDocumentKeepsWriteModeWithoutRichTree(t *testing.T) {
	a := testApp(t)
	paragraph := strings.Repeat("A responsive paragraph.\n\n", 30000)
	n, err := a.lib.Create("large-rich-safe", paragraph, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	a.current = n
	a.loadEditors()
	if a.mode != "Write" {
		t.Fatalf("large rich-safe document opened in %q mode, want Write", a.mode)
	}
	if a.blockEditor == nil {
		t.Fatal("large rich-safe document did not use the virtualized block editor")
	}
	if a.doc != nil {
		t.Fatal("large rich-safe document eagerly built a rich-text tree")
	}
}

func TestTableLayoutCachesUnchangedTable(t *testing.T) {
	a := testApp(t)
	a.createNote("| a | b |\n| --- | --- |\n| 1 | 2 |\n", "table")
	first, simple := a.tableLayout(a.blockEditor.doc.Root.Children[0])
	second, secondSimple := a.tableLayout(a.blockEditor.doc.Root.Children[0])
	if !simple || !secondSimple {
		t.Fatal("ordinary table was not recognized as simple")
	}
	if first != second {
		t.Fatal("unchanged table layout was rebuilt")
	}
}

func TestLargeTableUsesCompactFallback(t *testing.T) {
	a := testApp(t)
	a.createNote("| a | b |\n| --- | --- |\n"+strings.Repeat("| value | value |\n", 220), "table")
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("Large table") || !tt.HasText("value") {
		t.Fatal("large table did not render the compact fallback")
	}
}

func TestLargeDocumentWriteModeUsesCompactFallback(t *testing.T) {
	a := testApp(t)
	source := "| a | b |\n| --- | --- |\n" + strings.Repeat("| value | value |\n", 32000)
	n, err := a.lib.Create("large-write", source, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	a.current = n
	a.mode = "Write"
	a.loadEditors()
	a.changeMode("Write")
	if a.blockEditor == nil {
		t.Fatal("switching to Write did not create the block editor")
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("Large table") {
		t.Fatal("large document Write mode did not use the compact table fallback")
	}
}

func TestBlockFieldsAreBoundedWithoutDroppingDirtyState(t *testing.T) {
	s := &blockSession{fields: map[string]*blockField{}, active: "active", focus: "focus"}
	for i := 0; i < 300; i++ {
		doc := richtext.NewDocument(richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "stable"}}}))
		s.fields[string(rune(i+1))] = &blockField{doc: doc, version: doc.Version(), lastUsed: uint64(i)}
	}
	dirty := richtext.NewDocument(richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "dirty"}}}))
	dirty.InsertText("!")
	s.fields["dirty"] = &blockField{doc: dirty, version: dirty.Version() - 1, lastUsed: 0}
	s.fields["active"] = &blockField{}
	s.fields["focus"] = &blockField{}
	s.pruneFields(64)
	if len(s.fields) > 64+3 {
		t.Fatalf("field cache grew to %d entries", len(s.fields))
	}
	if s.fields["active"] == nil || s.fields["focus"] == nil || s.fields["dirty"] == nil {
		t.Fatal("pruning dropped active, focused, or dirty field")
	}
}

func TestVisibleBlocksCacheTracksDocumentAndFoldRevisions(t *testing.T) {
	s, err := newBlockSession("# One\n\nA\n\n## Two\n\nB\n", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	first := s.visibleBlocks()
	second := s.visibleBlocks()
	if &first[0] != &second[0] {
		t.Fatal("visible block cache was rebuilt without a revision change")
	}
	heading := s.doc.Root.Children[0]
	s.foldedHeadings = map[string]bool{heading.ID: true}
	s.foldVersion++
	third := s.visibleBlocks()
	if len(third) >= len(first) {
		t.Fatal("fold revision did not invalidate visible block cache")
	}
}
