package blocks

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/richtext"
)

func TestRichCodeParagraphsBecomeOneBlock(t *testing.T) {
	text := richtext.TextOf(
		richtext.Paragraph{Runs: []richtext.Run{{Text: "Before"}}},
		richtext.Paragraph{Style: richtext.ParagraphStyle{CodeBlock: true}, Runs: []richtext.Run{{Text: "package main"}}},
		richtext.Paragraph{Style: richtext.ParagraphStyle{CodeBlock: true}},
		richtext.Paragraph{Style: richtext.ParagraphStyle{CodeBlock: true}, Runs: []richtext.Run{{Text: "func main() {}"}}},
		richtext.Paragraph{Runs: []richtext.Run{{Text: "After"}}},
	)
	nodes := FromRich(text, nil)
	if len(nodes) != 3 || nodes[0].Plain() != "Before" || nodes[2].Plain() != "After" || nodes[1].Kind != "pre" || nodes[1].Plain() != "package main\n\nfunc main() {}" {
		t.Fatalf("code conversion duplicated or lost paragraphs: %s", New("root", nodes...).XML())
	}
	markdown := New("root", nodes...).Markdown()
	if strings.Count(markdown, "package main") != 1 || strings.Count(markdown, "```") != 2 {
		t.Fatal(markdown)
	}
}
