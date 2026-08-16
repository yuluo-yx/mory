const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const root = path.join(__dirname, "..");

test("桌面宿主使用原子工作区快照避免文件列表竞态", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  assert.match(electron, /window\.Mory\.setWorkspaceSnapshot/);
  assert.match(macOS, /window\.Mory\.setWorkspaceSnapshot/);
});

test("桌面宿主都提供文件创建时间供侧栏稳定排序", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "workspaces.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "WorkspaceManager.swift"), "utf8");
  assert.match(electron, /stat\.birthtimeMs/);
  assert.match(electron, /compareDocumentsByCreation/);
  assert.match(macOS, /\.creationDateKey/);
  assert.match(macOS, /"createdAt"/);
});

test("macOS 与 Windows 宿主递归监听工作区并刷新原子快照", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "workspace-watcher.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "WorkspaceWatcher.swift"), "utf8");
  assert.match(electron, /fs\.watch\(nextRoot, \{ recursive: true \}/);
  assert.match(electron, /setInterval\(scheduleRefresh, pollIntervalMs\)/);
  assert.match(electron, /Promise\.resolve\(onChange\(\)\)/);
  assert.match(macOS, /FSEventStreamCreate\(/);
  assert.match(macOS, /kFSEventStreamCreateFlagFileEvents/);
  assert.match(macOS, /kFSEventStreamCreateFlagWatchRoot/);
});

test("macOS 与 Windows 都通过系统废纸篓删除文稿", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(electron, /case "deleteDocument"/);
  assert.match(electron, /shell\.trashItem\(filePath\)/);
  assert.match(macOS, /case "deleteDocument"/);
  assert.match(macOS, /FileManager\.default\.trashItem/);
  assert.match(web, /hostRequest\("deleteDocument"/);
  assert.match(web, /localized\("文档已移到废纸篓"\)/);
});

test("macOS PDF 导出使用 WebKit 异步生成并在后台分页", () => {
  const source = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const paginator = fs.readFileSync(path.join(root, "Sources", "Mory", "PDFPaginator.swift"), "utf8");
  assert.match(source, /webView\.createPDF\(/);
  assert.match(source, /Task\.detached\(priority: \.userInitiated\)/);
  assert.match(source, /PDFPaginator\.write\(/);
  assert.match(paginator, /context\.beginPDFPage/);
  assert.match(paginator, /context\.drawPDFPage/);
  assert.doesNotMatch(source, /operation\.run\(\)/);
});

test("macOS 左侧顶部与正文标题栏共用原生窗口放大与还原交互", () => {
  const host = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const typingSmoke = fs.readFileSync(path.join(root, "Tests", "MacTypingSmoke.swift"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(web, /\$\$\("\.titlebar, \.traffic-space"\)/);
  assert.match(web, /region\.addEventListener\("dblclick", handleWindowTitlebarDoubleClick\)/);
  assert.match(web, /windowTitlebarDoubleClick/);
  assert.match(host, /case "windowTitlebarDoubleClick"/);
  assert.match(host, /window\.performZoom\(nil\)/);
  assert.match(host, /let restored = !window\.isZoomed\s+&& abs\(actual\.width - restoredFrame\.width\)/);
  assert.match(host, /document\.querySelector\('\.traffic-space'\)/);
  assert.match(host, /new MouseEvent\('dblclick'/);
  assert.match(typingSmoke, /code: document\.querySelector\('#write > pre code'\)\?\.innerText/);
});

test("知识图谱在 HTML 画布捕获滚轮并按 D3 官方公式归一化", () => {
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(web, /#graph-canvas"\)\.addEventListener\("wheel", handleGraphWheel, \{ passive: false \}\)/);
  assert.match(web, /event\.deltaMode === 1 \? \.05 : event\.deltaMode \? 1 : \.002/);
  assert.match(web, /state\.graphZoom\.scaleBy/);
  assert.match(web, /svg\.call\(zoom\)\.on\("wheel\.zoom", null\)/);
});
