package blocks

import (
	"strings"
	"testing"
)

const docusaurusFixture = `---
id: intro
title: Introduction
tags: [guide, native]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Introduction {#intro}

:::tip[Remember]

A **formatted** tip.

:::warning Nested warning
Keep this content.
:::

:::

<details>
<summary>More information</summary>

## Hidden heading

Nested body.

</details>

<Tabs groupId="platform" defaultValue="mac">
  <TabItem value="mac" label="macOS" default>

Mac instructions.

  </TabItem>
  <TabItem value="linux" label="Linux">Linux instructions.</TabItem>
</Tabs>

` + "```jsx title=\"example.jsx\" {1,3-4} showLineNumbers\nconst x = <Example />;\n```\n"

func TestDocusaurusLosslessEditing(t *testing.T) {
	root, err := ParseMarkdown(docusaurusFixture)
	if err != nil {
		t.Fatal(err)
	}
	if got := root.Markdown(); got != docusaurusFixture {
		t.Fatalf("round trip changed bytes:\n%s", got)
	}
	var heading, callout, code, tab *Node
	root.Walk(func(n *Node) {
		switch {
		case n.Kind == "h1":
			heading = n
		case n.Attr("title") == "Remember":
			callout = n
		case n.Kind == "pre":
			code = n
		case n.Attr("value") == "linux":
			tab = n
		}
	})
	if heading == nil || heading.Plain() != "Introduction" || heading.Attr("heading-id") != "intro" || callout == nil || code == nil || tab == nil {
		t.Fatalf("missing parsed blocks: %#v", root)
	}
	heading.Children = []*Node{Text("New introduction")}
	code.Children = []*Node{New("code", Text("const x = <Changed />;\n"))}
	tab.Children = []*Node{Paragraph("Updated Linux instructions.")}
	out := root.Markdown()
	for _, want := range []string{"tags: [guide, native]", "# New introduction {#intro}", ":::tip[Remember]", ":::warning Nested warning", "title=\"example.jsx\" {1,3-4} showLineNumbers", "const x = <Changed />;", "<TabItem value=\"linux\" label=\"Linux\">Updated Linux instructions.", "import Tabs from '@theme/Tabs';"} {
		if !strings.Contains(out, want) {
			t.Fatalf("lost %q:\n%s", want, out)
		}
	}
	reparsed, err := ParseMarkdown(out)
	if err != nil || reparsed.Markdown() != out {
		t.Fatal("edited source is not stable", err)
	}
	html, err := (&Document{Root: root}).HTML(ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="intro"`, "<summary>More information</summary>", "<strong>Remember</strong>", "Updated Linux instructions.", "macOS"} {
		if !strings.Contains(html, want) {
			t.Fatalf("export missing %q", want)
		}
	}
}

func TestDocusaurusFenceAndMalformedPreservation(t *testing.T) {
	for _, source := range []string{
		"```md\n:::tip\nNot a directive\n:::\n```\n",
		"~~~md\n<Tabs>\n</Tabs>\n~~~\n",
		":::unknown\nBody\n:::\n", ":::tip\nUnclosed\n", "---\nUnclosed metadata\n",
		"<details>\nUnclosed\n", "<Tabs title=\"unterminated>\n", "plain without newline",
		":::note\n```md\n:::\n```\nStill inside.\n:::\n",
		"---\r\ntitle: Windows\r\n...\r\n\r\nBody\r\n",
		"<details><summary>A</summary><details><summary>B</summary>Nested</details></details>",
		"<Tabs\n defaultValue=\"one\">\n<TabItem value='one'>One</TabItem>\n</Tabs>",
		"import {\n  A, B\n} from './parts';\n\n# Body\n", "export const version = 1\n\nBody\n",
		"<Widget value={compute()}/>\n\nBody\n", ":::info\nInfo\n:::\n:::danger\nDanger\n:::\n:::caution\nCaution\n:::\n",
		"<details>\n```html\n</details>\n```\nStill inside.\n</details>\n",
	} {
		t.Run(source, func(t *testing.T) {
			n, err := ParseMarkdown(source)
			if err != nil || n.Markdown() != source {
				t.Fatalf("changed source: %v\n%q", err, n.Markdown())
			}
		})
	}
	if tagEnd(`<Tabs title={"a>b"} data={'x'}>`, 0) < 0 || tagEnd(`<Tabs title="a\"b">`, 0) < 0 {
		t.Fatal("quoted tag parsing")
	}
	if a, b := matchingTag("<Tabs title=\"broken", 0, "Tabs"); a != -1 || b != -1 {
		t.Fatal("malformed close")
	}
}

func TestDocusaurusNestedRulesAndOpaqueAttributes(t *testing.T) {
	source := ":::note\n---\n\nBody\n\n---\n:::\n"
	n, err := ParseMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	n.Walk(func(n *Node) {
		if n.Kind == "hr" {
			count++
		}
	})
	if count != 2 {
		t.Fatal("nested thematic rules became front matter", count)
	}
	custom := New("custom", Text("Text"))
	custom.Set("opening", "not a wrapper")
	if _, ok := extensionMarkdown(custom); ok {
		t.Fatal("unknown XML attribute treated as MDX")
	}
}
