package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestDocusaurusCodeHighlights(t *testing.T) {
	for _, tc := range []struct {
		source, meta string
		want         []int
	}{
		{"a\nb\nc\nd", `title="test" {1,3-4} showLineNumbers`, []int{0, 2, 3}},
		{"a\nb\nc", `{0,2-1000000000,7-5, 1-}`, []int{1, 2}},
		{"// highlight-next-line\na\nb\n// highlight-start\nc\nd\n// highlight-end\ne", "", []int{1, 4, 5}},
		{"# highlight-next-line\na\n/* highlight-start */\nb\n<!-- highlight-end -->\nc", "", []int{1, 3}},
		{"const s = 'highlight-start';\na", "", nil},
	} {
		got := []int(nil)
		for _, mark := range docusaurusCodeHighlights(tc.source, tc.meta, ui.Hex("#123456")) {
			got = append(got, mark.First)
			if mark.Last != mark.First {
				t.Fatal(mark)
			}
		}
		if !slices.Equal(got, tc.want) {
			t.Fatalf("%q: %v != %v", tc.source, got, tc.want)
		}
	}
}

func TestDocusaurusAnchorNavigation(t *testing.T) {
	a := testApp(t)
	a.createNote("# Overview {#overview}\n\n## Target {#stable-id}\n\nBody", "Anchors")
	tt := ui.NewTester(a.view, 1280, 820)
	s := a.blockEditor
	a.toggleHeading(s.doc.Root.Children[0])
	a.openLink("#stable-id")
	tt.Frame()
	if s.headingFolded(s.doc.Root.Children[0]) || s.active == "" {
		t.Fatal("anchor did not reveal heading")
	}
	a.openLink("#missing")
	if a.preview.Anchor != "#missing" {
		t.Fatal("fallback missing")
	}
	a.createNote("<Tabs groupId=\"navigation\"><TabItem value=\"first\">First.</TabItem>\n<TabItem value=\"second\"><details><summary>Nested</summary>\n\n## Deep {#deep}\n\nTarget body.\n\n</details></TabItem></Tabs>", "Nested anchors")
	tt.Frame()
	a.openLink("#deep")
	tt.Frame()
	s = a.blockEditor
	if s.componentChoice["group:navigation"] != "second" {
		t.Fatal("anchor did not select containing tab")
	}
	var target *blocks.Node
	s.doc.Root.Walk(func(n *blocks.Node) {
		if n.Attr("heading-id") == "deep" {
			target = n
		}
	})
	if target == nil {
		t.Fatal("missing deep heading")
	}
	if _, ok := tt.Find("Block text " + target.ID); !ok {
		t.Fatal("anchor did not open details")
	}
}
