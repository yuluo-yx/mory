import assert from "node:assert/strict";
import test from "node:test";

import { buildKnowledgeGraph, documentReferences } from "../Sources/Mory/Web/knowledge.js";

test("知识图谱解析双向链接和相对 Markdown 链接", () => {
  const graph = buildKnowledgeGraph([
    { name: "首页.md", path: "/workspace/首页.md", markdown: "# 首页\n[[专题/设计|设计入口]]\n[英文](notes/english.md#top)" },
    { name: "专题/设计.md", path: "/workspace/专题/设计.md", markdown: "# 设计系统\n[返回](../首页.md)" },
    { name: "notes/english.md", path: "/workspace/notes/english.md", markdown: "# English" },
    { name: "孤立.md", path: "/workspace/孤立.md", markdown: "# 孤立文稿" }
  ]);
  assert.equal(graph.nodes.length, 4);
  assert.deepEqual(graph.edges.map(edge => [edge.source, edge.target]), [
    ["首页.md", "专题/设计.md"], ["首页.md", "notes/english.md"], ["专题/设计.md", "首页.md"]
  ]);
  assert.equal(graph.nodes.find(node => node.id === "孤立.md").degree, 0);
  assert.equal(graph.nodes.find(node => node.id === "首页.md").title, "首页");
  assert.deepEqual(graph.nodes.find(node => node.id === "首页.md").forwardLinks, ["专题/设计.md", "notes/english.md"]);
  assert.deepEqual(graph.nodes.find(node => node.id === "首页.md").backlinks, ["专题/设计.md"]);
  assert.deepEqual(graph.nodes.find(node => node.id === "专题/设计.md").backlinks, ["首页.md"]);
  assert.deepEqual(graph.edges.map(edge => edge.mutual), [true, false, true]);
});

test("知识图谱忽略代码、图片和外部链接", () => {
  const references = documentReferences("`[[行内]]`\n```md\n[[代码]]\n```\n![图](目标.md)\n[站点](https://example.com)\n[[正文]]");
  assert.deepEqual(references, [{ type: "wiki", target: "正文" }]);
});

test("同名文稿的短双链不会产生歧义边", () => {
  const graph = buildKnowledgeGraph([
    { name: "入口.md", markdown: "[[说明]]" },
    { name: "A/说明.md", markdown: "" },
    { name: "B/说明.md", markdown: "" }
  ]);
  assert.equal(graph.edges.length, 0);
});
