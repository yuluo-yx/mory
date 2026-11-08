package blocks

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func TestStructuredExportContract(t *testing.T) {
	b, err := os.ReadFile("testdata/all-blocks.xml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(b) + `<countdown duration="30"/><figure><source href="assets/a.pdf" name="Attachment"/></figure><video src="assets/a.mov"/><p><time date="2026-11-09" time="10:30"/><cite title="Note"/><doc-link title="Another"/><button text="Open" href="https://example.com"/></p><sub-page-list/>`
	d, err := Parse(source, "xml")
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.HTML(ExportOptions{Title: "Title <safe>", Cover: "assets/cover.png", Icon: "Book", Math: func(s string, display bool) (string, error) { return "<math>" + s + "</math>", nil }})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`rowspan="2"`, `colspan="2"`, `class="columns"`, `<math>`, `class="cover"`, `Title &lt;safe&gt;`, `Timer: 30`, `Attachment`, `<h6>`} {
		if !strings.Contains(page, want) {
			t.Fatalf("missing %q", want)
		}
	}
	md := d.Root.Markdown()
	for _, want := range []string{"<grid>", "<table>", "<time ", "<cite ", "<button ", "<whiteboard "} {
		if !strings.Contains(md, want) {
			t.Fatalf("Markdown lost %q", want)
		}
	}
	if d.Source() != source {
		t.Fatal("export mutated source")
	}
	math, _ := Parse(`<latex>x</latex>`, "xml")
	if _, err := math.HTML(ExportOptions{Math: func(string, bool) (string, error) { return "", errors.New("invalid") }}); err == nil {
		t.Fatal("formula error hidden")
	}
	if safeURL("javascript:alert(1)") != "" || safeURL("%") != "" {
		t.Fatal("active URL allowed")
	}
	if cssColor("invalid") != "inherit" || cssColor("#ff0000") != "#ff0000" || cssColor("#xx") != "inherit" {
		t.Fatal("invalid color")
	}
}

func TestClipboardHTMLContract(t *testing.T) {
	source := `<html><head><script>bad</script><style>bad</style></head><body><article><h2>Heading</h2><p><strong>Bold</strong> <em>Italic</em><span style="color:#123456;background-color:#eeeeee;font-weight:700;font-style:italic;text-decoration:underline;text-align:center">Color</span></p><table><tbody><tr><td rowspan="2" colspan="2"><p>Cell</p></td><td>Other</td></tr><tr><td>Last</td></tr></tbody></table><pre data-lark-language="python"><code class="language-python">x = 1</code></pre><ul><li><input type="checkbox" checked>Done</li></ul><img src="assets/pic.png" width="80" height="60" title="Image"><a href="javascript:alert(1)">unsafe</a><iframe>bad</iframe><section>Paragraph</section><aside>Note</aside><video>Fallback</video></article></body></html>`
	root, err := ParseHTML(source)
	if err != nil {
		t.Fatal(err)
	}
	xml := root.XML()
	for _, want := range []string{"<h2>", `colspan="2"`, `rowspan="2"`, `lang="python"`, `done="true"`, `href="assets/pic.png"`, `text-color="#123456"`, `<u>`} {
		if !strings.Contains(xml, want) {
			t.Fatalf("lost %s in %s", want, xml)
		}
	}
	if strings.Contains(xml, "javascript:") || strings.Contains(xml, "bad") {
		t.Fatal("active content retained")
	}
	d, _ := Parse(xml, "xml")
	page, err := d.HTML(ExportOptions{})
	if err != nil || !strings.Contains(page, "Cell") {
		t.Fatal(err)
	}
	inline, _ := ParseHTML(`Some <b>inline</b> text<br>with a line`)
	if inline.Elements()[0].Kind != "p" {
		t.Fatal("inline fragment not wrapped")
	}
}

