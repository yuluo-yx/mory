package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func TestEmptyEditorFormattingDoesNotOverlapPlaceholder(t *testing.T) {
	for _, label := range []string{"Numbered list", "Bullet list", "Task list", "Quote"} {
		t.Run(label, func(t *testing.T) {
			a := testApp(t)
			a.lib.Settings.Language = "zh-CN"
			a.createNote("", "Test")
			tt := ui.NewTester(a.view, 1280, 820)
			click(t, tt, a.tr("Document editor"))
			click(t, tt, a.tr(label))
			tt.Frame()
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, label+".png"))
			}
			editor, ok := tt.Access(a.tr("Document editor"))
			if !ok || editor.Placeholder != "" {
				t.Fatalf("formatted empty paragraph still paints an unindented placeholder: %+v", editor)
			}
			click(t, tt, a.tr(label))
			editor, _ = tt.Access(a.tr("Document editor"))
			if editor.Placeholder != a.tr("Start writing…") {
				t.Fatal("returning to plain body did not restore the placeholder")
			}
		})
	}
}

func TestNumberedHeadingShortcut(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, prefix := range []string{"1.", "12)", "123."} {
			a := testApp(t)
			a.createNote("", "Test")
			label := "Document editor"
			if structured {
				a.createNote("Body", "Test")
				if !a.enableBlocks() {
					t.Fatal(a.problem)
				}
				label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
			}
			tt := ui.NewTester(a.view, 1280, 820)
			click(t, tt, label)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyBackspace)
			for _, r := range "## " {
				tt.Type(string(r))
			}
			if a.doc.Formatting().Paragraph.Heading != 2 {
				t.Fatal("heading shortcut did not create H2")
			}
			if !structured {
				ed, _ := tt.Access(a.tr(label))
				if ed.Placeholder != "" {
					t.Fatal("empty heading still shows the body placeholder")
				}
			}
			for _, r := range prefix + " " {
				tt.Type(string(r))
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			// Structured editing groups this typing session into a block change;
			// the plain editor records the protected space as its own step.
			wantUndo, wantHeading := prefix, 2
			if structured {
				wantUndo, wantHeading = "Body", 0
			}
			if a.doc.Text().String() != wantUndo || a.doc.Formatting().Paragraph.Heading != wantHeading {
				t.Fatalf("undo mismatch (structured=%t, prefix=%q): text=%q style=%+v", structured, prefix, a.doc.Text().String(), a.doc.Formatting().Paragraph)
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			if a.doc.Text().String() != prefix+" " || a.doc.Formatting().Paragraph.Heading != 2 {
				t.Fatal("redo restored the unwanted list conversion")
			}
			tt.Key(ui.Cmd, ui.KeyEnd)
			for _, r := range "Overview **bold**" {
				tt.Type(string(r))
			}
			style := a.doc.Formatting().Paragraph
			if style.Heading != 2 || style.List != richtext.NoList || a.doc.Text().String() != prefix+" Overview bold" {
				t.Fatalf("numbered heading changed structure: %+v %q", style, a.doc.Text().String())
			}
			if a.doc.Text().AttributesAt(len(prefix)+10).Weight != 700 {
				t.Fatal("inline bold shortcut stopped working")
			}
			a.capture()
			if !a.save() {
				t.Fatal(a.problem)
			}
			parsed := a.richText(a.current.Source)
			if parsed.ParagraphStyleAt(0).Heading != 2 || parsed.String() != prefix+" Overview bold" {
				t.Fatalf("saved heading lost formatting or text: %q", a.current.Source)
			}
			a.loadEditors()
			if a.doc.Text().ParagraphStyleAt(0).Heading != 2 || a.doc.Text().String() != prefix+" Overview bold" {
				t.Fatal("saved heading changed after reopening")
			}
		}
	}
}

func TestPlainNumberedListShortcutStillWorks(t *testing.T) {
	a := testApp(t)
	a.createNote("", "List")
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Document editor")
	for _, r := range "1. Item" {
		tt.Type(string(r))
	}
	if a.doc.Formatting().Paragraph.List != richtext.Numbers || a.doc.Text().String() != "Item" {
		t.Fatal("body list shortcut stopped working")
	}
}
