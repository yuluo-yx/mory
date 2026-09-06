import assert from "node:assert/strict";
import test from "node:test";
import pangu from "pangu";
import {
  calendarDateFromKey,
  calendarMarkdown,
  calendarMonthDays,
  calendarRangeDayCount,
  findTextMatches,
  formatFileSize,
  formatUpdatedAt,
  headingFoldVisibility,
  localDateKey,
  markdownHeadingTree,
  mermaidColorThemes,
  mindMapHTML,
  normalizeCalendarDocument,
  normalizeMermaidColorTheme,
  optimizeMarkdownTypography,
  replaceTextMatches
} from "../Sources/Mory/Web/editor-features.js";

test("literal search returns non-overlapping original source ranges", () => {
  assert.deepEqual(findTextMatches("cat CAT cat", "cat"), [
    { start: 0, end: 3 }, { start: 4, end: 7 }, { start: 8, end: 11 }
  ]);
  assert.deepEqual(findTextMatches("aaa", "aa"), [{ start: 0, end: 2 }]);
  assert.deepEqual(findTextMatches("text", ""), []);
  assert.deepEqual(findTextMatches("text", "absent"), []);
});

test("mind maps ignore headings inside long and unfinished code fences", () => {
  const markdown = "# Roadmap\n````md\n```\n# Hidden\n```\n````\n## Visible\n~~~\n# Unfinished";
  const tree = markdownHeadingTree(markdown, "Roadmap");
  assert.deepEqual(tree.children.map(node => node.text), ["Visible"]);
});

test("typography preserves complete code fences including original line endings", () => {
  for (const newline of ["\n", "\r\n", "\r"]) {
    for (const close of [true, false]) {
      const lines = ["````md", "```", "\u4E2D\u6587English", "```"];
      if (close) lines.push("````");
      const source = lines.join(newline);
      assert.equal(optimizeMarkdownTypography(source, value => pangu.spacingText(value)), source);
    }
  }
});

test("literal search escapes every regular expression metacharacter", () => {
  const query = ".*+?^${}()|[]\\";
  assert.deepEqual(findTextMatches(`before ${query} after`, query), [{ start: 7, end: 7 + query.length }]);
});

test("Unicode search preserves UTF-16 offsets used by textarea selections", () => {
  assert.deepEqual(findTextMatches("\u0130 \uD83D\uDE00 CAT", "cat"), [{ start: 5, end: 8 }]);
  assert.deepEqual(findTextMatches("\u03A3 \u03C2 \u03C3", "\u03C3"), [
    { start: 0, end: 1 }, { start: 2, end: 3 }, { start: 4, end: 5 }
  ]);
  assert.deepEqual(findTextMatches("\u4E2D\u6587 \u4E2D\u6587", "\u4E2D\u6587"), [
    { start: 0, end: 2 }, { start: 3, end: 5 }
  ]);
});

test("replacement preserves literal dollar tokens and unrelated text", () => {
  const source = "before cat CAT after";
  const matches = findTextMatches(source, "cat");
  const replacement = "$& $$ $' $` $1";
  assert.equal(replaceTextMatches(source, matches, replacement), `before ${replacement} ${replacement} after`);
  assert.equal(replaceTextMatches(source, matches.slice(1), "fox"), "before cat fox after");
  assert.equal(replaceTextMatches(source, matches, ""), "before   after");
  assert.equal(replaceTextMatches(source, [], "fox"), source);
});

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

test("normalizes Mermaid color themes to a controlled portable set", () => {
  assert.deepEqual(mermaidColorThemes, ["auto", "ocean", "forest", "sunset", "mono"]);
  assert.equal(normalizeMermaidColorTheme("FOREST"), "forest");
  assert.equal(normalizeMermaidColorTheme("unknown"), "auto");
  assert.equal(normalizeMermaidColorTheme(), "auto");
});

test("generates a versioned calendar fence for the selected month", () => {
  const result = calendarMarkdown(new Date(2026, 7, 1));
  assert.match(result, /^```calendar\n/);
  assert.match(result, /"version": 1/);
  assert.match(result, /"month": "2026-08"/);
  assert.match(result, /"marks": \[\]/);
  assert.match(result, /\n```$/);
});

