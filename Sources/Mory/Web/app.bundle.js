/* 此文件由 scripts/build-web.mjs 生成，请勿直接编辑。
 * 经典脚本用于兼容 file:// 下不执行 ES module 的 macOS WKWebView。 */

const blockStart = /^(#{1,6}\s|>|[-*+]\s|\d+[.)]\s|```|~~~| {0,3}([-*_])(?:\s*\2){2,}\s*$)/;

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function safeURL(value) {
  const url = value.trim().replaceAll('"', "%22");
  return /^(?:javascript|vbscript):/i.test(url) ? "#" : url;
}

function inlineMarkdown(source) {
  const tokens = [];
  let value = escapeHTML(source);

  value = value.replace(/`([^`\n]+)`/g, (_, code) => {
    const token = `\u0000CODE${tokens.length}\u0000`;
    tokens.push(`<code>${code}</code>`);
    return token;
  });
  value = value.replace(/!\[([^\]]*)\]\(([^\s)]+)(?:\s+(?:&quot;([^&]*?)&quot;|'([^']*)'))?\)/g, (_, alt, url, doubleTitle, singleTitle) => {
    const title = doubleTitle ?? singleTitle;
    const titleAttr = title ? ` title="${escapeHTML(title)}"` : "";
    return `<img src="${escapeHTML(safeURL(url))}" alt="${alt}"${titleAttr}>`;
  });
  value = value.replace(/\[([^\]]+)\]\(([^\s)]+)(?:\s+(?:&quot;([^&]*?)&quot;|'([^']*)'))?\)/g, (_, label, url, doubleTitle, singleTitle) => {
    const title = doubleTitle ?? singleTitle;
    const titleAttr = title ? ` title="${escapeHTML(title)}"` : "";
    return `<a href="${escapeHTML(safeURL(url))}"${titleAttr}>${label}</a>`;
  });
  value = value.replace(/\*\*([^*\n]+)\*\*|__([^_\n]+)__/g, "<strong>$1$2</strong>");
  value = value.replace(/~~([^~\n]+)~~/g, "<del>$1</del>");
  value = value.replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, "$1<em>$2</em>");
  value = value.replace(/(^|[^_])_([^_\n]+)_(?!_)/g, "$1<em>$2</em>");
  value = value.replace(/ {2}\n/g, "<br>");
  value = value.replace(/\u0000CODE(\d+)\u0000/g, (_, index) => tokens[Number(index)]);
  return value;
}

function isTableSeparator(line) {
  return /^\s*\|?\s*:?-{3,}:?\s*(?:\|\s*:?-{3,}:?\s*)+\|?\s*$/.test(line);
}

function splitTableRow(line) {
  return line.trim().replace(/^\|/, "").replace(/\|$/, "").split(/(?<!\\)\|/).map(cell => cell.trim().replaceAll("\\|", "|"));
}

function parseFenceInfo(value) {
  const info = String(value ?? "").trim();
  const titleMatch = info.match(/(?:^|\s)title=("(?:\\.|[^"])*"|'[^']*'|\S+)/i);
  let title = "";
  if (titleMatch) {
    const token = titleMatch[1];
    if (token.startsWith('"')) {
      try { title = JSON.parse(token); } catch { title = token.slice(1, -1); }
    } else {
      title = token.startsWith("'") ? token.slice(1, -1) : token;
    }
  }
  const languagePart = (titleMatch ? `${info.slice(0, titleMatch.index)} ${info.slice((titleMatch.index ?? 0) + titleMatch[0].length)}` : info).trim();
  const attributeLanguage = languagePart.match(/^\{\s*\.([^\s}]+)/)?.[1];
  return { language: attributeLanguage || languagePart.split(/\s+/)[0] || "", title };
}

function fenceTitle(value) {
  return value ? ` title=${JSON.stringify(String(value))}` : "";
}

function markdownToHTML(markdown) {
  const lines = String(markdown ?? "").replace(/\r\n?/g, "\n").split("\n");
  const html = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    if (!line.trim()) { index += 1; continue; }

    const fence = line.match(/^\s*(```|~~~)\s*(.*?)\s*$/);
    if (fence) {
      const marker = fence[1];
      const { language, title } = parseFenceInfo(fence[2]);
      const code = [];
      index += 1;
      while (index < lines.length && !new RegExp(`^\\s*${marker}`).test(lines[index])) code.push(lines[index++]);
      if (index < lines.length) index += 1;
      const source = escapeHTML(code.join("\n"));
      if (language.toLocaleLowerCase() === "mermaid") {
        html.push(`<div class="mermaid-diagram" data-mermaid-source="${source}" data-mermaid-state="pending" contenteditable="false"><pre class="mermaid-source"><code>${source}</code></pre></div>`);
      } else {
        const titleAttribute = title ? ` data-title="${escapeHTML(title)}"` : "";
        html.push(`<pre data-language="${escapeHTML(language)}"${titleAttribute}><code>${source}</code></pre>`);
      }
      continue;
    }

    const heading = line.match(/^(#{1,6})\s+(.+?)\s*#*$/);
    if (heading) {
      const level = heading[1].length;
      html.push(`<h${level}>${inlineMarkdown(heading[2])}</h${level}>`);
      index += 1;
      continue;
    }

    if (index + 1 < lines.length && line.includes("|") && isTableSeparator(lines[index + 1])) {
      const headers = splitTableRow(line);
      index += 2;
      const rows = [];
      while (index < lines.length && lines[index].includes("|") && lines[index].trim()) rows.push(splitTableRow(lines[index++]));
      const head = headers.map(cell => `<th>${inlineMarkdown(cell)}</th>`).join("");
      const body = rows.map(row => `<tr>${headers.map((_, cellIndex) => `<td>${inlineMarkdown(row[cellIndex] ?? "")}</td>`).join("")}</tr>`).join("");
      html.push(`<table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`);
      continue;
    }

    if (/^\s*>/.test(line)) {
      const quoted = [];
      while (index < lines.length && /^\s*>/.test(lines[index])) quoted.push(lines[index++].replace(/^\s*>\s?/, ""));
      html.push(`<blockquote>${quoted.map(item => `<p>${inlineMarkdown(item)}</p>`).join("")}</blockquote>`);
      continue;
    }

    const listMatch = line.match(/^\s*([-*+]|\d+[.)])\s+(.+)$/);
    if (listMatch) {
      const ordered = /^\d/.test(listMatch[1]);
      const tag = ordered ? "ol" : "ul";
      const items = [];
      while (index < lines.length) {
        const item = lines[index].match(/^\s*([-*+]|\d+[.)])\s+(.+)$/);
        if (!item || /^\d/.test(item[1]) !== ordered) break;
        const task = item[2].match(/^\[([ xX])\]\s*(.*)$/);
        if (task) {
          const checked = task[1].toLowerCase() === "x" ? " checked" : "";
          items.push(`<li class="task-item"><input type="checkbox"${checked}>${inlineMarkdown(task[2])}</li>`);
        } else {
          items.push(`<li>${inlineMarkdown(item[2])}</li>`);
        }
        index += 1;
      }
      html.push(`<${tag}>${items.join("")}</${tag}>`);
      continue;
    }

    if (/^\s*([-*_])(?:\s*\1){2,}\s*$/.test(line)) {
      html.push("<hr>");
      index += 1;
      continue;
    }

    const paragraph = [line];
    index += 1;
    while (index < lines.length && lines[index].trim() && !blockStart.test(lines[index]) && !(index + 1 < lines.length && isTableSeparator(lines[index + 1]))) {
      paragraph.push(lines[index++]);
    }
    html.push(`<p>${inlineMarkdown(paragraph.join("\n")).replaceAll("\n", " ")}</p>`);
  }

  return html.join("\n");
}

function inlineNodeToMarkdown(node) {
  if (node.nodeType === Node.TEXT_NODE) return (node.nodeValue ?? "").replaceAll("\u200b", "");
  if (node.nodeType !== Node.ELEMENT_NODE) return "";
  const element = /** @type {HTMLElement} */ (node);
  const content = [...element.childNodes].map(inlineNodeToMarkdown).join("");
  switch (element.tagName) {
    case "STRONG": case "B": return `**${content}**`;
    case "EM": case "I": return `*${content}*`;
    case "DEL": case "S": case "STRIKE": return `~~${content}~~`;
    case "CODE": return `\`${content.replaceAll("`", "\\`")}\``;
    case "A": return `[${content}](${element.getAttribute("href") ?? ""})`;
    case "IMG": return `![${element.getAttribute("alt") ?? ""}](${element.getAttribute("src") ?? ""})`;
    case "BR": return "  \n";
    case "INPUT": return element.getAttribute("type") === "checkbox" ? `[${/** @type {HTMLInputElement} */ (element).checked ? "x" : " "}] ` : "";
    default: return content;
  }
}

function tableToMarkdown(table) {
  const rows = [...table.querySelectorAll("tr")].map(row => [...row.children].map(cell => inlineNodeToMarkdown(cell).replaceAll("|", "\\|").trim()));
  if (!rows.length) return "";
  const width = Math.max(...rows.map(row => row.length));
  const normalized = rows.map(row => Array.from({ length: width }, (_, index) => row[index] ?? ""));
  const separator = Array.from({ length: width }, () => "---");
  return [normalized[0], separator, ...normalized.slice(1)].map(row => `| ${row.join(" | ")} |`).join("\n");
}

