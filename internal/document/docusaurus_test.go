package document

import "testing"

func TestDocusaurusOutlineAndRichSafety(t *testing.T) {
	for _, source := range []string{":::tip\nBody\n:::\n", "# Heading {#anchor}", "import X from './x';", "export const x = 1;", "---\ntitle: Test\n---\nBody", "```js title=\"demo\"\nx\n```", "```js\n// highlight-next-line\nx\n```"} {
		if RichSafe(source) {
			t.Fatalf("extension would be flattened: %q", source)
		}
	}
	for _, source := range []string{"---\ntitle: Test\n---\n# Heading {#anchor}\n", "---\r\ntitle: Test\r\n...\r\n# Heading {#anchor}\r\n", "---\ntitle: Unclosed", "# Heading {#anchor}"} {
		h := Headings(source)
		if len(h) > 0 && (h[0].Text != "Heading" || source[h[0].Offset] != '#') {
			t.Fatal(h)
		}
	}
}
