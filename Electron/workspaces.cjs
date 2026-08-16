const { spawn } = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");

const DOCUMENT_EXTENSIONS = new Set([".md", ".markdown", ".mmd", ".mdown", ".mkd", ".txt", ".text"]);
const IMAGE_MIME = new Map([
  ["image/png", ".png"], ["image/jpeg", ".jpg"], ["image/gif", ".gif"],
  ["image/webp", ".webp"], ["image/svg+xml", ".svg"], ["image/bmp", ".bmp"]
]);
const SECRET_FIELDS = ["token", "accessKeySecret", "sessionToken", "password", "privateKey"];

function createWorkspaceManager({ userDataPath, sidecarPath, defaultRoot = path.join(os.homedir(), "Documents", "Mory") }) {
  const configPath = path.join(userDataPath, "workspaces.json");
  const cacheRoot = path.join(userDataPath, "workspaces");
  let workspaces = [];
  let activeId = "";

  async function initialize() {
    await fs.mkdir(cacheRoot, { recursive: true });
    try {
      const stored = JSON.parse(await fs.readFile(configPath, "utf8"));
      workspaces = Array.isArray(stored.workspaces) ? stored.workspaces : [];
      activeId = typeof stored.activeId === "string" ? stored.activeId : "";
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    if (!workspaces.length) {
      const localPath = defaultRoot;
      await fs.mkdir(localPath, { recursive: true });
      workspaces = [{ id: crypto.randomUUID(), name: "本地工作区", provider: "local", localPath }];
      activeId = workspaces[0].id;
      await persist();
    }
    if (!workspaces.some(workspace => workspace.id === activeId)) activeId = workspaces[0].id;
    await fs.mkdir(activeRoot(), { recursive: true });
    return state();
  }

  function active() {
    return workspaces.find(workspace => workspace.id === activeId) || workspaces[0];
  }

  function rootFor(workspace) {
    return workspace.provider === "local" ? workspace.localPath : path.join(cacheRoot, workspace.id);
  }

  function activeRoot() {
    return rootFor(active());
  }

  function publicWorkspace(workspace) {
    const result = { ...workspace };
    for (const field of SECRET_FIELDS) {
      result[`${field}Configured`] = Boolean(result[field]);
      delete result[field];
    }
    result.localPath = rootFor(workspace);
    return result;
  }

  function state() {
    return { activeId, workspaces: workspaces.map(publicWorkspace) };
  }

  async function persist() {
    await fs.mkdir(path.dirname(configPath), { recursive: true });
    await fs.writeFile(configPath, JSON.stringify({ version: 1, activeId, workspaces }, null, 2), { mode: 0o600 });
  }

  async function save(input) {
    const existing = workspaces.find(workspace => workspace.id === input.id);
    const id = existing?.id || crypto.randomUUID();
    const provider = String(input.provider || "local");
    const workspace = { ...(existing || {}), ...input, id, provider };
    for (const field of SECRET_FIELDS) {
      if (!input[field] && existing?.[field]) workspace[field] = existing[field];
    }
    if (!workspace.name?.trim()) workspace.name = provider === "local" ? "本地工作区" : provider.toUpperCase();
    if (provider === "local") {
      if (!workspace.localPath) throw new Error("请选择本地工作目录。");
      workspace.localPath = path.resolve(workspace.localPath);
    }
    validateWorkspace(workspace);
    const index = workspaces.findIndex(item => item.id === id);
    if (index >= 0) workspaces[index] = workspace;
    else workspaces.push(workspace);
    activeId = id;
    await fs.mkdir(rootFor(workspace), { recursive: true });
    await persist();
    return state();
  }

  async function activate(id) {
    const workspace = workspaces.find(item => item.id === id);
    if (!workspace) throw new Error("工作区不存在。");
    activeId = id;
    await fs.mkdir(rootFor(workspace), { recursive: true });
    await persist();
    return state();
  }

  async function remove(id) {
    if (workspaces.length <= 1) throw new Error("至少保留一个工作区。");
    workspaces = workspaces.filter(workspace => workspace.id !== id);
    if (activeId === id) activeId = workspaces[0].id;
    await persist();
    return state();
  }

  async function sync(action) {
    const workspace = active();
    if (workspace.provider === "local") return { files: 0, bytes: 0, local: true };
    const payload = { action, root: rootFor(workspace), workspace };
    return runSidecar(sidecarPath(), payload);
  }

  return { initialize, state, save, activate, remove, sync, active, activeRoot };
}

function validateWorkspace(workspace) {
  if (workspace.provider === "github" && (!/^([^/]+)\/([^/]+)$/.test(workspace.repository || "") || !workspace.token)) {
    throw new Error("GitHub 工作区需要 owner/repo 格式的仓库和 Access Token。");
  }
  if (["s3", "s4", "oss"].includes(workspace.provider)
    && (!workspace.region || !workspace.bucket || !workspace.accessKeyId || !workspace.accessKeySecret)) {
    throw new Error("对象存储需要区域、Bucket、Access Key 和 Secret Key。");
  }
  if (workspace.provider === "s4" && !workspace.endpoint) throw new Error("S4 / S3 兼容存储需要 Endpoint。");
  if (workspace.provider === "sftp"
    && (!workspace.host || !workspace.username || !workspace.remotePath || (!workspace.password && !workspace.privateKey))) {
    throw new Error("SFTP 需要服务器、用户名、远端目录，以及密码或私钥。");
  }
}

async function listDocuments(root) {
  const files = [];
  async function visit(directory) {
    const entries = await fs.readdir(directory, { withFileTypes: true });
    entries.sort((left, right) => left.name.localeCompare(right.name, "zh-CN", { numeric: true }));
    for (const entry of entries) {
      if (entry.name === ".git" || entry.name === ".mory" || entry.name.startsWith(".")) continue;
      const fullPath = path.join(directory, entry.name);
      if (entry.isDirectory()) await visit(fullPath);
      else if (entry.isFile() && DOCUMENT_EXTENSIONS.has(path.extname(entry.name).toLowerCase())) {
        files.push({ name: path.relative(root, fullPath), path: fullPath });
      }
    }
  }
  await visit(root);
  return files;
}

function markdownImagePaths(markdown) {
  const paths = new Set();
  const expression = /!\[[^\]]*\]\((?:<([^>]+)>|([^\s)]+))(?:\s+["'][^"']*["'])?\)/g;
  for (const match of markdown.matchAll(expression)) {
    const value = match[1] || match[2] || "";
    if (value && !/^(?:data:|https?:|file:|\/)/i.test(value)) paths.add(decodeURI(value));
  }
  return [...paths];
}

