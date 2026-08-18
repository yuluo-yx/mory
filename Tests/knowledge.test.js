import assert from "node:assert/strict";
import test from "node:test";

import { buildKnowledgeGraph, documentReferences } from "../Sources/Mory/Web/knowledge.js";

test("the knowledge graph resolves wiki links and relative Markdown links", () => {
  const graph = buildKnowledgeGraph([
    { name: "\u9996\u9875.md", path: "/workspace/\u9996\u9875.md", markdown: "# \u9996\u9875\n[[\u4E13\u9898/\u8BBE\u8BA1|\u8BBE\u8BA1\u5165\u53E3]]\n[\u82F1\u6587](notes/english.md#top)" },
    { name: "\u4E13\u9898/\u8BBE\u8BA1.md", path: "/workspace/\u4E13\u9898/\u8BBE\u8BA1.md", markdown: "# \u8BBE\u8BA1\u7CFB\u7EDF\n[\u8FD4\u56DE](../\u9996\u9875.md)" },
    { name: "notes/english.md", path: "/workspace/notes/english.md", markdown: "# English" },
    { name: "\u5B64\u7ACB.md", path: "/workspace/\u5B64\u7ACB.md", markdown: "# \u5B64\u7ACB\u6587\u7A3F" }
  ]);
  assert.equal(graph.nodes.length, 4);
  assert.deepEqual(graph.edges.map(edge => [edge.source, edge.target]), [
    ["\u9996\u9875.md", "\u4E13\u9898/\u8BBE\u8BA1.md"], ["\u9996\u9875.md", "notes/english.md"], ["\u4E13\u9898/\u8BBE\u8BA1.md", "\u9996\u9875.md"]
  ]);
  assert.equal(graph.nodes.find(node => node.id === "\u5B64\u7ACB.md").degree, 0);
  assert.equal(graph.nodes.find(node => node.id === "\u9996\u9875.md").title, "\u9996\u9875");
  assert.deepEqual(graph.nodes.find(node => node.id === "\u9996\u9875.md").forwardLinks, ["\u4E13\u9898/\u8BBE\u8BA1.md", "notes/english.md"]);
  assert.deepEqual(graph.nodes.find(node => node.id === "\u9996\u9875.md").backlinks, ["\u4E13\u9898/\u8BBE\u8BA1.md"]);
  assert.deepEqual(graph.nodes.find(node => node.id === "\u4E13\u9898/\u8BBE\u8BA1.md").backlinks, ["\u9996\u9875.md"]);
  assert.deepEqual(graph.edges.map(edge => edge.mutual), [true, false, true]);
});

test("the knowledge graph ignores code, images, and external links", () => {
  const references = documentReferences("`[[\u884C\u5185]]`\n```md\n[[\u4EE3\u7801]]\n```\n![\u56FE](\u76EE\u6807.md)\n[\u7AD9\u70B9](https://example.com)\n[[\u6B63\u6587]]");
  assert.deepEqual(references, [{ type: "wiki", target: "\u6B63\u6587" }]);
});

test("short wiki links do not create ambiguous edges for duplicate names", () => {
  const graph = buildKnowledgeGraph([
    { name: "\u5165\u53E3.md", markdown: "[[\u8BF4\u660E]]" },
    { name: "A/\u8BF4\u660E.md", markdown: "" },
    { name: "B/\u8BF4\u660E.md", markdown: "" }
  ]);
  assert.equal(graph.edges.length, 0);
});
