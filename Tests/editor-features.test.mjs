import assert from "node:assert/strict";
import test from "node:test";
import pangu from "pangu";
import {
  calendarMarkdown,
  formatFileSize,
  formatUpdatedAt,
  markdownHeadingTree,
  mindMapHTML,
  optimizeMarkdownTypography
} from "../Sources/Mory/Web/editor-features.js";

test("optimizes CJK typography without changing Markdown code or URLs", () => {
  const source = "# Mory\u7F16\u8F91\u5668\n\n**\u4E2D\u6587English** and `const value=1`\n\n[\u94FE\u63A5](https://example.com/a1)\n\n```go\nfmt.Println(\"\u4E2D\u6587English\")\n```";
  const result = optimizeMarkdownTypography(source, value => pangu.spacingText(value));
  assert.match(result, /Mory \u7F16\u8F91\u5668/);
  assert.match(result, /\u4E2D\u6587 English/);
  assert.match(result, /`const value=1`/);
  assert.match(result, /https:\/\/example\.com\/a1/);
  assert.match(result, /fmt\.Println\("\u4E2D\u6587English"\)/);
  assert.doesNotMatch(result, /\*\*\s+\u4E2D\u6587/);
});

test("generates a Monday-first Markdown calendar for the selected month", () => {
  const result = calendarMarkdown(new Date(2026, 7, 1), "en");
  const lines = result.split("\n");
  assert.equal(lines[0], "| Mon | Tue | Wed | Thu | Fri | Sat | Sun |");
  assert.equal(lines[2], "|  |  |  |  |  | 1 | 2 |");
  assert.equal(lines.at(-1), "| 31 |  |  |  |  |  |  |");
});

test("formats file sizes and update timestamps for sidebar details", () => {
  assert.equal(formatFileSize(12), "12 B");
  assert.equal(formatFileSize(1536), "1.5 KB");
  assert.equal(formatFileSize(5 * 1024 * 1024), "5 MB");
  assert.equal(formatFileSize(-1), "");
  assert.match(formatUpdatedAt(Date.UTC(2026, 7, 18, 12, 30), "en"), /2026/);
  assert.equal(formatUpdatedAt("invalid", "en"), "");
});

test("builds and exports a heading-based mind map while ignoring fenced headings", () => {
  const markdown = "# Roadmap\n\n## Editor\n\n### Tables\n\n## Export\n\n```md\n# Not a node\n```";
  const tree = markdownHeadingTree(markdown, "Roadmap");
  assert.deepEqual(tree.children.map(node => node.text), ["Editor", "Export"]);
  assert.deepEqual(tree.children[0].children.map(node => node.text), ["Tables"]);
  const html = mindMapHTML(markdown, "Roadmap", "en");
  assert.match(html, /^<!doctype html>/);
  assert.match(html, /<svg/);
  assert.match(html, />Tables</);
  assert.doesNotMatch(html, /Not a node/);
  assert.doesNotMatch(html, /<script/);
});