test("normalizes calendar dates, colors, ranges, and items deterministically", () => {
  const document = normalizeCalendarDocument({
    version: 99,
    month: "2026-13",
    marks: [
      { date: "2026-08-24", color: "unknown", title: "  Important   day  " },
      { date: "2026-08-24", color: "red", title: "Launch" },
      { date: "2026-02-30", color: "green", title: "Invalid" }
    ],
    ranges: [
      { start: "2027-01-02", end: "2026-12-30", color: "green", title: "  Cross year " },
      { start: "bad", end: "2026-08-27", color: "blue", title: "Invalid" },
      { start: "2026-08-24", end: "2026-08-27", color: "blue", title: " " }
    ],
    items: [
      { date: "2026-08-24", text: "  Review   UI ", done: 1 },
      { date: "2026-08-25", text: " ", done: false }
    ]
  }, new Date(2026, 7, 1));
  assert.equal(document.version, 1);
  assert.equal(document.month, "2026-08");
  assert.deepEqual(document.marks, [{ date: "2026-08-24", color: "red", title: "Launch" }]);
  assert.deepEqual(document.ranges, [{ start: "2026-12-30", end: "2027-01-02", color: "green", title: "Cross year" }]);
  assert.deepEqual(document.items, [{ date: "2026-08-24", text: "Review UI", done: true }]);
});

test("builds a stable Monday-first six-week month and counts closed ranges", () => {
  const days = calendarMonthDays("2026-08");
  assert.equal(days.length, 42);
  assert.deepEqual(days[0], { date: "2026-07-27", day: 27, currentMonth: false, weekday: 0, week: 0 });
  assert.equal(days[5].date, "2026-08-01");
  assert.equal(days.at(-1).date, "2026-09-06");
  assert.equal(calendarRangeDayCount("2026-08-24", "2026-08-27"), 4);
  assert.equal(calendarRangeDayCount("2026-08-24", "2026-08-24"), 1);
  assert.equal(calendarRangeDayCount("2027-01-02", "2026-12-30"), 4);
  assert.equal(calendarRangeDayCount("bad", "2026-08-27"), 0);
});

test("validates leap dates without UTC date drift", () => {
  assert.equal(localDateKey(new Date(2028, 1, 29)), "2028-02-29");
  assert.equal(localDateKey("2028-02-29"), "2028-02-29");
  assert.ok(calendarDateFromKey("2028-02-29"));
  assert.equal(calendarDateFromKey("2027-02-29"), null);
});

test("formats file sizes and update timestamps for sidebar details", () => {
  assert.equal(formatFileSize(12), "12 B");
  assert.equal(formatFileSize(1536), "1.5 KB");
  assert.equal(formatFileSize(5 * 1024 * 1024), "5 MB");
  assert.equal(formatFileSize(-1), "");
  assert.match(formatUpdatedAt(Date.UTC(2026, 7, 18, 12, 30), "en"), /2026/);
  assert.equal(formatUpdatedAt("invalid", "en"), "");
});

test("keeps folded heading sections bounded by the next peer heading", () => {
  const blocks = [
    { headingLevel: 1 },
    { headingLevel: 2, collapsed: true },
    {},
    { headingLevel: 3 },
    {},
    { headingLevel: 2 },
    {},
    { headingLevel: 5, collapsed: true },
    {},
    { headingLevel: 1 },
    {}
  ];
  assert.deepEqual(headingFoldVisibility(blocks), [false, false, true, true, true, false, false, false, false, false, false]);
});

test("retains nested folds while their collapsed parent is hidden", () => {
  const blocks = [
    { headingLevel: 1, collapsed: true },
    {},
    { headingLevel: 2, collapsed: true },
    {},
    { headingLevel: 2 },
    {},
    { headingLevel: 1 },
    {}
  ];
  assert.deepEqual(headingFoldVisibility(blocks), [false, true, true, true, true, true, false, false]);
  assert.deepEqual(headingFoldVisibility(blocks.map((block, index) => index === 0 ? { ...block, collapsed: false } : block)), [false, false, false, true, false, false, false, false]);
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
