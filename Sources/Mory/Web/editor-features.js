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

export const calendarColors = ["red", "amber", "green", "blue", "violet", "gray"];
export const mermaidColorThemes = ["auto", "ocean", "forest", "sunset", "mono"];

export function normalizeMermaidColorTheme(value) {
  return mermaidColorThemes.includes(String(value || "").toLocaleLowerCase())
    ? String(value).toLocaleLowerCase()
    : "auto";
}

const calendarDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/;
const calendarMonthPattern = /^(\d{4})-(\d{2})$/;

function normalizedCalendarText(value, maximum = 120) {
  return typeof value === "string" ? value.replace(/\s+/g, " ").trim().slice(0, maximum) : "";
}

function calendarLocalDate(year, month, day) {
  const date = new Date(0);
  date.setHours(0, 0, 0, 0);
  date.setFullYear(year, month, day);
  return date;
}

export function localDateKey(value = new Date()) {
  const localMatch = typeof value === "string" ? value.match(calendarDatePattern) : null;
  const date = localMatch
    ? calendarLocalDate(Number(localMatch[1]), Number(localMatch[2]) - 1, Number(localMatch[3]))
    : new Date(value);
  if (Number.isNaN(date.getTime())) throw new TypeError("A valid calendar date is required");
  const year = String(date.getFullYear()).padStart(4, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function calendarDateFromKey(value) {
  const match = String(value ?? "").match(calendarDatePattern);
  if (!match) return null;
  const [, year, month, day] = match.map(Number);
  const date = calendarLocalDate(year, month - 1, day);
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day ? date : null;
}

function validCalendarMonth(value) {
  const match = String(value ?? "").match(calendarMonthPattern);
  if (!match) return "";
  const month = Number(match[2]);
  return month >= 1 && month <= 12 ? `${match[1]}-${match[2]}` : "";
}

function normalizedCalendarColor(value) {
  return calendarColors.includes(value) ? value : "blue";
}

export function normalizeCalendarDocument(value, fallbackDate = new Date()) {
  const source = value && typeof value === "object" ? value : {};
  const fallbackMonth = localDateKey(fallbackDate).slice(0, 7);
  const marksByDate = new Map();
  for (const mark of Array.isArray(source.marks) ? source.marks : []) {
    if (!calendarDateFromKey(mark?.date)) continue;
    marksByDate.set(mark.date, {
      date: mark.date,
      color: normalizedCalendarColor(mark.color),
      title: normalizedCalendarText(mark.title)
    });
  }
  const ranges = [];
  for (const range of Array.isArray(source.ranges) ? source.ranges : []) {
    if (!calendarDateFromKey(range?.start) || !calendarDateFromKey(range?.end)) continue;
    const [start, end] = range.start <= range.end ? [range.start, range.end] : [range.end, range.start];
    const title = normalizedCalendarText(range.title);
    if (!title) continue;
    ranges.push({ start, end, color: normalizedCalendarColor(range.color), title });
  }
  const items = [];
  for (const item of Array.isArray(source.items) ? source.items : []) {
    if (!calendarDateFromKey(item?.date)) continue;
    const text = normalizedCalendarText(item.text, 240);
    if (!text) continue;
    items.push({ date: item.date, text, done: Boolean(item.done) });
  }
  return {
    version: 1,
    month: validCalendarMonth(source.month) || fallbackMonth,
    marks: [...marksByDate.values()].sort((left, right) => left.date.localeCompare(right.date)),
    ranges: ranges.sort((left, right) => left.start.localeCompare(right.start) || left.end.localeCompare(right.end) || left.title.localeCompare(right.title)),
    items: items.sort((left, right) => left.date.localeCompare(right.date) || left.text.localeCompare(right.text))
  };
}

export function parseCalendarSource(source) {
  try {
    const value = JSON.parse(String(source ?? ""));
    if (!value || typeof value !== "object" || Array.isArray(value) || value.version !== 1) return null;
    return normalizeCalendarDocument(value);
  } catch {
    return null;
  }
}

export function serializeCalendarDocument(value) {
  return JSON.stringify(normalizeCalendarDocument(value), null, 2);
}

export function calendarMarkdown(value = new Date()) {
  const document = value instanceof Date || typeof value === "number" || typeof value === "string"
    ? normalizeCalendarDocument({}, value)
    : normalizeCalendarDocument(value);
  return `\`\`\`calendar\n${serializeCalendarDocument(document)}\n\`\`\``;
}

export function calendarMonthDays(month) {
  const normalizedMonth = validCalendarMonth(month);
  if (!normalizedMonth) throw new TypeError("A valid calendar month is required");
  const [year, monthNumber] = normalizedMonth.split("-").map(Number);
  const first = calendarLocalDate(year, monthNumber - 1, 1);
  const start = calendarLocalDate(year, monthNumber - 1, 1 - ((first.getDay() + 6) % 7));
  return Array.from({ length: 42 }, (_, index) => {
    const date = calendarLocalDate(start.getFullYear(), start.getMonth(), start.getDate() + index);
    return {
      date: localDateKey(date),
      day: date.getDate(),
      currentMonth: date.getFullYear() === year && date.getMonth() === monthNumber - 1,
      weekday: index % 7,
      week: Math.floor(index / 7)
    };
  });
}

export function calendarRangeDayCount(start, end) {
  const first = calendarDateFromKey(start);
  const last = calendarDateFromKey(end);
  if (!first || !last) return 0;
  const [minimum, maximum] = first <= last ? [first, last] : [last, first];
  const utcStart = Date.UTC(minimum.getFullYear(), minimum.getMonth(), minimum.getDate());
  const utcEnd = Date.UTC(maximum.getFullYear(), maximum.getMonth(), maximum.getDate());
  return Math.round((utcEnd - utcStart) / 86400000) + 1;
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

export function headingFoldVisibility(blocks) {
  const collapsedLevels = [];
  return Array.from(blocks ?? [], block => {
    const level = Number(block?.headingLevel);
    const isHeading = Number.isInteger(level) && level >= 1 && level <= 6;
    if (isHeading) {
      while (collapsedLevels.length && collapsedLevels.at(-1) >= level) collapsedLevels.pop();
    }
    const hidden = collapsedLevels.length > 0;
    if (isHeading && level <= 4 && block?.collapsed) collapsedLevels.push(level);
    return hidden;
  });
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
