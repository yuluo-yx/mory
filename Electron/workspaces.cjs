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
const IMAGE_EXTENSIONS = new Set([".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp"]);
const SECRET_FIELDS = ["token", "accessKeySecret", "sessionToken", "password", "privateKey"];
const STORAGE_FIELDS = [
  "id", "name", "provider", "endpoint", "region", "bucket", "prefix", "accessKeyId",
  "accessKeySecret", "sessionToken", "repository", "branch", "token", "host", "port",
  "username", "password", "privateKey", "knownHosts", "remotePath"
];

function storageWorkspace(workspace) {
  return Object.fromEntries(STORAGE_FIELDS.flatMap(field => workspace[field] === undefined ? [] : [[field, workspace[field]]]));
}

function createWorkspaceManager({ userDataPath, sidecarPath, defaultRoot = path.join(os.homedir(), "Documents", "Mory") }) {
  const configPath = path.join(userDataPath, "workspaces.json");
  const cacheRoot = path.join(userDataPath, "workspaces");
  let workspaces = [];
  let activeId = "";

  async function initialize() {
    await fs.mkdir(cacheRoot, { recursive: true });
    try {
      const stored = JSON.parse(await fs.readFile(configPath, "utf8"));
      workspaces = Array.isArray(stored.workspaces) ? stored.workspaces.map(workspace => ({
        ...workspace,
        isImplicit: typeof workspace.isImplicit === "boolean"
          ? workspace.isImplicit
          : workspace.provider === "local" && workspace.name === "本地工作区" && path.resolve(workspace.localPath || "") === path.resolve(defaultRoot)
      })) : [];
      activeId = typeof stored.activeId === "string" ? stored.activeId : "";
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    if (!workspaces.length) {
      const localPath = defaultRoot;
      await fs.mkdir(localPath, { recursive: true });
      workspaces = [{ id: crypto.randomUUID(), name: "本地工作区", provider: "local", localPath, isImplicit: true }];
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
    const workspace = { ...(existing || {}), ...input, id, provider, isImplicit: false };
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
    const payload = { action, root: rootFor(workspace), workspace: storageWorkspace(workspace) };
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
    for (const entry of entries) {
      if (entry.name === ".git" || entry.name === ".mory" || entry.name.startsWith(".")) continue;
      const fullPath = path.join(directory, entry.name);
      if (entry.isDirectory()) await visit(fullPath);
      else if (entry.isFile() && DOCUMENT_EXTENSIONS.has(path.extname(entry.name).toLowerCase())) {
        const stat = await fs.stat(fullPath);
        const birthtime = Number(stat.birthtimeMs);
        const createdAt = Number.isFinite(birthtime) && birthtime > 0 ? birthtime : Number(stat.ctimeMs);
        files.push({
          name: path.relative(root, fullPath),
          path: fullPath,
          createdAt,
          updatedAt: Number(stat.mtimeMs),
          size: Number(stat.size),
          images: await listDocumentImages(fullPath)
        });
      }
    }
  }
  await visit(root);
  return files.sort(compareDocumentsByCreation);
}

async function listDocumentImages(documentPath) {
  const directory = path.join(path.dirname(documentPath), sanitizeSegment(path.basename(documentPath, path.extname(documentPath))));
  const images = [];
  async function visit(current) {
    let entries;
    try { entries = await fs.readdir(current, { withFileTypes: true }); }
    catch (error) { if (error.code === "ENOENT") return; throw error; }
    for (const entry of entries) {
      if (entry.name.startsWith(".")) continue;
      const fullPath = path.join(current, entry.name);
      if (entry.isDirectory()) await visit(fullPath);
      else if (entry.isFile() && IMAGE_EXTENSIONS.has(path.extname(entry.name).toLowerCase())) {
        const stat = await fs.stat(fullPath);
        images.push({
          name: path.relative(directory, fullPath).replaceAll("\\", "/"),
          path: fullPath,
          relative: path.relative(path.dirname(documentPath), fullPath).replaceAll("\\", "/"),
          updatedAt: Number(stat.mtimeMs),
          size: Number(stat.size)
        });
      }
    }
  }
  await visit(directory);
  return images.sort((left, right) => left.name.localeCompare(right.name, "zh-CN", { numeric: true }));
}

async function readDocumentImage(root, imagePath) {
  const resolvedRoot = path.resolve(root);
  const resolved = path.resolve(String(imagePath || ""));
  const local = path.relative(resolvedRoot, resolved);
  if (!local || local === ".." || local.startsWith(`..${path.sep}`) || path.isAbsolute(local)) throw new Error("图片必须位于当前工作区内。");
  if (!IMAGE_EXTENSIONS.has(path.extname(resolved).toLowerCase())) throw new Error("不支持的图片格式。");
  const data = await fs.readFile(resolved);
  if (data.length > 50 * 1024 * 1024) throw new Error("图片超过 50 MB。");
  return { name: path.basename(resolved), path: resolved, dataURL: `data:${mimeForPath(resolved)};base64,${data.toString("base64")}` };
}

async function listDirectories(root) {
  const directories = [];
  async function visit(directory) {
    const entries = await fs.readdir(directory, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isDirectory() || entry.name === ".git" || entry.name === ".mory" || entry.name.startsWith(".")) continue;
      const fullPath = path.join(directory, entry.name);
      const stat = await fs.stat(fullPath);
      const birthtime = Number(stat.birthtimeMs);
      const createdAt = Number.isFinite(birthtime) && birthtime > 0 ? birthtime : Number(stat.ctimeMs);
      directories.push({ name: path.relative(root, fullPath), path: fullPath, createdAt });
      await visit(fullPath);
    }
  }
  await visit(root);
  return directories.sort((left, right) => String(left.name).localeCompare(String(right.name), "zh-CN", { numeric: true }));
}

function resolveWorkspaceDirectory(root, relativePath) {
  const value = String(relativePath || "").trim().replaceAll("\\", "/");
  if (!value || path.isAbsolute(value)) throw new Error("请输入工作区内的相对目录。");
  const segments = value.split("/");
  if (segments.some(segment => !segment || segment === "." || segment === "..")) throw new Error("目录路径不能包含空层级、. 或 ..。");
  const resolvedRoot = path.resolve(root);
  const resolved = path.resolve(resolvedRoot, ...segments);
  const local = path.relative(resolvedRoot, resolved);
  if (!local || local === ".." || local.startsWith(`..${path.sep}`) || path.isAbsolute(local)) throw new Error("目录必须位于当前工作区内。");
  return { resolved, relative: local };
}

async function createWorkspaceDirectory(root, relativePath) {
  const { resolved, relative } = resolveWorkspaceDirectory(root, relativePath);
  await fs.mkdir(resolved, { recursive: true });
  const stat = await fs.stat(resolved);
  const birthtime = Number(stat.birthtimeMs);
  const createdAt = Number.isFinite(birthtime) && birthtime > 0 ? birthtime : Number(stat.ctimeMs);
  return { name: relative, path: resolved, createdAt };
}

function workspaceEntryPath(root, value, kind = "条目") {
  const resolvedRoot = path.resolve(root);
  const resolved = path.resolve(String(value || ""));
  const local = path.relative(resolvedRoot, resolved);
  if (!local || local === ".." || local.startsWith(`..${path.sep}`) || path.isAbsolute(local)) {
    throw new Error(`${kind}必须位于当前工作区内。`);
  }
  return resolved;
}

async function workspaceDirectory(root, value) {
  const resolvedRoot = path.resolve(root);
  if (!String(value || "").trim()) return resolvedRoot;
  if (path.resolve(String(value)) === resolvedRoot) return resolvedRoot;
  const resolved = workspaceEntryPath(root, value, "目标目录");
  const stat = await fs.stat(resolved);
  if (!stat.isDirectory()) throw new Error("目标必须是当前工作区中的目录。");
  return resolved;
}

function companionAssets(documentPath) {
  return path.join(path.dirname(documentPath), sanitizeSegment(path.basename(documentPath, path.extname(documentPath))));
}

async function entryExists(target) {
  try { await fs.stat(target); return true; }
  catch (error) { if (error.code === "ENOENT") return false; throw error; }
}

async function availableEntryPath(directory, name, isDirectory) {
  const extension = isDirectory ? "" : path.extname(name);
  const base = extension ? name.slice(0, -extension.length) : name;
  for (let serial = 1; ; serial += 1) {
    const suffix = serial === 1 ? "" : serial === 2 ? " 副本" : ` 副本 ${serial - 1}`;
    const candidate = path.join(directory, `${base}${suffix}${extension}`);
    if (!await entryExists(candidate) && (isDirectory || !await entryExists(companionAssets(candidate)))) return candidate;
  }
}

function isSameOrDescendant(parent, candidate) {
  const local = path.relative(parent, candidate);
  return !local || (!local.startsWith(`..${path.sep}`) && local !== ".." && !path.isAbsolute(local));
}

async function createWorkspaceDocument(root, directoryPath, name) {
  const directory = await workspaceDirectory(root, directoryPath);
  const filename = `${sanitizeSegment(path.basename(String(name || "未命名.md"), path.extname(String(name || "未命名.md"))))}.md`;
  const target = await availableEntryPath(directory, filename, false);
  await fs.writeFile(target, "", { flag: "wx" });
  const stat = await fs.stat(target);
  return {
    name: path.basename(target),
    path: target,
    markdown: "",
    createdAt: Number(stat.birthtimeMs || stat.ctimeMs),
    updatedAt: Number(stat.mtimeMs),
    size: Number(stat.size),
    images: []
  };
}

async function copyWorkspaceEntry(root, sourcePath, destinationPath) {
  const source = workspaceEntryPath(root, sourcePath);
  const destination = await workspaceDirectory(root, destinationPath);
  const stat = await fs.stat(source);
  if (stat.isDirectory() && isSameOrDescendant(source, destination)) throw new Error("不能把目录复制到自身或子目录。");
  const target = await availableEntryPath(destination, path.basename(source), stat.isDirectory());
  if (stat.isDirectory()) {
    await fs.cp(source, target, { recursive: true, errorOnExist: true, force: false });
  } else {
    await fs.copyFile(source, target);
    const assets = companionAssets(source);
    if (await entryExists(assets)) await fs.cp(assets, companionAssets(target), { recursive: true, errorOnExist: true, force: false });
  }
  return { name: path.relative(root, target).replaceAll("\\", "/"), path: target, sourcePath: source, isDirectory: stat.isDirectory() };
}

async function moveWorkspaceEntry(root, sourcePath, destinationPath) {
  const source = workspaceEntryPath(root, sourcePath);
  const destination = await workspaceDirectory(root, destinationPath);
  const stat = await fs.stat(source);
  if (path.dirname(source) === destination) throw new Error("条目已经位于所选目录。");
  if (stat.isDirectory() && isSameOrDescendant(source, destination)) throw new Error("不能把目录移动到自身或子目录。");
  const target = await availableEntryPath(destination, path.basename(source), stat.isDirectory());
  try {
    await fs.rename(source, target);
  } catch (error) {
    if (error.code !== "EXDEV") throw error;
    if (stat.isDirectory()) {
      await fs.cp(source, target, { recursive: true, errorOnExist: true, force: false });
      await fs.rm(source, { recursive: true });
    } else {
      await fs.copyFile(source, target);
      await fs.unlink(source);
    }
  }
  if (!stat.isDirectory()) {
    const sourceAssets = companionAssets(source);
    const targetAssets = companionAssets(target);
    if (await entryExists(sourceAssets)) {
      try { await fs.rename(sourceAssets, targetAssets); }
      catch (error) {
        if (error.code !== "EXDEV") throw error;
        await fs.cp(sourceAssets, targetAssets, { recursive: true, errorOnExist: true, force: false });
        await fs.rm(sourceAssets, { recursive: true });
      }
    }
  }
  return { name: path.relative(root, target).replaceAll("\\", "/"), path: target, sourcePath: source, isDirectory: stat.isDirectory() };
}

async function renameWorkspaceEntry(root, sourcePath, requestedName) {
  const source = workspaceEntryPath(root, sourcePath);
  const stat = await fs.stat(source);
  const requested = String(requestedName || "").trim();
  if (!requested || /[\\/]/.test(requested) || path.basename(requested) !== requested || [".", ".."].includes(requested)) {
    throw new Error("名称不能为空或包含路径分隔符。");
  }
  const requestedExtension = stat.isDirectory() ? "" : path.extname(requested);
  const originalExtension = stat.isDirectory() ? "" : path.extname(source);
  const base = sanitizeSegment(stat.isDirectory() ? requested : path.basename(requested, requestedExtension));
  const filename = stat.isDirectory() ? base : `${base}${requestedExtension || originalExtension}`;
  const target = path.join(path.dirname(source), filename);
  if (target === source) throw new Error("名称没有变化。");
  if (await entryExists(target)) throw new Error("同名条目已经存在。");

  if (stat.isDirectory()) {
    await fs.rename(source, target);
    return { name: path.relative(root, target).replaceAll("\\", "/"), path: target, sourcePath: source, isDirectory: true };
  }

  const sourceAssets = companionAssets(source);
  const targetAssets = companionAssets(target);
  const hasAssets = await entryExists(sourceAssets);
  if (hasAssets && await entryExists(targetAssets)) throw new Error("同名图片目录已经存在。");
  const markdown = await fs.readFile(source, "utf8");
  const oldBase = path.basename(sourceAssets);
  const newBase = path.basename(targetAssets);
  const nextMarkdown = markdown.split(`](${oldBase}/`).join(`](${newBase}/`);
  let assetsMoved = false;
  await fs.rename(source, target);
  try {
    if (hasAssets) {
      await fs.rename(sourceAssets, targetAssets);
      assetsMoved = true;
    }
    if (nextMarkdown !== markdown) await fs.writeFile(target, nextMarkdown);
  } catch (error) {
    if (assetsMoved) await fs.rename(targetAssets, sourceAssets).catch(() => {});
    await fs.rename(target, source).catch(() => {});
    throw error;
  }
  return { name: path.relative(root, target).replaceAll("\\", "/"), path: target, sourcePath: source, isDirectory: false };
}

function compareDocumentsByCreation(left, right) {
  const leftTime = Number.isFinite(Number(left.createdAt)) ? Number(left.createdAt) : Number.MAX_SAFE_INTEGER;
  const rightTime = Number.isFinite(Number(right.createdAt)) ? Number(right.createdAt) : Number.MAX_SAFE_INTEGER;
  return leftTime - rightTime
    || String(left.name).localeCompare(String(right.name), "zh-CN", { numeric: true })
    || String(left.path).localeCompare(String(right.path));
}

async function readWorkspaceDocuments(root) {
  const documents = [];
  for (const file of await listDocuments(root)) {
    try {
      const stat = await fs.stat(file.path);
      if (stat.size > 2 * 1024 * 1024) continue;
      documents.push({ ...file, markdown: await fs.readFile(file.path, "utf8") });
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  return documents;
}

function markdownImagePaths(markdown) {
  const paths = new Set();
  const expressions = [
    /!\[[^\]]*\]\((?:<([^>]+)>|([^\s)]+))(?:\s+["'][^"']*["'])?\)/g,
    /<img\b[^>]*\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+))/gi
  ];
  for (const expression of expressions) {
    for (const match of markdown.matchAll(expression)) {
      const value = match.slice(1).find(Boolean) || "";
      if (!value || /^(?:data:|https?:|file:|\/)/i.test(value)) continue;
      try { paths.add(decodeURI(value)); }
      catch { paths.add(value); }
    }
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
      // Preserve the Markdown URL for missing images so the editor can expose the browser failure state.
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

module.exports = { compareDocumentsByCreation, copyWorkspaceEntry, createWorkspaceDirectory, createWorkspaceDocument, createWorkspaceManager, importImage, listDirectories, listDocumentImages, listDocuments, loadDocumentAssets, markdownImagePaths, moveWorkspaceEntry, readDocumentImage, readWorkspaceDocuments, relocateDocumentAssets, renameWorkspaceEntry, resolveWorkspaceDirectory, sanitizeSegment, storageWorkspace, validateWorkspace, workspaceEntryPath };
