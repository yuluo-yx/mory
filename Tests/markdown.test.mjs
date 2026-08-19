import test from "node:test";
import assert from "node:assert/strict";
import { documentStats, editorToMarkdown, escapeHTML, inlineMarkdown, markdownToHTML } from "../Sources/Mory/Web/markdown.js";

test("escapes HTML special characters", () => {
  assert.equal(escapeHTML('<script x="1">&'), "&lt;script x=&quot;1&quot;&gt;&amp;");
});

test("renders headings, paragraphs, and inline formatting", () => {
  const html = markdownToHTML("# \u6807\u9898\n\n\u4E00\u6BB5 **\u7C97\u4F53**, *\u659C\u4F53*, ~~\u5220\u9664~~ \u4E0E `code`.");
  assert.match(html, /<h1>\u6807\u9898<\/h1>/);
  assert.match(html, /<strong>\u7C97\u4F53<\/strong>/);
  assert.match(html, /<em>\u659C\u4F53<\/em>/);
  assert.match(html, /<del>\u5220\u9664<\/del>/);
  assert.match(html, /<code>code<\/code>/);
});

test("renders the first heading when the document starts with a UTF-8 BOM", () => {
  assert.equal(markdownToHTML("\uFEFF# \u9996\u884C\u6807\u9898\n\n\u6B63\u6587"), "<h1>\u9996\u884C\u6807\u9898</h1>\n<p>\u6B63\u6587</p>");
});

test("renders links and images while blocking script protocols", () => {
  const html = inlineMarkdown("[\u5B89\u5168](https://example.com) ![\u56FE](img/a.png) [\u5371\u9669](javascript:alert)");
  assert.match(html, /href="https:\/\/example.com"/);
  assert.match(html, /<img src="img\/a.png" alt="\u56FE">/);
  assert.match(html, /href="#"/);
});

test("marks workspace Markdown references as document links", () => {
  const regular = inlineMarkdown("[\u8BF4\u660E](./\u8D44\u6599/\u8BF4\u660E.md)");
  const legacyImageSyntax = inlineMarkdown("![Alt](./07-preemption.md)");
  assert.match(regular, /<a href="\.\/\u8D44\u6599\/\u8BF4\u660E\.md" class="document-link">\u8BF4\u660E<\/a>/);
  assert.match(legacyImageSyntax, /class="document-link" data-markdown-image-link="true"/);
  assert.match(legacyImageSyntax, />Alt<\/a>/);
  assert.doesNotMatch(legacyImageSyntax, /<img/);
});

test("preserves backslash-escaped Markdown punctuation as literal characters", () => {
  const html = inlineMarkdown("**\u6587\u672C\u52A0\u7C97**  \\*\\* \u6B63\u5E38\u663E\u793A\u661F\u53F7 \\*\\*");
  assert.equal(html, "<strong>\u6587\u672C\u52A0\u7C97</strong>  ** \u6B63\u5E38\u663E\u793A\u661F\u53F7 **");
});

test("renders fenced code blocks while preserving literal characters", () => {
  const html = markdownToHTML("```js\nconst value = 1 < 2;\n```");
  assert.equal(html, '<pre data-language="js"><code>const value = 1 &lt; 2;</code></pre>');
});

test("preserves optional code titles and attribute-style languages", () => {
  assert.equal(
    markdownToHTML('```go title="main.go"\nfmt.Println("hi")\n```'),
    '<pre data-language="go" data-title="main.go"><code>fmt.Println(&quot;hi&quot;)</code></pre>'
  );
  assert.match(markdownToHTML('``` {.ts title="\u670D\u52A1\u5165\u53E3"}\nstart()\n```'), /data-language="ts" data-title="\u670D\u52A1\u5165\u53E3"/);
  assert.match(markdownToHTML('```py title="bad\\q"\npass\n```'), /data-title="bad\\q"/);
  assert.match(markdownToHTML("```sh title='\u811A\u672C'\necho ok\n```"), /data-title="\u811A\u672C"/);
});

