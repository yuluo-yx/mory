const { app, BrowserWindow, dialog, ipcMain, Menu, shell } = require("electron");
const fs = require("node:fs/promises");
const path = require("node:path");
const { createWorkspaceDirectory, createWorkspaceManager, importImage, listDirectories, listDocuments, loadDocumentAssets, readDocumentImage, readWorkspaceDocuments, relocateDocumentAssets, sanitizeSegment } = require("./workspaces.cjs");
const { createThemeManager } = require("./themes.cjs");
const { createWorkspaceWatcher } = require("./workspace-watcher.cjs");

let mainWindow;
let currentFilePath = null;
let currentMarkdown = "";
let currentDocumentName = "未命名.md";
let editorReady = false;
let pendingDocument = null;
let workspaceManager;
let themeManager;
let interfaceLocale = "zh-CN";
const workspaceWatcher = createWorkspaceWatcher({
  onChange: () => refreshWorkspace(),
  onError: error => console.error("工作区文件监听失败：", error)
});

const menuEnglish = {
  "文件": "File", "新建": "New", "新建目录": "New Folder", "打开…": "Open…", "打开文件夹…": "Open Folder…", "保存": "Save", "另存为…": "Save As…", "导出": "Export", "退出": "Quit",
  "编辑": "Edit", "撤销": "Undo", "重做": "Redo", "剪切": "Cut", "复制": "Copy", "粘贴": "Paste", "全选": "Select All", "查找和替换": "Find and Replace",
  "格式": "Format", "加粗": "Bold", "斜体": "Italic", "删除线": "Strikethrough", "行内代码": "Inline Code",
  "显示": "View", "显示／隐藏侧边栏": "Show/Hide Sidebar", "源代码模式": "Source Mode", "专注模式": "Focus Mode", "打字机模式": "Typewriter Mode",
  "实际大小": "Actual Size", "放大": "Zoom In", "缩小": "Zoom Out", "全屏": "Full Screen", "帮助": "Help", "关于 Mory": "About Mory", "偏好设置": "Preferences"
};

function localizeMenu(items) {
  for (const item of items) {
    if (interfaceLocale === "en" && menuEnglish[item.label]) item.label = menuEnglish[item.label];
    else if (interfaceLocale === "en" && /^(\d) 级标题$/.test(item.label || "")) item.label = `Heading ${RegExp.$1}`;
    if (Array.isArray(item.submenu)) localizeMenu(item.submenu);
  }
  return items;
}

function storageSidecarPath() {
  const filename = process.platform === "win32" ? "mory-storage.exe" : "mory-storage";
  return app.isPackaged
    ? path.join(process.resourcesPath, "storage", filename)
    : path.join(__dirname, "..", ".build", "storage", filename);
}

function runEditor(source) {
  if (!mainWindow || mainWindow.isDestroyed()) return Promise.resolve();
  return mainWindow.webContents.executeJavaScript(source, true);
}

function sendJSON(functionName, value) {
  return runEditor(`${functionName}(${JSON.stringify(value)})`);
}

function setWindowTitle(title, dirty = false) {
  if (!mainWindow) return;
  mainWindow.setTitle(`${dirty ? "● " : ""}${title || "未命名"} — Mory`);
  mainWindow.setDocumentEdited?.(dirty);
}

async function loadFile(filePath) {
  try {
    const markdown = await fs.readFile(filePath, "utf8");
    currentFilePath = filePath;
    currentMarkdown = markdown;
    currentDocumentName = path.basename(filePath);
    setWindowTitle(path.basename(filePath));
    const assets = await loadDocumentAssets(filePath, markdown);
    const document = { markdown, path: filePath, name: path.basename(filePath), assets };
    if (editorReady) await sendJSON("window.Mory.openDocument", document);
    else pendingDocument = document;
  } catch (error) {
    await dialog.showMessageBox(mainWindow, {
      type: "error",
      title: "Mory",
      message: "无法打开文件",
      detail: error.message
    });
  }
}

async function openDocument() {
  const result = await dialog.showOpenDialog(mainWindow, {
    properties: ["openFile"],
    filters: [
      { name: "Markdown", extensions: ["md", "markdown", "mmd", "mdown", "mkd"] },
      { name: "文本文件", extensions: ["txt", "text"] },
      { name: "所有文件", extensions: ["*"] }
    ]
  });
  if (!result.canceled && result.filePaths[0]) await loadFile(result.filePaths[0]);
}

