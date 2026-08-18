function protectMarkdownSyntax(source) {
  const values = [];
  const token = value => {
    const index = values.push(value) - 1;
    return `\uE100${index}\uE101`;
  };
  const protectedSource = String(source)
    .replace(/```[\s\S]*?```|~~~[\s\S]*?~~~/g, token)
    .replace(/`[^`\n]*`/g, token)
    .replace(/(?<=\]\()[^)\s]+(?=(?:\s+["'][^"']*["'])?\))/g, token)
    .replace(/https?:\/\/[^\s)]+/g, token)
    .replace(/<[^>\n]+>/g, token)
    .replace(/\\[!-/:-@[-`{-~]|\*\*|__|~~/g, token);
  return {
    source: protectedSource,
    restore(value) {
      return String(value).replace(/\uE100(\d+)\uE101/g, (_, index) => values[Number(index)] ?? "");
    }
  };
}

export function optimizeMarkdownTypography(markdown, spacingText) {
  if (typeof spacingText !== "function") throw new TypeError("A text-spacing function is required");
  const protectedMarkdown = protectMarkdownSyntax(String(markdown ?? ""));
  return protectedMarkdown.restore(spacingText(protectedMarkdown.source));
}

export function calendarMarkdown(date = new Date(), locale = "zh-CN") {
  const selected = new Date(date);
  if (Number.isNaN(selected.getTime())) throw new TypeError("A valid calendar date is required");
  const year = selected.getFullYear();
  const month = selected.getMonth();
  const labels = locale === "en"
    ? ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
    : ["一", "二", "三", "四", "五", "六", "日"];
  const firstWeekday = (new Date(year, month, 1).getDay() + 6) % 7;
  const days = new Date(year, month + 1, 0).getDate();
  const cells = [...Array(firstWeekday).fill(""), ...Array.from({ length: days }, (_, index) => String(index + 1))];
  while (cells.length % 7) cells.push("");
  const rows = [];
  for (let index = 0; index < cells.length; index += 7) rows.push(`| ${cells.slice(index, index + 7).join(" | ")} |`);
  return `| ${labels.join(" | ")} |\n| ${labels.map(() => "---").join(" | ")} |\n${rows.join("\n")}`;
}

export function formatFileSize(bytes) {
  const value = Number(bytes);
  if (!Number.isFinite(value) || value < 0) return "";
  if (value < 1024) return `${Math.round(value)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let scaled = value / 1024;
  let unit = 0;
  while (scaled >= 1024 && unit < units.length - 1) {
    scaled /= 1024;
    unit += 1;
  }
  const digits = scaled >= 100 ? 0 : 1;
  return `${scaled.toFixed(digits).replace(/\.0$/, "")} ${units[unit]}`;
}

export function formatUpdatedAt(value, locale = "zh-CN") {
  const date = new Date(Number(value));
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(locale === "en" ? "en-US" : "zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}

function plainHeadingText(value) {
  return String(value)
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/[*_`~]/g, "")
    .replace(/<[^>]+>/g, "")
    .trim();
}

export function markdownHeadingTree(markdown, title = "Untitled") {
  const root = { text: String(title || "Untitled"), level: 0, children: [] };
  const stack = [{ level: 0, node: root }];
  let fence = "";
  for (const line of String(markdown ?? "").replace(/\r\n?/g, "\n").split("\n")) {
    const fenceMatch = line.match(/^\s*(```|~~~)/);
    if (fenceMatch) {
      fence = fence ? (fence === fenceMatch[1] ? "" : fence) : fenceMatch[1];
      continue;
    }
    if (fence) continue;
    const heading = line.match(/^(#{1,6})\s+(.+?)\s*#*\s*$/);
    if (!heading) continue;
    const text = plainHeadingText(heading[2]);
    if (!text) continue;
    const level = heading[1].length;
    if (root.children.length === 0 && level === 1 && text === root.text) {
      stack.splice(1);
      stack.push({ level, node: root });
      continue;
    }
    while (stack.length > 1 && stack.at(-1).level >= level) stack.pop();
    const node = { text, level, children: [] };
    stack.at(-1).node.children.push(node);
    stack.push({ level, node });
  }
  return root;
}

function mindMapEntries(tree) {
  const entries = [];
  const visit = (node, depth, parent = null) => {
    const entry = { node, depth, parent, index: entries.length };
    entries.push(entry);
    node.children.forEach(child => visit(child, depth + 1, entry));
  };
  visit(tree, 0);
  return entries;
}

export function mindMapHTML(markdown, title = "Untitled", locale = "zh-CN") {
  const tree = markdownHeadingTree(markdown, title);
  const entries = mindMapEntries(tree);
  const maxDepth = Math.max(...entries.map(entry => entry.depth));
  const width = Math.max(720, 260 + maxDepth * 240);
  const height = Math.max(320, 100 + entries.length * 70);
  const position = entry => ({ x: 38 + entry.depth * 240, y: 42 + entry.index * 70 });
  const edges = entries.filter(entry => entry.parent).map(entry => {
    const from = position(entry.parent);
    const to = position(entry);
    return `<path d="M${from.x + 190} ${from.y + 23} C${from.x + 215} ${from.y + 23},${to.x - 25} ${to.y + 23},${to.x} ${to.y + 23}"/>`;
  }).join("");
  const nodes = entries.map((entry, index) => {
    const { x, y } = position(entry);
    const label = entry.node.text.length > 28 ? `${entry.node.text.slice(0, 27)}…` : entry.node.text;
    return `<g class="node${index === 0 ? " root" : ""}" transform="translate(${x} ${y})"><rect width="190" height="46" rx="9"/><text x="14" y="28">${escapeFeatureHTML(label)}</text><title>${escapeFeatureHTML(entry.node.text)}</title></g>`;
  }).join("");
  const description = locale === "en" ? "Generated from Markdown headings by Mory" : "由 Mory 根据 Markdown 标题生成";
  return `<!doctype html>\n<html lang="${locale}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>${escapeFeatureHTML(title)} - Mind Map</title><style>*{box-sizing:border-box}html,body{margin:0;min-height:100%;background:#f6f8fa;color:#1f2328;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}header{padding:24px 32px 12px}h1{margin:0;font-size:22px}p{margin:7px 0;color:#656d76;font-size:13px}main{padding:8px 24px 32px;overflow:auto}svg{display:block;min-width:100%;height:auto}path{fill:none;stroke:#8c959f;stroke-width:1.5}.node rect{fill:#fff;stroke:#afb8c1}.node text{fill:#1f2328;font-size:13px}.node.root rect{fill:#0969da;stroke:#0969da}.node.root text{fill:#fff;font-weight:700}@media(prefers-color-scheme:dark){html,body{background:#0d1117;color:#e6edf3}p{color:#8b949e}path{stroke:#6e7681}.node rect{fill:#161b22;stroke:#30363d}.node text{fill:#e6edf3}.node.root rect{fill:#1f6feb;stroke:#1f6feb}}</style></head><body><header><h1>${escapeFeatureHTML(title)}</h1><p>${description}</p></header><main><svg viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" role="img" aria-label="${escapeFeatureHTML(title)}">${edges}${nodes}</svg></main></body></html>`;
}

function escapeFeatureHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}