test("marks Mermaid fences for diagram rendering", () => {
  const html = markdownToHTML("```mermaid\nflowchart LR\n  A[\u8F93\u5165] --> B[\u8F93\u51FA]\n```");
  assert.match(html, /class="mermaid-diagram"/);
  assert.match(html, /data-mermaid-source="flowchart LR\n  A\[\u8F93\u5165\] --&gt; B\[\u8F93\u51FA\]"/);
  assert.match(html, /data-mermaid-theme="auto"/);
  assert.match(html, /contenteditable="false"/);
  assert.doesNotMatch(html, /<pre data-language="mermaid">/);
});

test("preserves Mermaid color themes and falls back from unknown values", () => {
  assert.match(markdownToHTML("```mermaid theme=forest\nflowchart LR\nA-->B\n```"), /data-mermaid-theme="forest"/);
  assert.match(markdownToHTML("```mermaid theme=unknown\nflowchart LR\nA-->B\n```"), /data-mermaid-theme="auto"/);
});

test("renders valid calendar fences as non-editable calendar blocks", () => {
  const source = '```calendar\n{"version":1,"month":"2026-08","marks":[{"date":"2026-08-24","color":"red","title":"Important"}],"ranges":[],"items":[]}\n```';
  const html = markdownToHTML(source);
  assert.match(html, /class="calendar-block"/);
  assert.match(html, /data-calendar-source="\{/);
  assert.match(html, /&quot;month&quot;: &quot;2026-08&quot;/);
  assert.match(html, /contenteditable="false"/);
});

test("keeps invalid calendar fences editable as ordinary code", () => {
  assert.equal(markdownToHTML("```calendar\n{bad json}\n```"), '<pre data-language="calendar"><code>{bad json}</code></pre>');
  assert.match(markdownToHTML('```calendar\n{"version":2,"month":"2026-08"}\n```'), /<pre data-language="calendar">/);
});

test("renders blockquotes, horizontal rules, and mixed lists", () => {
  const html = markdownToHTML("> \u5F15\u7528\n> \u7B2C\u4E8C\u884C\n\n- \u666E\u901A\n- [x] \u5B8C\u6210\n\n1. \u7B2C\u4E00\n2. \u7B2C\u4E8C\n\n---");
  assert.match(html, /<blockquote><p>\u5F15\u7528<\/p><p>\u7B2C\u4E8C\u884C<\/p><\/blockquote>/);
  assert.match(html, /<input type="checkbox" checked>/);
  assert.match(html, /<ol><li>\u7B2C\u4E00<\/li><li>\u7B2C\u4E8C<\/li><\/ol>/);
  assert.match(html, /<hr>/);
});

test("renders tables and fills missing cells", () => {
  const html = markdownToHTML("| \u540D\u79F0 | \u503C |\n| --- | --- |\n| A | 1 |\n| B |");
  assert.match(html, /<thead><tr><th>\u540D\u79F0<\/th><th>\u503C<\/th><\/tr><\/thead>/);
  assert.match(html, /<tr><td>B<\/td><td><\/td><\/tr>/);
});

test("renders an unclosed code fence through the end of the document", () => {
  assert.equal(markdownToHTML("~~~txt\nhello"), '<pre data-language="txt"><code>hello</code></pre>');
});

test("counts CJK words, Latin words, characters, and lines", () => {
  assert.deepEqual(documentStats("\u4F60\u597D world-test\n\u7B2C\u4E8C\u884C"), { words: 7, characters: 14, lines: 2 });
});

test("returns zero counts for an empty document", () => {
  assert.deepEqual(documentStats(""), { words: 0, characters: 0, lines: 0 });
});

test("merges consecutive plain lines into one paragraph", () => {
  assert.equal(markdownToHTML("\u7B2C\u4E00\u884C\n\u7B2C\u4E8C\u884C"), "<p>\u7B2C\u4E00\u884C \u7B2C\u4E8C\u884C</p>");
});

class FakeText {
  constructor(value) {
    this.nodeType = 3;
    this.nodeValue = value;
    this.parentElement = null;
  }
}

class FakeElement {
  constructor(tagName, children = [], attributes = {}) {
    this.nodeType = 1;
    this.tagName = tagName.toUpperCase();
    this.childNodes = children.map(child => typeof child === "string" ? new FakeText(child) : child);
    this.childNodes.forEach(child => { child.parentElement = this; });
    this.children = this.childNodes.filter(child => child.nodeType === 1);
    this.attributes = attributes;
    this.dataset = attributes.dataset ?? {};
    this.checked = Boolean(attributes.checked);
  }

  get innerText() {
    return this.childNodes.map(child => child.nodeType === 3 ? child.nodeValue : child.innerText).join("");
  }

  getAttribute(name) {
    return this.attributes[name] ?? null;
  }

  querySelector(selector) {
    if (selector === ':scope > input[type="checkbox"]') {
      return this.children.find(child => child.tagName === "INPUT" && child.getAttribute("type") === "checkbox") ?? null;
    }
    return null;
  }

  querySelectorAll(selector) {
    const matches = [];
    const wanted = selector.toUpperCase();
    const visit = node => {
      if (node.tagName === wanted) matches.push(node);
      node.children?.forEach(visit);
    };
    this.children.forEach(visit);
    return matches;
  }
}

const element = (tag, children = [], attributes = {}) => new FakeElement(tag, children, attributes);

test("serializes editor DOM into Markdown", () => {
  globalThis.Node = { TEXT_NODE: 3, ELEMENT_NODE: 1 };
  const root = element("article", [
    new FakeText(" \u9876\u5C42\u200b\u6587\u672C "),
    element("h1", ["\u6807\u9898"]),
    element("p", ["\u666E\u901A ", element("strong", ["\u7C97\u4F53"]), " ", element("em", ["\u659C\u4F53"]), " ", element("del", ["\u5220\u9664"]), " ", element("code", ["a`b"]), "\u200b", element("br")]),
    element("p", [element("a", ["\u94FE\u63A5"], { href: "https://example.com" }), " ", element("img", [], { alt: "\u56FE", src: "data:image/png;base64,a", dataset: { markdownSrc: "\u6587\u7AE0/a.png" } })]),
    element("blockquote", ["\u7B2C\u4E00\u884C\n\u7B2C\u4E8C\u884C"]),
    element("ul", [
      element("li", [element("input", [], { type: "checkbox", checked: true }), "\u4EFB\u52A1"]),
      element("li", ["\u666E\u901A\u9879"])
    ]),
    element("ol", [element("li", ["\u7532"]), element("li", ["\u4E59"])]),
    element("pre", ["const a = 1;\n\u200b"], { dataset: { language: "js", title: "main.js" } }),
    element("div", [element("svg")], { class: "mermaid-diagram", dataset: { mermaidSource: "flowchart LR\nA-->B" } }),
    element("div", [], { class: "mermaid-diagram", dataset: { mermaidSource: "flowchart TD\nB-->C", mermaidTheme: "forest" } }),
    element("div", [], { class: "calendar-block", dataset: { calendarSource: '{"version":1,"month":"2026-08","marks":[],"ranges":[],"items":[]}' } }),
    element("table", [
      element("tr", [element("th", ["\u540D\u79F0"]), element("th", ["\u503C"])]),
      element("tr", [element("td", ["A|B"]), element("td", ["1"])])
    ]),
    element("hr"),
    element("section", ["\u4FDD\u7559\u5185\u5BB9"])
  ]);

  const markdown = editorToMarkdown(root);
  assert.match(markdown, /^\u9876\u5C42\u6587\u672C\n\n# \u6807\u9898/);
  assert.doesNotMatch(markdown, /\u200b/);
  assert.match(markdown, /\u666E\u901A \*\*\u7C97\u4F53\*\* \*\u659C\u4F53\* ~~\u5220\u9664~~ `a\\`b`/);
  assert.match(markdown, /\[\u94FE\u63A5\]\(https:\/\/example.com\) !\[\u56FE\]\(\u6587\u7AE0\/a.png\)/);
  assert.match(markdown, /> \u7B2C\u4E00\u884C\n> \u7B2C\u4E8C\u884C/);
  assert.match(markdown, /- \[x\] \u4EFB\u52A1\n- \u666E\u901A\u9879/);
  assert.match(markdown, /1\. \u7532\n2\. \u4E59/);
  assert.match(markdown, /```js title="main.js"\nconst a = 1;\n```/);
  assert.match(markdown, /```mermaid\nflowchart LR\nA-->B\n```/);
  assert.match(markdown, /```mermaid theme=forest\nflowchart TD\nB-->C\n```/);
  assert.match(markdown, /```calendar\n\{\n  "version": 1,\n  "month": "2026-08"/);
  assert.match(markdown, /\| \u540D\u79F0 \| \u503C \|\n\| --- \| --- \|\n\| A\\\|B \| 1 \|/);
  assert.match(markdown, /---\n\n\u4FDD\u7559\u5185\u5BB9$/);
});

test("covers inline boundaries required by themed exports", () => {
  const html = inlineMarkdown('__\u7C97\u4F53__ _\u659C\u4F53_ [\u6807\u9898](https://a.example "\u8BF4\u660E") ![\u56FE](a.png "\u56FE\u7247")  \n\u6362\u884C [\u62E6\u622A](vbscript:run)');
  assert.match(html, /<strong>\u7C97\u4F53<\/strong>/);
  assert.match(html, /<em>\u659C\u4F53<\/em>/);
  assert.match(html, /title="\u8BF4\u660E"/);
  assert.match(html, /title="\u56FE\u7247"/);
  assert.match(html, /<br>/);
  assert.match(html, /href="#"/);
});

test("re-escapes Markdown punctuation in plain editor text", () => {
  globalThis.Node = { TEXT_NODE: 3, ELEMENT_NODE: 1 };
  const root = element("article", [element("p", ["** \u6B63\u5E38\u663E\u793A\u661F\u53F7 **"])]);
  assert.equal(editorToMarkdown(root), "\\*\\* \u6B63\u5E38\u663E\u793A\u661F\u53F7 \\*\\*");
});

test("handles empty input and right-parenthesis ordered lists", () => {
  assert.equal(markdownToHTML(null), "");
  assert.equal(markdownToHTML("1) \u4E00\n2) \u4E8C"), "<ol><li>\u4E00</li><li>\u4E8C</li></ol>");
});

test("serializes default attributes and empty blocks", () => {
  globalThis.Node = { TEXT_NODE: 3, ELEMENT_NODE: 1 };
  const root = element("article", [
    new FakeText("   "),
    element("div", [element("b", ["\u7C97"]), element("i", ["\u659C"]), element("s", ["\u5220"]), element("strike", ["\u65E7"])]),
    element("p", [element("a", ["\u7A7A\u94FE\u63A5"]), element("img"), element("input", [], { type: "text" })]),
    element("pre", ["plain"]),
    element("table"),
    element("aside")
  ]);
  const markdown = editorToMarkdown(root);
  assert.match(markdown, /\*\*\u7C97\*\*\*\u659C\*~~\u5220~~~~\u65E7~~/);
  assert.match(markdown, /\[\u7A7A\u94FE\u63A5\]\(\)!\[\]\(\)/);
  assert.match(markdown, /```\nplain\n```/);
});

test("does not serialize table controls into Markdown", () => {
  globalThis.Node = { TEXT_NODE: 3, ELEMENT_NODE: 1 };
  const root = element("article", [
    element("table", [
      element("tr", [element("th", ["\u540D\u79F0"])]),
      element("tr", [element("td", ["Mory"])])
    ]),
    element("div", ["\u6DFB\u52A0\u884C \u6DFB\u52A0\u5217"], { class: "table-tools" })
  ]);
  assert.equal(editorToMarkdown(root), "| \u540D\u79F0 |\n| --- |\n| Mory |");
});