async function openFolder() {
  const result = await dialog.showOpenDialog(mainWindow, { properties: ["openDirectory"] });
  if (result.canceled || !result.filePaths[0]) return;
  try {
    await workspaceManager.save({ name: path.basename(result.filePaths[0]) || "本地工作区", provider: "local", localPath: result.filePaths[0] });
    await refreshWorkspace();
  } catch (error) {
    await dialog.showMessageBox(mainWindow, { type: "error", title: "Mory", message: "无法读取文件夹", detail: error.message });
  }
}

async function getMarkdown() {
  const markdown = await runEditor("window.Mory.getMarkdown()");
  return typeof markdown === "string" ? markdown : currentMarkdown;
}

function suggestedDocumentName(markdown) {
  const heading = String(markdown || "").match(/^#\s+(.+?)\s*#*\s*$/m)?.[1]?.replace(/[*_`~]/g, "").trim();
  const fallback = /^未命名(?: \d+)?\.md$/i.test(currentDocumentName) ? currentDocumentName.replace(/\.md$/i, "") : path.basename(currentDocumentName, path.extname(currentDocumentName));
  return `${sanitizeSegment(heading || fallback || "未命名")}.md`;
}

async function availableDocumentPath(root, filename) {
  const extension = path.extname(filename) || ".md";
  const base = path.basename(filename, extension);
  for (let serial = 1; ; serial += 1) {
    const candidate = path.join(root, serial === 1 ? `${base}${extension}` : `${base} ${serial}${extension}`);
    try { await fs.access(candidate); }
    catch (error) { if (error.code === "ENOENT") return candidate; throw error; }
  }
}

async function writeDocument(filePath, sourceMarkdown) {
  try {
    let markdown = typeof sourceMarkdown === "string" ? sourceMarkdown : await getMarkdown();
    markdown = await relocateDocumentAssets({
      root: workspaceManager.activeRoot(), markdown, oldPath: currentFilePath, oldName: currentDocumentName, newPath: filePath
    });
    await fs.writeFile(filePath, markdown, "utf8");
    currentMarkdown = markdown;
    currentFilePath = filePath;
    currentDocumentName = path.basename(filePath);
    setWindowTitle(path.basename(filePath));
    const assets = await loadDocumentAssets(filePath, markdown);
    await sendJSON("window.Mory.didSave", { path: filePath, name: path.basename(filePath), markdown, assets });
    await refreshWorkspace();
  } catch (error) {
    await dialog.showMessageBox(mainWindow, { type: "error", title: "Mory", message: "无法保存文件", detail: error.message });
  }
}

async function saveAs() {
  const result = await dialog.showSaveDialog(mainWindow, {
    defaultPath: currentFilePath || path.join(workspaceManager.activeRoot(), currentDocumentName),
    filters: [{ name: "Markdown", extensions: ["md"] }]
  });
  if (!result.canceled && result.filePath) await writeDocument(result.filePath);
}

async function refreshWorkspace() {
  const root = workspaceManager.activeRoot();
  await fs.mkdir(root, { recursive: true });
  workspaceWatcher.start(root);
  const [files, directories] = await Promise.all([listDocuments(root), listDirectories(root)]);
  await sendJSON("window.Mory.setWorkspaceSnapshot", { state: workspaceManager.state(), files, directories });
}

async function handleWorkspaceRequest(method, args = {}) {
  switch (method) {
    case "workspaceState":
      return workspaceManager.state();
    case "chooseLocalWorkspace": {
      const result = await dialog.showOpenDialog(mainWindow, { properties: ["openDirectory", "createDirectory"] });
      if (result.canceled || !result.filePaths[0]) return { canceled: true };
      const state = await workspaceManager.save({
        id: args.id || undefined,
        name: args.name || path.basename(result.filePaths[0]) || "本地工作区",
        provider: "local",
        localPath: result.filePaths[0]
      });
      await refreshWorkspace();
      return state;
    }
    case "saveWorkspace": {
      const state = await workspaceManager.save(args.workspace || {});
      await refreshWorkspace();
      return state;
    }
    case "activateWorkspace": {
      const state = await workspaceManager.activate(String(args.id || ""));
      await refreshWorkspace();
      return state;
    }
    case "removeWorkspace": {
      const state = await workspaceManager.remove(String(args.id || ""));
      await refreshWorkspace();
      return state;
    }
    case "deleteDocument": {
      const filePath = String(args.path || "");
      if (!filePath) throw new Error("文稿路径为空。");
      const name = String(args.name || path.basename(filePath));
      const english = interfaceLocale === "en";
      const confirmation = await dialog.showMessageBox(mainWindow, {
        type: "warning",
        title: english ? "Delete document" : "删除文稿",
        message: english ? `Move “${name}” to the Recycle Bin?` : `要将“${name}”移到废纸篓吗？`,
        detail: english ? "The document can be restored from the system Recycle Bin." : "可以从系统废纸篓中恢复该文稿。",
        buttons: english ? ["Move to Recycle Bin", "Cancel"] : ["移到废纸篓", "取消"],
        defaultId: 1,
        cancelId: 1,
        noLink: true
      });
      if (confirmation.response !== 0) return { canceled: true };
      try {
        await shell.trashItem(filePath);
      } catch (error) {
        if (error.code !== "ENOENT") throw error;
      }
      return { deleted: true };
    }
    case "createDirectory": {
      const directory = await createWorkspaceDirectory(workspaceManager.activeRoot(), args.relativePath);
      await refreshWorkspace();
      return directory;
    }
    case "syncWorkspace": {
      const action = args.action === "push" ? "push" : "pull";
      const summary = await workspaceManager.sync(action);
      await refreshWorkspace();
      return summary;
    }
    case "importImage":
      return importImage({ root: workspaceManager.activeRoot(), ...args });
    case "documentAssets":
      return currentFilePath ? loadDocumentAssets(currentFilePath, String(args.markdown || "")) : {};
    case "documentImage":
      return readDocumentImage(workspaceManager.activeRoot(), args.path);
    case "revealFile": {
      const root = path.resolve(workspaceManager.activeRoot());
      const filePath = path.resolve(String(args.path || ""));
      const local = path.relative(root, filePath);
      if (!local || local === ".." || local.startsWith(`..${path.sep}`) || path.isAbsolute(local)) throw new Error("文件必须位于当前工作区内。");
      shell.showItemInFolder(filePath);
      return { revealed: true };
    }
    case "readDocument": {
      const root = path.resolve(workspaceManager.activeRoot());
      const filePath = path.resolve(String(args.path || ""));
      const local = path.relative(root, filePath);
      if (!local || local === ".." || local.startsWith(`..${path.sep}`) || path.isAbsolute(local)) throw new Error("文稿必须位于当前工作区内。");
      const markdown = await fs.readFile(filePath, "utf8");
      return { name: path.basename(filePath), path: filePath, markdown, assets: await loadDocumentAssets(filePath, markdown) };
    }
    case "workspaceDocuments":
      return readWorkspaceDocuments(workspaceManager.activeRoot());
    case "listThemes":
      return themeManager.list();
    case "importTheme": {
      const result = await dialog.showOpenDialog(mainWindow, {
        properties: ["openFile"],
        filters: [{ name: "CSS Theme", extensions: ["css"] }]
      });
      if (result.canceled || !result.filePaths[0]) return { canceled: true };
      return { themes: await themeManager.importFile(result.filePaths[0]) };
    }
    case "openThemeFolder": {
      await themeManager.initialize();
      await fs.mkdir(themeManager.directory, { recursive: true });
      const error = await shell.openPath(themeManager.directory);
      if (error) throw new Error(error);
      return { opened: true };
    }
    case "chooseThemeFolder": {
      await themeManager.initialize();
      const result = await dialog.showOpenDialog(mainWindow, {
        defaultPath: themeManager.directory,
        properties: ["openDirectory", "createDirectory"]
      });
      if (result.canceled || !result.filePaths[0]) return { canceled: true };
      return themeManager.setDirectory(result.filePaths[0]);
    }
    default:
      throw new Error(`未知宿主请求：${method}`);
  }
}

async function saveDocument() {
  if (currentFilePath) await writeDocument(currentFilePath);
  else if (workspaceManager?.active()?.isImplicit !== true) {
    const markdown = await getMarkdown();
    await writeDocument(await availableDocumentPath(workspaceManager.activeRoot(), suggestedDocumentName(markdown)), markdown);
  } else await saveAs();
}

async function renderExportHTML(options) {
  return runEditor(`window.Mory.exportDocument(${JSON.stringify(options)})`);
}

async function createExportView(html, width = 900) {
  const view = new BrowserWindow({
    show: false,
    width,
    height: 900,
    backgroundColor: "#ffffff",
    webPreferences: { offscreen: true, sandbox: true }
  });
  await view.loadURL(`data:text/html;charset=UTF-8,${encodeURIComponent(html)}`);
  await new Promise(resolve => setTimeout(resolve, 180));
  return view;
}

async function exportRendered(options = {}) {
  const format = options.format || "html";
  const extension = format === "jpeg" ? "jpg" : format;
  const filterNames = { html: "HTML", pdf: "PDF", png: "PNG", jpeg: "JPEG" };
  const result = await dialog.showSaveDialog(mainWindow, {
    defaultPath: `${currentFilePath ? path.basename(currentFilePath, path.extname(currentFilePath)) : "未命名"}.${extension}`,
    filters: [{ name: filterNames[format] || format.toUpperCase(), extensions: [extension] }]
  });
  if (result.canceled || !result.filePath) return;
  let exportView;
  try {
    const html = await renderExportHTML(options);
    if (format === "html") {
      await fs.writeFile(result.filePath, html, "utf8");
    } else if (format === "pdf") {
      exportView = await createExportView(html, 920);
      const pdf = await exportView.webContents.printToPDF({
        printBackground: options.background !== false,
        pageSize: options.paper || "A4",
        margins: { top: 0.45, bottom: 0.45, left: 0.5, right: 0.5 }
      });
      await fs.writeFile(result.filePath, pdf);
    } else {
      const width = Math.min(2400, Math.max(480, Number(options.width) || 900));
      exportView = await createExportView(html, width);
      const height = await exportView.webContents.executeJavaScript("Math.max(document.documentElement.scrollHeight, document.body.scrollHeight)");
      if (height > 28000) throw new Error("文档超过 28000 像素，请降低图片宽度或改用 PDF 导出。");
      exportView.setContentSize(width, Math.max(300, Math.ceil(height)));
      await new Promise(resolve => setTimeout(resolve, 120));
      const image = await exportView.webContents.capturePage({ x: 0, y: 0, width, height: Math.ceil(height) });
      const buffer = format === "jpeg" ? image.toJPEG(92) : image.toPNG();
      await fs.writeFile(result.filePath, buffer);
    }
    await runEditor(`window.Mory.didExport(${JSON.stringify(format)})`);
  } catch (error) {
    await dialog.showMessageBox(mainWindow, { type: "error", title: "Mory", message: "导出失败", detail: error.message });
  } finally {
    exportView?.destroy();
  }
}

function newDocument() {
  currentFilePath = null;
  currentMarkdown = "";
  currentDocumentName = "未命名.md";
  setWindowTitle("未命名");
  runEditor("window.Mory.newDocument()");
}

function buildMenu() {
  const template = [
    {
      label: "文件",
      submenu: [
        { label: "新建", accelerator: "CmdOrCtrl+N", click: newDocument },
        { label: "新建目录", accelerator: "CmdOrCtrl+Shift+N", click: () => runEditor("window.Mory.newFolder()") },
        { label: "打开…", accelerator: "CmdOrCtrl+O", click: openDocument },
        { label: "打开文件夹…", accelerator: "CmdOrCtrl+Shift+O", click: openFolder },
        { type: "separator" },
        { label: "保存", accelerator: "CmdOrCtrl+S", click: saveDocument },
        { label: "另存为…", accelerator: "CmdOrCtrl+Shift+S", click: saveAs },
        { type: "separator" },
        {
          label: "导出",
          submenu: [
            { label: "PDF…", click: () => exportRendered({ format: "pdf", theme: "current", paper: "A4", background: true }) },
            { label: "HTML…", click: () => exportRendered({ format: "html", theme: "current", background: true }) },
            { label: "PNG…", click: () => exportRendered({ format: "png", theme: "current", width: 900, background: true }) },
            { label: "JPEG…", click: () => exportRendered({ format: "jpeg", theme: "current", width: 900, background: true }) }
          ]
        },
        { type: "separator" },
        { role: "quit", label: "退出" }
      ]
    },
    {
      label: "编辑",
      submenu: [
        { role: "undo", label: "撤销" }, { role: "redo", label: "重做" },
        { type: "separator" },
        { role: "cut", label: "剪切" }, { role: "copy", label: "复制" },
        { role: "paste", label: "粘贴" }, { role: "selectAll", label: "全选" },
        { type: "separator" },
        { label: "查找和替换", accelerator: "CmdOrCtrl+F", click: () => runEditor("window.Mory.showFind()") }
      ]
    },
    {
      label: "格式",
      submenu: [
        { label: "加粗", accelerator: "CmdOrCtrl+B", click: () => runEditor("window.Mory.command('bold')") },
        { label: "斜体", accelerator: "CmdOrCtrl+I", click: () => runEditor("window.Mory.command('italic')") },
        { label: "删除线", click: () => runEditor("window.Mory.command('strike')") },
        { label: "行内代码", click: () => runEditor("window.Mory.command('code')") },
        { type: "separator" },
        ...Array.from({ length: 6 }, (_, index) => ({ label: `${index + 1} 级标题`, click: () => runEditor(`window.Mory.heading(${index + 1})`) }))
      ]
    },
    {
      label: "显示",
      submenu: [
        { label: "显示／隐藏侧边栏", accelerator: "CmdOrCtrl+Shift+L", click: () => runEditor("window.Mory.toggleSidebar()") },
        { label: "源代码模式", accelerator: "CmdOrCtrl+/", click: () => runEditor("window.Mory.toggleSource()") },
        { label: "专注模式", click: () => runEditor("window.Mory.toggleFocus()") },
        { label: "打字机模式", click: () => runEditor("window.Mory.toggleTypewriter()") },
        { type: "separator" },
        { role: "resetZoom", label: "实际大小" }, { role: "zoomIn", label: "放大" }, { role: "zoomOut", label: "缩小" },
        { role: "togglefullscreen", label: "全屏" }
      ]
    },
    {
      label: "帮助",
      submenu: [
        { label: "关于 Mory", click: () => dialog.showMessageBox(mainWindow, { title: "关于 Mory", message: "Mory 0.1.0", detail: "一个跨平台、专注的 Markdown 编辑器。" }) },
        { label: "偏好设置", accelerator: "CmdOrCtrl+,", click: () => runEditor("window.Mory.togglePreferences()") }
      ]
    }
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(localizeMenu(template)));
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1180,
    height: 790,
    minWidth: 760,
    minHeight: 520,
    backgroundColor: "#fbfbfa",
    show: false,
    title: "未命名 — Mory",
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    }
  });
  mainWindow.loadFile(path.join(__dirname, "..", "Sources", "Mory", "Web", "index.html"));
  mainWindow.once("ready-to-show", () => mainWindow.show());
  mainWindow.on("closed", () => { mainWindow = null; });
  buildMenu();
}

