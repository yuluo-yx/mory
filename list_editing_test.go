package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/document"
)

func TestSourceListPrefixOutline(t *testing.T) {
	for _, source := range []string{
		"## 1. Section\n\nBody\n",
		"## 1. Section\n\n- Body\n",
		"## 1. Section\n\n- Parent\n    - Body\n",
		"## 1. Section\n\n- Parent\n    - Body\n    ",
	} {
		t.Run(source, func(t *testing.T) {
			a := testApp(t)
			a.createNote(source, "Stable document")
			a.changeMode("Source")
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, "Source editor")
			tt.Key(ui.Cmd, ui.KeyEnd)
			for _, part := range []string{"-", " ", "Text"} {
				tt.Type(part)
				if len(a.headings) != 1 {
					t.Fatalf("%q caused transient outline: source=%q headings=%+v", part, a.current.Source, a.headings)
				}
			}
		})
	}
}

func TestDraftListOutlineCommitAndCompatibility(t *testing.T) {
	for _, commit := range []string{"newline", "arrow", "selection", "blur", "mode", "reopen", "second dash"} {
		t.Run(commit, func(t *testing.T) {
			a := testApp(t)
			const initial = "## 正式标题\n\n普通段落\n"
			a.createNote(initial, "Draft outline")
			a.changeMode("Source")
			tt := ui.NewTester(a.view, 1280, 900)
			click(t, tt, "Source editor")
			tt.Key(ui.Cmd, ui.KeyEnd)
			tt.Type("-")
			tt.Type(" ")
			if len(a.headings) != 1 || a.current.Source != initial+"- " || !tt.Focused("Source editor") {
				t.Fatal("the unfinished prefix changed content, focus, or outline", a.headings, a.current.Source)
			}
			if !a.save() {
				t.Fatal(a.problem)
			}
			tt.Frame()
			if len(a.headings) != 1 {
				t.Fatal("autosave finalized the incomplete prefix")
			}
			switch commit {
			case "newline":
				tt.Key(0, ui.KeyEnter)
			case "arrow":
				tt.Key(0, ui.KeyLeft)
			case "selection":
				tt.Key(ui.Shift, ui.KeyLeft)
			case "blur":
				click(t, tt, "Document title")
			case "mode":
				a.changeMode("Read")
			case "reopen":
				a.openNote(a.current)
			case "second dash":
				tt.Key(0, ui.KeyBackspace)
				tt.Type("-")
			}
			tt.Frame()
			if !tt.HasText("Heading 普通段落") {
				t.Fatalf("%s failed to restore valid Setext heading: source=%q headings=%+v", commit, a.current.Source, a.headings)
			}
		})
	}
	for _, mode := range []string{"Write", "Source", "Read", "Split"} {
		a := testApp(t)
		const source = "Existing\n-\n\n## Other\n"
		a.createNote(source, "Imported heading")
		a.changeMode(mode)
		tt := ui.NewTester(a.view, 1280, 900)
		if mode != "Split" && !tt.HasText("Heading Existing") {
			t.Fatalf("existing Setext heading missing in %s", mode)
		}
		if !a.save() || a.current.Source != source {
			t.Fatalf("viewing rewrote the source in %s", mode)
		}
	}
}