async function loadDocumentAssets(documentPath, markdown) {
  const assets = {};
  for (const relative of markdownImagePaths(markdown)) {
    const resolved = path.resolve(path.dirname(documentPath), relative);
    const local = path.relative(path.dirname(documentPath), resolved);
    if (local === ".." || local.startsWith(`..${path.sep}`)) continue;
    try {
      const data = await fs.readFile(resolved);
      const mime = mimeForPath(resolved);
      assets[relative.replaceAll("\\", "/")] = `data:${mime};base64,${data.toString("base64")}`;
    } catch {
      // 缺失图片仍保留 Markdown 原始地址，编辑器会显示浏览器的加载失败状态。
    }
  }
  return assets;
}

async function importImage({ root, documentPath, documentName, name, mime, data }) {
  const extension = IMAGE_MIME.get(mime);
  if (!extension) throw new Error("仅支持 PNG、JPEG、GIF、WebP、SVG 和 BMP 图片。");
  const source = Buffer.from(String(data || ""), "base64");
  if (!source.length || source.length > 50 * 1024 * 1024) throw new Error("图片为空或超过 50 MB。");
  const documentBase = sanitizeSegment(path.basename(documentName || "未命名.md", path.extname(documentName || "未命名.md")) || "未命名");
  const documentDirectory = documentPath ? path.dirname(documentPath) : root;
  const assetDirectory = path.join(documentDirectory, documentBase);
  await fs.mkdir(assetDirectory, { recursive: true });
  const originalBase = sanitizeSegment(path.basename(name || `图片${extension}`, path.extname(name || "")) || "图片");
  let filename = `${originalBase}${extension}`;
  for (let serial = 2; ; serial += 1) {
    try {
      await fs.writeFile(path.join(assetDirectory, filename), source, { flag: "wx" });
      break;
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      filename = `${originalBase}-${serial}${extension}`;
    }
  }
  const relative = `${documentBase}/${filename}`;
  return { relative, dataURL: `data:${mime};base64,${source.toString("base64")}` };
}

async function relocateDocumentAssets({ root, markdown, oldPath, oldName, newPath }) {
  const oldBase = sanitizeSegment(path.basename(oldName || "未命名.md", path.extname(oldName || "未命名.md")) || "未命名");
  const newName = path.basename(newPath);
  const newBase = sanitizeSegment(path.basename(newName, path.extname(newName)) || "未命名");
  if (oldBase === newBase && (!oldPath || path.dirname(oldPath) === path.dirname(newPath))) return markdown;
  const oldDirectory = path.join(oldPath ? path.dirname(oldPath) : root, oldBase);
  const newDirectory = path.join(path.dirname(newPath), newBase);
  try {
    await fs.access(oldDirectory);
  } catch {
    return markdown;
  }
  try {
    await fs.rename(oldDirectory, newDirectory);
  } catch (error) {
    if (error.code !== "EEXIST" && error.code !== "EXDEV" && error.code !== "ENOTEMPTY") throw error;
    await fs.cp(oldDirectory, newDirectory, { recursive: true, force: false, errorOnExist: false });
  }
  return String(markdown).split(`](${oldBase}/`).join(`](${newBase}/`);
}

function sanitizeSegment(value) {
  const cleaned = value.replace(/[<>:"/\\|?*\u0000-\u001f\s]+/g, "-").replace(/^\.+|[. ]+$/g, "").trim();
  return cleaned || "未命名";
}

function mimeForPath(filePath) {
  const extension = path.extname(filePath).toLowerCase();
  return ({ ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml", ".bmp": "image/bmp" })[extension] || "application/octet-stream";
}

function runSidecar(executable, payload) {
  return new Promise((resolve, reject) => {
    const child = spawn(executable, [], { stdio: ["pipe", "pipe", "pipe"], windowsHide: true });
    const stdout = [];
    const stderr = [];
    child.stdout.on("data", chunk => stdout.push(chunk));
    child.stderr.on("data", chunk => stderr.push(chunk));
    child.on("error", error => reject(new Error(`无法启动存储插件：${error.message}`)));
    child.on("close", code => {
      let result;
      try { result = JSON.parse(Buffer.concat(stdout).toString("utf8")); }
      catch { reject(new Error(`存储插件返回无效结果：${Buffer.concat(stderr).toString("utf8")}`)); return; }
      if (code !== 0 || !result.ok) reject(new Error(result.error || "存储插件执行失败。"));
      else resolve(result.summary || {});
    });
    child.stdin.end(JSON.stringify(payload));
  });
}

module.exports = { createWorkspaceManager, importImage, listDocuments, loadDocumentAssets, markdownImagePaths, relocateDocumentAssets, sanitizeSegment, validateWorkspace };
