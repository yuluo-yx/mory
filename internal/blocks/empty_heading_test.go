package blocks

import (
	"strings"
	"testing"
)

func TestEmptyMarkdownHeadingOwnsItsMarker(t *testing.T) {
	for _, marker := range []string{"#", "##", "### ", "  ####", "## ##", "######\t"} {
		for _, suffix := range []string{"", "\n", "\r\n\r\nBody"} {
			source := "<p/>\n\n" + marker + suffix
			d, err := Parse(source, "markdown")
			if err != nil {
				t.Fatal(err)
			}
			if d.Source() != source {
				t.Fatalf("round trip changed source: %q", d.Source())
			}
			var heading *Node
			for _, n := range d.Root.Children {
				if strings.HasPrefix(n.Kind, "h") {
					heading = n
				}
				if n.Kind == "#raw" && strings.TrimSpace(n.Text) != "" {
					t.Fatalf("heading marker escaped into raw text: %q from %q", n.Text, source)
				}
			}
			if heading == nil {
				t.Fatal("missing empty heading")
			}
			if err := d.Remove(heading.ID); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(d.Source(), "#") {
				t.Fatalf("deleted heading left marker behind: %q", d.Source())
			}
			if !d.Undo() || d.Source() != source {
				t.Fatal("undo did not restore exact heading")
			}
		}
	}
}
