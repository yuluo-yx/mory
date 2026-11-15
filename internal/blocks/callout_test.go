package blocks

import (
	"strings"
	"testing"
)

func TestCalloutTypeRoundTrips(t *testing.T) {
	for _, source := range []string{"> [!NOTE]\n> Body **bold**\n", "::::tip[Keep title]\nBody **bold**\n::::\n", `<callout><p>Body <b>bold</b></p></callout>`} {
		for _, kind := range []string{"note", "info", "tip", "important", "warning", "caution", "danger"} {
			t.Run(kind+"/"+source[:6], func(t *testing.T) {
				format := "markdown"
				if strings.HasPrefix(source, "<") {
					format = "xml"
				}
				d, err := Parse(source, format)
				if err != nil {
					t.Fatal(err)
				}
				n := Visible(d.Root.Children)[0]
				if err := d.Transform(n.ID, func(n *Node) { SetCalloutType(n, kind) }); err != nil {
					t.Fatal(err)
				}
				n = d.Find(n.ID)
				if CalloutType(n) != kind {
					t.Fatal("type did not change", n.Attrs)
				}
				// Editing content forces serialization instead of using cached source.
				if err := d.Transform(n.Children[0].ID, func(n *Node) { n.Children = append(n.Children, Text(" edited")) }); err != nil {
					t.Fatal(err)
				}
				for _, output := range []struct{ source, format string }{{d.Source(), format}, {d.Root.Markdown(), "markdown"}} {
					reopened, err := Parse(output.source, output.format)
					if err != nil {
						t.Fatal(err)
					}
					got := Visible(reopened.Root.Children)[0]
					if got.Kind != "callout" || CalloutType(got) != kind || !strings.Contains(got.Plain(), "bold edited") {
						t.Fatalf("reopening lost type or content: %s", output.source)
					}
					if strings.HasPrefix(source, "::::") && !strings.Contains(output.source, "::::"+kind+"[Keep title]") {
						t.Fatal("type change lost directive marker or custom title", output.source)
					}
				}
				html, err := d.HTML(ExportOptions{})
				if err != nil || !strings.Contains(html, "border-color:"+CalloutColor(d.Find(n.ID))) {
					t.Fatal("HTML lost semantic color", html, err)
				}
			})
		}
	}
}

func TestCalloutExistingTypesAndCustomColors(t *testing.T) {
	for _, kind := range []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"} {
		source := "> [!" + kind + "]\n> Existing text\n"
		d, _ := Parse(source, "markdown")
		n := Visible(d.Root.Children)[0]
		SetCalloutType(n, strings.ToLower(kind))
		if d.Source() != source {
			t.Fatal("selecting the existing type rewrote original source")
		}
		n.Children = []*Node{Paragraph("New text")}
		if !strings.Contains(d.Source(), "[!"+kind+"]") {
			t.Fatal("editing reset the alert to NOTE", d.Source())
		}
	}
	d, _ := Parse(`<callout alert="warn" background-color="#123456" border-color="#654321"><p>Body</p></callout>`, "xml")
	n := Visible(d.Root.Children)[0]
	if CalloutType(n) != "warning" {
		t.Fatal("warn alias was not recognized")
	}
	SetCalloutType(n, "danger")
	html, _ := d.HTML(ExportOptions{})
	if !strings.Contains(html, "background-color:#123456") || !strings.Contains(html, "border-color:#654321") {
		t.Fatal("type change discarded custom colors", html)
	}
}

func TestEmptyDirectiveHasEditableParagraph(t *testing.T) {
	for _, source := range []string{":::info\n:::\n", ":::danger\n\n:::\n"} {
		d, err := Parse(source, "markdown")
		if err != nil {
			t.Fatal(err)
		}
		n := Visible(d.Root.Children)[0]
		if n.Kind != "callout" || len(Visible(n.Children)) != 1 || Visible(n.Children)[0].Kind != "p" || d.Source() != source {
			t.Fatal("empty directive has no editor or viewing changed source", d.Source())
		}
	}
}