function editorToMarkdown(root) {
  const blocks = [];
  for (const node of root.childNodes) {
    if (node.nodeType === Node.TEXT_NODE) {
      const value = (node.nodeValue ?? "").replaceAll("\u200b", "").trim();
      if (value) blocks.push(value);
      continue;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) continue;
    const element = /** @type {HTMLElement} */ (node);
    if (element.matches?.(".code-meta")) continue;
    if (element.tagName === "DIV" && (element.getAttribute("class") ?? "").split(/\s+/).includes("mermaid-diagram")) {
      const source = element.dataset.mermaidSource ?? "";
      blocks.push(`\`\`\`mermaid\n${source}\n\`\`\``);
      continue;
    }
    const content = [...element.childNodes].map(inlineNodeToMarkdown).join("").trim();
    switch (element.tagName) {
      case "H1": case "H2": case "H3": case "H4": case "H5": case "H6":
        blocks.push(`${"#".repeat(Number(element.tagName[1]))} ${content}`); break;
      case "P": case "DIV": blocks.push(content); break;
      case "BLOCKQUOTE":
        blocks.push(element.innerText.split("\n").map(line => `> ${line}`).join("\n")); break;
      case "UL": case "OL": {
        const ordered = element.tagName === "OL";
        const items = [...element.children].map((item, index) => {
          const task = item.querySelector(':scope > input[type="checkbox"]');
          const text = [...item.childNodes].filter(child => child !== task).map(inlineNodeToMarkdown).join("").trim();
          if (task) return `- [${task.checked ? "x" : " "}] ${text}`;
          return `${ordered ? `${index + 1}.` : "-"} ${text}`;
        });
        blocks.push(items.join("\n"));
        break;
      }
      case "PRE": {
        const language = element.dataset.language ?? "";
        const title = fenceTitle(element.dataset.title ?? "");
        const code = element.innerText.replaceAll("\u200b", "").replace(/\n$/, "");
        blocks.push(`\`\`\`${language}${title}\n${code}\n\`\`\``); break;
      }
      case "TABLE": blocks.push(tableToMarkdown(element)); break;
      case "HR": blocks.push("---"); break;
      default: if (content) blocks.push(content);
    }
  }
  return blocks.join("\n\n").replace(/\n{3,}/g, "\n\n").trimEnd();
}

