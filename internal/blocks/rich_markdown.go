package blocks

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
)

// RichMarkdown uses the existing XML-in-Markdown representation for empty and
// indented paragraphs, which CommonMark otherwise discards or treats as code.
// Keep ordinary paragraphs together so list numbering and code fences survive.
func RichMarkdown(text richtext.Text) string {
	var parts []string
	var pending []richtext.Paragraph
	flush := func() {
		if len(pending) > 0 {
			parts = append(parts, richtext.TextOf(pending...).Markdown())
			pending = nil
		}
	}
	for _, p := range text.Paragraphs() {
		empty := len(p.Runs) == 0 && p.Style.Heading == 0 && !p.Style.Quote && text.Len() > 0
		if (p.Style.Indent > 0 || empty) && p.Style.List == richtext.NoList && !p.Style.CodeBlock {
			flush()
			for _, node := range FromRich(richtext.TextOf(p), nil) {
				parts = append(parts, node.XML())
			}
		} else {
			pending = append(pending, p)
		}
	}
	flush()
	return strings.Join(parts, "\n\n")
}