ipcMain.on("mory:message", async (_event, payload) => {
  if (!payload || typeof payload.type !== "string") return;
  if (payload.type === "ready") {
    editorReady = true;
    await refreshWorkspace();
    if (pendingDocument !== null) {
      await sendJSON("window.Mory.openDocument", pendingDocument);
      pendingDocument = null;
    }
  } else if (payload.type === "changed") {
    currentMarkdown = typeof payload.markdown === "string" ? payload.markdown : currentMarkdown;
    currentDocumentName = typeof payload.name === "string" ? payload.name : currentDocumentName;
    setWindowTitle(currentFilePath ? path.basename(currentFilePath) : currentDocumentName.replace(/\.md$/i, ""), true);
  } else if (payload.type === "documentSelected") {
    currentFilePath = typeof payload.path === "string" && payload.path ? payload.path : null;
    currentMarkdown = typeof payload.markdown === "string" ? payload.markdown : "";
    currentDocumentName = typeof payload.name === "string" && payload.name ? payload.name : "未命名.md";
    setWindowTitle(currentFilePath ? path.basename(currentFilePath) : currentDocumentName.replace(/\.md$/i, ""), payload.dirty === true);
  } else if (payload.type === "openFile" && typeof payload.path === "string") {
    await loadFile(payload.path);
  } else if (payload.type === "title" && !currentFilePath && typeof payload.value === "string") {
    setWindowTitle(payload.value || currentDocumentName.replace(/\.md$/i, ""), payload.dirty === true);
  } else if (payload.type === "export" && payload.options && typeof payload.options === "object") {
    await exportRendered(payload.options);
  } else if (payload.type === "localeChanged") {
    interfaceLocale = payload.locale === "en" ? "en" : "zh-CN";
    buildMenu();
  }
});

ipcMain.handle("mory:request", async (_event, payload) => {
  if (!payload || typeof payload.method !== "string") throw new Error("宿主请求格式无效。");
  return handleWorkspaceRequest(payload.method, payload.args);
});

app.whenReady().then(async () => {
  workspaceManager = createWorkspaceManager({ userDataPath: app.getPath("userData"), sidecarPath: storageSidecarPath });
  themeManager = createThemeManager({ userDataPath: app.getPath("userData") });
  await workspaceManager.initialize();
  await themeManager.initialize();
  createWindow();
  const argument = process.argv.find(value => /\.(?:md|markdown|mmd|mdown|mkd|txt)$/i.test(value));
  if (argument) loadFile(path.resolve(argument));
  app.on("activate", () => { if (BrowserWindow.getAllWindows().length === 0) createWindow(); });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});

app.on("before-quit", () => workspaceWatcher.stop());