function documentStats(markdown) {
  const value = String(markdown ?? "");
  const plain = value
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/!?(?:\[([^\]]*)\])\([^)]*\)/g, "$1")
    .replace(/[#>*_~`|\-[\]]/g, " ");
  const chinese = plain.match(/[\u3400-\u9fff]/g)?.length ?? 0;
  const words = plain.match(/[A-Za-z0-9]+(?:['’-][A-Za-z0-9]+)*/g)?.length ?? 0;
  return {
    words: chinese + words,
    characters: plain.replace(/\s/g, "").length,
    lines: value ? value.split(/\r?\n/).length : 0
  };
}


const $ = selector => document.querySelector(selector);
const $$ = selector => [...document.querySelectorAll(selector)];
const write = $("#write");
const sourceEditor = $("#source-editor");
const workspace = $(".workspace");
const editorScroll = $("#editor-scroll");
const nativeMacHost = Boolean(window.webkit?.messageHandlers?.mory);
document.documentElement.dataset.host = nativeMacHost ? "mac-native" : (window.moryNative?.platform || "browser");

const defaultMarkdown = `# Mory Markdown 编辑器

这是一份可直接编辑的 Markdown 文档。界面和工作流参考了本机 Typora 的模块组织，但代码为独立实现。

## 所见即所得

在正文里直接输入，Markdown 标记会被渲染为排版后的内容。你也可以按 **⌘ /** 切换到源代码模式。

> 写作工具应该安静地待在文字后面。

### 已实现

- [x] 原生 macOS 窗口与菜单
- [x] Markdown 打开、保存与另存为
- [x] 文件列表、大纲和快速打开
- [x] 查找替换、源码模式和 HTML 导出
- [x] 浅色、深色、专注与打字机模式
- [x] Mermaid 图表渲染与主题化导出
- [ ] 数学公式扩展

| 快捷键 | 功能 |
| --- | --- |
| ⌘ B | 加粗 |
| ⌘ I | 斜体 |
| ⌘ F | 查找 |
| ⌘ P | 快速打开 |

\`开始修改这份文档吧。\`
`;

const state = {
  markdown: "",
  documents: [],
  files: [],
  activeDocumentId: null,
  documentSerial: 0,
  untitledSequence: 0,
  sourceMode: false,
  dirty: false,
  findMatches: [],
  findIndex: -1,
  zoom: 1,
  titleTouched: false,
  documentTheme: "yuluo-css",
  themeCSS: new Map()
};

let changeTimer;
let toastTimer;
let mermaidSequence = 0;
let mermaidQueue = Promise.resolve();
let windowDragPointer = null;
let windowDragFrame = 0;
let pendingCodeExit = null;
let recentCompositionCommit = null;
let activeComposition = null;
const caretMarker = "\u200b";
const renderCaretMarker = "\ue000";
const doubleEnterWindow = 650;

function bridge(payload) {
  if (window.webkit?.messageHandlers?.mory) {
    window.webkit.messageHandlers.mory.postMessage(payload);
  } else {
    window.moryNative?.send(payload);
  }
}

function toast(message) {
  const element = $("#toast");
  element.textContent = message;
  element.classList.add("is-visible");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => element.classList.remove("is-visible"), 1500);
}

function activeDocument() {
  return state.documents.find(document => document.id === state.activeDocumentId) || null;
}

function nextDocumentId() {
  state.documentSerial += 1;
  return `document-${state.documentSerial}`;
}

function untitledName(sequence) {
  return sequence === 1 ? "未命名.md" : `未命名 ${sequence}.md`;
}

function renderDocument(document, announce = false) {
  clearTimeout(changeTimer);
  state.markdown = document.markdown;
  state.dirty = document.dirty;
  state.titleTouched = false;
  sourceEditor.value = state.markdown;
  write.innerHTML = markdownToHTML(state.markdown) || "<p><br></p>";
  highlightCodeBlocks(write);
  $("#save-state").textContent = state.dirty ? "未保存" : "已保存";
  $("#save-state").classList.toggle("is-visible", state.dirty);
  updateDerivedState();
  renderFiles();
  void renderMermaidDiagrams(write, state.documentTheme);
  if (announce) toast("已切换文档");
}

function notifyDocumentSelected(document) {
  bridge({
    type: "documentSelected",
    documentId: document.id,
    name: document.name,
    path: document.path || "",
    markdown: document.markdown,
    dirty: document.dirty
  });
}

function activateDocument(documentId, { announce = false, notifyHost = true } = {}) {
  const document = state.documents.find(item => item.id === documentId);
  if (!document) return;
  state.activeDocumentId = document.id;
  if (notifyHost) notifyDocumentSelected(document);
  renderDocument(document, announce);
  requestAnimationFrame(() => (state.sourceMode ? sourceEditor : write).focus());
}

function createUntitledDocument(markdown = "", { announce = true, notifyHost = true } = {}) {
  state.untitledSequence += 1;
  const document = {
    id: nextDocumentId(),
    name: untitledName(state.untitledSequence),
    path: "",
    markdown: String(markdown ?? ""),
    dirty: false
  };
  state.documents.push(document);
  activateDocument(document.id, { announce, notifyHost });
  return document;
}

function loadMarkdown(markdown, announce = false) {
  const document = activeDocument() || createUntitledDocument("", { announce: false, notifyHost: false });
  document.markdown = String(markdown ?? "");
  document.dirty = false;
  renderDocument(document, announce);
}

function openDocument(payload = {}) {
  const path = typeof payload.path === "string" ? payload.path : "";
  const markdown = String(payload.markdown ?? "");
  const name = String(payload.name || path.split(/[\\/]/).pop() || "未命名.md");
  let document = path ? state.documents.find(item => item.path === path) : null;
  if (document) {
    document.name = name;
    document.markdown = markdown;
    document.dirty = false;
  } else {
    document = { id: nextDocumentId(), name, path, markdown, dirty: false };
    state.documents.push(document);
  }
  activateDocument(document.id, { announce: true, notifyHost: true });
}

function syncFromWrite() {
  state.markdown = editorToMarkdown(write);
  sourceEditor.value = state.markdown;
  const document = activeDocument();
  if (document) document.markdown = state.markdown;
  // 大纲属于编辑视图状态，不能等待草稿持久化的防抖计时器。
  updateOutline();
  markChanged();
}

function currentWriteBlock() {
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount) return null;
  let block = selection.anchorNode;
  if (block?.nodeType === Node.TEXT_NODE) block = block.parentElement;
  while (block && block.parentElement !== write) block = block.parentElement;
  return block instanceof HTMLElement ? block : null;
}

function writeBlockForNode(node) {
  let block = node;
  if (block?.nodeType === Node.TEXT_NODE) block = block.parentElement;
  while (block && block.parentElement !== write) block = block.parentElement;
  return block instanceof HTMLElement ? block : null;
}

function textOffsetWithin(element, node, offset) {
  const range = document.createRange();
  range.selectNodeContents(element);
  try {
    range.setEnd(node, offset);
  } catch {
    return 0;
  }
  return range.toString().replaceAll(caretMarker, "").length;
}

function placeTextCaret(element, offset) {
  const selection = window.getSelection();
  const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
  let remaining = Math.max(0, offset);
  let target = element;
  let targetOffset = 0;
  while (walker.nextNode()) {
    const node = walker.currentNode;
    const length = node.nodeValue?.length || 0;
    if (remaining <= length) {
      target = node;
      targetOffset = remaining;
      break;
    }
    remaining -= length;
    target = node;
    targetOffset = length;
  }
  const caret = document.createRange();
  caret.setStart(target, targetOffset);
  caret.collapse(true);
  selection?.removeAllRanges();
  selection?.addRange(caret);
}

function beginComposition(event) {
  pendingCodeExit = null;
  recentCompositionCommit = null;
  const block = writeBlockForNode(event.target) || currentWriteBlock();
  const selection = window.getSelection();
  if (!block?.matches("h1, h2, h3, h4, h5, h6") || !selection?.isCollapsed || !selection.rangeCount) {
    activeComposition = null;
    return;
  }
  const offset = block.contains(selection.anchorNode)
    ? textOffsetWithin(block, selection.anchorNode, selection.anchorOffset)
    : (block.textContent || "").length;
  const text = (block.textContent || "").replaceAll(caretMarker, "");
  activeComposition = {
    block,
    index: [...write.childNodes].indexOf(block),
    previousSibling: block.previousSibling,
    nextSibling: block.nextSibling,
    prefix: text.slice(0, offset),
    suffix: text.slice(offset),
    committed: false
  };
}

function restoreHeadingCompositionBlock(composition) {
  const { block } = composition;
  if (!block.isConnected) {
    const next = composition.nextSibling?.parentNode === write ? composition.nextSibling : null;
    const previous = composition.previousSibling?.parentNode === write ? composition.previousSibling : null;
    const indexed = write.childNodes[Math.max(0, composition.index)] || null;
    const placeholder = indexed?.nodeName === "BR" ? indexed : null;
    if (next) write.insertBefore(block, next);
    else if (previous) previous.after(block);
    else write.insertBefore(block, indexed);
    placeholder?.remove();
  }
  block.textContent = composition.prefix + composition.suffix;
  if (!block.textContent) block.append(document.createElement("br"));
  placeTextCaret(block, composition.prefix.length);
}

function handleHeadingCompositionInput(event) {
  const composition = activeComposition;
  if (!composition || !composition.block.isConnected || !event.cancelable) return false;
  if (event.inputType === "deleteCompositionText") {
    // WebKit 会在提交中文候选词前删除空标题节点；保留节点并等待最终文本。
    event.preventDefault();
    return true;
  }
  if (event.inputType !== "insertFromComposition") return false;
  event.preventDefault();
  const committedText = String(event.data ?? "");
  composition.block.textContent = composition.prefix + committedText + composition.suffix;
  placeTextCaret(composition.block, composition.prefix.length + committedText.length);
  composition.committed = true;
  syncFromWrite();
  updateFocusLine();
  return true;
}

function rawHeadingMatch(block) {
  return block?.matches("p, div") ? block.textContent?.match(/^(#{1,6})\s+(.+?)\s*#*$/) : null;
}

function selectionAtEnd(element) {
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount || !element.contains(selection.anchorNode)) return false;
  const tail = document.createRange();
  tail.selectNodeContents(element);
  try {
    tail.setStart(selection.anchorNode, selection.anchorOffset);
  } catch {
    return false;
  }
  return tail.toString().replaceAll(caretMarker, "") === "";
}

function paragraphAfterCode(block) {
  let paragraph = block.nextElementSibling;
  if (paragraph?.matches(".code-meta")) paragraph = paragraph.nextElementSibling;
  if (paragraph?.matches("p")) return paragraph;
  paragraph = document.createElement("p");
  paragraph.append(document.createElement("br"));
  const meta = block.nextElementSibling?.matches(".code-meta") ? block.nextElementSibling : null;
  (meta || block).after(paragraph);
  return paragraph;
}

function exitCodeBlock(block) {
  const code = block.querySelector("code") || block;
  const value = (code.innerText || code.textContent || "").replaceAll(caretMarker, "").replace(/\n+$/, "");
  code.textContent = value;
  if (!value) code.append(document.createElement("br"));
  const meta = block.nextElementSibling?.matches(".code-meta") ? block.nextElementSibling : null;
  if (meta) meta.hidden = true;
  const paragraph = paragraphAfterCode(block);
  write.focus({ preventScroll: true });
  const selection = window.getSelection();
  const caret = document.createRange();
  caret.setStart(paragraph, 0);
  caret.collapse(true);
  selection?.removeAllRanges();
  selection?.addRange(caret);
  pendingCodeExit = null;
  highlightCodeBlock(block, true);
  syncFromWrite();
  updateFocusLine();
  return true;
}

function ensureCodeMeta(block) {
  let panel = block.nextElementSibling;
  if (panel?.matches(".code-meta")) return panel;
  panel = document.createElement("div");
  panel.className = "code-meta";
  panel.contentEditable = "false";
  panel.hidden = true;
  const language = document.createElement("input");
  language.className = "code-language";
  language.placeholder = "语言";
  language.setAttribute("aria-label", "代码语言");
  const title = document.createElement("input");
  title.className = "code-title";
  title.placeholder = "名称（可选）";
  title.setAttribute("aria-label", "代码片段名称（可选）");
  panel.append(language, title);
  block.after(panel);

  const updateMetadata = () => {
    block.dataset.language = language.value.trim();
    const name = title.value.trim();
    if (name) block.dataset.title = name;
    else delete block.dataset.title;
    syncFromWrite();
  };
  panel.addEventListener("input", event => {
    event.stopPropagation();
    updateMetadata();
  });
  panel.addEventListener("keydown", event => {
    if (event.isComposing || event.keyCode === 229) return;
    if (["ArrowLeft", "ArrowRight"].includes(event.key)) {
      event.preventDefault();
      event.stopPropagation();
      (event.target === language ? title : language).focus();
      (event.target === language ? title : language).select();
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      event.stopPropagation();
      updateMetadata();
      exitCodeBlock(block);
    }
  });
  return panel;
}

function showCodeMeta(block) {
  const panel = ensureCodeMeta(block);
  const language = panel.querySelector(".code-language");
  const title = panel.querySelector(".code-title");
  language.value = block.dataset.language || "";
  title.value = block.dataset.title || "";
  panel.hidden = false;
  language.focus();
  language.select();
}

function insertRenderCaretMarker() {
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount || !write.contains(selection.anchorNode)) return false;
  const range = selection.getRangeAt(0);
  range.insertNode(document.createTextNode(renderCaretMarker));
  return true;
}

function restoreRenderCaret() {
  const walker = document.createTreeWalker(write, NodeFilter.SHOW_TEXT);
  let restored = false;
  while (walker.nextNode()) {
    const node = walker.currentNode;
    const value = node.nodeValue ?? "";
    const markerIndex = value.indexOf(renderCaretMarker);
    if (markerIndex < 0) continue;
    const valueWithoutMarker = value.replaceAll(renderCaretMarker, "");
    const needsInlineBoundary = !valueWithoutMarker && node.previousSibling instanceof HTMLElement
      && node.previousSibling.matches("code, strong, b, em, i, del, s, strike, a");
    node.nodeValue = value.replaceAll(renderCaretMarker, needsInlineBoundary ? caretMarker : "");
    if (restored) continue;
    const selection = window.getSelection();
    const caret = document.createRange();
    caret.setStart(node, markerIndex + (needsInlineBoundary ? caretMarker.length : 0));
    caret.collapse(true);
    selection?.removeAllRanges();
    selection?.addRange(caret);
    restored = true;
  }
  return restored;
}

function closeFencedCodeAtCaret(block) {
  if (!block.matches("pre")) return false;
  const selection = window.getSelection();
  const code = block.querySelector("code") || block;
  const tail = document.createRange();
  tail.selectNodeContents(code);
  try {
    tail.setStart(selection.anchorNode, selection.anchorOffset);
  } catch {
    return false;
  }
  const text = (code.innerText || code.textContent || "").replaceAll(caretMarker, "").replace(/\n$/, "");
  if (tail.toString() || !/(```|~~~)$/.test(text)) return false;
  code.textContent = text.replace(/(```|~~~)$/, "").replace(/\n$/, "");
  return exitCodeBlock(block);
}

function renderMarkdownBlockAtCaret() {
  const block = currentWriteBlock();
  if (!block || block.matches(".mermaid-diagram")) return false;
  if (closeFencedCodeAtCaret(block)) return true;
  if (block.matches("pre, ul, ol, table")) return false;

  const text = block.textContent ?? "";
  const rawHeading = block.matches("p, div") && /^(#{1,6})\s+\S/.test(text);
  const rawInline = /`[^`\n]+`|\*\*[^*\n]+\*\*|__[^_\n]+__|~~[^~\n]+~~|(^|[^*])\*[^*\n]+\*(?!\*)|(^|[^_])_[^_\n]+_(?!_)|!?\[[^\]\n]+\]\([^\n)]+\)/.test(text);
  if (!rawHeading && !rawInline) return false;
  if (!insertRenderCaretMarker()) return false;

  const wrapper = document.createElement("div");
  wrapper.append(block.cloneNode(true));
  const html = markdownToHTML(editorToMarkdown(wrapper));
  const template = document.createElement("template");
  template.innerHTML = html || "<p><br></p>";
  block.replaceWith(template.content);
  restoreRenderCaret();
  return true;
}

function renderMarkdownDocumentAtCaret() {
  const hasCaret = insertRenderCaretMarker();
  let markdown = editorToMarkdown(write);
  if (hasCaret) {
    // 围栏闭合行会被解析器丢弃，把光标标记移到代码块后的新段落。
    const closingFenceWithCaret = new RegExp("(^|\\n)(\\s*(?:```|~~~)\\s*)" + renderCaretMarker + "(?=\\n|$)", "g");
    markdown = markdown.replace(closingFenceWithCaret, `$1$2\n\n${renderCaretMarker}`);
  }
  write.innerHTML = markdownToHTML(markdown) || "<p><br></p>";
  if (hasCaret && !restoreRenderCaret()) {
    const paragraph = document.createElement("p");
    paragraph.append(document.createElement("br"));
    write.append(paragraph);
    const selection = window.getSelection();
    const caret = document.createRange();
    caret.setStart(paragraph, 0);
    caret.collapse(true);
    selection?.removeAllRanges();
    selection?.addRange(caret);
  }
  highlightCodeBlocks(write);
  void renderMermaidDiagrams(write, state.documentTheme);
  return true;
}

