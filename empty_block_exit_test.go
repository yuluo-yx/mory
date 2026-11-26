package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestEmptyBlockDeleteBecomesWritableParagraph(t *testing.T) {
	for _, tc := range []struct{ name, xml, markdown string }{
		{"quote", "<blockquote><p/></blockquote>", ">\n"},
		{"code", "<pre lang=\"go\"><code/></pre>", "```go\n```\n"},
		{"callout", "<callout alert=\"NOTE\"><p/></callout>", ":::note\n:::\n"},
		{"bullet", "<ul><li/></ul>", "-\n"},
		{"numbered", "<ol><li/></ol>", "1.\n"},
		{"task", "<checkbox/>", "<checkbox/>\n"},
		{"heading", "<h2/>", "##\n"},
	} {
		for _, format := range []string{"xml", "markdown"} {
			for _, key := range []ui.Key{ui.KeyBackspace, ui.KeyDelete} {
				for _, surround := range []bool{false, true} {
					if tc.name == "heading" && surround {
						continue // Existing blank-heading tests cover adjacent removal.
					}
					t.Run(fmt.Sprintf("%s/%s/%v/surround=%t", tc.name, format, key, surround), func(t *testing.T) {
						a := testApp(t)
						source := tc.xml
						if surround {
							source = "<ul><li>Water Cube</li><li>National Stadium</li></ul>" + source + "<p>Following</p>"
						}
						if format == "markdown" {
							source = tc.markdown
							if surround {
								source = "- Water Cube\n- National Stadium\n\n" + source + "\nFollowing\n"
							}
						}
						n, err := a.lib.Create("Empty deletion", source, format)
						if err != nil {
							t.Fatal(err)
						}
						a.openNote(n)
						if !a.enableBlocks() {
							t.Fatal(a.problem)
						}
						tt := ui.NewTester(a.view, 1280, 900)
						var target *blocks.Node
						a.blockEditor.doc.Root.Walk(func(node *blocks.Node) {
							if (blocks.IsText(node) || node.Kind == "pre") && node.Plain() == "" {
								target = node
							}
						})
						if target == nil {
							t.Fatal("empty field is missing", source, a.blockEditor.doc.Root.XML())
						}
						label := "Block text " + target.ID
						if target.Kind == "pre" {
							label = "Code editor"
						}
						click(t, tt, label)
						tt.Key(0, key)
						a.capture()
						kept := a.blockEditor.doc.Find(target.ID)
						parent, _ := a.blockEditor.doc.Parent(target.ID)
						if kept == nil || kept.Kind != "p" || parent != a.blockEditor.doc.Root || !tt.Focused("Block text "+target.ID) {
							t.Fatalf("empty %s did not become focused body text: source=%q", tc.name, a.current.Source)
						}
						clean := a.current.Source
						tt.Key(ui.Cmd, ui.KeyZ)
						if a.current.Source != source {
							t.Fatal("single undo did not restore exact source", a.current.Source)
						}
						tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
						if a.current.Source != clean || !tt.Focused("Block text "+target.ID) {
							t.Fatal("redo lost deletion or the typing caret")
						}
						tt.Type("Continue")
						tt.Type(" writing")
						a.capture()
						if !strings.Contains(a.current.Source, "Continue writing") || !a.save() {
							t.Fatal("continued typing failed", a.problem)
						}
						stored, err := library.Open(a.lib.Dir)
						if err != nil || stored.Get(n.ID).Source != a.current.Source {
							t.Fatal("saved source differs", err)
						}
						a.loadEditors()
						tt.Frame()
						if !surround && strings.TrimSpace(a.current.Source) != "Continue writing" && format == "markdown" {
							t.Fatal("deleting the empty block left Markdown structure behind", a.current.Source)
						}
						if surround && (!strings.Contains(a.current.Source, "Water Cube") || !strings.Contains(a.current.Source, "National Stadium") || !strings.Contains(a.current.Source, "Following")) {
							t.Fatal("deletion damaged surrounding content")
						}
					})
				}
			}
		}
	}
}

