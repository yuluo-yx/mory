// Preserve Markdown's two-space hard line breaks and fenced code verbatim.
export function transform({ markdown }) {
  let fence = null;
  const lines = markdown.split("\n").map(line => {
    const marker = line.match(/^\s{0,3}(`{3,}|~{3,})/);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length) fence = null;
      return line;
    }
    if (fence) return line;
    return /  $/.test(line) ? line.replace(/[ \t]+$/, "  ") : line.replace(/[ \t]+$/, "");
  });
  return { markdown: lines.join("\n").replace(/\n*$/, "\n"), message: "Whitespace cleaned" };
}