function removeCaretMarkers() {
  const selection = window.getSelection();
  const anchorNode = selection?.isCollapsed ? selection.anchorNode : null;
  const anchorOffset = selection?.anchorOffset ?? 0;
  let nextOffset = anchorOffset;
  const walker = document.createTreeWalker(write, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    const value = node.nodeValue ?? "";
    if (!value.includes(caretMarker)) continue;
    if (node === anchorNode) nextOffset = value.slice(0, anchorOffset).replaceAll(caretMarker, "").length;
    node.nodeValue = value.replaceAll(caretMarker, "");
  }
  if (anchorNode?.isConnected && selection) {
    const caret = document.createRange();
    caret.setStart(anchorNode, Math.min(nextOffset, anchorNode.nodeValue?.length ?? 0));
    caret.collapse(true);
    selection.removeAllRanges();
    selection.addRange(caret);
  }
}

function handleWriteInput(event) {
  if (event?.target?.matches?.(".code-meta input")) return;
  if (currentWriteBlock()?.matches("pre")) pendingCodeExit = null;
  if (activeComposition && event?.inputType === "deleteCompositionText") {
    restoreHeadingCompositionBlock(activeComposition);
    syncFromWrite();
    return;
  }
  if (activeComposition && event?.inputType === "insertFromComposition") {
    if (!activeComposition.block.isConnected) {
      restoreHeadingCompositionBlock(activeComposition);
      activeComposition.block.textContent = activeComposition.prefix + String(event.data ?? "") + activeComposition.suffix;
      placeTextCaret(activeComposition.block, activeComposition.prefix.length + String(event.data ?? "").length);
    }
    activeComposition.committed = true;
    syncFromWrite();
    return;
  }
  if (event?.isComposing || event?.inputType === "insertCompositionText") {
    removeCaretMarkers();
    syncFromWrite();
    return;
  }
  const converted = renderMarkdownBlockAtCaret();
  if (!converted) removeCaretMarkers();
  syncFromWrite();
}

function syncFromSource(render = false) {
  state.markdown = sourceEditor.value;
  const document = activeDocument();
  if (document) document.markdown = state.markdown;
  if (render) {
    write.innerHTML = markdownToHTML(state.markdown) || "<p><br></p>";
    highlightCodeBlocks(write);
    void renderMermaidDiagrams(write, state.documentTheme);
  }
  markChanged();
}

function markChanged() {
  state.dirty = true;
  const document = activeDocument();
  const becameDirty = document && !document.dirty;
  if (document) {
    document.markdown = state.markdown;
    document.dirty = true;
  }
  if (becameDirty) renderFiles();
  $("#save-state").textContent = "未保存";
  $("#save-state").classList.add("is-visible");
  clearTimeout(changeTimer);
  changeTimer = setTimeout(() => {
    updateDerivedState();
    localStorage.setItem("mory.draft", state.markdown);
    bridge({
      type: "changed",
      documentId: document?.id || "",
      name: document?.name || "未命名.md",
      path: document?.path || "",
      markdown: state.markdown
    });
  }, 180);
}

