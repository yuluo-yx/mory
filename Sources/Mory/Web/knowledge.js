const markdownExtensions = [".md", ".markdown", ".mmd", ".mdown", ".mkd", ".txt", ".text"];

function normalizeGraphPath(value) {
  return String(value || "")
    .replaceAll("\\", "/")
    .replace(/^\.\//, "")
    .replace(/\/{2,}/g, "/")
    .replace(/^\//, "");
}

function withoutExtension(value) {
  const normalized = normalizeGraphPath(value);
  const extension = markdownExtensions.find(item => normalized.toLocaleLowerCase().endsWith(item));
  return extension ? normalized.slice(0, -extension.length) : normalized;
}

function graphTitle(document) {
  const heading = String(document.markdown || "").match(/^#\s+(.+)$/m)?.[1]
    ?.replace(/[*_`~]/g, "").trim();
  const filename = withoutExtension(document.name || document.path || "未命名").split("/").pop();
  return heading || filename || "未命名";
}

function stripIgnoredMarkdown(markdown) {
  return String(markdown || "")
    .replace(/```[\s\S]*?```/g, "")
    .replace(/~~~[\s\S]*?~~~/g, "")
    .replace(/`[^`\n]*`/g, "");
}

function documentReferences(markdown) {
  const source = stripIgnoredMarkdown(markdown);
  const references = [];
  for (const match of source.matchAll(/!?\[\[([^\]\n]+)\]\]/g)) {
    const target = match[1].split("|")[0].split("#")[0].trim();
    if (target) references.push({ type: "wiki", target });
  }
  for (const match of source.matchAll(/(?<!!)\[[^\]]*\]\((?:<([^>]+)>|([^\s)]+))(?:\s+["'][^"']*["'])?\)/g)) {
    let target = (match[1] || match[2] || "").trim();
    if (!target || /^(?:[a-z][a-z\d+.-]*:|\/|#)/i.test(target)) continue;
    try { target = decodeURI(target); } catch { /* Continue matching with the original link. */ }
    target = target.split("#")[0].split("?")[0];
    if (target) references.push({ type: "markdown", target });
  }
  return references;
}

function dirname(value) {
  const normalized = normalizeGraphPath(value);
  const index = normalized.lastIndexOf("/");
  return index < 0 ? "" : normalized.slice(0, index);
}

function resolveRelative(base, target) {
  const parts = normalizeGraphPath(`${base}/${target}`).split("/");
  const resolved = [];
  for (const part of parts) {
    if (!part || part === ".") continue;
    if (part === "..") resolved.pop();
    else resolved.push(part);
  }
  return resolved.join("/");
}

export function buildKnowledgeGraph(documents = []) {
  const unique = new Map();
  for (const document of documents) {
    const relative = normalizeGraphPath(document.name || document.path);
    if (!relative) continue;
    unique.set(relative.toLocaleLowerCase(), { ...document, relative });
  }
  const nodes = [...unique.values()].map(document => ({
    id: document.relative,
    name: document.relative,
    path: document.path || "",
    title: graphTitle(document),
    markdown: String(document.markdown || ""),
    degree: 0,
    incoming: 0,
    outgoing: 0,
    backlinks: [],
    forwardLinks: []
  }));
  const byPath = new Map();
  const byStem = new Map();
  const byBasename = new Map();
  for (const node of nodes) {
    const pathKey = normalizeGraphPath(node.name).toLocaleLowerCase();
    const stemKey = withoutExtension(pathKey).toLocaleLowerCase();
    const baseKey = stemKey.split("/").pop();
    byPath.set(pathKey, node);
    if (!byStem.has(stemKey)) byStem.set(stemKey, []);
    byStem.get(stemKey).push(node);
    if (!byBasename.has(baseKey)) byBasename.set(baseKey, []);
    byBasename.get(baseKey).push(node);
  }
  const findNode = (source, reference) => {
    const raw = normalizeGraphPath(reference.target);
    const relative = reference.type === "markdown" ? resolveRelative(dirname(source.name), raw) : raw;
    const candidates = [relative, ...markdownExtensions.map(extension => `${relative}${extension}`)];
    for (const candidate of candidates) {
      const node = byPath.get(candidate.toLocaleLowerCase());
      if (node) return node;
    }
    const stem = withoutExtension(relative).toLocaleLowerCase();
    const exact = byStem.get(stem);
    if (exact?.length === 1) return exact[0];
    const basename = byBasename.get(stem.split("/").pop());
    return basename?.length === 1 ? basename[0] : null;
  };
  const edgeKeys = new Set();
  const edges = [];
  for (const source of nodes) {
    for (const reference of documentReferences(source.markdown)) {
      const target = findNode(source, reference);
      if (!target || target.id === source.id) continue;
      const key = `${source.id}\u0000${target.id}`;
      if (edgeKeys.has(key)) continue;
      edgeKeys.add(key);
      edges.push({ source: source.id, target: target.id, type: reference.type });
      source.outgoing += 1;
      target.incoming += 1;
      source.degree += 1;
      target.degree += 1;
      source.forwardLinks.push(target.id);
      target.backlinks.push(source.id);
    }
  }
  const edgeSet = new Set(edges.map(edge => `${edge.source}\u0000${edge.target}`));
  for (const edge of edges) edge.mutual = edgeSet.has(`${edge.target}\u0000${edge.source}`);
  return { nodes, edges };
}

export { documentReferences, normalizeGraphPath };