func TestEmptyBlockDeletionPreservesContainerBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source, targetKind, parentKind string }{
		{"nested quote", `<callout><p>Keep outer</p><blockquote><p/></blockquote></callout>`, "p", "callout"},
		{"nested code", `<blockquote><p>Keep outer</p><pre><code/></pre></blockquote>`, "pre", "blockquote"},
		{"code in cell", `<table><tr><td><p>Keep cell</p><pre><code/></pre></td></tr></table>`, "pre", "td"},
		{"nested callout", `<callout><p>Keep outer</p><callout><p/></callout></callout>`, "p", "callout"},
		{"task list", `<ul><checkbox>Keep before</checkbox><checkbox/><checkbox>Keep after</checkbox></ul>`, "checkbox", "root"},
	} {
		for _, key := range []ui.Key{ui.KeyBackspace, ui.KeyDelete} {
			t.Run(fmt.Sprintf("%s/%v", tc.name, key), func(t *testing.T) {
				a, tt := xmlApp(t, tc.source)
				var target *blocks.Node
				a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
					if n.Kind == tc.targetKind && n.Plain() == "" {
						target = n
					}
				})
				label := "Block text " + target.ID
				if target.Kind == "pre" {
					label = "Code editor"
				}
				click(t, tt, label)
				tt.Key(0, key)
				tt.Type("Continue")
				a.capture()
				parent, _ := a.blockEditor.doc.Parent(target.ID)
				if parent == nil || parent.Kind != tc.parentKind || a.blockEditor.doc.Find(target.ID).Plain() != "Continue" || !strings.Contains(a.current.Source, "Keep") {
					t.Fatal("empty-block exit crossed its owning container", a.current.Source)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				tt.Key(ui.Cmd, ui.KeyZ)
				if a.current.Source != tc.source {
					t.Fatal("undo lost container boundaries", a.current.Source)
				}
			})
		}
	}
}

func TestDeletionOfTextLeavesBlockUntilNextKey(t *testing.T) {
	for _, kind := range []string{"blockquote", "callout", "pre"} {
		for _, key := range []ui.Key{ui.KeyBackspace, ui.KeyDelete} {
			t.Run(fmt.Sprintf("%s/%v", kind, key), func(t *testing.T) {
				body := "<p>Authored 世界</p>"
				if kind == "pre" {
					body = "<code>Authored 世界</code>"
				}
				source := "<" + kind + ">" + body + "</" + kind + "><p>Keep</p>"
				a, tt := xmlApp(t, source)
				n := firstBlock(a, kind)
				label := "Code editor"
				if kind != "pre" {
					label = "Block text " + n.Children[0].ID
				}
				click(t, tt, label)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Key(0, key)
				a.capture()
				if firstBlock(a, kind) == nil || firstBlock(a, kind).Plain() != "" || !strings.Contains(a.current.Source, "Keep") {
					t.Fatal("deleting selected text also deleted its block", a.current.Source)
				}
				tt.Key(0, key)
				tt.Type("Outside")
				a.capture()
				if firstBlock(a, kind) != nil || !strings.Contains(a.current.Source, "<p>Outside</p>") {
					t.Fatal("second deletion failed to exit the empty block", a.current.Source)
				}
			})
		}
	}
}

func TestEmptyBlockDeleteProtectsAuthoredAndOpaqueContent(t *testing.T) {
	for _, source := range []string{
		`<callout title="Keep title"><p/></callout>`,
		`<pre caption="Keep caption"><code/></pre>`,
		`<pre><code/><custom value="Keep"/></pre>`,
		`<blockquote><p/><img href="missing.png"/></blockquote>`,
		`<blockquote><p/><custom value="Keep"/></blockquote>`,
		`<table><tr><td><p/></td></tr></table>`,
	} {
		for _, key := range []ui.Key{ui.KeyBackspace, ui.KeyDelete} {
			t.Run(fmt.Sprintf("%s/%v", source, key), func(t *testing.T) {
				a, tt := xmlApp(t, source)
				var target *blocks.Node
				a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
					if n.Kind == "p" || n.Kind == "pre" {
						target = n
					}
				})
				label := "Block text " + target.ID
				if target.Kind == "pre" {
					label = "Code editor"
				}
				click(t, tt, label)
				tt.Key(0, key)
				a.capture()
				if a.current.Source != source {
					t.Fatal("deletion dropped protected block content", a.current.Source)
				}
			})
		}
	}
}

func TestReopenedInsertedEmptyBlocksRemainDeletable(t *testing.T) {
	for _, kind := range []string{"Quote", "Code block", "Callout", "Bullet list", "To-do list"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			a.createNote("", "Reopened empty block")
			a.insertEmptyBlock(kind)
			if !a.save() {
				t.Fatal(a.problem)
			}
			a.loadEditors()
			if a.blockEditor == nil && !a.enableBlocks() {
				t.Fatal(a.problem)
			}
			tt := ui.NewTester(a.view, 1280, 900)
			var target *blocks.Node
			a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
				if (blocks.IsText(n) || n.Kind == "pre") && n.Plain() == "" {
					target = n
				}
			})
			if target == nil {
				t.Fatalf("reopened empty block gained nonempty content: %q tree=%s", a.current.Source, a.blockEditor.doc.Root.XML())
			}
			if kind == "Callout" || kind == "Quote" {
				parent, _ := a.blockEditor.doc.Parent(target.ID)
				outer, _ := a.blockEditor.doc.Parent(parent.ID)
				if outer != a.blockEditor.doc.Root {
					t.Fatal("reopening introduced an extra wrapper", a.blockEditor.doc.Root.XML())
				}
			}
			label := "Block text " + target.ID
			if target.Kind == "pre" {
				label = "Code editor"
			}
			click(t, tt, label)
			tt.Key(0, ui.KeyBackspace)
			tt.Type("Body")
			a.capture()
			n := a.blockEditor.doc.Find(target.ID)
			if n == nil || n.Kind != "p" || n.Plain() != "Body" {
				t.Fatal("reopened empty block could not be exited", a.current.Source)
			}
		})
	}
}