function updateDerivedState() {
  const stats = documentStats(state.markdown);
  $("#word-count").textContent = `${stats.words} 字`;
  $("#line-count").textContent = `${stats.lines} 行`;
  updateOutline();

  if (!state.titleTouched) {
    const title = state.markdown.match(/^#\s+(.+)$/m)?.[1]?.replace(/[*_`~]/g, "").trim() || "未命名";
    $("#document-title").value = title;
      bridge({ type: "title", value: title, dirty: state.dirty });
  }
}

function updateOutline() {
  const entries = [...write.querySelectorAll("h1, h2, h3, h4, h5, h6")];
  const outline = $("#outline-list");
  outline.innerHTML = "";
  entries.forEach((heading, index) => {
    heading.id = `heading-${index}`;
    const button = document.createElement("button");
    button.className = "outline-item";
    button.dataset.level = heading.tagName.slice(1);
    button.textContent = heading.textContent || "无标题";
    button.addEventListener("click", () => heading.scrollIntoView({ behavior: "smooth", block: "start" }));
    outline.append(button);
  });
  $("#outline-count").textContent = `${entries.length} 项`;
  $("#outline-empty").hidden = entries.length > 0;
}

function visibleFileEntries() {
  const openPaths = new Set(state.documents.map(document => document.path).filter(Boolean));
  return [
    ...state.documents.map(document => ({ ...document, documentId: document.id, open: true })),
    ...state.files.filter(file => !openPaths.has(file.path)).map(file => ({ ...file, documentId: "", markdown: "", dirty: false, open: false }))
  ];
}

function closeDocument(documentId) {
  const index = state.documents.findIndex(document => document.id === documentId);
  if (index < 0) return;

  const [removed] = state.documents.splice(index, 1);
  const message = removed.path ? "文档已关闭" : "草稿已移除";
  if (removed.id !== state.activeDocumentId) {
    renderFiles();
    toast(message);
    return;
  }

  if (state.documents.length) {
    const next = state.documents[Math.min(index, state.documents.length - 1)];
    activateDocument(next.id, { announce: false, notifyHost: true });
  } else {
    state.untitledSequence = 0;
    createUntitledDocument("", { announce: false, notifyHost: true });
  }
  toast(message);
}

function renderFiles() {
  const list = $("#file-list");
  list.innerHTML = "";
  const entries = visibleFileEntries();
  entries.forEach(file => {
    const row = document.createElement("div");
    row.className = "file-row";
    const button = document.createElement("button");
    button.className = `file-item${file.documentId === state.activeDocumentId ? " is-active" : ""}`;
    button.dataset.path = file.path;
    if (file.documentId) button.dataset.documentId = file.documentId;
    button.innerHTML = `<span class="file-symbol">${file.path ? "M" : "M↓"}</span><span class="file-name"></span><span class="file-dirty" aria-label="未保存"></span>`;
    button.querySelector(".file-name").textContent = file.name;
    button.querySelector(".file-dirty").hidden = !file.dirty;
    button.addEventListener("click", () => {
      if (file.documentId) activateDocument(file.documentId, { announce: true, notifyHost: true });
      else if (file.path) bridge({ type: "openFile", path: file.path });
    });
    row.append(button);
    if (file.documentId) {
      const close = document.createElement("button");
      const label = file.path ? "关闭文档" : "移除草稿";
      close.className = "file-close";
      close.dataset.documentId = file.documentId;
      close.title = label;
      close.setAttribute("aria-label", label);
      close.innerHTML = '<svg aria-hidden="true"><use href="#i-close"/></svg>';
      close.addEventListener("click", event => {
        event.stopPropagation();
        closeDocument(file.documentId);
      });
      row.append(close);
    }
    list.append(row);
  });
  renderQuickResults(entries);
}

function renderQuickResults(files = visibleFileEntries(), query = "") {
  const normalized = query.trim().toLocaleLowerCase();
  const results = files.filter(file => file.name.toLocaleLowerCase().includes(normalized));
  const container = $("#quick-open-results");
  container.innerHTML = "";
  results.forEach((file, index) => {
    const button = document.createElement("button");
    button.className = `quick-result${index === 0 ? " is-active" : ""}`;
    const name = document.createElement("span");
    name.textContent = file.name;
    const path = document.createElement("small");
    path.textContent = file.path || "当前草稿";
    button.append(name, path);
    button.addEventListener("click", () => {
      if (file.documentId) activateDocument(file.documentId, { announce: true, notifyHost: true });
      else if (file.path) bridge({ type: "openFile", path: file.path });
      closeQuickOpen();
    });
    container.append(button);
  });
  if (!results.length) container.innerHTML = '<p class="empty-state">没有匹配的文件</p>';
}

function toggleSource(force) {
  const next = typeof force === "boolean" ? force : !state.sourceMode;
  if (next === state.sourceMode) return;
  if (next) {
    syncFromWrite();
    sourceEditor.value = state.markdown;
  } else {
    syncFromSource(true);
  }
  state.sourceMode = next;
  workspace.classList.toggle("source-mode", next);
  const sourceButton = $("#source-toggle");
  const label = next ? "预览模式（⌘/）" : "源代码模式（⌘/）";
  sourceButton.classList.toggle("is-active", next);
  sourceButton.dataset.tooltip = label;
  sourceButton.setAttribute("aria-label", label);
  requestAnimationFrame(() => (next ? sourceEditor : write).focus());
}

function execute(command) {
  if (state.sourceMode) toggleSource(false);
  write.focus();
  const builtins = {
    bold: ["bold"], italic: ["italic"], strike: ["strikeThrough"],
    paragraph: ["formatBlock", "p"], h1: ["formatBlock", "h1"], h2: ["formatBlock", "h2"],
    quote: ["formatBlock", "blockquote"], ul: ["insertUnorderedList"], ol: ["insertOrderedList"],
    hr: ["insertHorizontalRule"]
  };
  if (builtins[command]) {
    document.execCommand(builtins[command][0], false, builtins[command][1]);
  } else if (command === "code") {
    wrapSelection("code");
  } else if (command === "link") {
    const selection = window.getSelection();
    const label = selection?.toString() || "链接文本";
    const url = window.prompt("链接地址", "https://");
    if (url) document.execCommand("insertHTML", false, `<a href="${url.replaceAll('"', '&quot;')}">${label}</a>`);
  } else if (command === "task") {
    document.execCommand("insertHTML", false, '<ul><li class="task-item"><input type="checkbox">待办事项</li></ul>');
  } else if (command === "table") {
    document.execCommand("insertHTML", false, '<table><thead><tr><th>标题</th><th>标题</th></tr></thead><tbody><tr><td>内容</td><td>内容</td></tr><tr><td>内容</td><td>内容</td></tr></tbody></table><p><br></p>');
  }
  syncFromWrite();
}

function wrapSelection(tagName) {
  const selection = window.getSelection();
  if (!selection?.rangeCount) return;
  const range = selection.getRangeAt(0);
  const element = document.createElement(tagName);
  if (range.collapsed) element.textContent = "代码";
  else element.append(range.extractContents());
  range.insertNode(element);
  range.selectNodeContents(element);
  selection.removeAllRanges();
  selection.addRange(range);
}

function setHeading(level) {
  if (state.sourceMode) toggleSource(false);
  write.focus();
  document.execCommand("formatBlock", false, `h${level}`);
  syncFromWrite();
}

function updateFocusLine() {
  write.querySelectorAll(".focus-line").forEach(element => element.classList.remove("focus-line"));
  let node = window.getSelection()?.anchorNode;
  if (node?.nodeType === Node.TEXT_NODE) node = node.parentElement;
  while (node && node.parentElement !== write) node = node.parentElement;
  if (node instanceof HTMLElement) node.classList.add("focus-line");

  if (workspace.classList.contains("typewriter-mode") && node instanceof HTMLElement) {
    const target = node.offsetTop - editorScroll.clientHeight * .45;
    editorScroll.scrollTo({ top: Math.max(0, target), behavior: "smooth" });
  }
}

function showFind() {
  const panel = $("#find-panel");
  panel.classList.add("is-open");
  panel.setAttribute("aria-hidden", "false");
  $("#find-input").focus();
  $("#find-input").select();
  updateFindMatches();
}

function closeFind() {
  $("#find-panel").classList.remove("is-open");
  $("#find-panel").setAttribute("aria-hidden", "true");
  (state.sourceMode ? sourceEditor : write).focus();
}

function updateFindMatches() {
  const query = $("#find-input").value;
  state.findMatches = [];
  state.findIndex = -1;
  if (query) {
    const haystack = state.markdown.toLocaleLowerCase();
    const needle = query.toLocaleLowerCase();
    let start = 0;
    while ((start = haystack.indexOf(needle, start)) >= 0) {
      state.findMatches.push(start);
      start += Math.max(needle.length, 1);
    }
  }
  $("#find-count").textContent = state.findMatches.length ? `0 / ${state.findMatches.length}` : "0 / 0";
}

function stepFind(direction = 1) {
  if (!state.findMatches.length) return;
  state.findIndex = (state.findIndex + direction + state.findMatches.length) % state.findMatches.length;
  const index = state.findMatches[state.findIndex];
  const length = $("#find-input").value.length;
  if (!state.sourceMode) toggleSource(true);
  sourceEditor.focus();
  sourceEditor.setSelectionRange(index, index + length);
  const lineHeight = Number.parseFloat(getComputedStyle(sourceEditor).lineHeight);
  const line = state.markdown.slice(0, index).split("\n").length;
  editorScroll.scrollTop = Math.max(0, line * lineHeight - editorScroll.clientHeight / 2);
  $("#find-count").textContent = `${state.findIndex + 1} / ${state.findMatches.length}`;
}

function replaceOne() {
  if (state.findIndex < 0) stepFind(1);
  if (state.findIndex < 0) return;
  const index = state.findMatches[state.findIndex];
  const query = $("#find-input").value;
  const replacement = $("#replace-input").value;
  sourceEditor.value = state.markdown.slice(0, index) + replacement + state.markdown.slice(index + query.length);
  syncFromSource(false);
  updateFindMatches();
  stepFind(1);
}

function replaceAll() {
  const query = $("#find-input").value;
  if (!query) return;
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const count = state.findMatches.length;
  sourceEditor.value = state.markdown.replace(new RegExp(escaped, "gi"), $("#replace-input").value);
  syncFromSource(false);
  updateFindMatches();
  toast(`已替换 ${count} 处`);
}

function openQuickOpen() {
  $("#quick-open").classList.add("is-open");
  $("#quick-open").setAttribute("aria-hidden", "false");
  $("#quick-open-input").value = "";
  renderQuickResults(visibleFileEntries());
  requestAnimationFrame(() => $("#quick-open-input").focus());
}

function closeQuickOpen() {
  $("#quick-open").classList.remove("is-open");
  $("#quick-open").setAttribute("aria-hidden", "true");
}

function togglePreferences(force) {
  const panel = $("#preferences");
  const open = typeof force === "boolean" ? force : !panel.classList.contains("is-open");
  panel.classList.toggle("is-open", open);
  panel.setAttribute("aria-hidden", String(!open));
}

function mermaidTheme(theme) {
  const palettes = {
    "yuluo-css": { theme: "base", primaryColor: "#effaff", primaryTextColor: "#333333", primaryBorderColor: "#1a8f37", lineColor: "#4183c4", background: "#ffffff" },
    github: { theme: "base", primaryColor: "#f6f8fa", primaryTextColor: "#1f2328", primaryBorderColor: "#8c959f", lineColor: "#59636e", background: "#ffffff" },
    whitey: { theme: "base", primaryColor: "#f5f5f3", primaryTextColor: "#2c2c2b", primaryBorderColor: "#b8b8b2", lineColor: "#74746f", background: "#ffffff" },
    newsprint: { theme: "neutral", primaryColor: "#eee9dd", primaryTextColor: "#191816", primaryBorderColor: "#8d877b", lineColor: "#5f5a52", background: "#f7f4ed" },
    pixyll: { theme: "base", primaryColor: "#f8eeea", primaryTextColor: "#333333", primaryBorderColor: "#d04f4a", lineColor: "#c14e4a", background: "#fffdf9" },
    gothic: { theme: "neutral", primaryColor: "#efede8", primaryTextColor: "#201f1d", primaryBorderColor: "#77716a", lineColor: "#55514d", background: "#f8f7f3" },
    night: { theme: "dark", primaryColor: "#292f36", primaryTextColor: "#e7ebef", primaryBorderColor: "#6f7b87", lineColor: "#9ca7b1", background: "#1f2328" }
  };
  return palettes[theme] || palettes.github;
}

function normalizedCodeLanguage(value) {
  return String(value || "").trim().toLocaleLowerCase().replace(/[^a-z0-9_+-]/g, "");
}

function highlightCodeBlock(block, force = false) {
  if (!block?.matches?.("pre") || block.closest(".mermaid-diagram") || !window.hljs) return false;
  const code = block.querySelector("code");
  if (!code || code.dataset.moryHighlighted === "true") return false;
  if (!force && block === currentWriteBlock()) return false;
  const source = code.textContent || "";
  const language = normalizedCodeLanguage(block.dataset.language);
  try {
    const result = language && window.hljs.getLanguage(language)
      ? window.hljs.highlight(source, { language, ignoreIllegals: true })
      : (language ? { value: escapeHTML(source), language: "plaintext" } : window.hljs.highlightAuto(source));
    code.innerHTML = result.value;
    code.classList.add("hljs");
    if (result.language) code.classList.add(`language-${normalizedCodeLanguage(result.language)}`);
    code.dataset.moryHighlighted = "true";
    block.dataset.highlighted = "true";
    return true;
  } catch {
    code.textContent = source;
    return false;
  }
}

function highlightCodeBlocks(root, force = false) {
  root.querySelectorAll("pre").forEach(block => highlightCodeBlock(block, force));
}

function clearCodeHighlight(block) {
  const code = block?.querySelector?.("code");
  if (!code || code.dataset.moryHighlighted !== "true") return false;
  const selection = window.getSelection();
  const inside = selection?.isCollapsed && selection.rangeCount && code.contains(selection.anchorNode);
  const offset = inside ? textOffsetWithin(code, selection.anchorNode, selection.anchorOffset) : 0;
  const source = code.textContent || "";
  code.textContent = source;
  code.className = "";
  delete code.dataset.moryHighlighted;
  delete block.dataset.highlighted;
  if (inside) placeTextCaret(code, offset);
  return true;
}

function enqueueMermaid(task) {
  const next = mermaidQueue.then(task, task);
  mermaidQueue = next.catch(() => {});
  return next;
}

function showMermaidError(element, source, error) {
  const message = error instanceof Error ? error.message.split("\n")[0] : String(error || "语法错误");
  element.dataset.mermaidState = "error";
  element.replaceChildren();
  const heading = document.createElement("strong");
  heading.textContent = "Mermaid 无法渲染";
  const detail = document.createElement("small");
  detail.textContent = message;
  const pre = document.createElement("pre");
  pre.className = "mermaid-source";
  const code = document.createElement("code");
  code.textContent = source;
  pre.append(code);
  element.append(heading, detail, pre);
}

function renderMermaidDiagrams(root, theme = state.documentTheme) {
  const diagrams = [...root.querySelectorAll(".mermaid-diagram")];
  if (!diagrams.length) return Promise.resolve();
  return enqueueMermaid(async () => {
    if (!window.mermaid?.initialize || !window.mermaid?.render) {
      diagrams.forEach(element => showMermaidError(element, element.dataset.mermaidSource || "", "Mermaid 运行时未加载"));
      throw new Error("Mermaid 运行时未加载");
    }
    const palette = mermaidTheme(theme);
    window.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      suppressErrorRendering: true,
      theme: palette.theme,
      themeVariables: {
        primaryColor: palette.primaryColor,
        primaryTextColor: palette.primaryTextColor,
        primaryBorderColor: palette.primaryBorderColor,
        lineColor: palette.lineColor,
        background: palette.background,
        fontFamily: '-apple-system, BlinkMacSystemFont, "PingFang SC", sans-serif'
      },
      flowchart: { htmlLabels: true, useMaxWidth: true }
    });
    for (const element of diagrams) {
      const source = element.dataset.mermaidSource || "";
      element.dataset.mermaidState = "rendering";
      try {
        const result = await window.mermaid.render(`mory-mermaid-${++mermaidSequence}`, source);
        element.innerHTML = result.svg;
        element.dataset.mermaidState = "rendered";
        element.setAttribute("aria-label", "Mermaid 图表");
        result.bindFunctions?.(element);
      } catch (error) {
        showMermaidError(element, source, error);
      }
    }
  });
}

