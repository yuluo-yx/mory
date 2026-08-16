import test from "node:test";
import assert from "node:assert/strict";
import { documentStats, editorToMarkdown, escapeHTML, inlineMarkdown, markdownToHTML } from "../Sources/Mory/Web/markdown.js";

test("转义 HTML 特殊字符", () => {
  assert.equal(escapeHTML('<script x="1">&'), "&lt;script x=&quot;1&quot;&gt;&amp;");
});

test("渲染标题、段落和内联格式", () => {
  const html = markdownToHTML("# 标题\n\n一段 **粗体**、*斜体*、~~删除~~ 与 `code`。");
  assert.match(html, /<h1>标题<\/h1>/);
  assert.match(html, /<strong>粗体<\/strong>/);
  assert.match(html, /<em>斜体<\/em>/);
  assert.match(html, /<del>删除<\/del>/);
  assert.match(html, /<code>code<\/code>/);
});

test("带 UTF-8 BOM 的文稿仍渲染首行标题", () => {
  assert.equal(markdownToHTML("\uFEFF# 首行标题\n\n正文"), "<h1>首行标题</h1>\n<p>正文</p>");
});

test("渲染链接、图片并阻止脚本协议", () => {
  const html = inlineMarkdown("[安全](https://example.com) ![图](img/a.png) [危险](javascript:alert)");
  assert.match(html, /href="https:\/\/example.com"/);
  assert.match(html, /<img src="img\/a.png" alt="图">/);
  assert.match(html, /href="#"/);
});

test("渲染代码围栏且保留原始字符", () => {
  const html = markdownToHTML("```js\nconst value = 1 < 2;\n```");
  assert.equal(html, '<pre data-language="js"><code>const value = 1 &lt; 2;</code></pre>');
});

test("代码围栏保留可选名称和属性式语言", () => {
  assert.equal(
    markdownToHTML('```go title="main.go"\nfmt.Println("hi")\n```'),
    '<pre data-language="go" data-title="main.go"><code>fmt.Println(&quot;hi&quot;)</code></pre>'
  );
  assert.match(markdownToHTML('``` {.ts title="服务入口"}\nstart()\n```'), /data-language="ts" data-title="服务入口"/);
  assert.match(markdownToHTML('```py title="bad\\q"\npass\n```'), /data-title="bad\\q"/);
  assert.match(markdownToHTML("```sh title='脚本'\necho ok\n```"), /data-title="脚本"/);
});

test("把 Mermaid 围栏标记为待渲染图表", () => {
  const html = markdownToHTML("```mermaid\nflowchart LR\n  A[输入] --> B[输出]\n```");
  assert.match(html, /class="mermaid-diagram"/);
  assert.match(html, /data-mermaid-source="flowchart LR\n  A\[输入\] --&gt; B\[输出\]"/);
  assert.match(html, /contenteditable="false"/);
  assert.doesNotMatch(html, /<pre data-language="mermaid">/);
});

test("渲染引用、分隔线和混合列表", () => {
  const html = markdownToHTML("> 引用\n> 第二行\n\n- 普通\n- [x] 完成\n\n1. 第一\n2. 第二\n\n---");
  assert.match(html, /<blockquote><p>引用<\/p><p>第二行<\/p><\/blockquote>/);
  assert.match(html, /<input type="checkbox" checked>/);
  assert.match(html, /<ol><li>第一<\/li><li>第二<\/li><\/ol>/);
  assert.match(html, /<hr>/);
});

test("渲染表格并补齐缺失单元格", () => {
  const html = markdownToHTML("| 名称 | 值 |\n| --- | --- |\n| A | 1 |\n| B |");
  assert.match(html, /<thead><tr><th>名称<\/th><th>值<\/th><\/tr><\/thead>/);
  assert.match(html, /<tr><td>B<\/td><td><\/td><\/tr>/);
});

test("未闭合代码围栏渲染到文末", () => {
  assert.equal(markdownToHTML("~~~txt\nhello"), '<pre data-language="txt"><code>hello</code></pre>');
});

test("统计中文、英文、字符和行数", () => {
  assert.deepEqual(documentStats("你好 world-test\n第二行"), { words: 7, characters: 14, lines: 2 });
});

