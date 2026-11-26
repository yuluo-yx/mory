package blocks

import (
	"strings"
	"testing"
)

func TestEmptyBlockMarkdownRoundTrip(t *testing.T) {
	for _, source := range []string{
		`<pre lang="go"><code/></pre>`,
		`<pre lang="go" code-meta="showLineNumbers"><code/></pre>`,
		`<pre lang="go" background-color="light-yellow"><code/></pre>`,
		`<pre lang="go" wrap="true" caption="Keep caption"><code/></pre>`,
		`<pre><code/><custom value="Keep"/></pre>`,
		`<callout alert="NOTE" emoji="💡"><p/></callout>`,
		`<checkbox done="true"/>`,
	} {
		t.Run(source, func(t *testing.T) {
			root, err := ParseXML(source)
			if err != nil {
				t.Fatal(err)
			}
			n := Visible(root.Children)[0]
			encoded := root.Markdown()
			reopened, err := ParseMarkdown(encoded)
			if err != nil {
				t.Fatal(err)
			}
			children := Visible(reopened.Children)
			if len(children) != 1 || children[0].Kind != n.Kind || strings.TrimSpace(children[0].Plain()) != "" {
				t.Fatalf("empty block gained content or lost its structure: %q tree=%s", encoded, reopened.XML())
			}
			if n.Kind == "pre" && children[0].Plain() != "" {
				t.Fatal("serialization manufactured a blank code line", encoded)
			}
			for key, value := range n.Attrs {
				if got := children[0].Attr(key); got != value {
					t.Fatalf("attribute %s changed from %q to %q in %q", key, value, got, encoded)
				}
			}
			if n.Kind == "pre" && len(n.Children) > 1 && children[0].XML() != n.XML() {
				t.Fatal("serialization dropped opaque code content", encoded)
			}
			if reopened.Markdown() != encoded {
				t.Fatal("reopening rewrote the saved empty block")
			}
		})
	}
}

func TestEmptyMarkdownContainerOwnsItsMarker(t *testing.T) {
	for _, marker := range []string{">", "> ", ">\n>", "  >", "-", "1."} {
		for _, suffix := range []string{"", "\n", "\r\n\r\nFollowing"} {
			source := marker + suffix
			t.Run(source, func(t *testing.T) {
				d, err := Parse(source, "markdown")
				if err != nil {
					t.Fatal(err)
				}
				if d.Source() != source {
					t.Fatal("opening changed source", d.Source())
				}
				for _, n := range d.Root.Children {
					if n.Kind == "#raw" && strings.TrimSpace(n.Text) != "" {
						t.Fatalf("container marker escaped into raw text: %q tree=%s", n.Text, d.Root.XML())
					}
				}
				n := Visible(d.Root.Children)[0]
				if err := d.Remove(n.ID); err != nil {
					t.Fatal(err)
				}
				want := ""
				if strings.Contains(suffix, "Following") {
					want = "Following"
				}
				if strings.TrimSpace(d.Source()) != want {
					t.Fatal("removing container left its marker or changed adjacent content", d.Source())
				}
				if !d.Undo() || d.Source() != source {
					t.Fatal("undo lost original marker spelling", d.Source())
				}
			})
		}
	}
}