const exportBaseCSS = `
*{box-sizing:border-box}html,body{margin:0;min-height:100%}body{background:#fff;color:#2c2c2b}.editor-scroll{min-height:100vh;padding:1px 0}.write{width:min(calc(100% - 72px),820px);margin:48px auto 72px;font-size:17px;line-height:1.8}.write h1,.write h2,.write h3,.write h4,.write h5,.write h6{margin:1.7em 0 .65em;line-height:1.35}.write h1{margin-top:.7em;padding-bottom:.28em;border-bottom:1px solid #ddd;font-size:2em}.write h2{padding-bottom:.24em;border-bottom:1px solid #e5e5e5;font-size:1.55em}.write h3{font-size:1.24em}.write p{margin:.75em 0}.write a{text-decoration:none}.write code{padding:.14em .35em;border-radius:3px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.84em}.write pre{position:relative;margin:1.1em 0;padding:16px 18px;overflow:auto;border-radius:4px;line-height:1.55}.write pre[data-title]:not([data-title=""]){padding-top:36px}.write pre[data-title]:not([data-title=""])::before{content:attr(data-title);position:absolute;top:8px;left:18px;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;font-size:10px;font-weight:600;opacity:.6}.write pre code{padding:0;background:transparent}.write .hljs-comment,.write .hljs-quote{color:#6a737d;font-style:italic}.write .hljs-keyword,.write .hljs-selector-tag,.write .hljs-type{color:#8250df;font-weight:600}.write .hljs-title,.write .hljs-section,.write .hljs-function{color:#0550ae}.write .hljs-string,.write .hljs-attr,.write .hljs-symbol{color:#0a3069}.write .hljs-number,.write .hljs-literal,.write .hljs-built_in{color:#953800}.write blockquote{margin:1.1em 0;padding:.1em 1.1em;border-left:3px solid}.write ul,.write ol{padding-left:1.6em}.write li{margin:.24em 0}.write hr{margin:2.2em 0;border:0;border-top:1px solid}.write table{width:100%;margin:1.2em 0;border-collapse:collapse;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;font-size:.88em}.write th,.write td{min-width:80px;padding:7px 10px;border:1px solid;text-align:left}.write img{max-width:100%}.write .task-item{list-style:none;margin-left:-1.4em}.write input[type=checkbox]{margin-right:.55em}.mermaid-diagram{margin:1.5em 0;padding:16px;overflow:auto;text-align:center}.mermaid-diagram svg{display:block;max-width:100%;height:auto;margin:auto}@page{margin:18mm 17mm}@media print{.editor-scroll{background:transparent!important}.write{width:auto;margin:0}.mermaid-diagram{break-inside:avoid}}`;

async function readThemeCSS(theme) {
  if (state.themeCSS.has(theme)) return state.themeCSS.get(theme);
  try {
    const response = await fetch(new URL(`themes/${theme}.css`, document.baseURI));
    if (!response.ok) throw new Error(String(response.status));
    const css = await response.text();
    state.themeCSS.set(theme, css);
    return css;
  } catch {
    const sheet = [...document.styleSheets].find(item => item.href?.endsWith(`/themes/${theme}.css`));
    const css = sheet ? [...sheet.cssRules].map(rule => rule.cssText).join("\n") : "";
    state.themeCSS.set(theme, css);
    return css;
  }
}

function setDocumentTheme(theme) {
  const next = ["yuluo-css", "github", "whitey", "newsprint", "pixyll", "gothic", "night"].includes(theme) ? theme : "yuluo-css";
  state.documentTheme = next;
  document.documentElement.dataset.docTheme = next;
  $("#document-theme").href = `themes/${next}.css`;
  $("#document-theme-select").value = next;
  localStorage.setItem("mory.documentTheme", next);
  readThemeCSS(next);
  void renderMermaidDiagrams(write, next);
}

async function exportDocument(options = {}) {
  const theme = options.theme && options.theme !== "current" ? options.theme : state.documentTheme;
  const themeCSS = await readThemeCSS(theme);
  const title = $("#document-title").value || "Mory 文档";
  const backgroundOverride = options.background === false ? ".editor-scroll{background:#fff!important}" : "";
  const exportRoot = document.createElement("article");
  exportRoot.className = "write";
  exportRoot.innerHTML = markdownToHTML(state.markdown);
  highlightCodeBlocks(exportRoot, true);
  await renderMermaidDiagrams(exportRoot, theme);
  return `<!doctype html>\n<html lang="zh-CN" data-doc-theme="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>${escapeHTML(title)}</title><style>${exportBaseCSS}\n${themeCSS}\n${backgroundOverride}</style></head><body><main class="editor-scroll"><article class="write">${exportRoot.innerHTML}</article></main></body></html>`;
}

function toggleExportDialog(force) {
  const panel = $("#export-dialog");
  const open = typeof force === "boolean" ? force : !panel.classList.contains("is-open");
  panel.classList.toggle("is-open", open);
  panel.setAttribute("aria-hidden", String(!open));
  if (open) {
    $("#export-theme").value = "current";
    $("#export-format").focus();
  }
}

function syncExportOptions() {
  const format = $("#export-format").value;
  $("#paper-setting").hidden = format !== "pdf";
  $("#image-width-setting").hidden = !["png", "jpeg"].includes(format);
  $("#export-hint").textContent = ["png", "jpeg"].includes(format) ? "长文档将在独立离屏页面中渲染" : "HTML、PDF 不需要 Pandoc";
}

async function confirmExport() {
  const options = {
    format: $("#export-format").value,
    theme: $("#export-theme").value,
    paper: $("#export-paper").value,
    width: Number($("#export-width").value),
    background: $("#export-background").checked
  };
  if (window.webkit?.messageHandlers?.mory || window.moryNative) {
    bridge({ type: "export", options });
    toggleExportDialog(false);
    return;
  }
  const html = await exportDocument(options);
  if (options.format === "html") {
    const link = document.createElement("a");
    link.href = URL.createObjectURL(new Blob([html], { type: "text/html;charset=utf-8" }));
    link.download = `${$("#document-title").value || "未命名"}.html`;
    link.click();
    URL.revokeObjectURL(link.href);
  } else if (options.format === "pdf") {
    const popup = window.open(URL.createObjectURL(new Blob([html], { type: "text/html" })));
    popup?.addEventListener("load", () => popup.print());
  } else {
    toast("图片导出请在桌面版中使用");
  }
  toggleExportDialog(false);
}

