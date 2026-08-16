const blockStart = /^(#{1,6}\s|>|[-*+]\s|\d+[.)]\s|```|~~~| {0,3}([-*_])(?:\s*\2){2,}\s*$)/;

export function escapeHTML(value) {
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

export function inlineMarkdown(source) {
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

export function markdownToHTML(markdown) {
  // UTF-8 BOM 只可能位于文稿开头；去除它，避免首个块无法匹配 Markdown 标记。
  const lines = String(markdown ?? "").replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n").split("\n");
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

export function editorToMarkdown(root) {
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

export function documentStats(markdown) {
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