func TestDraftListOutlineUndoAndFollowingOffsets(t *testing.T) {
	a := testApp(t)
	const source = "## Before\n\n正文\n\n## After"
	a.createNote(source, "Draft undo")
	a.changeMode("Source")
	tt := ui.NewTester(a.view, 1280, 900)
	click(t, tt, "Source editor")
	at := strings.Index(source, "\n\n## After") + 1
	a.code.Select(at, at)
	tt.Type("-")
	tt.Type(" ")
	if len(a.headings) != 2 || a.headings[1].Offset != strings.Index(a.current.Source, "## After") {
		t.Fatalf("draft shifted following heading: %+v", a.headings)
	}
	tt.Type("Item")
	tt.Key(ui.Cmd, ui.KeyZ)
	if len(a.headings) != 2 {
		t.Fatalf("undo reintroduced transient heading: %+v", a.headings)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	if len(a.headings) != 2 {
		t.Fatalf("redo changed the outline: %+v", a.headings)
	}
	want := []string{"Before", "After"}
	got := []string{}
	for _, h := range a.headings {
		got = append(got, h.Text)
	}
	if !slices.Equal(got, want) || !strings.Contains(a.current.Source, "- Item") {
		t.Fatal(got, a.current.Source)
	}
}

func TestListPrefixDoesNotBecomeOutlineHeading(t *testing.T) {
	for _, source := range []string{
		"## 1. Section\n\nBody",
		"## 1. Section\n\n- Body",
		"## 1. Section\n\n- Parent\n    - Body",
		"## 1. Section\n\n- Parent\n    - Body\n\n## 2. Next",
	} {
		for _, structured := range []bool{false, true} {
			for _, softBreak := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/blocks=%t/soft=%t", source, structured, softBreak), func(t *testing.T) {
					a := testApp(t)
					a.createNote(source, "Stable document")
					wantHeadings := len(document.Headings(source))
					label := "Document editor"
					if structured {
						if !a.enableBlocks() {
							t.Fatal(a.problem)
						}
						a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
							if blocks.IsText(n) && n.Plain() == "Body" {
								label = "Block text " + n.ID
							}
						})
					}
					tt := ui.NewTester(a.view, 1280, 1000)
					click(t, tt, label)
					at := strings.Index(a.doc.Text().String(), "Body") + 4
					a.doc.SetSelection(at, at)
					mods := ui.Modifiers(0)
					if softBreak {
						mods = ui.Shift
					}
					tt.Key(mods, ui.KeyEnter)
					for _, part := range []string{"-", " ", "N", "e", "w"} {
						tt.Type(part)
						tt.Frame()
						count := len(a.headings)
						if structured {
							count = len(a.structuredHeadings)
						}
						if count != wantHeadings {
							t.Fatalf("typing %q created a temporary heading: source=%q headings=%+v structured=%+v live=%+v", part, a.current.Source, a.headings, a.structuredHeadings, currentHeadingTitles(a))
						}
					}
				})
			}
		}
	}
}

func TestListEditingKeepsOutlineAndText(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, route := range []string{"typed", "toolbar", "shortcut", "existing"} {
			t.Run(fmt.Sprintf("blocks=%t/%s", structured, route), func(t *testing.T) {
				a := testApp(t)
				source := "# Section\n\nBody\n\n## Next\n\nKeep"
				if route == "existing" {
					source = "# Section\n\n- Body\n\n## Next\n\nKeep"
				}
				a.createNote(source, "Stable document")
				label := "Document editor"
				if structured {
					if !a.enableBlocks() {
						t.Fatal(a.problem)
					}
					a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
						if blocks.IsText(n) && n.Plain() == "Body" {
							label = "Block text " + n.ID
						}
					})
				}
				tt := ui.NewTester(a.view, 1280, 900)
				click(t, tt, label)
				start := strings.Index(a.doc.Text().String(), "Body")
				a.doc.SetSelection(start, start+4)
				check := func(step string) {
					t.Helper()
					tt.Frame()
					if !tt.HasText("Heading Section") || !tt.HasText("Heading Next") || a.current.Title != "Stable document" {
						t.Fatalf("%s changed headings/title: title=%q source=%q rich=%q headings=%+v structured=%+v", step, a.current.Title, a.current.Source, a.doc.Text().String(), a.headings, a.structuredHeadings)
					}
					count := len(a.headings)
					if structured {
						count = len(a.structuredHeadings)
					}
					if count != 2 {
						t.Fatalf("%s created a spurious outline entry: source=%q headings=%+v structured=%+v", step, a.current.Source, a.headings, a.structuredHeadings)
					}
					if !tt.Focused(label) {
						t.Fatalf("%s lost editor focus: source=%q", step, a.current.Source)
					}
				}
				if route == "typed" {
					tt.Type("-")
					check("dash")
					tt.Type(" ")
					check("space")
				} else if route == "toolbar" {
					click(t, tt, "Bullet list")
					check("toolbar")
				} else if route == "shortcut" {
					tt.Key(ui.Cmd|ui.Shift, ui.Key8)
					check("shortcut")
				}
				for _, r := range "First item" {
					tt.Type(string(r))
					check("typing " + string(r))
				}
				if !strings.Contains(a.current.Source, "First item") {
					t.Fatal("list text disappeared", a.current.Source)
				}
			})
		}
	}
}
