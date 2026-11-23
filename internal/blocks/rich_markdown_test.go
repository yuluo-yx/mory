package blocks

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/richtext"
)

func TestRichMarkdownIndentRoundTrip(t *testing.T) {
	for _, source := range []string{"", "# Heading\n\nBody", "- Parent\n    - Child", "1. First\n2. Second", "```go\ncode\n```"} {
		text := richtext.ParseMarkdown(source)
		if got := RichMarkdown(text); got != text.Markdown() {
			t.Fatalf("ordinary Markdown changed: %q", got)
		}
	}
	for _, source := range []string{"Text **bold**", "## Heading", "> Quote", ""} {
		doc := richtext.NewDocument(richtext.ParseMarkdown(source))
		doc.Indent(2)
		encoded := RichMarkdown(doc.Text())
		root, err := ParseMarkdown(encoded)
		if err != nil {
			t.Fatal(err)
		}
		var paragraph *Node
		root.Walk(func(n *Node) {
			if IsText(n) {
				paragraph = n
			}
		})
		if paragraph == nil {
			t.Fatalf("paragraph was lost: %q", encoded)
		}
		restored := ToRich(paragraph, RichOptions{})
		if restored.ParagraphStyleAt(0).Indent != 2 || restored.String() != doc.Text().String() {
			t.Fatalf("indent round trip failed: %q", encoded)
		}
		outdent := richtext.NewDocument(restored)
		outdent.Indent(-2)
		updated := FromRich(outdent.Text(), paragraph)[0]
		if updated.Attr("indent") != "" {
			t.Fatal("outdent retained a stale attribute")
		}
	}
	text := richtext.TextOf(
		richtext.Paragraph{Runs: []richtext.Run{{Text: "Before"}}},
		richtext.Paragraph{Style: richtext.ParagraphStyle{Indent: 1}, Runs: []richtext.Run{{Text: "Middle"}}},
		richtext.Paragraph{Runs: []richtext.Run{{Text: "After"}}},
	)
	encoded := RichMarkdown(text)
	if !strings.HasPrefix(encoded, "Before\n\n<p ") || !strings.HasSuffix(encoded, "</p>\n\nAfter") {
		t.Fatal(encoded)
	}
	root, err := ParseMarkdown(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	root.Walk(func(n *Node) {
		if IsText(n) {
			count++
		}
	})
	if count != 3 {
		t.Fatalf("mixed Markdown merged paragraphs: %q", encoded)
	}
}

func TestEmptyParagraphMarkdownRoundTrip(t *testing.T) {
	for _, count := range []int{1, 3} {
		for _, position := range []string{"before", "between", "after"} {
			paragraphs := []richtext.Paragraph{{Runs: []richtext.Run{{Text: "Before"}}}}
			if position == "before" {
				paragraphs = nil
			}
			for range count {
				paragraphs = append(paragraphs, richtext.Paragraph{})
			}
			if position != "after" {
				paragraphs = append(paragraphs, richtext.Paragraph{Runs: []richtext.Run{{Text: "After"}}})
			}
			encoded := RichMarkdown(richtext.TextOf(paragraphs...))
			root, err := ParseMarkdown(encoded)
			if err != nil {
				t.Fatal(err)
			}
			empty := 0
			root.Walk(func(n *Node) {
				if n.Kind == "p" && n.Plain() == "" {
					empty++
					if got := FromRich(ToRich(n, RichOptions{}), n)[0].Markdown(); got != "<p/>\n\n" {
						t.Fatalf("edited empty paragraph was lost: %q", got)
					}
				}
			})
			if empty != count {
				t.Fatalf("%s: got %d blanks, want %d in %q", position, empty, count, encoded)
			}
		}
	}
}
