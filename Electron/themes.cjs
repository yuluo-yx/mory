const crypto = require("node:crypto");
const fs = require("node:fs/promises");
const path = require("node:path");

const MAX_THEME_BYTES = 1024 * 1024;
const MAX_ASSET_BYTES = 5 * 1024 * 1024;

function themeID(filename) {
  const basename = path.basename(filename, path.extname(filename)).normalize("NFKC").toLocaleLowerCase();
  const readable = basename.replace(/[^\p{Letter}\p{Number}-]+/gu, "-").replace(/^-+|-+$/g, "").slice(0, 48) || "theme";
  return `user-${readable}-${crypto.createHash("sha256").update(filename).digest("hex").slice(0, 8)}`;
}

function assetMime(filename) {
  return ({
    ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
    ".webp": "image/webp", ".svg": "image/svg+xml", ".woff": "font/woff", ".woff2": "font/woff2",
    ".ttf": "font/ttf", ".otf": "font/otf"
  })[path.extname(filename).toLocaleLowerCase()] || "application/octet-stream";
}

async function inlineThemeAssets(css, directory) {
  const matches = [...String(css).matchAll(/url\(\s*(["']?)([^"')]+)\1\s*\)/gi)];
  let result = String(css);
  let total = 0;
  for (const match of matches) {
    const reference = match[2].trim();
    if (!reference || /^(?:data:|https?:|file:|#|\/)/i.test(reference)) continue;
    let decoded = reference;
    try { decoded = decodeURI(reference); } catch { /* Keep the original relative path. */ }
    const resolved = path.resolve(directory, decoded.split(/[?#]/)[0]);
    const relative = path.relative(directory, resolved);
    if (relative === ".." || relative.startsWith(`..${path.sep}`)) continue;
    try {
      const data = await fs.readFile(resolved);
      total += data.length;
      if (total > MAX_ASSET_BYTES) throw new Error("主题资源总大小不能超过 5 MB。");
      const dataURL = `data:${assetMime(resolved)};base64,${data.toString("base64")}`;
      result = result.split(match[0]).join(`url("${dataURL}")`);
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  return result;
}

function createThemeManager({ userDataPath }) {
  const defaultDirectory = path.join(userDataPath, "themes");
  const settingsPath = path.join(userDataPath, "theme-settings.json");
  let directory = defaultDirectory;
  let initialized = false;

  async function initialize() {
    if (initialized) return directory;
    try {
      const saved = JSON.parse(await fs.readFile(settingsPath, "utf8"));
      if (typeof saved.directory === "string" && path.isAbsolute(saved.directory)) directory = path.resolve(saved.directory);
    } catch (error) {
      if (error.code !== "ENOENT" && !(error instanceof SyntaxError)) throw error;
    }
    await fs.mkdir(directory, { recursive: true });
    initialized = true;
    return directory;
  }

  async function list() {
    await initialize();
    const entries = (await fs.readdir(directory, { withFileTypes: true }))
      .filter(entry => entry.isFile() && path.extname(entry.name).toLocaleLowerCase() === ".css")
      .sort((left, right) => left.name.localeCompare(right.name, "zh-CN", { numeric: true }));
    const themes = [];
    for (const entry of entries) {
      const filePath = path.join(directory, entry.name);
      const stat = await fs.stat(filePath);
      if (stat.size > MAX_THEME_BYTES) continue;
      const css = await inlineThemeAssets(await fs.readFile(filePath, "utf8"), directory);
      themes.push({ id: themeID(entry.name), name: path.basename(entry.name, ".css"), filename: entry.name, css });
    }
    return themes;
  }

  async function importFile(source) {
    await initialize();
    if (path.extname(source).toLocaleLowerCase() !== ".css") throw new Error("请选择 CSS 主题文件。");
    const stat = await fs.stat(source);
    if (!stat.isFile() || stat.size > MAX_THEME_BYTES) throw new Error("主题文件无效或超过 1 MB。");
    await fs.mkdir(directory, { recursive: true });
    const destination = path.join(directory, path.basename(source));
    if (path.resolve(source) !== path.resolve(destination)) await fs.copyFile(source, destination);
    return list();
  }

  async function setDirectory(value) {
    if (typeof value !== "string" || !value.trim()) throw new Error("请选择有效的主题目录。");
    const next = path.resolve(value);
    await fs.mkdir(next, { recursive: true });
    await fs.mkdir(userDataPath, { recursive: true });
    directory = next;
    initialized = true;
    await fs.writeFile(settingsPath, `${JSON.stringify({ directory }, null, 2)}\n`, "utf8");
    return { directory, themes: await list() };
  }

  return { get directory() { return directory; }, importFile, initialize, list, setDirectory };
}

module.exports = { createThemeManager, inlineThemeAssets, themeID };