func TestWhitespaceOnlyCodeDeleteAndUndo(t *testing.T) {
	for _, whitespace := range []string{"\n", " \t\n\n"} {
		for _, key := range []ui.Key{ui.KeyBackspace, ui.KeyDelete} {
			t.Run(fmt.Sprintf("%q/%v", whitespace, key), func(t *testing.T) {
				a := testApp(t)
				source := "```go\n" + whitespace + "```\n"
				a.createNote(source, "Whitespace-only code")
				tt := ui.NewTester(a.view, 1280, 900)
				click(t, tt, "Code editor")
				tt.Key(0, key)
				a.capture()
				if firstBlock(a, "pre") != nil {
					t.Fatal("blank code from an older saved document remained trapped")
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				if a.current.Source != source {
					t.Fatal("undo did not preserve the original whitespace", a.current.Source)
				}
			})
		}
	}
}

func TestEmptyMarkdownDeletionUsesReopenedEditor(t *testing.T) {
	for _, source := range []string{"-\n", "1.\n", "##\n"} {
		for _, key := range []ui.Key{ui.KeyBackspace, ui.KeyDelete} {
			t.Run(fmt.Sprintf("%q/%v", source, key), func(t *testing.T) {
				a := testApp(t)
				a.createNote(source, "Empty Markdown")
				a.loadEditors()
				tt := ui.NewTester(a.view, 1280, 900)
				if a.blockEditor != nil {
					t.Fatal("fixture did not exercise the ordinary rich editor")
				}
				original := blocks.RichMarkdown(a.doc.Text())
				click(t, tt, "Document editor")
				tt.Key(0, key)
				a.capture()
				if a.current.Source == source || !tt.Focused("Document editor") {
					t.Fatal("deletion failed to exit empty Markdown", a.current.Source)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				a.capture()
				if a.current.Source != original {
					t.Fatal("undo changed original Markdown", a.current.Source)
				}
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				tt.Type("Ordinary body")
				a.capture()
				if strings.TrimSpace(a.current.Source) != "Ordinary body" {
					t.Fatal("continued typing retained the exited format", a.current.Source)
				}
				if !a.save() {
					t.Fatal(a.problem)
				}
				a.loadEditors()
				tt.Frame()
				if a.doc.Text().String() != "Ordinary body" {
					t.Fatal("reopening lost continued text")
				}
			})
		}
	}
}

func TestEmptyBlockDeletionRespectsReadOnlyAndCodeSelections(t *testing.T) {
	for _, source := range []string{"<blockquote><p/></blockquote>", "<callout><p/></callout>", "<pre><code/></pre>"} {
		t.Run(source, func(t *testing.T) {
			a, tt := xmlApp(t, source)
			a.changeMode("Read")
			tt.Frame()
			label := "Code editor"
			if n := firstBlock(a, "p"); n != nil {
				label = "Block text " + n.ID
			}
			click(t, tt, label)
			tt.Key(0, ui.KeyBackspace)
			tt.Key(0, ui.KeyDelete)
			a.capture()
			if a.current.Source != source {
				t.Fatal("read-only deletion changed source", a.current.Source)
			}
		})
	}
	a, tt := xmlApp(t, "<pre><code>\n</code></pre>")
	n := firstBlock(a, "pre")
	click(t, tt, "Code editor")
	f := a.blockEditor.fields[n.ID]
	f.code.SetSelections(editor.Selection{Anchor: 0, Head: 0}, editor.Selection{Anchor: 1, Head: 1})
	if a.exitEmptyBlock(n.ID) {
		t.Fatal("multiple carets unexpectedly removed the code block")
	}
	f.code.Select(0, 1)
	if a.exitEmptyBlock(n.ID) {
		t.Fatal("selected whitespace unexpectedly removed its enclosing block")
	}
	for _, mods := range []ui.Modifiers{ui.Cmd, ui.Ctrl, ui.Alt, ui.Shift} {
		if isBlockDeleteKey(ui.KeyBackspace, mods) || isBlockDeleteKey(ui.KeyDelete, mods) {
			t.Fatal("modified deletion became an empty-block exit", mods)
		}
	}
}
