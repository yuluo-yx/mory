export function transform({ markdown }) {
  let fence = false;
  const headings = [];
  const counts = {};
  for (const line of markdown.split("\n")) {
    if (/^\s*(```|~~~)/.test(line)) { fence = !fence; continue; }
    const match = !fence && line.match(/^(#{1,6})\s+(.+?)\s*#*$/);
    if (!match) continue;
    const text = match[2];
    const slug = text.toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, "").replace(/\s+/g, "-");
    const count = counts[slug] || 0; counts[slug] = count + 1;
    headings.push("  ".repeat(match[1].length - 1) + `- [${text}](#${slug}${count ? "-" + count : ""})`);
  }
  if (!headings.length) return { message: "Add headings before generating a table of contents" };
  const toc = "<!-- mory:toc -->\n" + headings.join("\n") + "\n<!-- /mory:toc -->";
  return { markdown: markdown.includes("<!-- mory:toc -->")
    ? markdown.replace(/<!-- mory:toc -->[\s\S]*?<!-- \/mory:toc -->/, toc)
    : toc + "\n\n" + markdown, message: "Table of contents updated" };
}
