package document

import (
	"strings"
	"testing"
)

func TestRichSafety(t *testing.T) {
	for _, s := range []string{"# Heading\n\n**bold** *italic*", "- [ ] Task\n- [x] Done"} {
		if !RichSafe(s) {
			t.Errorf("safe source rejected %q", s)
		}
	}
	for _, s := range []string{"```go\nfunc main() {}\n```", "```\nplain code\n```", "    indented code\n", "| A | B |\n| --- | --- |\n| a | b |", "---\n", "$$\nx\n$$", "<div>content</div>", "[x]: https://example.com", "[^1]: footnote", "$x$", "> [!NOTE]\n> Text"} {
		if RichSafe(s) {
			t.Errorf("unsafe source accepted %q", s)
		}
	}
}

func TestHeadingDisplayTextAndSourceOffsets(t *testing.T) {
	source := "---\ntitle: Outline\n---\n\n## 1\\. **Physical** &amp; [software](https://example.com)\n\n### 1\\.1 `IDC` facilities {#idc}\n\n## C#\n\n```md\n# Hidden\n```\n"
	want := []string{"1. Physical & software", "1.1 IDC facilities", "C#"}
	heads := Headings(source)
	if len(heads) != len(want) {
		t.Fatalf("unexpected headings: %+v", heads)
	}
	for i, h := range heads {
		if h.Text != want[i] || !strings.HasPrefix(source[h.Offset:], strings.Repeat("#", h.Level)+" ") {
			t.Fatalf("heading text or source offset changed: %+v", h)
		}
	}
}

func TestXMLProjection(t *testing.T) {
	source := `<title icon="x">A &amp; B</title><h1>Hello</h1><h9>Deep</h9><p><b>bold</b> <em>italic</em> <u>under</u> <del>gone</del> <a href="https://example.com">link</a> <code>code</code><br/>next <latex>x^2</latex></p><ul><li>one</li><li>two</li></ul><ol><li>first</li></ol><checkbox done="true">yes</checkbox><checkbox done="false">no</checkbox><blockquote><p>quote</p></blockquote><callout><p>remember</p></callout><pre lang="go"><code>fmt.Println("hi")</code></pre><hr/><img href="assets/a.png" caption="a"/><img src="b.png"/><img path="c.png"/><bookmark href="https://example.com" title="site"/><file path="f.txt" name="file"/><grid><column><p>left</p></column><column><p>right</p></column></grid><table><tr><th>A</th><th>B</th></tr><tr><td>cell</td></tr></table><sheet/><whiteboard/><unknown>kept</unknown>`
	title, md, err := FromXML(source)
	if err != nil {
		t.Fatal(err)
	}
	if title != "A & B" {
		t.Fatal(title)
	}
	for _, want := range []string{"# Hello", "###### Deep", "**bold**", "*italic*", "<u>under</u>", "~~gone~~", "[link](https://example.com)", "`code`", "$x^2$", "- one", "1. first", "- [x] yes", "- [ ] no", "> quote", "> [!NOTE]", "```go", "![a](assets/a.png)", "![](b.png)", "![](c.png)", "[file](f.txt)", "| A | B |", "left", "right", "[sheet]", "kept"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in %s", want, md)
		}
	}
	if _, _, err = FromXML("<p>broken"); err == nil {
		t.Fatal("malformed XML")
	}
	if _, md, err = FromXML("<table/>"); err != nil || strings.TrimSpace(md) != "" {
		t.Fatal(md, err)
	}
	_, md, err = FromXML("<pre><code>```</code></pre>")
	if err != nil || !strings.Contains(md, "````") {
		t.Fatal("fence collision")
	}
}

func TestExports(t *testing.T) {
	md := "# Title\n\nText **bold**\n\n![image](assets/image.png)\n\n---\n"
	xml := ToXML("A & B", md)
	if err := Validate(xml, "xml"); err != nil {
		t.Fatal(xml, err)
	}
	title, out, err := FromXML(xml)
	if err != nil || title != "A & B" || !strings.Contains(out, "**bold**") {
		t.Fatal(title, out, err)
	}
	html := HTML("<script>title</script>", md)
	if !strings.Contains(html, "&lt;script&gt;title") || !strings.Contains(html, "Content-Security-Policy") {
		t.Fatal("unsafe HTML")
	}
}

func TestUtilities(t *testing.T) {
	heads := Headings("# One\n\nText\n\n## Two\n\n```\n# not heading\n```\n")
	if len(heads) != 2 || heads[1].Level != 2 || heads[1].Text != "Two" {
		t.Fatal(heads)
	}
	words, chars := Counts("Hello 世界 🌱")
	if words != 4 || chars != 10 {
		t.Fatal(words, chars)
	}
	for _, s := range []string{"", ".", ".."} {
		if Filename(s) != "Untitled" {
			t.Fatal(s)
		}
	}
	if Filename("a/b:c") != "a-b-c" {
		t.Fatal("filename")
	}
	if len([]rune(Filename(strings.Repeat("a", 100)))) != 80 {
		t.Fatal("long filename")
	}
	if Validate(string([]byte{0xff}), "markdown") == nil {
		t.Fatal("UTF-8")
	}
	if Validate(strings.Repeat("x", (32<<20)+1), "markdown") == nil {
		t.Fatal("size")
	}
	if err := Validate("hello", "markdown"); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkMillionCharacters(b *testing.B) {
	source := strings.Repeat("A calm paragraph for writing.\n\n", 33000)
	b.ReportAllocs()
	for b.Loop() {
		Counts(source)
	}
}

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