test("空文档统计为零", () => {
  assert.deepEqual(documentStats(""), { words: 0, characters: 0, lines: 0 });
});

test("连续普通行合并为同一段落", () => {
  assert.equal(markdownToHTML("第一行\n第二行"), "<p>第一行 第二行</p>");
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

test("把编辑器 DOM 序列化为 Markdown", () => {
  globalThis.Node = { TEXT_NODE: 3, ELEMENT_NODE: 1 };
  const root = element("article", [
    new FakeText(" 顶层\u200b文本 "),
    element("h1", ["标题"]),
    element("p", ["普通 ", element("strong", ["粗体"]), " ", element("em", ["斜体"]), " ", element("del", ["删除"]), " ", element("code", ["a`b"]), "\u200b", element("br")]),
    element("p", [element("a", ["链接"], { href: "https://example.com" }), " ", element("img", [], { alt: "图", src: "a.png" })]),
    element("blockquote", ["第一行\n第二行"]),
    element("ul", [
      element("li", [element("input", [], { type: "checkbox", checked: true }), "任务"]),
      element("li", ["普通项"])
    ]),
    element("ol", [element("li", ["甲"]), element("li", ["乙"])]),
    element("pre", ["const a = 1;\n\u200b"], { dataset: { language: "js", title: "main.js" } }),
    element("div", [element("svg")], { class: "mermaid-diagram", dataset: { mermaidSource: "flowchart LR\nA-->B" } }),
    element("table", [
      element("tr", [element("th", ["名称"]), element("th", ["值"])]),
      element("tr", [element("td", ["A|B"]), element("td", ["1"])])
    ]),
    element("hr"),
    element("section", ["保留内容"])
  ]);

  const markdown = editorToMarkdown(root);
  assert.match(markdown, /^顶层文本\n\n# 标题/);
  assert.doesNotMatch(markdown, /\u200b/);
  assert.match(markdown, /普通 \*\*粗体\*\* \*斜体\* ~~删除~~ `a\\`b`/);
  assert.match(markdown, /\[链接\]\(https:\/\/example.com\) !\[图\]\(a.png\)/);
  assert.match(markdown, /> 第一行\n> 第二行/);
  assert.match(markdown, /- \[x\] 任务\n- 普通项/);
  assert.match(markdown, /1\. 甲\n2\. 乙/);
  assert.match(markdown, /```js title="main.js"\nconst a = 1;\n```/);
  assert.match(markdown, /```mermaid\nflowchart LR\nA-->B\n```/);
  assert.match(markdown, /\| 名称 \| 值 \|\n\| --- \| --- \|\n\| A\\\|B \| 1 \|/);
  assert.match(markdown, /---\n\n保留内容$/);
});

test("覆盖主题导出所需的内联边界", () => {
  const html = inlineMarkdown('__粗体__ _斜体_ [标题](https://a.example "说明") ![图](a.png "图片")  \n换行 [拦截](vbscript:run)');
  assert.match(html, /<strong>粗体<\/strong>/);
  assert.match(html, /<em>斜体<\/em>/);
  assert.match(html, /title="说明"/);
  assert.match(html, /title="图片"/);
  assert.match(html, /<br>/);
  assert.match(html, /href="#"/);
});

test("覆盖空输入和右括号有序列表", () => {
  assert.equal(markdownToHTML(null), "");
  assert.equal(markdownToHTML("1) 一\n2) 二"), "<ol><li>一</li><li>二</li></ol>");
});

test("序列化缺省属性和空块", () => {
  globalThis.Node = { TEXT_NODE: 3, ELEMENT_NODE: 1 };
  const root = element("article", [
    new FakeText("   "),
    element("div", [element("b", ["粗"]), element("i", ["斜"]), element("s", ["删"]), element("strike", ["旧"])]),
    element("p", [element("a", ["空链接"]), element("img"), element("input", [], { type: "text" })]),
    element("pre", ["plain"]),
    element("table"),
    element("aside")
  ]);
  const markdown = editorToMarkdown(root);
  assert.match(markdown, /\*\*粗\*\*\*斜\*~~删~~~~旧~~/);
  assert.match(markdown, /\[空链接\]\(\)!\[\]\(\)/);
  assert.match(markdown, /```\nplain\n```/);
});