func TestRichFormattingRoundTrip(t *testing.T) {
	for _, tag := range []string{"p", "h9", "checkbox", "li"} {
		n := New(tag, Text("A "), New("b", Text("bold")), New("em", Text("italic")), New("u", Text("underlined")), New("del", Text("strike")), New("code", Text("code")), New("br"), New("span", Text("color")), New("latex", Text("x")), New("mystery"), New("p", Text("Nested")))
		n.Set("align", "right")
		n.Set("done", "true")
		n.Children[7].Set("text-color", "red")
		n.Children[7].Set("background-color", "light-yellow")
		r := ToRich(n, RichOptions{Color: func(string) ui.Color { return ui.Hex("#aabbcc") }, Object: func(n *Node) *richtext.Attachment { return &richtext.Attachment{Name: n.Kind, Width: 20, Height: 20} }})
		d := richtext.NewDocument(r)
		d.SetSelection(0, r.Len())
		d.SetAlign(ui.Center)
		d.SetLink("https://example.com")
		d.ToggleBold()
		d.ToggleItalic()
		d.ToggleUnderline()
		d.ToggleStrikethrough()
		d.ToggleCode()
		d.Format(func(a *richtext.Attributes) { a.Color = ui.Hex("#123456"); a.Background = ui.Hex("#eeeeee") })
		out := FromRich(d.Text(), n)
		xml := New("root", out...).XML()
		for _, want := range []string{"Nested", "<latex>", "<mystery", `align="center"`, `href="https://example.com"`, `background-color=`} {
			if !strings.Contains(xml, want) {
				t.Fatal(tag, want, xml)
			}
		}
	}
	d := richtext.NewDocument(richtext.NewText("One\nTwo"))
	d.SetSelection(0, d.Text().Len())
	d.ToggleList(richtext.Numbers)
	if FromRich(d.Text(), nil)[0].Attr("list") != "number" {
		t.Fatal("numbered list")
	}
	d.ToggleList(richtext.Tasks)
	if FromRich(d.Text(), nil)[0].Kind != "checkbox" {
		t.Fatal("task list")
	}
	d.ToggleQuote()
	if FromRich(d.Text(), nil)[0].Kind != "blockquote" {
		t.Fatal("quote")
	}
	d.ToggleQuote()
	d.ToggleCodeBlock()
	if FromRich(d.Text(), nil)[0].Kind != "pre" {
		t.Fatal("code")
	}
	d = richtext.NewDocument(richtext.NewText(""))
	d.InsertAttachment(&richtext.Attachment{URL: "assets/p.png", Name: "Picture"})
	if !strings.Contains(FromRich(d.Text(), nil)[0].XML(), "assets/p.png") {
		t.Fatal("attachment")
	}
}

func TestMarkdownSyntaxCorpus(t *testing.T) {
	for _, source := range []string{"", "Plain text", "# Heading\n\nText *em* **bold** ~~strike~~ `code` [link](https://example.com) ![image](assets/p.png) $x$  \nline\n", "- a\n- b\n\n1. c\n2. d\n\n- [x] done\n- [ ] next\n", "> Quote\n\n> [!WARNING]\n> Note\n\n---\n", "```python\na=1\n```\n\n    indented\n", "[^1]: Footnote\n\nRef[^1]\n\n[x]: https://example.com\n", "<grid><column><p>A</p></column><column><p>B</p></column></grid>\n", "<div><img src=x></div>\n", "<p>A</p>\n<p>B</p>\n", "|A|B|\n|:--|--:|\n|a|b|\n"} {
		d, err := Parse(source, "markdown")
		if err != nil {
			t.Fatal(err)
		}
		if d.Source() != source {
			t.Fatal("normalization on read")
		}
		for _, n := range d.Root.Children {
			_ = n.Markdown()
			if IsText(n) {
				_ = ToRich(n, RichOptions{})
			}
		}
		n := Paragraph("Appended")
		if err := d.Splice(d.Root.ID, len(d.Root.Children), 0, n); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(d.Source(), "Appended") {
			t.Fatal("insert not serialized")
		}
		if !d.Undo() || d.Source() != source {
			t.Fatal("lossless undo")
		}
	}
}