function insertCodeLineBreak(block) {
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount) return false;
  const code = block.querySelector("code") || block;
  if (!code.contains(selection.anchorNode)) return false;
  const range = selection.getRangeAt(0);
  range.deleteContents();
  const newline = document.createElement("br");
  const boundary = document.createTextNode(caretMarker);
  const fragment = document.createDocumentFragment();
  fragment.append(newline, boundary);
  range.insertNode(fragment);
  const caret = document.createRange();
  caret.setStart(boundary, boundary.nodeValue.length);
  caret.collapse(true);
  selection.removeAllRanges();
  selection.addRange(caret);
  syncFromWrite();
  updateFocusLine();
  return true;
}

function exitHeadingToParagraph(block, renderedHeading = block) {
  const paragraph = document.createElement("p");
  paragraph.append(document.createElement("br"));
  if (renderedHeading === block) block.after(paragraph);
  else block.replaceWith(renderedHeading, paragraph);
  write.focus({ preventScroll: true });
  const selection = window.getSelection();
  const caret = document.createRange();
  caret.setStart(paragraph, 0);
  caret.collapse(true);
  selection?.removeAllRanges();
  selection?.addRange(caret);
  syncFromWrite();
  updateFocusLine();
}

function handleEditorShortcut(event) {
  if (event.isComposing || event.keyCode === 229) return;
  if (event.key !== "Enter") recentCompositionCommit = null;
  const command = event.metaKey || event.ctrlKey;
  if (command && event.key.toLowerCase() === "b") { event.preventDefault(); execute("bold"); }
  if (command && event.key.toLowerCase() === "i") { event.preventDefault(); execute("italic"); }
  if (command && event.key.toLowerCase() === "k") { event.preventDefault(); execute("link"); }
  if (command && event.key === "/") { event.preventDefault(); toggleSource(); }

  if (!command && event.key === "ArrowDown" && !state.sourceMode) {
    const block = currentWriteBlock();
    const code = block?.matches("pre") ? (block.querySelector("code") || block) : null;
    if (code && selectionAtEnd(code)) {
      event.preventDefault();
      pendingCodeExit = null;
      showCodeMeta(block);
      return;
    }
  }

  if (!command && event.key === "Enter" && !state.sourceMode) {
    const selection = window.getSelection();
    if (!selection?.isCollapsed || !selection.rangeCount) return;
    let block = selection.anchorNode;
    if (block?.nodeType === Node.TEXT_NODE) block = block.parentElement;
    while (block && block.parentElement !== write) block = block.parentElement;
    if (!(block instanceof HTMLElement)) return;
    const tail = document.createRange();
    tail.selectNodeContents(block);
    try {
      tail.setStart(selection.anchorNode, selection.anchorOffset);
    } catch {
      return;
    }
    const followsCompositionCommit = recentCompositionCommit?.block === block
      && performance.now() - recentCompositionCommit.time <= 900;
    const atBlockEnd = tail.toString() === "" || followsCompositionCommit;
    const rawHeading = atBlockEnd ? rawHeadingMatch(block) : null;
    if (rawHeading) {
      event.preventDefault();
      recentCompositionCommit = null;
      const template = document.createElement("template");
      template.innerHTML = markdownToHTML(block.textContent || "");
      const heading = template.content.firstElementChild;
      if (heading?.matches("h1, h2, h3, h4, h5, h6")) {
        exitHeadingToParagraph(block, heading);
        return;
      }
    }
    const fence = atBlockEnd ? block.textContent?.match(/^(```|~~~)\s*(.*?)\s*$/) : null;
    if (fence && block.matches("p, div")) {
      event.preventDefault();
      const template = document.createElement("template");
      template.innerHTML = markdownToHTML(`${block.textContent}\n${fence[1]}`);
      const pre = template.content.querySelector("pre") || document.createElement("pre");
      const code = pre.querySelector("code") || document.createElement("code");
      code.textContent = "";
      code.append(document.createElement("br"));
      if (!code.parentElement) pre.append(code);
      const paragraph = document.createElement("p");
      paragraph.append(document.createElement("br"));
      block.replaceWith(pre, paragraph);
      const caret = document.createRange();
      caret.setStart(code, 0);
      caret.collapse(true);
      selection.removeAllRanges();
      selection.addRange(caret);
      syncFromWrite();
      updateFocusLine();
      return;
    }
    if (atBlockEnd && /^H[1-6]$/.test(block.tagName)) {
      event.preventDefault();
      recentCompositionCommit = null;
      exitHeadingToParagraph(block);
      return;
    }
  }

  if (!command && event.key === " " && !state.sourceMode) {
    const selection = window.getSelection();
    if (!selection?.isCollapsed || !selection.rangeCount) return;
    let block = selection.anchorNode;
    if (block?.nodeType === Node.TEXT_NODE) block = block.parentElement;
    while (block && block.parentElement !== write) block = block.parentElement;
    if (!(block instanceof HTMLElement)) return;
    const prefix = document.createRange();
    prefix.selectNodeContents(block);
    try {
      prefix.setEnd(selection.anchorNode, selection.anchorOffset);
    } catch {
      return;
    }
    const marker = prefix.toString();
    const heading = marker.match(/^(#{1,6})$/);
    if (heading) {
      event.preventDefault();
      const next = document.createElement(`h${heading[1].length}`);
      next.append(document.createElement("br"));
      block.replaceWith(next);
      const caret = document.createRange();
      caret.setStart(next, 0);
      caret.collapse(true);
      selection.removeAllRanges();
      selection.addRange(caret);
      syncFromWrite();
      updateFocusLine();
    } else if (marker === ">") {
      event.preventDefault();
      const quote = document.createElement("blockquote");
      quote.append(document.createElement("br"));
      block.replaceWith(quote);
      const caret = document.createRange();
      caret.setStart(quote, 0);
      caret.collapse(true);
      selection.removeAllRanges();
      selection.addRange(caret);
      syncFromWrite();
      updateFocusLine();
    }
  }
}

$$('.tab').forEach(tab => tab.addEventListener("click", () => {
  $$('.tab').forEach(item => item.classList.toggle("is-active", item === tab));
  $$('.side-panel').forEach(panel => panel.classList.toggle("is-active", panel.id === `${tab.dataset.panel}-panel`));
  if (tab.dataset.panel === "outline") updateOutline();
}));

$("#toolbar").addEventListener("mousedown", event => event.preventDefault());
$("#toolbar").addEventListener("click", event => {
  const button = event.target.closest("button[data-command]");
  if (button) execute(button.dataset.command);
});
function showToolbarTooltip(button) {
  const tooltip = $("#toolbar-tooltip");
  const label = button.dataset.tooltip;
  if (!label) return;
  tooltip.textContent = label;
  tooltip.classList.add("is-visible");
  tooltip.setAttribute("aria-hidden", "false");
  const buttonRect = button.getBoundingClientRect();
  const tooltipRect = tooltip.getBoundingClientRect();
  tooltip.style.left = `${Math.max(8, buttonRect.left - tooltipRect.width - 9)}px`;
  tooltip.style.top = `${Math.min(innerHeight - tooltipRect.height - 8, Math.max(8, buttonRect.top + (buttonRect.height - tooltipRect.height) / 2))}px`;
}

function hideToolbarTooltip() {
  const tooltip = $("#toolbar-tooltip");
  tooltip.classList.remove("is-visible");
  tooltip.setAttribute("aria-hidden", "true");
}

$("#toolbar").addEventListener("mouseover", event => {
  const button = event.target.closest("button[data-tooltip]");
  if (button) showToolbarTooltip(button);
});
$("#toolbar").addEventListener("mouseout", event => {
  const button = event.target.closest("button[data-tooltip]");
  if (button && !button.contains(event.relatedTarget)) hideToolbarTooltip();
});
$("#toolbar").addEventListener("scroll", hideToolbarTooltip);
write.addEventListener("input", handleWriteInput);
write.addEventListener("beforeinput", event => {
  if (handleHeadingCompositionInput(event)) return;
  if (state.sourceMode || event.isComposing || !["insertParagraph", "insertLineBreak"].includes(event.inputType)) return;
  const block = currentWriteBlock();
  if (!block?.matches("pre")) return;
  event.preventDefault();
  const code = block.querySelector("code") || block;
  const now = performance.now();
  if (pendingCodeExit?.block === block && now - pendingCodeExit.time <= doubleEnterWindow && selectionAtEnd(code)) {
    exitCodeBlock(block);
    return;
  }
  const atEnd = selectionAtEnd(code);
  if (insertCodeLineBreak(block)) pendingCodeExit = atEnd ? { block, time: now } : null;
});
write.addEventListener("keyup", updateFocusLine);
write.addEventListener("mouseup", updateFocusLine);
write.addEventListener("keydown", handleEditorShortcut);
write.addEventListener("change", event => {
  if (event.target.matches('input[type="checkbox"]')) syncFromWrite();
});
write.addEventListener("paste", event => {
  event.preventDefault();
  const text = event.clipboardData.getData("text/plain");
  const block = currentWriteBlock();
  if (block?.matches("p, div") && !block.textContent) {
    block.textContent = text;
    const selection = window.getSelection();
    const caret = document.createRange();
    caret.selectNodeContents(block);
    caret.collapse(false);
    selection?.removeAllRanges();
    selection?.addRange(caret);
  } else {
    document.execCommand("insertText", false, text);
  }
  requestAnimationFrame(() => {
    renderMarkdownDocumentAtCaret();
    syncFromWrite();
  });
});
write.addEventListener("compositionstart", beginComposition);
write.addEventListener("compositionend", event => {
  let committedBlock = activeComposition?.block || writeBlockForNode(event.target);
  recentCompositionCommit = committedBlock instanceof HTMLElement
    ? { block: committedBlock, time: performance.now() }
    : null;
  activeComposition = null;
  requestAnimationFrame(() => {
    if (!(committedBlock instanceof HTMLElement) || !committedBlock.isConnected) return;
    if (rawHeadingMatch(committedBlock) && currentWriteBlock() === committedBlock && !selectionAtEnd(committedBlock)) {
      const selection = window.getSelection();
      const caret = document.createRange();
      caret.selectNodeContents(committedBlock);
      caret.collapse(false);
      selection?.removeAllRanges();
      selection?.addRange(caret);
    }
    if (renderMarkdownBlockAtCaret()) syncFromWrite();
  });
});
document.addEventListener("selectionchange", () => {
  if (document.activeElement !== write) return;
  const block = currentWriteBlock();
  if (block?.matches("pre[data-highlighted='true']")) clearCodeHighlight(block);
});
sourceEditor.addEventListener("input", () => syncFromSource(false));
sourceEditor.addEventListener("keydown", handleEditorShortcut);

$(".titlebar").addEventListener("pointerdown", event => {
  if (!nativeMacHost || event.button !== 0 || event.target.closest("button, input, select, a, [contenteditable='true']")) return;
  event.preventDefault();
  windowDragPointer = event.pointerId;
  event.currentTarget.setPointerCapture?.(event.pointerId);
  bridge({ type: "windowDragStart", screenX: event.screenX, screenY: event.screenY });
});
document.addEventListener("pointermove", event => {
  if (event.pointerId !== windowDragPointer) return;
  cancelAnimationFrame(windowDragFrame);
  windowDragFrame = requestAnimationFrame(() => bridge({ type: "windowDragMove", screenX: event.screenX, screenY: event.screenY }));
});
document.addEventListener("pointerup", event => {
  if (event.pointerId !== windowDragPointer) return;
  cancelAnimationFrame(windowDragFrame);
  windowDragPointer = null;
  bridge({ type: "windowDragEnd" });
});

$("#source-toggle").addEventListener("click", () => toggleSource());
$("#sidebar-toggle").addEventListener("click", () => $("#sidebar").classList.toggle("is-hidden"));
$("#new-file-button").addEventListener("click", () => createUntitledDocument());
$("#quick-open-button").addEventListener("click", openQuickOpen);
$("#settings-button").addEventListener("click", () => togglePreferences(true));
$("#export-button").addEventListener("click", () => toggleExportDialog(true));
$("#export-close").addEventListener("click", () => toggleExportDialog(false));
$("#export-dialog").addEventListener("mousedown", event => { if (event.target === $("#export-dialog")) toggleExportDialog(false); });
$("#export-format").addEventListener("change", syncExportOptions);
$("#export-confirm").addEventListener("click", confirmExport);
$("#preferences-close").addEventListener("click", () => togglePreferences(false));
$("#quick-open").addEventListener("mousedown", event => { if (event.target === $("#quick-open")) closeQuickOpen(); });
$("#preferences").addEventListener("mousedown", event => { if (event.target === $("#preferences")) togglePreferences(false); });
$("#quick-open-input").addEventListener("input", event => renderQuickResults(visibleFileEntries(), event.target.value));
$("#focus-button").addEventListener("click", () => {
  workspace.classList.toggle("focus-mode");
  $("#focus-button").classList.toggle("is-active");
  updateFocusLine();
});
$("#typewriter-button").addEventListener("click", () => {
  workspace.classList.toggle("typewriter-mode");
  $("#typewriter-button").classList.toggle("is-active");
  updateFocusLine();
});
$("#word-count").addEventListener("click", () => {
  const stats = documentStats(state.markdown);
  toast(`${stats.words} 字 · ${stats.characters} 字符 · ${stats.lines} 行`);
});

$("#find-input").addEventListener("input", updateFindMatches);
$("#find-input").addEventListener("keydown", event => { if (event.key === "Enter") stepFind(event.shiftKey ? -1 : 1); });
$("#find-next").addEventListener("click", () => stepFind(1));
$("#find-prev").addEventListener("click", () => stepFind(-1));
$("#find-close").addEventListener("click", closeFind);
$("#replace-toggle").addEventListener("click", () => $("#replace-row").classList.toggle("is-open"));
$("#replace-one").addEventListener("click", replaceOne);
$("#replace-all").addEventListener("click", replaceAll);

$("#theme-select").addEventListener("change", event => {
  const theme = event.target.value;
  document.documentElement.dataset.theme = theme === "system" ? "" : theme;
  localStorage.setItem("mory.theme", theme);
});
$("#document-theme-select").addEventListener("change", event => setDocumentTheme(event.target.value));
$("#width-select").addEventListener("change", event => {
  document.documentElement.style.setProperty("--editor-width", `${event.target.value}px`);
  localStorage.setItem("mory.width", event.target.value);
});
$("#status-toggle").addEventListener("change", event => {
  $("#statusbar").hidden = !event.target.checked;
  localStorage.setItem("mory.status", String(event.target.checked));
});
$("#spell-toggle").addEventListener("change", event => {
  write.spellcheck = event.target.checked;
  localStorage.setItem("mory.spell", String(event.target.checked));
});
$("#document-title").addEventListener("input", event => {
  state.titleTouched = true;
  bridge({ type: "title", value: event.target.value });
});

document.addEventListener("keydown", event => {
  const command = event.metaKey || event.ctrlKey;
  if (command && event.key.toLowerCase() === "p") { event.preventDefault(); openQuickOpen(); }
  if (command && event.key.toLowerCase() === "f") { event.preventDefault(); showFind(); }
  if (event.key === "Escape") { closeQuickOpen(); closeFind(); togglePreferences(false); toggleExportDialog(false); }
});

function restorePreferences() {
  const theme = localStorage.getItem("mory.theme") || "system";
  const width = localStorage.getItem("mory.width") || "820";
  const showStatus = localStorage.getItem("mory.status") !== "false";
  const spellcheck = localStorage.getItem("mory.spell") !== "false";
  const savedDocumentTheme = localStorage.getItem("mory.documentTheme");
  const defaultThemeVersion = localStorage.getItem("mory.documentThemeDefaultVersion");
  const documentTheme = !defaultThemeVersion && (!savedDocumentTheme || savedDocumentTheme === "github")
    ? "yuluo-css"
    : (savedDocumentTheme || "yuluo-css");
  localStorage.setItem("mory.documentThemeDefaultVersion", "yuluo-css-v1");
  $("#theme-select").value = theme;
  $("#width-select").value = width;
  $("#status-toggle").checked = showStatus;
  $("#spell-toggle").checked = spellcheck;
  document.documentElement.dataset.theme = theme === "system" ? "" : theme;
  document.documentElement.style.setProperty("--editor-width", `${width}px`);
  $("#statusbar").hidden = !showStatus;
  write.spellcheck = spellcheck;
  setDocumentTheme(documentTheme);
}

window.Mory = {
  loadMarkdown: markdown => loadMarkdown(markdown, true),
  openDocument,
  newDocument: () => createUntitledDocument(),
  closeDocument,
  normalizeMarkdown: renderMarkdownDocumentAtCaret,
  getMarkdown: () => state.sourceMode ? sourceEditor.value : editorToMarkdown(write),
  setFiles: files => { state.files = files; renderFiles(); },
  didSave: payload => {
    const document = activeDocument();
    if (document) {
      document.path = typeof payload?.path === "string" ? payload.path : document.path;
      document.name = String(payload?.name || document.name);
      document.markdown = state.sourceMode ? sourceEditor.value : editorToMarkdown(write);
      document.dirty = false;
      state.documents = state.documents.filter(item => item === document || !document.path || item.path !== document.path);
    }
    state.dirty = false;
    renderFiles();
    $("#save-state").textContent = "已保存";
    setTimeout(() => $("#save-state").classList.remove("is-visible"), 900);
    toast("已保存");
  },
  didExport: format => toast(`已导出 ${String(format).toUpperCase()}`),
  exportHTML: () => exportDocument({ theme: "current", background: true }),
  exportDocument,
  command: execute,
  heading: setHeading,
  toggleSidebar: () => $("#sidebar").classList.toggle("is-hidden"),
  toggleSource,
  toggleFocus: () => $("#focus-button").click(),
  toggleTypewriter: () => $("#typewriter-button").click(),
  togglePreferences,
  toggleExport: toggleExportDialog,
  showFind,
  zoom: direction => {
    state.zoom = direction === 0 ? 1 : Math.min(1.8, Math.max(.7, state.zoom + direction * .1));
    write.style.fontSize = `${17 * state.zoom}px`;
    sourceEditor.style.fontSize = `${14 * state.zoom}px`;
    toast(`${Math.round(state.zoom * 100)}%`);
  }
};

restorePreferences();
const browserDraft = (window.webkit || window.moryNative) ? null : localStorage.getItem("mory.draft");
createUntitledDocument(browserDraft || defaultMarkdown, { announce: false, notifyHost: false });
bridge({ type: "ready" });
