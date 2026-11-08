export function transform({ markdown }) {
  const words = markdown.trim().split(/\s+/).filter(Boolean).length;
  const characters = Array.from(markdown).length;
  return { message: `${words} words · ${characters} characters · ${Math.max(1, Math.ceil(words / 220))} min read` };
}
