import { documentStats, editorToMarkdown, escapeHTML, markdownToHTML } from "./markdown.js";
import { buildKnowledgeGraph } from "./knowledge.js";
import {
  calendarColors,
  calendarDateFromKey,
  calendarMarkdown,
  calendarMonthDays,
  calendarRangeDayCount,
  formatFileSize,
  formatUpdatedAt,
  localDateKey,
  mermaidColorThemes,
  mindMapHTML,
  normalizeCalendarDocument,
  normalizeMermaidColorTheme,
  optimizeMarkdownTypography,
  parseCalendarSource,
  serializeCalendarDocument
} from "./editor-features.js";

const $ = selector => document.querySelector(selector);
const $$ = selector => [...document.querySelectorAll(selector)];
const write = $("#write");
const sourceEditor = $("#source-editor");
const workspace = $(".workspace");
const editorScroll = $("#editor-scroll");
const nativeMacHost = Boolean(window.webkit?.messageHandlers?.mory);
const nativeWailsHost = () => window.go?.main?.WindowsHost;
document.documentElement.dataset.host = nativeMacHost ? "mac-native" : (nativeWailsHost() ? "windows-webview2" : (window.moryNative?.platform || "browser"));

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
  directories: [],
  expandedDirectoryPaths: new Set(),
  expandedImagePaths: new Set(),
  selectedWorkspaceEntry: null,
  manualFileOrder: [],
  workspaceDocuments: [],
  activeDocumentId: null,
  documentSerial: 0,
  untitledSequence: 0,
  sourceMode: false,
  dirty: false,
  findMatches: [],
  findIndex: -1,
  zoom: 1,
  titleTouched: false,
  documentTheme: "github",
  themeCSS: new Map(),
  customThemes: [],
  locale: "zh-CN",
  graph: { nodes: [], edges: [] },
  graphSimulation: null,
  graphZoom: null,
  graphZoomScale: 1,
  selectedGraphNodeId: "",
  workspaces: [],
  activeWorkspaceId: "",
  editingWorkspaceId: "",
  showFileDetails: false
};

try {
  const savedFileOrder = JSON.parse(localStorage.getItem("mory.fileOrder") || "[]");
  state.manualFileOrder = Array.isArray(savedFileOrder) ? savedFileOrder.filter(item => typeof item === "string") : [];
} catch {
  state.manualFileOrder = [];
}

let changeTimer;
let toastTimer;
let mermaidSequence = 0;
let mermaidQueue = Promise.resolve();
let expandedMermaidDiagram = null;
const mermaidInputTimers = new WeakMap();
const mermaidRenderRequests = new WeakMap();
let windowDragPointer = null;
let windowDragFrame = 0;
let viewportTypographyFrame = 0;
let pendingCodeExit = null;
let recentCompositionCommit = null;
let activeComposition = null;
let hostRequestSequence = 0;
let workspaceKnowledgeRequest = 0;
let markdownNormalizationFrame = 0;
let documentAssetTimer = 0;
let documentAssetRequest = 0;
let contextEntry = null;
let tableResize = null;
const editorHistoryLimit = 100;
const editorHistoryGroupWindow = 800;
let pendingEntryOperation = null;
let draggedFileEntry = null;
let pathSuggestionContext = null;
let pathSuggestionEntries = [];
let pathSuggestionIndex = 0;
let calendarEditor = null;
let calendarInsertRange = null;
let calendarQuickEditor = null;
let calendarDrag = null;
const pendingHostRequests = new Map();
const caretMarker = "\u200b";
const renderCaretMarker = "\ue000";
const doubleEnterWindow = 650;
const builtInThemes = ["yuluo-css", "lapis-cv", "github", "whitey", "newsprint", "pixyll", "gothic", "night"];
const bundledThemeFonts = {
  "yuluo-css": [["Mory LXGW WenKai", "汉字 Aa", "400"]],
  "lapis-cv": [
    ["Mory Source Han Sans CN", "汉字 Aa", "400"],
    ["Mory Source Han Sans CN", "汉字 Aa", "500"],
    ["Mory Source Han Sans CN", "汉字 Aa", "700"],
    ["Mory JetBrains Mono", "Code 0123", "400"],
    ["Mory LapisCV Icon", "\ue60f\ue618\ue635\uecfa", "400"]
  ]
};
const bundledThemeAssets = {
  "yuluo-css": ["LXGWWenKai-Regular.ttf"],
  "lapis-cv": [
    "SourceHanSansCN-Regular.ttf",
    "SourceHanSansCN-Medium.ttf",
    "SourceHanSansCN-Bold.ttf",
    "JetBrainsMono-Regular.ttf",
    "LapisCV-Icon.ttf"
  ]
};
const bundledThemeAssetData = new Map();
const appearanceMedia = window.matchMedia("(prefers-color-scheme: dark)");
const englishText = {
  "文件": "Files", "大纲": "Outline", "工作区": "Workspace", "文档还没有标题": "No headings yet",
  "本地工作区": "Local workspace", "未命名": "Untitled", "未命名.md": "Untitled.md", "已保存": "Saved", "未保存": "Unsaved",
  "查找": "Find", "替换为": "Replace with", "替换": "Replace", "全部替换": "Replace all", "上一个": "Previous", "下一个": "Next", "关闭": "Close",
  "加粗（⌘B）": "Bold (⌘B)", "斜体（⌘I）": "Italic (⌘I)", "删除线": "Strikethrough", "行内代码": "Inline code",
  "引用": "Quote", "无序列表": "Bulleted list", "有序列表": "Numbered list", "任务列表": "Task list", "链接（⌘K）": "Link (⌘K)", "表格": "Table", "插入日历": "Insert calendar", "一键优化排版": "Optimize typography", "分隔线": "Horizontal rule",
  "知识图谱": "Knowledge graph", "源代码模式（⌘/）": "Source mode (⌘/)", "导出文档": "Export document",
  "专注模式": "Focus mode", "打字机模式": "Typewriter mode", "正在读取工作区…": "Reading workspace…", "筛选文稿": "Filter notes", "刷新": "Refresh",
  "当前工作区还没有可显示的文稿": "There are no notes to display in this workspace", "当前文稿": "Current note", "工作区文稿": "Workspace note", "文稿链接关系图": "Note connection graph",
  "正向": "Outgoing", "反向": "Backlink", "滚轮缩放 · 单击查看关系 · 双击打开": "Wheel to zoom · Click for connections · Double-click to open", "图谱缩放比例": "Graph zoom level", "选择文稿": "Select a note", "收起关系": "Hide connections",
  "链接到": "Links to", "被链接": "Linked from", "没有正向链接": "No outgoing links", "没有反向链接": "No backlinks", "反向链接": "Backlinks",
  "当前文稿的反向链接": "Backlinks for the current note", "篇文稿引用当前文稿": "notes link to this note", "没有文稿引用当前文稿": "No notes link to this note",
  "文件结果": "File results", "按文件名搜索": "Search by filename", "偏好设置": "Preferences", "调整工作区、外观与写作体验": "Shape your workspace, appearance, and writing experience", "工作区与存储": "Workspace & storage",
  "文稿在本地目录编辑，远端插件负责同步": "Edit locally; remote plugins handle sync", "新增": "Add", "当前工作区": "Current workspace", "尚未连接宿主": "Desktop host unavailable",
  "选择本地目录": "Choose local folder", "拉取": "Pull", "推送": "Push", "新增工作区": "Add workspace", "配置工作区": "Configure workspace",
  "名称": "Name", "存储插件": "Storage plugin", "删除工作区": "Remove workspace", "保存并启用": "Save & activate", "编辑器": "Editor",
  "界面语言": "Interface language", "切换 Mory 的菜单与操作文字": "Switch Mory menus and controls", "简体中文": "Simplified Chinese",
  "外观": "Appearance", "选择编辑器使用的颜色主题": "Choose the editor color scheme", "跟随系统": "System", "浅色": "Light", "深色": "Dark",
  "文档主题": "Document theme", "独立 CSS 控制正文渲染和导出样式": "CSS controls editor rendering and exports", "使用简历模板": "Use resume template", "简历模板已创建": "Resume template created", "简历模板不可用": "Resume template unavailable", "用户主题": "Custom themes",
  "导入 CSS，或把主题与资源放入主题目录": "Import CSS, or place themes and assets in the theme folder", "导入 CSS": "Import CSS", "主题目录": "Theme folder",
  "更改目录": "Change folder", "打开目录": "Open folder", "主题目录已更新": "Theme folder updated",
  "编辑器宽度": "Editor width", "控制正文最大行宽": "Control maximum text width", "窄": "Narrow", "标准": "Standard", "宽": "Wide",
  "显示状态栏": "Show status bar", "展示行数、字数与模式开关": "Show counts and mode controls", "显示文件详情": "Show file details", "在文件树中显示大小与更新时间": "Show size and update time in the file tree", "拼写检查": "Spell check", "使用系统拼写检查能力": "Use the system spell checker",
  "格式": "Format", "导出主题": "Export theme", "使用当前主题": "Use current theme", "纸张": "Paper", "图片宽度": "Image width", "保留主题背景": "Keep theme background", "思维导图（HTML）": "Mind map (HTML)", "PowerPoint（Slidev）": "PowerPoint (Slidev)",
  "PDF 与图片包含当前主题的纸张颜色": "Include theme paper color in PDF and images", "HTML、PDF 不需要 Pandoc": "HTML and PDF do not require Pandoc", "PPTX 由官方 Slidev 生成，并保留演讲者备注": "PPTX is generated by official Slidev and retains presenter notes", "PPTX 导出需要桌面版与 Slidev": "PPTX export requires the desktop app and Slidev", "选择位置并导出": "Choose location and export",
  "开始写作…": "Start writing…", "新建文档（⌘N）": "New document (⌘N)", "新建目录": "New folder", "目录名称或路径": "Folder name or path", "创建目录": "Create folder", "目录已创建": "Folder created", "创建目录失败": "Failed to create folder", "取消": "Cancel", "打开文稿": "Open document", "在此新建文稿": "New document here", "在此新建目录": "New folder here", "在文件管理器中显示": "Show in file manager", "复制绝对路径": "Copy Absolute Path", "复制相对路径": "Copy Relative Path", "绝对路径已复制": "Absolute path copied", "相对路径已复制": "Relative path copied", "重命名…": "Rename…", "重命名条目": "Rename entry", "新名称": "New name", "重命名完成": "Renamed", "复制到…": "Copy to…", "移动到…": "Move to…", "导出…": "Export…", "删除目录": "Delete folder", "选择目标目录": "Choose destination", "工作区根目录": "Workspace root", "复制条目": "Copy entry", "移动条目": "Move entry", "复制完成": "Copied", "移动完成": "Moved", "新文稿已创建": "Document created", "操作失败": "Operation failed", "图片预览": "Image preview", "图片加载失败": "Failed to load image", "展开图片": "Expand images", "收起图片": "Collapse images", "展开目录": "Expand folder", "收起目录": "Collapse folder", "切换或配置工作区": "Switch or configure workspace", "显示／隐藏侧边栏": "Show/hide sidebar", "添加行": "Add row", "删除行": "Delete row", "添加列": "Add column", "删除列": "Delete column", "调整列宽": "Resize column",
  "内置主题字体加载失败，请重新启动 Mory 后再试。": "Bundled theme fonts failed to load. Restart Mory and try again.",
  "已切换文档": "Document switched", "关闭文档": "Close document", "删除文档": "Delete document", "移除草稿": "Remove draft", "文档已关闭": "Document closed", "文档已移到废纸篓": "Document moved to Trash", "目录已移到废纸篓": "Folder moved to Trash", "删除文档失败": "Failed to delete entry", "草稿已移除": "Draft removed", "当前草稿": "Current draft",
  "磁盘文件已删除": "deleted from disk", "文件已从磁盘删除，未保存内容已保留为草稿": "The file was deleted from disk; unsaved content was kept as a draft", "文稿": "Document", "图片": "Image",
  "本地": "Local", "工作目录": "Working folder", "使用“选择本地目录”填写": "Use “Choose local folder”", "仓库": "Repository", "分支": "Branch",
  "API 地址": "API endpoint", "仓库内目录": "Repository path", "S3 兼容服务地址": "S3-compatible endpoint", "服务器": "Server", "端口": "Port",
  "用户名": "Username", "密码": "Password", "私钥或私钥路径": "Private key or path", "默认 ~/.ssh/known_hosts": "Default: ~/.ssh/known_hosts",
  "远端目录": "Remote path", "区域": "Region", "路径前缀": "Path prefix", "已配置；留空则保持不变": "Configured; leave blank to keep it",
  "排版已优化": "Typography optimized", "当前文稿无需优化": "This note already follows the typography rules",
  "编辑日历": "Edit calendar", "日历预览": "Calendar preview", "日历编辑模式": "Calendar editing mode", "上个月": "Previous month", "下个月": "Next month", "今天": "Today",
  "日期标记": "Date mark", "日期范围": "Date range", "日期事项": "Date items", "标题": "Title", "颜色": "Color", "重要日期": "Important date", "前端开发": "Frontend development",
  "移除标记": "Remove mark", "保存标记": "Save mark", "重新选择": "Select again", "添加范围": "Add range", "添加事项": "Add item", "添加": "Add", "删除日历": "Delete calendar", "保存日历": "Save calendar",
  "Mermaid 源码": "Mermaid source", "Mermaid 图表": "Mermaid diagram", "Mermaid 无法渲染": "Mermaid could not render", "Mermaid 运行时未加载": "Mermaid runtime is unavailable",
  "收起源码": "Collapse source", "展开源码": "Expand source", "放大编辑框": "Expand editor", "退出放大": "Exit expanded view"
};
const staticLocaleNodes = new WeakMap();
const staticLocaleAttributes = new WeakMap();

function locale() { return state.locale === "en" ? "en" : "zh-CN"; }
function localized(chinese) { return locale() === "en" ? (englishText[chinese] || chinese) : chinese; }

function applyAppearanceTheme(theme, { persist = true } = {}) {
  const next = ["system", "light", "dark"].includes(theme) ? theme : "system";
  const previousAppearance = document.documentElement.dataset.appearance;
  document.documentElement.dataset.theme = next === "system" ? "" : next;
  document.documentElement.dataset.appearance = next === "system" ? (appearanceMedia.matches ? "dark" : "light") : next;
  $("#theme-select").value = next;
  if (persist) localStorage.setItem("mory.theme", next);
  if (previousAppearance && previousAppearance !== document.documentElement.dataset.appearance) {
    void renderMermaidDiagrams(write, state.documentTheme);
  }
}

const refreshSystemAppearance = () => {
  if ((localStorage.getItem("mory.theme") || "system") === "system") applyAppearanceTheme("system", { persist: false });
};
if (typeof appearanceMedia.addEventListener === "function") appearanceMedia.addEventListener("change", refreshSystemAppearance);
else appearanceMedia.addListener?.(refreshSystemAppearance);

function applyLocale(next = state.locale) {
  state.locale = next === "en" ? "en" : "zh-CN";
  document.documentElement.lang = state.locale;
  document.documentElement.style.setProperty("--empty-editor-label", state.locale === "en" ? '"Start writing…"' : '"开始写作…"');
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    if (!staticLocaleNodes.has(node)) staticLocaleNodes.set(node, node.nodeValue);
    const original = staticLocaleNodes.get(node);
    const trimmed = original.trim();
    if (englishText[trimmed]) node.nodeValue = original.replace(trimmed, localized(trimmed));
  }
  document.querySelectorAll("[title], [aria-label], [placeholder], [data-tooltip]").forEach(element => {
    if (!staticLocaleAttributes.has(element)) {
      staticLocaleAttributes.set(element, Object.fromEntries(["title", "aria-label", "placeholder", "data-tooltip"].map(name => [name, element.getAttribute(name)])));
    }
    const originals = staticLocaleAttributes.get(element);
    for (const [name, value] of Object.entries(originals)) if (value) element.setAttribute(name, localized(value));
  });
  $("#language-select").value = state.locale;
  localStorage.setItem("mory.locale", state.locale);
  renderFiles();
  updateDerivedState();
  $("#save-state").textContent = localized(state.dirty ? "未保存" : "已保存");
  const sourceLabel = state.sourceMode ? (state.locale === "en" ? "Preview mode (⌘/)" : "预览模式（⌘/）") : localized("源代码模式（⌘/）");
  $("#source-toggle").dataset.tooltip = sourceLabel;
  $("#source-toggle").setAttribute("aria-label", sourceLabel);
  renderWorkspaceSettings();
  syncThemeOptions();
  updateThemeFontWarning();
  updateDocumentBacklinks();
  enhanceCalendars(write);
  updateMermaidWorkbenchLocale(write);
  if ($("#knowledge-graph").classList.contains("is-open")) updateGraphLabels();
  bridge({ type: "localeChanged", locale: state.locale });
}

function bridge(payload) {
  if (window.webkit?.messageHandlers?.mory) {
    window.webkit.messageHandlers.mory.postMessage(payload);
  } else if (nativeWailsHost()?.Send) {
    void nativeWailsHost().Send(payload).catch(error => {
      console.error("Windows 宿主消息失败：", error);
      toast(state.locale === "en" ? `Desktop operation failed: ${error.message}` : `桌面操作失败：${error.message}`, 5000);
    });
  } else {
    window.moryNative?.send(payload);
  }
}

function hostRequest(method, args = {}) {
  if (window.moryNative?.request) return window.moryNative.request(method, args);
  if (nativeWailsHost()?.Request) return nativeWailsHost().Request(method, args);
  if (!window.webkit?.messageHandlers?.mory) return Promise.reject(new Error("当前环境没有桌面宿主。"));
  const requestId = `host-${++hostRequestSequence}`;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pendingHostRequests.delete(requestId);
      reject(new Error("宿主请求超时。"));
    }, 10 * 60 * 1000);
    pendingHostRequests.set(requestId, { resolve, reject, timer });
    bridge({ type: "hostRequest", requestId, method, args });
  });
}

function resolveHostRequest(payload = {}) {
  const pending = pendingHostRequests.get(payload.requestId);
  if (!pending) return;
  clearTimeout(pending.timer);
  pendingHostRequests.delete(payload.requestId);
  if (payload.error) pending.reject(new Error(String(payload.error)));
  else pending.resolve(payload.result);
}

function toast(message, duration = 1500) {
  const element = $("#toast");
  element.textContent = message;
  element.classList.add("is-visible");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => element.classList.remove("is-visible"), duration);
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

function firstLevelHeading(markdown) {
  let fence = "";
  for (const line of String(markdown || "").replace(/\r\n?/g, "\n").split("\n")) {
    const marker = line.match(/^\s*(```|~~~)/)?.[1] || "";
    if (marker) { fence = fence ? (fence === marker ? "" : fence) : marker; continue; }
    if (fence) continue;
    const heading = line.match(/^#\s+(.+?)\s*#*\s*$/)?.[1]?.replace(/[*_`~]/g, "").trim();
    if (heading) return heading;
  }
  return "";
}

function documentDisplayNameTracksHeading(document) {
  // Use the first level-one heading in the tree while preserving the on-disk name for host operations.
  return Boolean(document);
}

function documentDisplayName(document) {
  if (!document) return localized("未命名.md");
  const heading = documentDisplayNameTracksHeading(document) ? firstLevelHeading(document.markdown) : "";
  return heading ? (heading.toLocaleLowerCase().endsWith(".md") ? heading : `${heading}.md`) : (document.name || localized("未命名.md"));
}

function documentHostName(document) {
  return document?.path ? (document.name || localized("未命名.md")) : documentDisplayName(document);
}

function enhanceRawHTML(root, { interactive = true } = {}) {
  const purifier = globalThis.DOMPurify;
  root.querySelectorAll(".mory-raw-html-placeholder").forEach(placeholder => {
    const source = placeholder.dataset.rawHtml || "";
    if (!purifier?.sanitize) {
      placeholder.textContent = source;
      return;
    }
    const fragment = purifier.sanitize(source, {
      RETURN_DOM_FRAGMENT: true,
      USE_PROFILES: { html: true },
      FORBID_TAGS: ["script", "style", "iframe", "object", "embed", "base", "meta", "link", "form", "input", "button", "textarea", "select", "option"],
      FORBID_ATTR: ["srcdoc", "formaction"]
    });
    fragment.querySelectorAll("a[target='_blank']").forEach(link => link.setAttribute("rel", "noopener noreferrer"));
    if (!interactive) {
      placeholder.replaceWith(fragment);
      return;
    }
    placeholder.classList.remove("mory-raw-html-placeholder");
    placeholder.classList.add("mory-raw-html");
    placeholder.replaceChildren(fragment);
  });
}

function renderDocument(document, announce = false) {
  closeExpandedMermaidWorkbench();
  closePathSuggestions();
  closeCalendarQuickEditor();
  clearTimeout(changeTimer);
  clearTimeout(documentAssetTimer);
  documentAssetRequest += 1;
  state.markdown = document.markdown;
  state.dirty = document.dirty;
  state.titleTouched = false;
  sourceEditor.value = state.markdown;
  write.innerHTML = markdownToHTML(state.markdown) || "<p><br></p>";
  enhanceRawHTML(write);
  enhanceTables(write);
  enhanceCalendars(write);
  applyDocumentAssets(write, document);
  highlightCodeBlocks(write);
  $("#save-state").textContent = localized(state.dirty ? "未保存" : "已保存");
  $("#save-state").classList.toggle("is-visible", state.dirty);
  updateDerivedState();
  renderFiles();
  void renderMermaidDiagrams(write, state.documentTheme);
  updateDocumentBacklinks();
  if (announce) toast(localized("已切换文档"));
}

function notifyDocumentSelected(document) {
  bridge({
    type: "documentSelected",
    documentId: document.id,
    name: documentHostName(document),
    path: document.path || "",
    markdown: document.markdown,
    dirty: document.dirty
  });
}

function activateDocument(documentId, { announce = false, notifyHost = true, focusEditor = true } = {}) {
  const document = state.documents.find(item => item.id === documentId);
  if (!document) return;
  state.activeDocumentId = document.id;
  const workspaceFile = state.files.find(file => file.path === document.path);
  // Documents and directories share one selection source so only one tree entry is highlighted.
  state.selectedWorkspaceEntry = workspaceFile
    ? { kind: "file", path: workspaceFile.path, name: workspaceFile.name }
    : null;
  if (notifyHost) notifyDocumentSelected(document);
  renderDocument(document, announce);
  if (focusEditor) requestAnimationFrame(() => (state.sourceMode ? sourceEditor : write).focus());
}

function createUntitledDocument(markdown = "", { announce = true, notifyHost = true, workspacePlaceholder = false } = {}) {
  state.untitledSequence += 1;
  const document = {
    id: nextDocumentId(),
    name: untitledName(state.untitledSequence),
    path: "",
    markdown: String(markdown ?? ""),
    dirty: false,
    createdAt: Date.now(),
    assets: {},
    workspacePlaceholder
  };
  state.documents.push(document);
  activateDocument(document.id, { announce, notifyHost });
  return document;
}

function createResumeFromTemplate() {
  const markdown = globalThis.__MORY_DOCUMENT_TEMPLATES__?.["lapis-cv-cn"];
  if (typeof markdown !== "string" || !markdown.trim()) {
    toast(localized("简历模板不可用"), 3200);
    return null;
  }
  const document = createUntitledDocument(markdown, { announce: false });
  document.dirty = true;
  state.dirty = true;
  setDocumentTheme("lapis-cv", { announceFontWarning: true });
  renderDocument(document);
  notifyDocumentSelected(document);
  localStorage.setItem("mory.draft", markdown);
  togglePreferences(false);
  requestAnimationFrame(() => write.focus());
  toast(localized("简历模板已创建"));
  return document;
}

function loadMarkdown(markdown, announce = false) {
  const document = activeDocument() || createUntitledDocument("", { announce: false, notifyHost: false });
  document.markdown = String(markdown ?? "");
  document.dirty = false;
  delete document.editorHistory;
  renderDocument(document, announce);
}

function openDocument(payload = {}) {
  const path = typeof payload.path === "string" ? payload.path : "";
  const markdown = String(payload.markdown ?? "");
  const name = String(payload.name || path.split(/[\\/]/).pop() || "未命名.md");
  const workspaceFile = state.files.find(file => file.path === path);
  const createdAt = Number(payload.createdAt ?? workspaceFile?.createdAt ?? Date.now());
  let document = path ? state.documents.find(item => item.path === path) : null;
  if (document) {
    document.name = name;
    document.markdown = markdown;
    document.dirty = false;
    document.assets = payload.assets && typeof payload.assets === "object" ? payload.assets : document.assets || {};
    delete document.editorHistory;
  } else {
    document = { id: nextDocumentId(), name, path, markdown, dirty: false, createdAt, assets: payload.assets && typeof payload.assets === "object" ? payload.assets : {} };
    state.documents.push(document);
  }
  activateDocument(document.id, { announce: true, notifyHost: true });
}

function isLocalDocumentImage(source) {
  return Boolean(source) && !/^(?:data:|https?:|file:|\/\/)/i.test(source);
}

function scheduleDocumentAssetRefresh(document) {
  if (!document?.path || (!window.moryNative && !nativeMacHost && !nativeWailsHost())) return;
  clearTimeout(documentAssetTimer);
  const request = ++documentAssetRequest;
  documentAssetTimer = setTimeout(async () => {
    try {
      const markdown = document === activeDocument() ? editorToMarkdown(write) : document.markdown;
      const assets = await hostRequest("documentAssets", { markdown });
      if (request !== documentAssetRequest || document !== activeDocument() || !assets || typeof assets !== "object") return;
      document.assets = { ...(document.assets || {}), ...assets };
      applyDocumentAssets(write, document, { refreshMissing: false });
    } catch {
      // Keep unresolved URLs intact; a later edit will retry after the path or file becomes available.
    }
  }, 90);
}

function applyDocumentAssets(root, document = activeDocument(), { refreshMissing = true } = {}) {
  const assets = document?.assets || {};
  let missingLocalAsset = false;
  root.querySelectorAll("img[src]").forEach(image => {
    const raw = image.dataset.markdownSrc || image.getAttribute("src") || "";
    let decoded = raw;
    try { decoded = decodeURI(raw); } catch { /* Keep the original path when it cannot be decoded. */ }
    const candidates = [...new Set([raw, decoded].flatMap(source => {
      const normalized = source.replaceAll("\\", "/");
      return [normalized, normalized.replace(/^\.\//, "")];
    }))];
    const asset = candidates.map(source => assets[source]).find(Boolean);
    if (asset) {
      if (!image.dataset.markdownSrc) image.dataset.markdownSrc = raw;
      image.src = asset;
    }
    else if (candidates.some(isLocalDocumentImage)) missingLocalAsset = true;
  });
  if (refreshMissing && missingLocalAsset) scheduleDocumentAssetRefresh(document);
}

function syncFromWrite() {
  state.markdown = editorToMarkdown(write);
  sourceEditor.value = state.markdown;
  const document = activeDocument();
  if (document) document.markdown = state.markdown;
  // Outline state belongs to the live editor and must not wait for draft persistence.
  updateOutline();
  markChanged();
}

function editorCaretOffset(root) {
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount || !root.contains(selection.anchorNode)) return null;
  const range = document.createRange();
  range.selectNodeContents(root);
  try { range.setEnd(selection.anchorNode, selection.anchorOffset); }
  catch { return null; }
  return range.toString().length;
}

function placeEditorCaret(root, offset) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let remaining = Math.max(0, Number(offset) || 0);
  let last = null;
  while (walker.nextNode()) {
    const node = walker.currentNode;
    last = node;
    const length = node.nodeValue?.length || 0;
    if (remaining <= length) {
      const range = document.createRange();
      range.setStart(node, remaining);
      range.collapse(true);
      const selection = window.getSelection();
      selection?.removeAllRanges();
      selection?.addRange(range);
      return;
    }
    remaining -= length;
  }
  const range = document.createRange();
  range.selectNodeContents(last || root);
  range.collapse(false);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
}

function editorHistory(document = activeDocument()) {
  if (!document) return null;
  document.editorHistory ||= { undo: [], redo: [], group: "", time: 0 };
  return document.editorHistory;
}

function currentEditorSnapshot() {
  const root = state.sourceMode ? sourceEditor : write;
  return {
    markdown: state.sourceMode ? sourceEditor.value : editorToMarkdown(write),
    caret: state.sourceMode ? sourceEditor.selectionStart : editorCaretOffset(write)
  };
}

function beginEditorHistory(group = "structural", { force = false } = {}) {
  const history = editorHistory();
  if (!history) return;
  const now = performance.now();
  const coalesced = !force && group && history.group === group && now - history.time <= editorHistoryGroupWindow;
  const snapshot = currentEditorSnapshot();
  if (!coalesced && history.undo.at(-1)?.markdown !== snapshot.markdown) {
    history.undo.push(snapshot);
    if (history.undo.length > editorHistoryLimit) history.undo.shift();
  }
  history.redo = [];
  history.group = group;
  history.time = now;
}

function restoreEditorHistory(snapshot) {
  const document = activeDocument();
  if (!document || !snapshot) return false;
  document.markdown = snapshot.markdown;
  document.dirty = true;
  state.markdown = snapshot.markdown;
  state.dirty = true;
  renderDocument(document);
  markChanged();
  const root = state.sourceMode ? sourceEditor : write;
  requestAnimationFrame(() => {
    if (activeDocument() !== document || state.markdown !== snapshot.markdown) return;
    root.focus({ preventScroll: true });
    if (state.sourceMode) sourceEditor.setSelectionRange(snapshot.caret ?? sourceEditor.value.length, snapshot.caret ?? sourceEditor.value.length);
    else placeEditorCaret(write, snapshot.caret ?? write.textContent.length);
    updateFocusLine();
  });
  return true;
}

function undoEditor() {
  const history = editorHistory();
  const snapshot = history?.undo.pop();
  if (!snapshot) return document.execCommand("undo");
  history.redo.push(currentEditorSnapshot());
  history.group = "";
  return restoreEditorHistory(snapshot);
}

function redoEditor() {
  const history = editorHistory();
  const snapshot = history?.redo.pop();
  if (!snapshot) return document.execCommand("redo");
  history.undo.push(currentEditorSnapshot());
  history.group = "";
  return restoreEditorHistory(snapshot);
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

function workspaceRelativePath(targetName) {
  const active = activeDocument();
  const workspaceFile = state.files.find(file => file.path === active?.path);
  const currentName = String(workspaceFile?.name || active?.name || "").replaceAll("\\", "/");
  const from = currentName.split("/").filter(Boolean).slice(0, -1);
  const target = String(targetName || "").replaceAll("\\", "/").split("/").filter(Boolean);
  let common = 0;
  while (common < from.length && common < target.length && from[common] === target[common]) common += 1;
  const relative = [...Array.from({ length: from.length - common }, () => ".."), ...target.slice(common)].join("/") || ".";
  const safe = relative.replaceAll(" ", "%20").replaceAll("(", "%28").replaceAll(")", "%29");
  return safe.startsWith("../") ? safe : `./${safe}`;
}

function workspaceFileForLink(reference) {
  let target = String(reference || "").split(/[?#]/, 1)[0];
  try { target = decodeURI(target); } catch { /* Keep malformed escapes visible and unresolved. */ }
  if (!target || /^(?:[a-z][a-z\d+.-]*:|\/)/i.test(target)) return null;
  const active = activeDocument();
  const current = state.files.find(file => file.path === active?.path);
  const base = String(current?.name || active?.name || "").replaceAll("\\", "/").split("/").slice(0, -1);
  const segments = [...base];
  for (const segment of target.replaceAll("\\", "/").split("/")) {
    if (!segment || segment === ".") continue;
    if (segment === "..") {
      if (!segments.length) return null;
      segments.pop();
    } else {
      segments.push(segment);
    }
  }
  const normalized = segments.join("/");
  return state.files.find(file => String(file.name || "").replaceAll("\\", "/") === normalized) || null;
}

function workspacePathCandidates() {
  const entries = [];
  const seen = new Set();
  const add = entry => {
    const key = `${entry.kind}:${entry.path}`;
    if (!entry.path || seen.has(key)) return;
    seen.add(key);
    entries.push(entry);
  };
  for (const file of state.files) {
    add({ kind: "document", name: String(file.name || "").split(/[\\/]/).pop() || localized("文稿"), path: workspaceRelativePath(file.name) });
    const parent = String(file.name || "").replaceAll("\\", "/").split("/").slice(0, -1).join("/");
    for (const image of file.images || []) {
      const target = [parent, image.relative || image.name].filter(Boolean).join("/");
      add({ kind: "image", name: image.name || String(target).split("/").pop() || localized("图片"), path: workspaceRelativePath(target) });
    }
  }
  return entries;
}

function pathSuggestionMatch() {
  if (state.sourceMode || document.activeElement !== write) return null;
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount) return null;
  const block = currentWriteBlock();
  if (!block?.matches("p, div")) return null;
  const offset = textOffsetWithin(block, selection.anchorNode, selection.anchorOffset);
  const source = (block.textContent || "").replaceAll(caretMarker, "");
  const match = source.slice(0, offset).match(/(!?)\[([^\]\n]*)\]\(((?:\.{1,2}\/)[^)\s]*)$/);
  if (!match) return null;
  return { block, source, start: match.index, end: offset, imageSyntax: match[1] === "!", label: match[2], query: match[3] };
}

function closePathSuggestions() {
  const popup = $("#path-suggestions");
  if (!popup) return;
  popup.classList.remove("is-open");
  popup.setAttribute("aria-hidden", "true");
  popup.innerHTML = "";
  pathSuggestionContext = null;
  pathSuggestionEntries = [];
  pathSuggestionIndex = 0;
}

function selectPathSuggestion(index) {
  if (!pathSuggestionEntries.length) return;
  pathSuggestionIndex = (index + pathSuggestionEntries.length) % pathSuggestionEntries.length;
  const buttons = $$("#path-suggestions button");
  buttons.forEach((button, buttonIndex) => button.classList.toggle("is-selected", buttonIndex === pathSuggestionIndex));
  buttons[pathSuggestionIndex]?.scrollIntoView({ block: "nearest" });
}

function acceptPathSuggestion(index = pathSuggestionIndex) {
  const context = pathSuggestionContext;
  const entry = pathSuggestionEntries[index];
  if (!context?.block?.isConnected || !entry) return false;
  const defaultLabel = entry.name.replace(/\.(?:md|markdown)$/i, "");
  const imageMarker = context.imageSyntax && entry.kind === "image" ? "!" : "";
  const replacement = `${imageMarker}[${context.label || defaultLabel}](${entry.path})`;
  const nextSource = context.source.slice(0, context.start) + replacement + context.source.slice(context.end);
  closePathSuggestions();
  context.block.textContent = nextSource;
  placeTextCaret(context.block, context.start + replacement.length);
  renderMarkdownBlockAtCaret();
  syncFromWrite();
  highlightCodeBlocks(write);
  updateFocusLine();
  return true;
}

function updatePathSuggestions() {
  const context = pathSuggestionMatch();
  if (!context) {
    closePathSuggestions();
    return false;
  }
  const query = context.query.toLocaleLowerCase();
  const candidates = workspacePathCandidates().filter(entry => entry.path.toLocaleLowerCase().startsWith(query)).slice(0, 24);
  if (!candidates.length) {
    closePathSuggestions();
    return false;
  }
  pathSuggestionContext = context;
  pathSuggestionEntries = candidates;
  pathSuggestionIndex = 0;
  const popup = $("#path-suggestions");
  popup.innerHTML = "";
  candidates.forEach((entry, index) => {
    const button = document.createElement("button");
    button.type = "button";
    button.dataset.path = entry.path;
    button.setAttribute("role", "option");
    button.innerHTML = `<span class="path-suggestion-name">${escapeHTML(entry.path)}</span><span class="path-suggestion-kind">${localized(entry.kind === "image" ? "图片" : "文稿")}</span>`;
    button.addEventListener("mousedown", event => event.preventDefault());
    button.addEventListener("click", event => {
      event.preventDefault();
      acceptPathSuggestion(index);
    });
    popup.append(button);
  });
  popup.classList.add("is-open");
  popup.setAttribute("aria-hidden", "false");
  selectPathSuggestion(0);
  const selection = window.getSelection();
  const caretRect = selection?.rangeCount ? selection.getRangeAt(0).getBoundingClientRect() : context.block.getBoundingClientRect();
  const blockRect = context.block.getBoundingClientRect();
  const left = caretRect?.left || blockRect.left;
  const top = (caretRect?.bottom || blockRect.bottom) + 6;
  popup.style.left = `${Math.max(8, Math.min(innerWidth - popup.offsetWidth - 8, left))}px`;
  popup.style.top = `${Math.max(8, Math.min(innerHeight - popup.offsetHeight - 8, top))}px`;
  return true;
}

function handlePathSuggestionKey(event) {
  if (!$("#path-suggestions").classList.contains("is-open")) return false;
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    event.preventDefault();
    selectPathSuggestion(pathSuggestionIndex + (event.key === "ArrowDown" ? 1 : -1));
    return true;
  }
  if (event.key === "Enter") {
    event.preventDefault();
    return acceptPathSuggestion();
  }
  if (event.key === "Escape") {
    event.preventDefault();
    closePathSuggestions();
    return true;
  }
  return false;
}

function beginComposition(event) {
  pendingCodeExit = null;
  recentCompositionCommit = null;
  const block = writeBlockForNode(event.target) || currentWriteBlock();
  const selection = window.getSelection();
  const blockText = (block?.textContent || "").replaceAll(caretMarker, "");
  const isHeadingInput = block?.matches("h1, h2, h3, h4, h5, h6")
    || (block?.matches("p, div") && /^(#{1,6})\s/.test(blockText));
  if (!isHeadingInput || !selection?.isCollapsed || !selection.rangeCount) {
    activeComposition = null;
    return;
  }
  const offset = block.contains(selection.anchorNode)
    ? textOffsetWithin(block, selection.anchorNode, selection.anchorOffset)
    : (block.textContent || "").length;
  const text = blockText;
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
    // WebKit can remove an empty heading before committing IME text; retain it until composition completes.
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

function selectionAtStart(element) {
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount || !element.contains(selection.anchorNode)) return false;
  const head = document.createRange();
  head.selectNodeContents(element);
  try {
    head.setEnd(selection.anchorNode, selection.anchorOffset);
  } catch {
    return false;
  }
  return head.toString().replaceAll(caretMarker, "") === "";
}

function selectedTableCell(table) {
  let node = window.getSelection()?.anchorNode;
  if (node?.nodeType === Node.TEXT_NODE) node = node.parentElement;
  const cell = node instanceof Element ? node.closest("th, td") : null;
  return cell && table.contains(cell) ? cell : null;
}

function focusTableCell(cell) {
  if (!cell) return;
  if (!cell.childNodes.length) cell.append(document.createElement("br"));
  const selection = window.getSelection();
  const caret = document.createRange();
  const handle = cell.querySelector(":scope > .table-resize-handle");
  if (handle) caret.setStart(cell, [...cell.childNodes].indexOf(handle));
  else {
    caret.selectNodeContents(cell);
    caret.collapse(false);
  }
  caret.collapse(true);
  selection?.removeAllRanges();
  selection?.addRange(caret);
  write.focus({ preventScroll: true });
}

function addTableRow(table) {
  beginEditorHistory("table-row", { force: true });
  const selected = selectedTableCell(table);
  const columns = Math.max(1, table.rows[0]?.cells.length || 1);
  const body = table.tBodies[0] || table.createTBody();
  const row = document.createElement("tr");
  for (let index = 0; index < columns; index += 1) {
    const cell = document.createElement("td");
    cell.append(document.createElement("br"));
    row.append(cell);
  }
  const selectedRow = selected?.parentElement;
  if (selectedRow?.parentElement === body) body.insertBefore(row, selectedRow.nextSibling);
  else body.append(row);
  addTableResizeHandles(table);
  focusTableCell(row.cells[0]);
  syncFromWrite();
}

function addTableColumn(table) {
  beginEditorHistory("table-column", { force: true });
  const selected = selectedTableCell(table);
  const width = Math.max(1, table.rows[0]?.cells.length || 1);
  const column = selected ? selected.cellIndex + 1 : width;
  let focusTarget = null;
  for (const row of table.rows) {
    const cell = document.createElement(row.parentElement?.tagName === "THEAD" ? "th" : "td");
    cell.append(document.createElement("br"));
    row.insertBefore(cell, row.cells[column] || null);
    if (!focusTarget && cell.tagName === "TD") focusTarget = cell;
  }
  addTableResizeHandles(table);
  focusTableCell(focusTarget || table.rows[0]?.cells[column]);
  syncFromWrite();
}

function deleteTableRow(table) {
  beginEditorHistory("table-row", { force: true });
  const selected = selectedTableCell(table);
  const body = table.tBodies[0];
  const row = selected?.tagName === "TD" ? selected.parentElement : body?.rows[0];
  if (!body || !row) return;
  const rowIndex = [...body.rows].indexOf(row);
  let focusTarget;
  if (body.rows.length === 1) {
    for (const cell of row.cells) cell.replaceChildren(document.createElement("br"));
    focusTarget = row.cells[0];
  } else {
    const nextRow = body.rows[Math.min(rowIndex + 1, body.rows.length - 1)]
      || body.rows[Math.max(0, rowIndex - 1)];
    const column = Math.min(selected?.cellIndex || 0, Math.max(0, nextRow.cells.length - 1));
    row.remove();
    focusTarget = nextRow.cells[column];
  }
  addTableResizeHandles(table);
  focusTableCell(focusTarget);
  syncFromWrite();
}

function deleteTableColumn(table) {
  beginEditorHistory("table-column", { force: true });
  const selected = selectedTableCell(table);
  const width = Math.max(1, table.rows[0]?.cells.length || 1);
  const column = Math.min(selected?.cellIndex ?? width - 1, width - 1);
  let focusTarget;
  if (width === 1) {
    for (const row of table.rows) row.cells[0]?.replaceChildren(document.createElement("br"));
    focusTarget = table.tBodies[0]?.rows[0]?.cells[0] || table.rows[0]?.cells[0];
  } else {
    for (const row of table.rows) row.cells[column]?.remove();
    const targetColumn = Math.min(column, width - 2);
    focusTarget = table.tBodies[0]?.rows[0]?.cells[targetColumn] || table.rows[0]?.cells[targetColumn];
  }
  addTableResizeHandles(table);
  focusTableCell(focusTarget);
  syncFromWrite();
}

function tableColumnWidths(table) {
  const cells = [...(table.rows[0]?.cells || [])];
  return cells.map(cell => cell.getBoundingClientRect().width);
}

function applyTableColumnWidths(table, widths) {
  let colgroup = table.querySelector(":scope > colgroup");
  if (!colgroup) {
    colgroup = document.createElement("colgroup");
    table.prepend(colgroup);
  }
  while (colgroup.children.length < widths.length) colgroup.append(document.createElement("col"));
  while (colgroup.children.length > widths.length) colgroup.lastElementChild.remove();
  const total = widths.reduce((sum, width) => sum + width, 0) || 1;
  [...colgroup.children].forEach((column, index) => {
    column.style.width = `${widths[index] / total * 100}%`;
  });
  table.style.tableLayout = "fixed";
}

function resizeTableColumn(table, column, delta, initialWidths = tableColumnWidths(table)) {
  if (column < 0 || column + 1 >= initialWidths.length) return;
  const minimum = 48;
  const available = initialWidths[column] + initialWidths[column + 1];
  const left = Math.max(minimum, Math.min(available - minimum, initialWidths[column] + delta));
  const widths = [...initialWidths];
  widths[column] = left;
  widths[column + 1] = available - left;
  applyTableColumnWidths(table, widths);
}

function beginTableResize(event, table, column) {
  if (event.button !== 0) return;
  event.preventDefault();
  event.stopPropagation();
  const handle = event.currentTarget;
  tableResize = { table, column, pointerId: event.pointerId, originX: event.clientX, widths: tableColumnWidths(table), handle };
  try { handle.setPointerCapture?.(event.pointerId); } catch { /* Pointer capture can be unavailable in synthetic or older webviews. */ }
  document.body.classList.add("is-resizing-table");
}

function addTableResizeHandles(table) {
  const colgroup = table.querySelector(":scope > colgroup");
  if (colgroup && colgroup.children.length !== (table.rows[0]?.cells.length || 0)) applyTableColumnWidths(table, tableColumnWidths(table));
  for (const row of table.rows) {
    [...row.cells].forEach((cell, column) => {
      cell.querySelector(":scope > .table-resize-handle")?.remove();
      if (column >= row.cells.length - 1) return;
      const handle = document.createElement("span");
      handle.className = "table-resize-handle";
      handle.contentEditable = "false";
      handle.tabIndex = 0;
      handle.setAttribute("role", "separator");
      handle.setAttribute("aria-orientation", "vertical");
      handle.setAttribute("aria-label", localized("调整列宽"));
      handle.addEventListener("pointerdown", event => beginTableResize(event, table, column));
      handle.addEventListener("keydown", event => {
        if (!['ArrowLeft', 'ArrowRight'].includes(event.key)) return;
        event.preventDefault();
        resizeTableColumn(table, column, event.key === "ArrowLeft" ? -12 : 12);
      });
      cell.append(handle);
    });
  }
}

function enhanceTables(root = write) {
  root.querySelectorAll(":scope > .table-tools").forEach(tools => {
    if (!tools.previousElementSibling?.matches("table")) tools.remove();
  });
  root.querySelectorAll(":scope > table").forEach(table => {
    if (!table.tBodies[0]?.rows.length) {
      const body = table.tBodies[0] || table.createTBody();
      const row = body.insertRow();
      const columns = Math.max(1, table.tHead?.rows[0]?.cells.length || 1);
      for (let index = 0; index < columns; index += 1) row.insertCell().append(document.createElement("br"));
    }
    addTableResizeHandles(table);
    if (table.nextElementSibling?.matches(".table-tools")) return;
    const tools = document.createElement("div");
    tools.className = "table-tools";
    tools.contentEditable = "false";
    const actions = [
      ["add-row", "添加行", addTableRow],
      ["delete-row", "删除行", deleteTableRow],
      ["add-column", "添加列", addTableColumn],
      ["delete-column", "删除列", deleteTableColumn]
    ];
    actions.forEach(([action, label, operation]) => {
      const button = document.createElement("button");
      button.type = "button";
      button.dataset.tableAction = action;
      button.classList.toggle("is-danger", action.startsWith("delete"));
      button.textContent = `${action.startsWith("add") ? "＋" : "−"} ${localized(label)}`;
      button.setAttribute("aria-label", localized(label));
      button.addEventListener("mousedown", event => event.preventDefault());
      button.addEventListener("click", event => {
        event.preventDefault();
        event.stopPropagation();
        operation(table);
      });
      tools.append(button);
    });
    table.after(tools);
  });
}

function calendarWeekdayLabels() {
  return locale() === "en"
    ? ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
    : ["一", "二", "三", "四", "五", "六", "日"];
}

function calendarMonthLabel(month) {
  const [year, monthNumber] = month.split("-").map(Number);
  return new Intl.DateTimeFormat(locale() === "en" ? "en-US" : "zh-CN", {
    year: "numeric",
    month: "long"
  }).format(new Date(year, monthNumber - 1, 1));
}

function calendarDateLabel(date, includeYear = true) {
  const value = calendarDateFromKey(date);
  if (!value) return date;
  return new Intl.DateTimeFormat(locale() === "en" ? "en-US" : "zh-CN", {
    ...(includeYear ? { year: "numeric" } : {}),
    month: "short",
    day: "numeric"
  }).format(value);
}

function offsetCalendarMonth(month, amount) {
  const [year, monthNumber] = month.split("-").map(Number);
  return localDateKey(new Date(year, monthNumber - 1 + amount, 1)).slice(0, 7);
}

function calendarColorName(color) {
  const names = locale() === "en"
    ? { red: "Red", amber: "Amber", green: "Green", blue: "Blue", violet: "Violet", gray: "Gray" }
    : { red: "红色", amber: "琥珀色", green: "绿色", blue: "蓝色", violet: "紫色", gray: "灰色" };
  return names[color] || color;
}

function renderCalendarRangeBars(container, calendar, day, maximum = 3) {
  const ranges = calendar.ranges.filter(range => day.date >= range.start && day.date <= range.end);
  for (const range of ranges.slice(0, maximum)) {
    const bar = document.createElement("span");
    bar.className = "calendar-range-bar";
    bar.dataset.calendarRangeIndex = String(calendar.ranges.indexOf(range));
    bar.dataset.calendarColor = range.color;
    bar.classList.toggle("is-segment-start", day.date === range.start || day.weekday === 0);
    bar.classList.toggle("is-segment-end", day.date === range.end || day.weekday === 6);
    const showTitle = day.date === range.start || (day.weekday === 0 && day.date > range.start);
    if (showTitle) bar.textContent = range.title;
    bar.title = `${range.title} · ${calendarDateLabel(range.start)} - ${calendarDateLabel(range.end)}`;
    container.append(bar);
  }
  if (ranges.length > maximum) {
    const more = document.createElement("span");
    more.className = "calendar-range-more";
    more.textContent = `+${ranges.length - maximum}`;
    container.append(more);
  }
}

function closeCalendarQuickEditor() {
  calendarQuickEditor?.panel?.remove();
  calendarQuickEditor = null;
}

function calendarQuickColorField(color, onChange) {
  const field = document.createElement("fieldset");
  field.className = "calendar-color-field calendar-quick-colors";
  const legend = document.createElement("legend");
  legend.textContent = localized("颜色");
  field.append(legend);
  calendarColors.forEach(value => {
    const button = document.createElement("button");
    button.type = "button";
    button.dataset.calendarColor = value;
    button.title = calendarColorName(value);
    button.setAttribute("aria-label", button.title);
    button.classList.toggle("is-active", value === color);
    button.setAttribute("aria-pressed", String(value === color));
    button.addEventListener("click", () => {
      onChange(value);
      field.querySelectorAll("button").forEach(option => {
        const selected = option.dataset.calendarColor === value;
        option.classList.toggle("is-active", selected);
        option.setAttribute("aria-pressed", String(selected));
      });
    });
    field.append(button);
  });
  return field;
}

function positionCalendarQuickEditor(panel, anchor) {
  const block = panel.closest(".calendar-block");
  if (!block || !anchor?.isConnected) return;
  const blockRect = block.getBoundingClientRect();
  const anchorRect = anchor.getBoundingClientRect();
  const width = Math.min(310, Math.max(240, blockRect.width - 16));
  panel.style.width = `${width}px`;
  const left = Math.min(blockRect.width - width - 8, Math.max(8, anchorRect.left - blockRect.left + anchorRect.width / 2 - width / 2));
  let top = anchorRect.bottom - blockRect.top + 7;
  if (anchorRect.bottom + panel.offsetHeight > window.innerHeight - 18 && anchorRect.top - panel.offsetHeight > 18) {
    top = anchorRect.top - blockRect.top - panel.offsetHeight - 7;
  }
  panel.style.left = `${left}px`;
  panel.style.top = `${top}px`;
}

function calendarQuickHeader(title, close) {
  const header = document.createElement("header");
  const heading = document.createElement("strong");
  heading.textContent = title;
  const button = document.createElement("button");
  button.type = "button";
  button.className = "calendar-quick-close";
  button.setAttribute("aria-label", localized("关闭"));
  button.innerHTML = '<svg aria-hidden="true"><use href="#i-close"/></svg>';
  button.addEventListener("click", close);
  header.append(heading, button);
  return header;
}

function saveCalendarQuickEditor() {
  if (!calendarQuickEditor) return;
  const { block, calendar } = calendarQuickEditor;
  block.dataset.calendarSource = serializeCalendarDocument(calendar);
  closeCalendarQuickEditor();
  renderCalendarBlock(block);
  syncFromWrite();
}

function renderCalendarDateQuickEditor() {
  const editor = calendarQuickEditor;
  if (!editor || editor.kind !== "date") return;
  const { block, date } = editor;
  const anchor = block.querySelector(`.calendar-day-cell[data-date="${date}"]`);
  if (!anchor) return;
  editor.panel?.remove();
  const panel = document.createElement("section");
  panel.className = "calendar-quick-editor calendar-date-quick-editor";
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-label", locale() === "en" ? "Edit date" : "编辑日期");
  panel.addEventListener("pointerdown", event => event.stopPropagation());
  panel.addEventListener("click", event => event.stopPropagation());
  editor.panel = panel;
  panel.append(calendarQuickHeader(calendarDateLabel(date), closeCalendarQuickEditor));

  const titleField = document.createElement("label");
  titleField.className = "calendar-quick-field";
  const titleLabel = document.createElement("span");
  titleLabel.textContent = localized("标题");
  const title = document.createElement("input");
  title.maxLength = 120;
  title.autocomplete = "off";
  title.placeholder = locale() === "en" ? "Important date" : "重要日期";
  title.value = editor.markTitle;
  title.addEventListener("input", () => {
    editor.markTitle = title.value;
    editor.markTouched = true;
  });
  titleField.append(titleLabel, title);
  panel.append(titleField, calendarQuickColorField(editor.markColor, color => {
    editor.markColor = color;
    editor.markTouched = true;
  }));

  const itemSection = document.createElement("section");
  itemSection.className = "calendar-quick-items";
  const itemHeading = document.createElement("strong");
  itemHeading.textContent = locale() === "en" ? "Items" : "日期事项";
  itemSection.append(itemHeading);
  const items = editor.calendar.items.filter(item => item.date === date);
  const list = document.createElement("div");
  list.className = "calendar-quick-item-list";
  items.forEach(item => {
    const row = document.createElement("label");
    row.className = "calendar-quick-item";
    const toggle = document.createElement("input");
    toggle.type = "checkbox";
    toggle.checked = item.done;
    toggle.addEventListener("change", () => { item.done = toggle.checked; });
    const text = document.createElement("span");
    text.textContent = item.text;
    const remove = document.createElement("button");
    remove.type = "button";
    remove.setAttribute("aria-label", locale() === "en" ? "Remove item" : "删除事项");
    remove.innerHTML = '<svg aria-hidden="true"><use href="#i-trash"/></svg>';
    remove.addEventListener("click", event => {
      event.preventDefault();
      editor.calendar.items = editor.calendar.items.filter(candidate => candidate !== item);
      renderCalendarDateQuickEditor();
    });
    row.append(toggle, text, remove);
    list.append(row);
  });
  itemSection.append(list);
  const form = document.createElement("form");
  form.className = "calendar-quick-item-form";
  const itemInput = document.createElement("input");
  itemInput.maxLength = 240;
  itemInput.autocomplete = "off";
  itemInput.placeholder = locale() === "en" ? "Add item" : "添加事项";
  const add = document.createElement("button");
  add.type = "submit";
  add.className = "primary-button";
  add.textContent = localized("添加");
  form.append(itemInput, add);
  form.addEventListener("submit", event => {
    event.preventDefault();
    const text = itemInput.value.replace(/\s+/g, " ").trim();
    if (!text) return;
    editor.calendar.items.push({ date, text, done: false });
    editor.calendar = normalizeCalendarDocument(editor.calendar);
    renderCalendarDateQuickEditor();
    requestAnimationFrame(() => editor.panel.querySelector(".calendar-quick-item-form input")?.focus());
  });
  itemSection.append(form);
  panel.append(itemSection);

  const footer = document.createElement("footer");
  if (editor.hadMark) {
    const removeMark = document.createElement("button");
    removeMark.type = "button";
    removeMark.className = "quiet-button";
    removeMark.textContent = localized("移除标记");
    removeMark.addEventListener("click", () => {
      editor.calendar.marks = editor.calendar.marks.filter(mark => mark.date !== date);
      editor.hadMark = false;
      editor.markTouched = false;
      editor.markTitle = "";
      renderCalendarDateQuickEditor();
    });
    footer.append(removeMark);
  }
  const spacer = document.createElement("span");
  const save = document.createElement("button");
  save.type = "button";
  save.className = "primary-button";
  save.textContent = locale() === "en" ? "Save date" : "保存日期";
  save.addEventListener("click", () => {
    editor.calendar.marks = editor.calendar.marks.filter(mark => mark.date !== date);
    const markTitle = editor.markTitle.replace(/\s+/g, " ").trim();
    if (editor.hadMark || editor.markTouched || markTitle) editor.calendar.marks.push({ date, color: editor.markColor, title: markTitle });
    editor.calendar = normalizeCalendarDocument(editor.calendar);
    saveCalendarQuickEditor();
  });
  footer.append(spacer, save);
  panel.append(footer);
  block.append(panel);
  requestAnimationFrame(() => {
    positionCalendarQuickEditor(panel, anchor);
    title.focus();
  });
}

function openCalendarDateQuickEditor(block, date) {
  const calendar = parseCalendarSource(block.dataset.calendarSource || "");
  if (!calendar || !calendarDateFromKey(date)) return;
  closeCalendarQuickEditor();
  const mark = calendar.marks.find(item => item.date === date);
  calendarQuickEditor = {
    kind: "date",
    block,
    calendar,
    date,
    markTitle: mark?.title || "",
    markColor: mark?.color || "blue",
    markTouched: false,
    hadMark: Boolean(mark),
    panel: null
  };
  renderCalendarDateQuickEditor();
}

function renderCalendarRangeQuickEditor() {
  const editor = calendarQuickEditor;
  if (!editor || editor.kind !== "range") return;
  const { block, start, end } = editor;
  const anchor = block.querySelector(`.calendar-day-cell[data-date="${end}"]`)
    || block.querySelector(`.calendar-day-cell[data-date="${start}"]`);
  if (!anchor) return;
  const panel = document.createElement("section");
  panel.className = "calendar-quick-editor calendar-range-quick-editor";
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-label", locale() === "en" ? "Edit date range" : "编辑日期范围");
  panel.addEventListener("pointerdown", event => event.stopPropagation());
  panel.addEventListener("click", event => event.stopPropagation());
  editor.panel = panel;
  const count = calendarRangeDayCount(start, end);
  const label = locale() === "en"
    ? `${calendarDateLabel(start)} - ${calendarDateLabel(end)} · ${count} days`
    : `${calendarDateLabel(start)} - ${calendarDateLabel(end)} · ${count} 天`;
  panel.append(calendarQuickHeader(label, closeCalendarQuickEditor));
  const titleField = document.createElement("label");
  titleField.className = "calendar-quick-field";
  const titleLabel = document.createElement("span");
  titleLabel.textContent = localized("标题");
  const title = document.createElement("input");
  title.maxLength = 120;
  title.autocomplete = "off";
  title.placeholder = locale() === "en" ? "Frontend development" : "前端开发";
  title.value = editor.title;
  title.addEventListener("input", () => { editor.title = title.value; });
  titleField.append(titleLabel, title);
  panel.append(titleField, calendarQuickColorField(editor.color, color => { editor.color = color; }));
  const footer = document.createElement("footer");
  if (editor.rangeIndex >= 0) {
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "quiet-button";
    remove.textContent = locale() === "en" ? "Remove range" : "删除范围";
    remove.addEventListener("click", () => {
      editor.calendar.ranges.splice(editor.rangeIndex, 1);
      editor.calendar = normalizeCalendarDocument(editor.calendar);
      saveCalendarQuickEditor();
    });
    footer.append(remove);
  }
  const spacer = document.createElement("span");
  const save = document.createElement("button");
  save.type = "button";
  save.className = "primary-button";
  save.textContent = editor.rangeIndex >= 0
    ? (locale() === "en" ? "Update range" : "更新范围")
    : (locale() === "en" ? "Add range" : "添加范围");
  save.addEventListener("click", () => {
    const titleValue = editor.title.replace(/\s+/g, " ").trim();
    if (!titleValue) {
      title.classList.add("is-invalid");
      title.focus();
      return;
    }
    const range = { start, end, color: editor.color, title: titleValue };
    if (editor.rangeIndex >= 0) editor.calendar.ranges.splice(editor.rangeIndex, 1, range);
    else editor.calendar.ranges.push(range);
    editor.calendar = normalizeCalendarDocument(editor.calendar);
    saveCalendarQuickEditor();
  });
  footer.append(spacer, save);
  panel.append(footer);
  block.append(panel);
  requestAnimationFrame(() => {
    positionCalendarQuickEditor(panel, anchor);
    title.focus();
  });
}

function openCalendarRangeQuickEditor(block, startDate, endDate, rangeIndex = -1) {
  const calendar = parseCalendarSource(block.dataset.calendarSource || "");
  if (!calendar || !calendarDateFromKey(startDate) || !calendarDateFromKey(endDate)) return;
  closeCalendarQuickEditor();
  const [start, end] = [startDate, endDate].sort();
  const existing = rangeIndex >= 0 ? calendar.ranges[rangeIndex] : null;
  calendarQuickEditor = {
    kind: "range",
    block,
    calendar,
    start: existing?.start || start,
    end: existing?.end || end,
    title: existing?.title || "",
    color: existing?.color || "green",
    rangeIndex: existing ? rangeIndex : -1,
    panel: null
  };
  renderCalendarRangeQuickEditor();
}

function clearCalendarDragPreview() {
  calendarDrag?.block?.querySelectorAll(".is-drag-selected, .is-drag-start, .is-drag-end").forEach(cell => {
    cell.classList.remove("is-drag-selected", "is-drag-start", "is-drag-end");
  });
  calendarDrag?.block?.classList.remove("is-dragging-range");
}

function updateCalendarDragPreview(endDate) {
  if (!calendarDrag || !calendarDateFromKey(endDate)) return;
  calendarDrag.end = endDate;
  const [start, end] = [calendarDrag.start, endDate].sort();
  calendarDrag.block.querySelectorAll(".calendar-day-cell").forEach(cell => {
    const selected = cell.dataset.date >= start && cell.dataset.date <= end;
    cell.classList.toggle("is-drag-selected", selected);
    cell.classList.toggle("is-drag-start", selected && cell.dataset.date === start);
    cell.classList.toggle("is-drag-end", selected && cell.dataset.date === end);
  });
}

function beginCalendarRangeDrag(block, cell, event) {
  if (event.button !== 0 || event.pointerType === "touch") return;
  closeCalendarQuickEditor();
  calendarDrag = {
    block,
    start: cell.dataset.date,
    end: cell.dataset.date,
    pointerId: event.pointerId,
    originX: event.clientX,
    originY: event.clientY,
    moved: false,
    capture: cell
  };
  try { cell.setPointerCapture?.(event.pointerId); } catch { /* Synthetic events do not own a native pointer. */ }
  block.classList.add("is-dragging-range");
  updateCalendarDragPreview(cell.dataset.date);
  event.preventDefault();
}

function renderCalendarBlock(block, { interactive = true } = {}) {
  const calendar = parseCalendarSource(block.dataset.calendarSource || "");
  if (!calendar) return;
  if (calendarQuickEditor?.block === block) closeCalendarQuickEditor();
  if (interactive) block.dataset.calendarSource = serializeCalendarDocument(calendar);
  else block.removeAttribute("data-calendar-source");
  block.replaceChildren();

  const header = document.createElement("header");
  header.className = "calendar-block-header";
  const heading = document.createElement("div");
  heading.innerHTML = `<small>${locale() === "en" ? "MONTHLY CALENDAR" : "月历"}</small><h3>${escapeHTML(calendarMonthLabel(calendar.month))}</h3>`;
  header.append(heading);
  if (interactive) {
    const edit = document.createElement("button");
    edit.type = "button";
    edit.className = "calendar-edit-button";
    edit.title = locale() === "en" ? "Manage calendar" : "管理日历";
    edit.setAttribute("aria-label", edit.title);
    edit.innerHTML = '<svg aria-hidden="true"><use href="#i-edit"/></svg>';
    edit.addEventListener("click", () => openCalendarEditor(block));
    header.append(edit);
  }
  block.append(header);

  const weekdays = document.createElement("div");
  weekdays.className = "calendar-block-weekdays";
  calendarWeekdayLabels().forEach(label => {
    const element = document.createElement("span");
    element.textContent = label;
    weekdays.append(element);
  });
  block.append(weekdays);

  const days = document.createElement("div");
  days.className = "calendar-block-days";
  days.setAttribute("role", "grid");
  const today = localDateKey();
  calendarMonthDays(calendar.month).forEach(day => {
    const cell = document.createElement("div");
    cell.className = "calendar-day-cell";
    cell.dataset.date = day.date;
    cell.classList.toggle("is-outside", !day.currentMonth);
    cell.classList.toggle("is-today", day.date === today);
    cell.setAttribute("role", "gridcell");
    if (interactive) {
      cell.tabIndex = 0;
      cell.setAttribute("aria-label", calendarDateLabel(day.date));
      cell.addEventListener("pointerdown", event => {
        if (event.target.closest(".calendar-range-bar, .calendar-day-items")) return;
        beginCalendarRangeDrag(block, cell, event);
      });
      cell.addEventListener("click", event => {
        if (block.dataset.calendarSuppressClick === "true" || event.target.closest(".calendar-range-bar, .calendar-day-items")) return;
        openCalendarDateQuickEditor(block, day.date);
      });
      cell.addEventListener("keydown", event => {
        if (!calendarQuickEditor && ["Enter", " "].includes(event.key)) {
          event.preventDefault();
          openCalendarDateQuickEditor(block, day.date);
        }
      });
    }
    const mark = calendar.marks.find(item => item.date === day.date);
    if (mark) {
      cell.dataset.marked = "true";
      cell.dataset.calendarColor = mark.color;
      if (mark.title) cell.title = mark.title;
    }

    const dayHead = document.createElement("div");
    dayHead.className = "calendar-day-head";
    const dateControl = document.createElement("span");
    dateControl.className = "calendar-date-number";
    dateControl.textContent = String(day.day);
    dayHead.append(dateControl);
    if (mark) {
      const marker = document.createElement("span");
      marker.className = "calendar-mark-dot";
      marker.dataset.calendarColor = mark.color;
      dayHead.append(marker);
    }
    cell.append(dayHead);

    const rangeStack = document.createElement("div");
    rangeStack.className = "calendar-range-stack";
    renderCalendarRangeBars(rangeStack, calendar, day);
    if (interactive) {
      rangeStack.querySelectorAll(".calendar-range-bar").forEach(bar => {
        bar.tabIndex = 0;
        bar.setAttribute("role", "button");
        const openRange = event => {
          event.preventDefault();
          event.stopPropagation();
          openCalendarRangeQuickEditor(block, day.date, day.date, Number(bar.dataset.calendarRangeIndex));
        };
        bar.addEventListener("click", openRange);
        bar.addEventListener("keydown", event => {
          if (["Enter", " "].includes(event.key)) openRange(event);
        });
      });
    }
    cell.append(rangeStack);

    if (mark?.title) {
      const title = document.createElement("span");
      title.className = "calendar-mark-title";
      title.textContent = mark.title;
      cell.append(title);
    }

    const items = calendar.items.filter(item => item.date === day.date);
    if (items.length) {
      const details = document.createElement("details");
      details.className = "calendar-day-items";
      if (interactive) {
        details.addEventListener("pointerdown", event => event.stopPropagation());
        details.addEventListener("click", event => event.stopPropagation());
      }
      const summary = document.createElement("summary");
      summary.textContent = locale() === "en" ? `${items.length} item${items.length === 1 ? "" : "s"}` : `${items.length} 项`;
      const list = document.createElement("ul");
      items.forEach(item => {
        const entry = document.createElement("li");
        entry.classList.toggle("is-done", item.done);
        entry.textContent = item.text;
        list.append(entry);
      });
      details.append(summary, list);
      cell.append(details);
    }
    days.append(cell);
  });
  block.append(days);
}

function enhanceCalendars(root = write, options = {}) {
  root.querySelectorAll(".calendar-block").forEach(block => renderCalendarBlock(block, options));
}

function selectedCalendarColor(field, color) {
  field.querySelectorAll("[data-calendar-color]").forEach(button => {
    const selected = button.dataset.calendarColor === color;
    button.title = calendarColorName(button.dataset.calendarColor);
    button.setAttribute("aria-label", button.title);
    button.classList.toggle("is-active", selected);
    button.setAttribute("aria-pressed", String(selected));
  });
}

function initializeCalendarColors(field, kind) {
  if (field.querySelector("button")) return;
  calendarColors.forEach(color => {
    const button = document.createElement("button");
    button.type = "button";
    button.dataset.calendarColor = color;
    button.title = calendarColorName(color);
    button.setAttribute("aria-label", button.title);
    button.addEventListener("click", () => {
      if (!calendarEditor) return;
      calendarEditor[`${kind}Color`] = color;
      selectedCalendarColor(field, color);
    });
    field.append(button);
  });
}

function setCalendarActiveDate(date) {
  if (!calendarEditor || !calendarDateFromKey(date)) return;
  calendarEditor.activeDate = date;
  const mark = calendarEditor.calendar.marks.find(item => item.date === date);
  calendarEditor.markTitle = mark?.title || "";
  calendarEditor.markColor = mark?.color || "blue";
}

function calendarRangeSelectionText() {
  if (!calendarEditor?.rangeStart) return locale() === "en" ? "No range selected" : "未选择日期范围";
  if (!calendarEditor.rangeEnd) return `${calendarDateLabel(calendarEditor.rangeStart)} → …`;
  const [start, end] = calendarEditor.rangeStart <= calendarEditor.rangeEnd
    ? [calendarEditor.rangeStart, calendarEditor.rangeEnd]
    : [calendarEditor.rangeEnd, calendarEditor.rangeStart];
  const count = calendarRangeDayCount(start, end);
  return locale() === "en"
    ? `${calendarDateLabel(start)} → ${calendarDateLabel(end)} · ${count} days`
    : `${calendarDateLabel(start)} → ${calendarDateLabel(end)} · ${count} 天`;
}

function renderCalendarRecordLists() {
  const rangeList = $("#calendar-range-list");
  rangeList.replaceChildren();
  calendarEditor.calendar.ranges.forEach((range, index) => {
    const row = document.createElement("div");
    row.className = "calendar-record-row";
    row.dataset.calendarColor = range.color;
    const select = document.createElement("button");
    select.type = "button";
    select.className = "calendar-record-main";
    select.innerHTML = `<strong>${escapeHTML(range.title)}</strong><small>${escapeHTML(calendarDateLabel(range.start))} - ${escapeHTML(calendarDateLabel(range.end))}</small>`;
    select.addEventListener("click", () => {
      calendarEditor.rangeStart = range.start;
      calendarEditor.rangeEnd = range.end;
      calendarEditor.rangeTitle = range.title;
      calendarEditor.rangeColor = range.color;
      calendarEditor.editingRange = index;
      renderCalendarEditor();
    });
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "calendar-record-remove";
    remove.setAttribute("aria-label", locale() === "en" ? "Remove range" : "删除范围");
    remove.innerHTML = '<svg aria-hidden="true"><use href="#i-trash"/></svg>';
    remove.addEventListener("click", () => {
      calendarEditor.calendar.ranges.splice(index, 1);
      resetCalendarRangeSelection();
      renderCalendarEditor();
    });
    row.append(select, remove);
    rangeList.append(row);
  });

  const itemList = $("#calendar-item-list");
  itemList.replaceChildren();
  calendarEditor.calendar.items.forEach((item, index) => {
    if (item.date !== calendarEditor.activeDate) return;
    const row = document.createElement("div");
    row.className = "calendar-record-row calendar-item-row";
    const toggle = document.createElement("input");
    toggle.type = "checkbox";
    toggle.checked = item.done;
    toggle.setAttribute("aria-label", locale() === "en" ? "Mark item complete" : "标记事项完成");
    toggle.addEventListener("change", () => { item.done = toggle.checked; });
    const text = document.createElement("span");
    text.textContent = item.text;
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "calendar-record-remove";
    remove.setAttribute("aria-label", locale() === "en" ? "Remove item" : "删除事项");
    remove.innerHTML = '<svg aria-hidden="true"><use href="#i-trash"/></svg>';
    remove.addEventListener("click", () => {
      calendarEditor.calendar.items.splice(index, 1);
      renderCalendarEditor();
    });
    row.append(toggle, text, remove);
    itemList.append(row);
  });
}

function renderCalendarEditor() {
  if (!calendarEditor) return;
  $("#calendar-editor-month").textContent = calendarMonthLabel(calendarEditor.calendar.month);
  const weekdays = $("#calendar-editor-weekdays");
  weekdays.replaceChildren(...calendarWeekdayLabels().map(label => {
    const span = document.createElement("span");
    span.textContent = label;
    return span;
  }));
  const days = $("#calendar-editor-days");
  days.replaceChildren();
  const today = localDateKey();
  calendarMonthDays(calendarEditor.calendar.month).forEach(day => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "calendar-editor-day";
    button.dataset.date = day.date;
    button.setAttribute("role", "gridcell");
    button.classList.toggle("is-outside", !day.currentMonth);
    button.classList.toggle("is-today", day.date === today);
    button.classList.toggle("is-active", day.date === calendarEditor.activeDate);
    button.setAttribute("aria-label", calendarDateLabel(day.date));
    const rangeMinimum = calendarEditor.rangeStart && calendarEditor.rangeEnd ? [calendarEditor.rangeStart, calendarEditor.rangeEnd].sort()[0] : "";
    const rangeMaximum = calendarEditor.rangeStart && calendarEditor.rangeEnd ? [calendarEditor.rangeStart, calendarEditor.rangeEnd].sort()[1] : "";
    button.classList.toggle("is-range-start", day.date === calendarEditor.rangeStart);
    button.classList.toggle("is-range-end", day.date === calendarEditor.rangeEnd);
    button.classList.toggle("is-in-draft-range", Boolean(rangeMinimum && day.date >= rangeMinimum && day.date <= rangeMaximum));
    const number = document.createElement("span");
    number.textContent = String(day.day);
    button.append(number);
    const mark = calendarEditor.calendar.marks.find(item => item.date === day.date);
    if (mark) {
      const marker = document.createElement("i");
      marker.className = "calendar-editor-mark";
      marker.dataset.calendarColor = mark.color;
      button.append(marker);
    }
    const ranges = document.createElement("span");
    ranges.className = "calendar-editor-range-stack";
    renderCalendarRangeBars(ranges, calendarEditor.calendar, day, 2);
    button.append(ranges);
    const itemCount = calendarEditor.calendar.items.filter(item => item.date === day.date).length;
    if (itemCount) {
      const badge = document.createElement("b");
      badge.textContent = String(itemCount);
      button.append(badge);
    }
    button.addEventListener("click", () => {
      if (calendarEditor.mode === "range") {
        if (!calendarEditor.rangeStart || calendarEditor.rangeEnd) {
          calendarEditor.rangeStart = day.date;
          calendarEditor.rangeEnd = "";
          calendarEditor.editingRange = -1;
        } else {
          calendarEditor.rangeEnd = day.date;
        }
        setCalendarActiveDate(day.date);
      } else {
        setCalendarActiveDate(day.date);
      }
      renderCalendarEditor();
    });
    days.append(button);
  });

  $("#calendar-active-date").textContent = calendarDateLabel(calendarEditor.activeDate);
  $$('[data-calendar-mode]').forEach(button => button.classList.toggle("is-active", button.dataset.calendarMode === calendarEditor.mode));
  $$('[data-calendar-panel]').forEach(panel => panel.classList.toggle("is-active", panel.dataset.calendarPanel === calendarEditor.mode));
  $("#calendar-mark-title").value = calendarEditor.markTitle;
  $("#calendar-range-title").value = calendarEditor.rangeTitle;
  selectedCalendarColor($("#calendar-mark-colors"), calendarEditor.markColor);
  selectedCalendarColor($("#calendar-range-colors"), calendarEditor.rangeColor);
  $("#calendar-remove-mark").disabled = !calendarEditor.calendar.marks.some(mark => mark.date === calendarEditor.activeDate);
  $("#calendar-range-selection").textContent = calendarRangeSelectionText();
  $("#calendar-save-range").textContent = calendarEditor.editingRange >= 0
    ? (locale() === "en" ? "Update range" : "更新范围")
    : (locale() === "en" ? "Add range" : "添加范围");
  $("#calendar-delete").hidden = !calendarEditor.block;
  renderCalendarRecordLists();
}

function openCalendarEditor(block = null, selectedDate = "") {
  closeCalendarQuickEditor();
  if (!block) {
    const selection = window.getSelection();
    calendarInsertRange = selection?.rangeCount ? selection.getRangeAt(0).cloneRange() : null;
  }
  const calendar = block
    ? parseCalendarSource(block.dataset.calendarSource || "")
    : normalizeCalendarDocument({}, new Date());
  if (!calendar) return;
  const preferredDate = calendarDateFromKey(selectedDate)
    ? selectedDate
    : (localDateKey().startsWith(calendar.month) ? localDateKey() : `${calendar.month}-01`);
  calendarEditor = {
    block,
    calendar,
    mode: "mark",
    activeDate: preferredDate,
    markTitle: "",
    markColor: "blue",
    rangeStart: "",
    rangeEnd: "",
    rangeTitle: "",
    rangeColor: "green",
    editingRange: -1
  };
  setCalendarActiveDate(preferredDate);
  $("#calendar-dialog").classList.add("is-open");
  $("#calendar-dialog").setAttribute("aria-hidden", "false");
  renderCalendarEditor();
  requestAnimationFrame(() => $("#calendar-mark-title").focus());
}

function closeCalendarEditor() {
  $("#calendar-dialog").classList.remove("is-open");
  $("#calendar-dialog").setAttribute("aria-hidden", "true");
  calendarEditor = null;
  calendarInsertRange = null;
}

function resetCalendarRangeSelection() {
  if (!calendarEditor) return;
  calendarEditor.rangeStart = "";
  calendarEditor.rangeEnd = "";
  calendarEditor.rangeTitle = "";
  calendarEditor.rangeColor = "green";
  calendarEditor.editingRange = -1;
}

function applyCalendarEditor() {
  if (!calendarEditor) return;
  const editorState = calendarEditor;
  const source = serializeCalendarDocument(editorState.calendar);
  if (editorState.block?.isConnected) {
    editorState.block.dataset.calendarSource = source;
    renderCalendarBlock(editorState.block);
    closeCalendarEditor();
    syncFromWrite();
    return;
  }
  const template = document.createElement("template");
  template.innerHTML = markdownToHTML(calendarMarkdown(editorState.calendar));
  const calendarBlock = template.content.querySelector(".calendar-block");
  if (!calendarBlock) return;
  const paragraph = document.createElement("p");
  paragraph.append(document.createElement("br"));
  const rangeBlock = calendarInsertRange && write.contains(calendarInsertRange.commonAncestorContainer)
    ? writeBlockForNode(calendarInsertRange.commonAncestorContainer)
    : null;
  const insertionBlock = rangeBlock === write ? null : rangeBlock;
  const emptyInsertionBlock = insertionBlock?.matches("p, div")
    && !(insertionBlock.textContent || "").replaceAll(caretMarker, "").trim();
  if (emptyInsertionBlock) insertionBlock.replaceWith(calendarBlock, paragraph);
  else if (insertionBlock) {
    const reference = insertionBlock.matches("table") && insertionBlock.nextElementSibling?.matches(".table-tools")
      ? insertionBlock.nextElementSibling
      : insertionBlock;
    reference.after(calendarBlock, paragraph);
  } else {
    write.append(calendarBlock, paragraph);
  }
  closeCalendarEditor();
  enhanceCalendars(write);
  placeTextCaret(paragraph, 0);
  syncFromWrite();
}

function deleteCalendarEditorBlock() {
  const block = calendarEditor?.block;
  if (!block?.isConnected) return;
  const next = block.nextElementSibling;
  block.remove();
  if (!next) {
    const paragraph = document.createElement("p");
    paragraph.append(document.createElement("br"));
    write.append(paragraph);
  }
  closeCalendarEditor();
  syncFromWrite();
}

function rawTableCells(block) {
  if (!block?.matches?.("p, div")) return null;
  const value = (block.textContent || "").trim();
  if (!/^\|.+\|$/.test(value)) return null;
  const cells = value.replace(/^\|/, "").replace(/\|$/, "").split(/(?<!\\)\|/).map(cell => cell.trim());
  return cells.length >= 2 ? cells : null;
}

function renderRawTableAtCaret({ allowHeaderOnly = false } = {}) {
  const current = currentWriteBlock();
  if (!rawTableCells(current)) return false;
  const blocks = [...write.children];
  const currentIndex = blocks.indexOf(current);
  const tableRow = block => Boolean(rawTableCells(block));
  let start = currentIndex;
  let end = currentIndex;
  while (start > 0 && tableRow(blocks[start - 1])) start -= 1;
  while (end + 1 < blocks.length && tableRow(blocks[end + 1])) end += 1;
  const separatorPattern = /^\s*\|?\s*:?-{3,}:?\s*(?:\|\s*:?-{3,}:?\s*)+\|?\s*$/;
  const separator = blocks.findIndex((block, index) => index >= start && index <= end && separatorPattern.test(block.textContent || ""));
  if (separator <= start && !allowHeaderOnly) return false;
  const tableBlocks = separator > start ? blocks.slice(separator - 1, end + 1) : blocks.slice(start, end + 1);
  const rows = tableBlocks.filter(block => !separatorPattern.test(block.textContent || "")).map(rawTableCells);
  if (!rows.length) return false;
  const width = Math.max(...rows.map(row => row.length));
  const normalized = rows.map(row => Array.from({ length: width }, (_, index) => row[index] || ""));
  const source = [normalized[0], Array.from({ length: width }, () => "---"), ...normalized.slice(1)]
    .map(row => `| ${row.join(" | ")} |`).join("\n");
  const template = document.createElement("template");
  template.innerHTML = markdownToHTML(source);
  const table = template.content.querySelector("table");
  if (!table) return false;
  tableBlocks[0].replaceWith(table);
  tableBlocks.slice(1).forEach(block => block.remove());
  enhanceTables(write);
  focusTableCell(table.tBodies[0]?.rows[0]?.cells[0]);
  return true;
}

function deleteTableBeforeCaret(event) {
  if (state.sourceMode || event.isComposing || event.keyCode === 229 || !["Backspace", "Delete"].includes(event.key)) return false;
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount) return false;
  const block = currentWriteBlock();
  let previous = null;
  if (block && selectionAtStart(block)) previous = block.previousElementSibling;
  else if (selection.anchorNode === write && selection.anchorOffset > 0) previous = write.children[selection.anchorOffset - 1] || null;
  const tools = previous?.matches?.(".table-tools") ? previous : null;
  const table = tools ? tools.previousElementSibling : (previous?.matches?.("table") ? previous : null);
  if (!table?.matches("table")) return false;
  event.preventDefault();
  const next = (tools || table).nextSibling;
  table.remove();
  tools?.remove();
  let target = block?.isConnected ? block : null;
  if (!target) {
    target = document.createElement("p");
    target.append(document.createElement("br"));
    write.insertBefore(target, next?.parentNode === write ? next : null);
  }
  placeTextCaret(target, 0);
  syncFromWrite();
  updateFocusLine();
  return true;
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
  const rawList = block.matches("p, div") && /^\s*(?:[-*+]|\d+[.)])\s+\S/.test(text);
  const rawInline = /`[^`\n]+`|\*\*[^*\n]+\*\*|__[^_\n]+__|~~[^~\n]+~~|(^|[^*])\*[^*\n]+\*(?!\*)|(^|[^_])_[^_\n]+_(?!_)|!?\[[^\]\n]+\]\([^\n)]+\)|<\/?[A-Za-z][^>]*>/.test(text);
  if (!rawHeading && !rawList && !rawInline) return false;
  if (!insertRenderCaretMarker()) return false;
  const html = markdownToHTML(block.textContent || "");
  const template = document.createElement("template");
  template.innerHTML = html || "<p><br></p>";
  enhanceRawHTML(template.content);
  block.replaceWith(template.content);
  restoreRenderCaret();
  enhanceTables(write);
  applyDocumentAssets(write);
  return true;
}

function renderMarkdownDocumentAtCaret() {
  const hasCaret = insertRenderCaretMarker();
  let markdown = editorToMarkdown(write, { escapeText: false });
  if (hasCaret) {
    // The parser drops a closing fence, so move the caret marker into the following paragraph.
    const closingFenceWithCaret = new RegExp("(^|\\n)(\\s*(?:```|~~~)\\s*)" + renderCaretMarker + "(?=\\n|$)", "g");
    markdown = markdown.replace(closingFenceWithCaret, `$1$2\n\n${renderCaretMarker}`);
  }
  write.innerHTML = markdownToHTML(markdown) || "<p><br></p>";
  enhanceRawHTML(write);
  enhanceTables(write);
  enhanceCalendars(write);
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
  applyDocumentAssets(write);
  highlightCodeBlocks(write);
  void renderMermaidDiagrams(write, state.documentTheme);
  return true;
}

function removeCaretMarkers() {
  const selection = window.getSelection();
  const anchorNode = selection?.isCollapsed ? selection.anchorNode : null;
  const anchorOffset = selection?.anchorOffset ?? 0;
  let nextOffset = anchorOffset;
  let anchorMarkerRemoved = false;
  const walker = document.createTreeWalker(write, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    const value = node.nodeValue ?? "";
    if (!value.includes(caretMarker)) continue;
    if (node === anchorNode) {
      nextOffset = value.slice(0, anchorOffset).replaceAll(caretMarker, "").length;
      anchorMarkerRemoved = true;
    }
    node.nodeValue = value.replaceAll(caretMarker, "");
  }
  if (anchorMarkerRemoved && anchorNode?.isConnected && selection) {
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
  const converted = renderRawTableAtCaret() || renderMarkdownBlockAtCaret();
  if (!converted) removeCaretMarkers();
  syncFromWrite();
  highlightCodeBlocks(write);
  updatePathSuggestions();
  scheduleMarkdownNormalization();
}

function normalizeInactiveRawHeadings(activeBlock = currentWriteBlock()) {
  let changed = false;
  for (const block of [...write.children]) {
    if (block === activeBlock || !rawHeadingMatch(block)) continue;
    const template = document.createElement("template");
    template.innerHTML = markdownToHTML(block.textContent || "");
    const heading = template.content.firstElementChild;
    if (!heading?.matches("h1, h2, h3, h4, h5, h6")) continue;
    block.replaceWith(heading);
    changed = true;
  }
  return changed;
}

function scheduleMarkdownNormalization() {
  if (markdownNormalizationFrame || state.sourceMode || activeComposition) return;
  if (![...write.children].some(block => rawHeadingMatch(block))) return;
  markdownNormalizationFrame = requestAnimationFrame(() => {
    markdownNormalizationFrame = 0;
    if (state.sourceMode || activeComposition) return;
    const converted = renderMarkdownBlockAtCaret();
    const normalizedInactive = normalizeInactiveRawHeadings(currentWriteBlock());
    if (converted || normalizedInactive) syncFromWrite();
  });
}

function syncFromSource(render = false) {
  state.markdown = sourceEditor.value;
  const document = activeDocument();
  if (document) document.markdown = state.markdown;
  if (render) {
    write.innerHTML = markdownToHTML(state.markdown) || "<p><br></p>";
    enhanceRawHTML(write);
    enhanceTables(write);
    enhanceCalendars(write);
    applyDocumentAssets(write);
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
  $("#save-state").textContent = localized("未保存");
  $("#save-state").classList.add("is-visible");
  clearTimeout(changeTimer);
  changeTimer = setTimeout(() => {
    updateDerivedState();
    rebuildWorkspaceKnowledge();
    localStorage.setItem("mory.draft", state.markdown);
    bridge({
      type: "changed",
      documentId: document?.id || "",
      name: documentHostName(document),
      path: document?.path || "",
      markdown: state.markdown
    });
  }, 180);
}

function updateDerivedState() {
  const stats = documentStats(state.markdown);
  $("#word-count").textContent = locale() === "en" ? `${stats.words} words` : `${stats.words} 字`;
  $("#line-count").textContent = locale() === "en" ? `${stats.lines} lines` : `${stats.lines} 行`;
  updateOutline();

  if (!state.titleTouched) {
    const title = firstLevelHeading(state.markdown) || localized("未命名");
    $("#document-title").value = title;
    bridge({ type: "title", value: title, dirty: state.dirty });
  }
  if (documentDisplayNameTracksHeading(activeDocument())) renderFiles();
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
  $("#outline-count").textContent = locale() === "en" ? `${entries.length} items` : `${entries.length} 项`;
  $("#outline-empty").hidden = entries.length > 0;
}

function fileEntryKey(file) {
  return file.path ? `path:${String(file.path).replaceAll("\\", "/")}` : `draft:${file.documentId || file.id}`;
}

function fileParentKey(file) {
  if (!file.path || !state.files.some(item => item.path === file.path)) return "";
  const parts = String(file.name || "").replaceAll("\\", "/").split("/").filter(Boolean);
  return parts.slice(0, -1).join("/");
}

function sortFilesByManualOrder(files) {
  const ranks = new Map(state.manualFileOrder.map((key, index) => [key, index]));
  return files
    .map((file, index) => ({ file, index, rank: ranks.get(fileEntryKey(file)) }))
    .sort((left, right) => {
      if (left.rank !== undefined || right.rank !== undefined) {
        if (left.rank === undefined) return 1;
        if (right.rank === undefined) return -1;
        if (left.rank !== right.rank) return left.rank - right.rank;
      }
      return left.index - right.index;
    })
    .map(item => item.file);
}

function reorderFileEntry(source, target, after) {
  if (!source || !target || source.parent !== target.parent || source.key === target.key) return;
  const siblings = sortFilesByManualOrder(visibleFileEntries().filter(file => fileParentKey(file) === source.parent));
  const keys = siblings.map(fileEntryKey).filter(key => key !== source.key);
  let index = keys.indexOf(target.key);
  if (index < 0) return;
  if (after) index += 1;
  keys.splice(index, 0, source.key);
  const siblingKeys = new Set(keys);
  state.manualFileOrder = [...state.manualFileOrder.filter(key => !siblingKeys.has(key)), ...keys];
  localStorage.setItem("mory.fileOrder", JSON.stringify(state.manualFileOrder));
  renderFiles();
}

function visibleFileEntries() {
  const drafts = state.documents
    .filter(document => !document.path)
    .map(document => ({ ...document, documentId: document.id, open: true }));
  const openByPath = new Map(state.documents.filter(document => document.path).map(document => [document.path, document]));
  const workspacePaths = new Set(state.files.map(file => file.path));
  const workspaceEntries = state.files.map(file => {
    const document = openByPath.get(file.path);
    return document
      ? { ...file, ...document, name: file.name, createdAt: file.createdAt, documentId: document.id, open: true }
      : { ...file, documentId: "", markdown: "", dirty: false, open: false };
  });
  const externalDocuments = state.documents
    .filter(document => document.path && !workspacePaths.has(document.path))
    .map(document => ({ ...document, documentId: document.id, open: true }));
  return [...workspaceEntries, ...externalDocuments, ...drafts];
}

function closeDocument(documentId) {
  const index = state.documents.findIndex(document => document.id === documentId);
  if (index < 0) return;

  const [removed] = state.documents.splice(index, 1);
  const message = localized(removed.path ? "文档已关闭" : "草稿已移除");
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

function removeDeletedDocument(file) {
  const index = state.documents.findIndex(document => document.id === file.documentId || document.path === file.path);
  const removed = index >= 0 ? state.documents.splice(index, 1)[0] : null;
  const removedActive = removed?.id === state.activeDocumentId;
  state.files = state.files.filter(document => document.path !== file.path);

  if (removedActive) {
    state.activeDocumentId = null;
    if (state.files[0]?.path) bridge({ type: "openFile", path: state.files[0].path });
    else if (state.documents.length) activateDocument(state.documents[0].id, { announce: false, notifyHost: true });
    else {
      state.untitledSequence = 0;
      createUntitledDocument("", { announce: false, notifyHost: true, workspacePlaceholder: true });
    }
  }
  renderFiles();
}

async function deleteDocument(file) {
  try {
    const result = await hostRequest("deleteDocument", { path: file.path, name: file.name });
    if (!result?.deleted) return;
    removeDeletedDocument(file);
    toast(localized("文档已移到废纸篓"));
  } catch (error) {
    toast(`${localized("删除文档失败")}：${error.message}`, 3200);
  }
}

function compareWorkspaceDirectories(left, right) {
  return String(left.name).localeCompare(String(right.name), "zh-CN", { numeric: true })
    || String(left.path).localeCompare(String(right.path));
}

function toggleNewFolderForm(force) {
  const form = $("#new-folder-form");
  const open = typeof force === "boolean" ? force : form.hidden;
  form.hidden = !open;
  $("#new-folder-button").classList.toggle("is-active", open);
  if (open) {
    const input = $("#new-folder-input");
    input.value = "";
    // An offscreen window may pause animation frames; focus synchronously before scheduling a retry.
    input.focus({ preventScroll: true });
    requestAnimationFrame(() => {
      if (!form.hidden) input.focus({ preventScroll: true });
    });
  }
}

async function createWorkspaceFolder(relativePath) {
  try {
    const directory = await hostRequest("createDirectory", { relativePath });
    if (!directory?.path) return;
    state.directories = [...state.directories.filter(item => item.path !== directory.path), directory].sort(compareWorkspaceDirectories);
    const parts = String(directory.name || "").replaceAll("\\", "/").split("/").filter(Boolean);
    const normalizedPath = String(directory.path).replaceAll("\\", "/");
    const suffix = parts.join("/");
    const prefix = normalizedPath.endsWith(suffix) ? normalizedPath.slice(0, -suffix.length) : "";
    parts.forEach((_, index) => state.expandedDirectoryPaths.add(`${prefix}${parts.slice(0, index + 1).join("/")}`));
    renderFiles();
    toggleNewFolderForm(false);
    toast(localized("目录已创建"));
  } catch (error) {
    toast(`${localized("创建目录失败")}：${error.message}`, 3200);
    $("#new-folder-input").focus();
  }
}

function closeFileContextMenu() {
  contextEntry = null;
  const menu = $("#file-context-menu");
  menu.classList.remove("is-open");
  menu.setAttribute("aria-hidden", "true");
}

function selectWorkspaceEntry(entry) {
  state.selectedWorkspaceEntry = entry?.path ? { kind: entry.kind, path: entry.path, name: entry.name } : null;
  renderFiles();
}

function showFileContextMenu(entry, event) {
  if (!entry.path) return;
  event.preventDefault();
  event.stopPropagation();
  state.selectedWorkspaceEntry = { kind: entry.kind, path: entry.path, name: entry.name };
  contextEntry = entry;
  const menu = $("#file-context-menu");
  menu.querySelectorAll("[data-entry-kind]").forEach(button => {
    button.hidden = button.dataset.entryKind !== entry.kind;
  });
  const deleteButton = menu.querySelector("[data-entry-action='delete']");
  deleteButton.textContent = localized(entry.kind === "directory" ? "删除目录" : "删除文档");
  menu.classList.add("is-open");
  menu.setAttribute("aria-hidden", "false");
  const bounds = menu.getBoundingClientRect();
  menu.style.left = `${Math.max(8, Math.min(event.clientX, window.innerWidth - bounds.width - 8))}px`;
  menu.style.top = `${Math.max(8, Math.min(event.clientY, window.innerHeight - bounds.height - 8))}px`;
  menu.querySelector("button")?.focus({ preventScroll: true });
}

async function openWorkspaceFile(file) {
  if (file.documentId) {
    activateDocument(file.documentId, { announce: true, notifyHost: true });
    return activeDocument();
  }
  const payload = await hostRequest("readDocument", { path: file.path });
  openDocument(payload);
  return activeDocument();
}

async function openEditorLink(linkOrURL) {
  const href = typeof linkOrURL === "string" ? linkOrURL : linkOrURL?.getAttribute("href") || "";
  if (/\.(?:md|markdown)(?:[?#].*)?$/i.test(href)) {
    const file = workspaceFileForLink(href);
    if (!file) {
      toast(locale() === "en" ? "Linked document was not found" : "未找到链接的文稿", 3200);
      return;
    }
    await openWorkspaceFile(file);
    return;
  }
  if (!/^(?:https?:|mailto:)/i.test(href)) return;
  if (window.moryNative || nativeMacHost || nativeWailsHost()) {
    await hostRequest("openExternal", { url: href });
    return;
  }
  window.open(href, "_blank", "noopener,noreferrer");
}

function bareURLAtEditorClick(event) {
  let node = null;
  let offset = 0;
  const hasPoint = event.clientX !== 0 || event.clientY !== 0;
  const position = hasPoint ? document.caretPositionFromPoint?.(event.clientX, event.clientY) : null;
  const range = hasPoint && !position ? document.caretRangeFromPoint?.(event.clientX, event.clientY) : null;
  if (position?.offsetNode) {
    node = position.offsetNode;
    offset = position.offset;
  } else if (range?.startContainer) {
    node = range.startContainer;
    offset = range.startOffset;
  }
  if (node?.nodeType !== Node.TEXT_NODE || !write.contains(node)) {
    const selection = window.getSelection();
    node = selection?.anchorNode || null;
    offset = selection?.anchorOffset || 0;
  }
  if (node?.nodeType !== Node.TEXT_NODE || !write.contains(node)) return "";
  const text = node.nodeValue || "";
  const expression = /https?:\/\/[^\s<>"']+|mailto:[^\s<>"']+/gi;
  for (const match of text.matchAll(expression)) {
    const value = match[0].replace(/[.,;:!?)}\]]+$/, "");
    if (offset >= match.index && offset <= match.index + value.length) return value;
  }
  return "";
}

function handleEditorLinkClick(event) {
  if (event.button !== 0 || (!event.ctrlKey && !event.metaKey)) return;
  const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
  const bareURL = link ? "" : bareURLAtEditorClick(event);
  if ((!link || !link.closest("#write")) && !bareURL) return;
  event.preventDefault();
  event.stopPropagation();
  void openEditorLink(link || bareURL).catch(error => {
    toast(locale() === "en" ? `Unable to open link: ${error.message}` : `无法打开链接：${error.message}`, 3200);
  });
}

function selectedDirectory() {
  if (state.selectedWorkspaceEntry?.kind === "directory") {
    return state.directories.find(directory => directory.path === state.selectedWorkspaceEntry.path)
      || state.selectedWorkspaceEntry;
  }
  const selectedFilePath = state.selectedWorkspaceEntry?.kind === "file"
    ? state.selectedWorkspaceEntry.path
    : "";
  const activePath = activeDocument()?.path || "";
  const workspaceFilePath = selectedFilePath || (state.files.some(file => file.path === activePath) ? activePath : "");
  if (!workspaceFilePath) return null;
  // The add button inherits the nearest parent of the selected or active workspace document.
  return state.directories
    .filter(directory => pathIsWithin(directory.path, workspaceFilePath) && directory.path !== workspaceFilePath)
    .sort((left, right) => String(right.path).length - String(left.path).length)[0] || null;
}

function beginSelectedEntryRename() {
  const selected = state.selectedWorkspaceEntry;
  if (!selected?.path) return false;
  toggleEntryOperation(true, { action: "rename", entry: selected });
  return true;
}

function pathIsWithin(parent, candidate) {
  const normalizedParent = String(parent || "").replaceAll("\\", "/").replace(/\/$/, "");
  const normalizedCandidate = String(candidate || "").replaceAll("\\", "/");
  return normalizedCandidate === normalizedParent || normalizedCandidate.startsWith(`${normalizedParent}/`);
}

function replacePathPrefix(value, source, target) {
  const normalized = String(value || "").replaceAll("\\", "/");
  const normalizedSource = String(source || "").replaceAll("\\", "/").replace(/\/$/, "");
  const normalizedTarget = String(target || "").replaceAll("\\", "/").replace(/\/$/, "");
  if (normalized === normalizedSource) return normalizedTarget;
  if (!normalized.startsWith(`${normalizedSource}/`)) return value;
  return `${normalizedTarget}${normalized.slice(normalizedSource.length)}`;
}

function renameDocumentAssets(document, sourcePath, targetPath) {
  const sourceBase = String(sourcePath).replaceAll("\\", "/").split("/").at(-1).replace(/\.[^.]+$/, "");
  const targetBase = String(targetPath).replaceAll("\\", "/").split("/").at(-1).replace(/\.[^.]+$/, "");
  document.markdown = String(document.markdown || "").split(`](${sourceBase}/`).join(`](${targetBase}/`);
  if (!document.assets || sourceBase === targetBase) return;
  document.assets = Object.fromEntries(Object.entries(document.assets).map(([key, value]) => [
    replacePathPrefix(key, sourceBase, targetBase),
    value
  ]));
}

function applyRenamedWorkspaceEntry(entry, result) {
  if (!result?.path) return;
  const isDirectory = entry.kind === "directory" || result.isDirectory;
  const renamePath = value => isDirectory ? replacePathPrefix(value, entry.path, result.path) : (value === entry.path ? result.path : value);
  const renameName = value => isDirectory ? replacePathPrefix(value, entry.name, result.name) : (value === entry.name ? result.name : value);
  state.files = state.files.map(file => pathIsWithin(entry.path, file.path) ? {
    ...file,
    name: renameName(file.name),
    path: renamePath(file.path),
    images: Array.isArray(file.images) ? file.images.map(image => ({ ...image, path: renamePath(image.path) })) : file.images
  } : file);
  state.directories = state.directories.map(directory => pathIsWithin(entry.path, directory.path) ? {
    ...directory,
    name: renameName(directory.name),
    path: renamePath(directory.path)
  } : directory);
  state.documents.forEach(document => {
    if (!pathIsWithin(entry.path, document.path)) return;
    if (!isDirectory) renameDocumentAssets(document, entry.path, result.path);
    document.path = renamePath(document.path);
    document.name = renameName(document.name);
  });
  state.expandedDirectoryPaths = new Set([...state.expandedDirectoryPaths].map(renamePath));
  state.expandedImagePaths = new Set([...state.expandedImagePaths].map(renamePath));
  state.manualFileOrder = state.manualFileOrder.map(key => key.startsWith("path:") ? `path:${renamePath(key.slice(5))}` : key);
  localStorage.setItem("mory.fileOrder", JSON.stringify(state.manualFileOrder));
  const active = activeDocument();
  if (active && !isDirectory && active.path === result.path) {
    state.markdown = active.markdown;
    sourceEditor.value = state.markdown;
    const sourceBase = String(entry.path).replaceAll("\\", "/").split("/").at(-1).replace(/\.[^.]+$/, "");
    const targetBase = String(result.path).replaceAll("\\", "/").split("/").at(-1).replace(/\.[^.]+$/, "");
    write.querySelectorAll("img[data-markdown-src]").forEach(image => {
      image.dataset.markdownSrc = replacePathPrefix(image.dataset.markdownSrc, sourceBase, targetBase);
    });
  }
}

async function createDocumentInSelectedDirectory(directory = selectedDirectory()) {
  if (!directory) {
    createUntitledDocument();
    return;
  }
  try {
    const document = await hostRequest("createDocument", { directoryPath: directory.path, name: localized("未命名.md") });
    if (!document?.path) return;
    state.expandedDirectoryPaths.add(directory.path);
    openDocument(document);
    toast(localized("新文稿已创建"));
  } catch (error) {
    toast(`${localized("操作失败")}：${error.message}`, 3200);
  }
}

function toggleEntryOperation(force, operation = pendingEntryOperation) {
  const dialog = $("#entry-operation-dialog");
  const open = typeof force === "boolean" ? force : !dialog.classList.contains("is-open");
  if (!open || !operation) {
    pendingEntryOperation = null;
    dialog.classList.remove("is-open");
    dialog.setAttribute("aria-hidden", "true");
    return;
  }
  pendingEntryOperation = operation;
  const isRename = operation.action === "rename";
  const isCopy = operation.action === "copy";
  const actionLabel = isRename ? "重命名条目" : (isCopy ? "复制条目" : "移动条目");
  $("#entry-operation-title").textContent = localized(actionLabel);
  $("#entry-operation-confirm").textContent = localized(actionLabel);
  $("#entry-operation-source").textContent = operation.entry.name;
  $("#entry-operation-destination-row").hidden = isRename;
  $("#entry-operation-name-row").hidden = !isRename;
  if (isRename) {
    const input = $("#entry-operation-name");
    input.value = String(operation.entry.name || "").replaceAll("\\", "/").split("/").at(-1);
    dialog.classList.add("is-open");
    dialog.setAttribute("aria-hidden", "false");
    input.focus({ preventScroll: true });
    const extension = operation.entry.kind === "file" ? input.value.lastIndexOf(".") : -1;
    input.setSelectionRange(0, extension > 0 ? extension : input.value.length);
    return;
  }
  const select = $("#entry-operation-destination");
  select.innerHTML = "";
  const sourceParent = String(operation.entry.path).replaceAll("\\", "/").split("/").slice(0, -1).join("/");
  const workspaceRoot = String(activeWorkspace()?.localPath || "").replaceAll("\\", "/").replace(/\/$/, "");
  const choices = [{ name: localized("工作区根目录"), path: "" }, ...state.directories]
    .filter(directory => operation.entry.kind !== "directory" || !directory.path || !pathIsWithin(operation.entry.path, directory.path))
    .filter(directory => operation.action !== "move" || String(directory.path || workspaceRoot).replaceAll("\\", "/").replace(/\/$/, "") !== sourceParent);
  choices.forEach(directory => {
    const option = document.createElement("option");
    option.value = directory.path || "";
    option.textContent = directory.name || localized("工作区根目录");
    select.append(option);
  });
  dialog.classList.add("is-open");
  dialog.setAttribute("aria-hidden", "false");
  select.focus({ preventScroll: true });
}

async function confirmEntryOperation() {
  const operation = pendingEntryOperation;
  if (!operation) return;
  if (operation.action === "rename") {
    const name = $("#entry-operation-name").value.trim();
    if (!name) {
      toast(localized("新名称"));
      $("#entry-operation-name").focus();
      return;
    }
    toggleEntryOperation(false);
    try {
      const result = await hostRequest("renameWorkspaceEntry", { path: operation.entry.path, name });
      applyRenamedWorkspaceEntry(operation.entry, result);
      state.selectedWorkspaceEntry = result?.path
        ? { kind: operation.entry.kind, path: result.path, name: result.name }
        : null;
      renderFiles();
      toast(localized("重命名完成"));
    } catch (error) {
      toast(`${localized("操作失败")}：${error.message}`, 3200);
    }
    return;
  }
  const destinationPath = $("#entry-operation-destination").value;
  toggleEntryOperation(false);
  try {
    const method = operation.action === "copy" ? "copyWorkspaceEntry" : "moveWorkspaceEntry";
    const result = await hostRequest(method, { path: operation.entry.path, destinationPath });
    if (result?.path && !result.isDirectory && operation.action === "move") {
      const payload = await hostRequest("readDocument", { path: result.path });
      openDocument(payload);
    }
    toast(localized(operation.action === "copy" ? "复制完成" : "移动完成"));
  } catch (error) {
    toast(`${localized("操作失败")}：${error.message}`, 3200);
  }
}

async function deleteWorkspaceEntry(entry) {
  try {
    const result = await hostRequest("deleteWorkspaceEntry", { path: entry.path, name: entry.name });
    if (!result?.deleted) return;
    if (entry.kind === "directory") {
      state.directories = state.directories.filter(directory => !pathIsWithin(entry.path, directory.path));
      setWorkspaceFiles(state.files.filter(file => !pathIsWithin(entry.path, file.path)));
    } else {
      removeDeletedDocument(entry);
    }
    state.selectedWorkspaceEntry = null;
    toast(localized(entry.kind === "directory" ? "目录已移到废纸篓" : "文档已移到废纸篓"));
  } catch (error) {
    toast(`${localized("删除文档失败")}：${error.message}`, 3200);
  }
}

async function handleFileContextAction(action) {
  const entry = contextEntry;
  closeFileContextMenu();
  if (!entry) return;
  try {
    if (action === "open") await openWorkspaceFile(entry);
    if (action === "new-document") await createDocumentInSelectedDirectory(entry);
    if (action === "new-folder") toggleNewFolderForm(true);
    if (action === "reveal") await hostRequest("revealFile", { path: entry.path });
    if (action === "copy-absolute-path" || action === "copy-relative-path") {
      const relative = action === "copy-relative-path";
      const text = relative ? String(entry.name || "").replaceAll("\\", "/") : String(entry.path || "");
      if (!text) throw new Error(localized("操作失败"));
      await hostRequest("copyText", { text });
      toast(localized(relative ? "相对路径已复制" : "绝对路径已复制"));
    }
    if (action === "rename") toggleEntryOperation(true, { action, entry });
    if (action === "copy" || action === "move") toggleEntryOperation(true, { action, entry });
    if (action === "export") {
      await openWorkspaceFile(entry);
      toggleExportDialog(true);
    }
    if (action === "delete") await deleteWorkspaceEntry(entry);
  } catch (error) {
    toast(error.message, 3200);
  }
}

function closeImagePreview() {
  const preview = $("#image-preview");
  preview.classList.remove("is-open");
  preview.setAttribute("aria-hidden", "true");
  $("#image-preview-content").removeAttribute("src");
}

async function previewDocumentImage(image) {
  try {
    const result = await hostRequest("documentImage", { path: image.path });
    if (!result?.dataURL) throw new Error(localized("图片加载失败"));
    $("#image-preview-name").textContent = image.name || localized("图片预览");
    $("#image-preview-content").src = result.dataURL;
    $("#image-preview-content").alt = image.name || localized("图片预览");
    const preview = $("#image-preview");
    preview.classList.add("is-open");
    preview.setAttribute("aria-hidden", "false");
  } catch (error) {
    toast(`${localized("图片加载失败")}：${error.message}`, 3200);
  }
}

function buildWorkspaceTree(entries) {
  const root = { directories: new Map(), files: [] };
  const ensureDirectory = (parts, paths = []) => {
    let node = root;
    const walked = [];
    parts.forEach((part, index) => {
      walked.push(part);
      if (!node.directories.has(part)) node.directories.set(part, { name: walked.join("/"), path: "", directories: new Map(), files: [] });
      node = node.directories.get(part);
      if (!node.path && paths[index]) node.path = paths[index];
    });
    return node;
  };
  state.directories.forEach(directory => {
    const parts = String(directory.name || "").replaceAll("\\", "/").split("/").filter(Boolean);
    if (!parts.length) return;
    const normalizedPath = String(directory.path || "").replaceAll("\\", "/");
    const suffix = parts.join("/");
    const prefix = normalizedPath.endsWith(suffix) ? normalizedPath.slice(0, -suffix.length) : "";
    const paths = parts.map((_, index) => prefix ? `${prefix}${parts.slice(0, index + 1).join("/")}` : "");
    const node = ensureDirectory(parts, paths);
    Object.assign(node, directory, { directories: node.directories, files: node.files });
  });
  entries.forEach(file => {
    const parts = file.path && state.files.some(item => item.path === file.path)
      ? String(file.name || "").replaceAll("\\", "/").split("/").filter(Boolean)
      : [documentDisplayName(file)];
    ensureDirectory(parts.slice(0, -1)).files.push(file);
  });
  const sortNode = node => {
    node.files = sortFilesByManualOrder(node.files);
    node.directories.forEach(sortNode);
  };
  sortNode(root);
  return root;
}

function workspaceEntryDetails(entry) {
  const size = formatFileSize(entry?.size);
  const updated = formatUpdatedAt(entry?.updatedAt, locale());
  return [size, updated].filter(Boolean).join(" · ");
}

function renderFileEntry(list, file, depth) {
    const row = document.createElement("div");
    row.className = "file-row";
    row.style.setProperty("--tree-depth", String(depth));
    const images = Array.isArray(file.images) ? file.images : [];
    const imagesExpanded = Boolean(file.path && state.expandedImagePaths.has(file.path));
    if (images.length) {
      row.classList.add("has-images");
      const expander = document.createElement("button");
      expander.className = "file-expander";
      expander.setAttribute("aria-expanded", String(imagesExpanded));
      expander.setAttribute("aria-label", localized(imagesExpanded ? "收起图片" : "展开图片"));
      expander.title = localized(imagesExpanded ? "收起图片" : "展开图片");
      expander.innerHTML = '<svg aria-hidden="true"><use href="#i-chevron"/></svg>';
      expander.addEventListener("click", event => {
        event.stopPropagation();
        if (imagesExpanded) state.expandedImagePaths.delete(file.path);
        else state.expandedImagePaths.add(file.path);
        renderFiles();
      });
      row.append(expander);
    }
    const button = document.createElement("button");
    const selected = state.selectedWorkspaceEntry?.kind === "file" && state.selectedWorkspaceEntry.path === file.path;
    const active = file.documentId === state.activeDocumentId && (!state.selectedWorkspaceEntry || selected);
    button.className = `file-item${active ? " is-active" : ""}${selected ? " is-selected" : ""}`;
    button.dataset.path = file.path;
    const entryKey = fileEntryKey(file);
    const parentKey = fileParentKey(file);
    button.dataset.entryKey = entryKey;
    button.dataset.parentKey = parentKey;
    button.draggable = true;
    if (file.documentId) button.dataset.documentId = file.documentId;
    button.innerHTML = `<span class="file-symbol">${file.path ? "M" : "M↓"}</span><span class="file-copy"><span class="file-name"></span></span><span class="file-dirty"></span>`;
    button.querySelector(".file-name").textContent = String(documentDisplayName(file)).replaceAll("\\", "/").split("/").at(-1);
    const details = state.showFileDetails ? workspaceEntryDetails(file) : "";
    if (details) {
      const metadata = document.createElement("span");
      metadata.className = "file-meta";
      metadata.textContent = details;
      button.querySelector(".file-copy").append(metadata);
    }
    button.querySelector(".file-dirty").setAttribute("aria-label", localized("未保存"));
    button.querySelector(".file-dirty").hidden = !file.dirty;
    button.addEventListener("click", () => {
      if (file.path) state.selectedWorkspaceEntry = { kind: "file", path: file.path, name: file.name };
      if (file.documentId) activateDocument(file.documentId, { announce: true, notifyHost: true, focusEditor: false });
      else if (file.path) bridge({ type: "openFile", path: file.path });
    });
    button.addEventListener("contextmenu", event => showFileContextMenu({ ...file, kind: "file" }, event));
    if (file.path) button.addEventListener("dblclick", () => toggleEntryOperation(true, { action: "rename", entry: { ...file, kind: "file" } }));
    button.addEventListener("dragstart", event => {
      draggedFileEntry = { key: entryKey, parent: parentKey };
      event.dataTransfer?.setData("text/plain", entryKey);
      if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
      button.classList.add("is-dragging");
    });
    button.addEventListener("dragend", () => {
      draggedFileEntry = null;
      button.classList.remove("is-dragging");
      $$(".file-row.is-drop-before, .file-row.is-drop-after").forEach(item => item.classList.remove("is-drop-before", "is-drop-after"));
    });
    button.addEventListener("dragover", event => {
      if (!draggedFileEntry || draggedFileEntry.parent !== parentKey || draggedFileEntry.key === entryKey) return;
      event.preventDefault();
      const after = event.clientY >= button.getBoundingClientRect().top + button.getBoundingClientRect().height / 2;
      row.classList.toggle("is-drop-before", !after);
      row.classList.toggle("is-drop-after", after);
    });
    button.addEventListener("dragleave", event => {
      if (!row.contains(event.relatedTarget)) row.classList.remove("is-drop-before", "is-drop-after");
    });
    button.addEventListener("drop", event => {
      if (!draggedFileEntry || draggedFileEntry.parent !== parentKey || draggedFileEntry.key === entryKey) return;
      event.preventDefault();
      const after = event.clientY >= button.getBoundingClientRect().top + button.getBoundingClientRect().height / 2;
      const source = draggedFileEntry;
      draggedFileEntry = null;
      reorderFileEntry(source, { key: entryKey, parent: parentKey }, after);
    });
    row.append(button);
    if (file.documentId || file.path) {
      const close = document.createElement("button");
      const label = localized(file.path ? "删除文档" : "移除草稿");
      close.className = "file-close";
      if (file.documentId) close.dataset.documentId = file.documentId;
      if (file.path) close.dataset.path = file.path;
      close.title = label;
      close.setAttribute("aria-label", label);
      close.innerHTML = '<svg aria-hidden="true"><use href="#i-close"/></svg>';
      close.addEventListener("click", event => {
        event.stopPropagation();
        if (file.path) void deleteDocument(file);
        else closeDocument(file.documentId);
      });
      row.append(close);
    }
    list.append(row);
    if (imagesExpanded) {
      const assets = document.createElement("div");
      assets.className = "file-assets";
      images.forEach(image => {
        const asset = document.createElement("button");
        asset.className = "file-asset";
        asset.title = image.relative || image.name;
        asset.innerHTML = '<svg aria-hidden="true"><use href="#i-image"/></svg><span class="file-asset-copy"><span class="file-asset-name"></span></span>';
        asset.querySelector(".file-asset-name").textContent = image.name;
        const imageDetails = state.showFileDetails ? workspaceEntryDetails(image) : "";
        if (imageDetails) {
          const metadata = document.createElement("span");
          metadata.className = "file-meta";
          metadata.textContent = imageDetails;
          asset.querySelector(".file-asset-copy").append(metadata);
        }
        asset.addEventListener("click", () => void previewDocumentImage(image));
        assets.append(asset);
      });
      list.append(assets);
    }
}

function renderDirectoryNode(list, directory, depth) {
  const expanded = state.expandedDirectoryPaths.has(directory.path);
  const row = document.createElement("div");
  row.className = "folder-row";
  row.style.setProperty("--tree-depth", String(depth));
  const toggle = document.createElement("button");
  toggle.className = "folder-toggle";
  toggle.setAttribute("aria-expanded", String(expanded));
  toggle.setAttribute("aria-label", localized(expanded ? "收起目录" : "展开目录"));
  toggle.innerHTML = '<svg aria-hidden="true"><use href="#i-chevron"/></svg>';
  toggle.addEventListener("click", event => {
    event.stopPropagation();
    if (expanded) state.expandedDirectoryPaths.delete(directory.path);
    else state.expandedDirectoryPaths.add(directory.path);
    renderFiles();
  });
  const button = document.createElement("button");
  const selected = state.selectedWorkspaceEntry?.kind === "directory" && state.selectedWorkspaceEntry.path === directory.path;
  button.className = `folder-item${selected ? " is-selected" : ""}`;
  button.title = directory.name;
  button.dataset.path = directory.path;
  button.innerHTML = '<svg aria-hidden="true"><use href="#i-files"/></svg><span class="folder-name"></span>';
  button.querySelector(".folder-name").textContent = String(directory.name).replaceAll("\\", "/").split("/").at(-1);
  button.addEventListener("click", () => selectWorkspaceEntry({ ...directory, kind: "directory" }));
  button.addEventListener("dblclick", () => toggleEntryOperation(true, { action: "rename", entry: { ...directory, kind: "directory" } }));
  button.addEventListener("contextmenu", event => showFileContextMenu({ ...directory, kind: "directory" }, event));
  row.append(toggle, button);
  list.append(row);
  if (!expanded) return;
  [...directory.directories.values()].sort(compareWorkspaceDirectories).forEach(child => renderDirectoryNode(list, child, depth + 1));
  directory.files.forEach(file => renderFileEntry(list, file, depth + 1));
}

function renderFiles() {
  const list = $("#file-list");
  list.innerHTML = "";
  const entries = visibleFileEntries();
  const tree = buildWorkspaceTree(entries);
  const pendingDrafts = tree.files.filter(file => !file.path && !state.manualFileOrder.includes(fileEntryKey(file)));
  const positionedFiles = tree.files.filter(file => !pendingDrafts.includes(file));
  const hasManualRootOrder = positionedFiles.some(file => state.manualFileOrder.includes(fileEntryKey(file)));
  if (hasManualRootOrder) {
    positionedFiles.forEach(file => renderFileEntry(list, file, 0));
    [...tree.directories.values()].sort(compareWorkspaceDirectories).forEach(directory => renderDirectoryNode(list, directory, 0));
  } else {
    positionedFiles.filter(file => file.path).forEach(file => renderFileEntry(list, file, 0));
    [...tree.directories.values()].sort(compareWorkspaceDirectories).forEach(directory => renderDirectoryNode(list, directory, 0));
  }
  pendingDrafts.forEach(file => renderFileEntry(list, file, 0));
  renderQuickResults(entries);
}

function renderQuickResults(files = visibleFileEntries(), query = "") {
  const normalized = query.trim().toLocaleLowerCase();
  const results = files.filter(file => documentDisplayName(file).toLocaleLowerCase().includes(normalized));
  const container = $("#quick-open-results");
  container.innerHTML = "";
  results.forEach((file, index) => {
    const button = document.createElement("button");
    button.className = `quick-result${index === 0 ? " is-active" : ""}`;
    const name = document.createElement("span");
    name.textContent = documentDisplayName(file);
    const path = document.createElement("small");
    path.textContent = file.path || localized("当前草稿");
    button.append(name, path);
    button.addEventListener("click", () => {
      if (file.documentId) activateDocument(file.documentId, { announce: true, notifyHost: true });
      else if (file.path) bridge({ type: "openFile", path: file.path });
      closeQuickOpen();
    });
    container.append(button);
  });
  if (!results.length) container.innerHTML = `<p class="empty-state">${locale() === "en" ? "No matching files" : "没有匹配的文件"}</p>`;
}

function toggleSource(force) {
  const next = typeof force === "boolean" ? force : !state.sourceMode;
  if (next === state.sourceMode) return;
  closePathSuggestions();
  if (next) {
    syncFromWrite();
    sourceEditor.value = state.markdown;
  } else {
    syncFromSource(true);
  }
  state.sourceMode = next;
  workspace.classList.toggle("source-mode", next);
  const sourceButton = $("#source-toggle");
  const label = next ? (locale() === "en" ? "Preview mode (⌘/)" : "预览模式（⌘/）") : localized("源代码模式（⌘/）");
  sourceButton.classList.toggle("is-active", next);
  sourceButton.dataset.tooltip = label;
  sourceButton.setAttribute("aria-label", label);
  requestAnimationFrame(() => (next ? sourceEditor : write).focus());
}

function execute(command) {
  if (state.sourceMode) toggleSource(false);
  write.focus();
  beginEditorHistory(`command-${command}`, { force: true });
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
    enhanceTables(write);
  } else if (command === "calendar") {
    openCalendarEditor();
    return;
  } else if (command === "typography") {
    optimizeActiveDocumentTypography();
    return;
  }
  syncFromWrite();
}

function optimizeActiveDocumentTypography() {
  const document = activeDocument();
  if (!document) return;
  const current = state.sourceMode ? sourceEditor.value : editorToMarkdown(write);
  const next = optimizeMarkdownTypography(current, value => globalThis.pangu.spacingText(value));
  if (next === current) {
    toast(localized("当前文稿无需优化"));
    return;
  }
  document.markdown = next;
  document.dirty = true;
  renderDocument(document);
  markChanged();
  requestAnimationFrame(() => (state.sourceMode ? sourceEditor : write).focus());
  toast(localized("排版已优化"));
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
  beginEditorHistory("set-heading", { force: true });
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

const workspacePluginFields = {
  local: [
    { name: "localPath", label: "工作目录", wide: true, placeholder: "使用“选择本地目录”填写", required: true }
  ],
  github: [
    { name: "repository", label: "仓库", placeholder: "owner/repository", required: true },
    { name: "branch", label: "分支", placeholder: "main" },
    { name: "endpoint", label: "API 地址", wide: true, placeholder: "https://api.github.com" },
    { name: "prefix", label: "仓库内目录", placeholder: "docs" },
    { name: "token", label: "Access Token", type: "password", placeholder: "GitHub Access Token", secret: true, required: true }
  ],
  s3: objectStorageFields("https://s3.amazonaws.com"),
  s4: objectStorageFields("S3 兼容服务地址"),
  oss: objectStorageFields("https://oss-cn-hangzhou.aliyuncs.com"),
  sftp: [
    { name: "host", label: "服务器", placeholder: "sftp.example.com", required: true },
    { name: "port", label: "端口", type: "number", placeholder: "22" },
    { name: "username", label: "用户名", required: true },
    { name: "password", label: "密码", type: "password", secret: true },
    { name: "privateKey", label: "私钥或私钥路径", type: "password", secret: true, wide: true },
    { name: "knownHosts", label: "known_hosts 路径", wide: true, placeholder: "默认 ~/.ssh/known_hosts" },
    { name: "remotePath", label: "远端目录", wide: true, placeholder: "/home/user/documents", required: true }
  ]
};

function objectStorageFields(endpointPlaceholder) {
  return [
    { name: "endpoint", label: "Endpoint", wide: true, placeholder: endpointPlaceholder },
    { name: "region", label: "区域", placeholder: "cn-hangzhou / us-east-1", required: true },
    { name: "bucket", label: "Bucket", required: true },
    { name: "prefix", label: "路径前缀", placeholder: "mory" },
    { name: "accessKeyId", label: "Access Key ID", required: true },
    { name: "accessKeySecret", label: "Secret Access Key", type: "password", secret: true, required: true },
    { name: "sessionToken", label: "Session / Security Token", type: "password", secret: true, wide: true }
  ];
}

function activeWorkspace() {
  return state.workspaces.find(item => item.id === state.activeWorkspaceId) || null;
}

function setWorkspaceState(payload = {}) {
  const previousId = state.activeWorkspaceId;
  const nextId = String(payload.activeId || payload.workspaces?.[0]?.id || "");
  state.workspaces = Array.isArray(payload.workspaces) ? payload.workspaces : [];
  state.activeWorkspaceId = nextId;
  if (previousId && nextId && previousId !== nextId) resetWorkspaceSession();
  renderWorkspaceSettings();
}

function discardWorkspacePlaceholders() {
  const placeholders = state.documents.filter(document => document.workspacePlaceholder && !document.path && !document.dirty);
  if (!placeholders.length) return false;
  const placeholderIds = new Set(placeholders.map(document => document.id));
  const activeWasPlaceholder = placeholderIds.has(state.activeDocumentId);
  state.documents = state.documents.filter(document => !placeholderIds.has(document.id));
  if (activeWasPlaceholder) state.activeDocumentId = null;
  return activeWasPlaceholder;
}

function deletedDraftName(name) {
  const filename = String(name || localized("未命名.md"));
  const dot = filename.lastIndexOf(".");
  const suffix = localized("磁盘文件已删除");
  return dot > 0
    ? `${filename.slice(0, dot)}（${suffix}）${filename.slice(dot)}`
    : `${filename}（${suffix}）`;
}

function reconcileDeletedWorkspaceDocuments(previousFiles, nextFiles) {
  const previousPaths = new Set(previousFiles.map(file => file.path).filter(Boolean));
  const nextPaths = new Set(nextFiles.map(file => file.path).filter(Boolean));
  const deletedPaths = new Set([...previousPaths].filter(path => !nextPaths.has(path)));
  if (!deletedPaths.size) return false;

  let activeDeleted = false;
  let activeDraft = null;
  state.documents = state.documents.filter(document => {
    if (!document.path || !deletedPaths.has(document.path)) return true;
    if (document.dirty) {
      document.path = "";
      document.name = deletedDraftName(document.name);
      document.workspacePlaceholder = false;
      if (document.id === state.activeDocumentId) activeDraft = document;
      return true;
    }
    if (document.id === state.activeDocumentId) activeDeleted = true;
    return false;
  });

  if (activeDeleted) state.activeDocumentId = null;
  if (activeDraft) {
    notifyDocumentSelected(activeDraft);
    toast(localized("文件已从磁盘删除，未保存内容已保留为草稿"), 3200);
  }
  return activeDeleted;
}

function setWorkspaceFiles(files = [], { openFirst = false } = {}) {
  const previousFiles = state.files;
  state.files = Array.isArray(files) ? [...files].sort(compareWorkspaceFiles) : [];
  const activeDeleted = reconcileDeletedWorkspaceDocuments(previousFiles, state.files);
  const activePlaceholderRemoved = state.files.length ? discardWorkspacePlaceholders() : false;
  const firstFile = state.files.length && (activeDeleted || (openFirst && activePlaceholderRemoved))
    ? state.files[0]
    : null;
  if (!firstFile && !activeDocument()) {
    const fallback = state.documents[0];
    if (fallback) activateDocument(fallback.id, { announce: false, notifyHost: true });
    else if (!state.files.length) createUntitledDocument("", {
      announce: false,
      notifyHost: true,
      workspacePlaceholder: true
    });
  }
  renderFiles();
  void refreshWorkspaceKnowledge();
  if (firstFile?.path) bridge({ type: "openFile", path: firstFile.path });
}

function compareWorkspaceFiles(left, right) {
  const leftTime = Number.isFinite(Number(left.createdAt)) ? Number(left.createdAt) : Number.MAX_SAFE_INTEGER;
  const rightTime = Number.isFinite(Number(right.createdAt)) ? Number(right.createdAt) : Number.MAX_SAFE_INTEGER;
  return leftTime - rightTime
    || String(left.name).localeCompare(String(right.name), "zh-CN", { numeric: true })
    || String(left.path).localeCompare(String(right.path));
}

function setWorkspaceSnapshot(payload = {}) {
  setWorkspaceState(payload.state || {});
  state.directories = Array.isArray(payload.directories) ? [...payload.directories].sort(compareWorkspaceDirectories) : [];
  const existingDirectories = new Set(state.directories.map(directory => directory.path));
  state.expandedDirectoryPaths = new Set([
    ...state.expandedDirectoryPaths,
    ...state.directories.map(directory => directory.path)
  ].filter(path => existingDirectories.has(path)));
  if (state.selectedWorkspaceEntry && !(
    state.directories.some(directory => directory.path === state.selectedWorkspaceEntry.path)
    || (payload.files || []).some(file => file.path === state.selectedWorkspaceEntry.path)
  )) state.selectedWorkspaceEntry = null;
  // Open the first sorted document in a non-empty workspace; keep a placeholder only for an empty one.
  setWorkspaceFiles(payload.files || [], { openFirst: true });
}

function resetWorkspaceSession() {
  state.documents = [];
  state.files = [];
  state.directories = [];
  state.expandedDirectoryPaths.clear();
  state.expandedImagePaths.clear();
  state.selectedWorkspaceEntry = null;
  state.workspaceDocuments = [];
  state.graph = { nodes: [], edges: [] };
  workspaceKnowledgeRequest += 1;
  updateDocumentBacklinks();
  toggleKnowledgeGraph(false);
  state.untitledSequence = 0;
  createUntitledDocument("", { announce: false, notifyHost: true, workspacePlaceholder: true });
}

function renderWorkspaceSettings() {
  const select = $("#workspace-select");
  select.innerHTML = "";
  state.workspaces.forEach(item => {
    const option = document.createElement("option");
    option.value = item.id;
    option.textContent = `${item.name} · ${item.provider === "local" ? localized("本地") : item.provider.toUpperCase()}`;
    select.append(option);
  });
  select.value = state.activeWorkspaceId;
  const current = activeWorkspace();
  $("#workspace-button").textContent = current?.name || localized("本地工作区");
  $("#folder-name").textContent = current?.name || localized("工作区");
  $("#workspace-path").textContent = current?.localPath || localized("尚未连接宿主");
  $("#workspace-path").title = current?.localPath || localized("尚未连接宿主");
  const local = !current || current.provider === "local";
  $("#workspace-pull").hidden = local;
  $("#workspace-push").hidden = local;
}

function renderWorkspaceFields(provider, workspaceValue = {}) {
  const container = $("#workspace-provider-fields");
  container.innerHTML = "";
  (workspacePluginFields[provider] || []).forEach(field => {
    const label = document.createElement("label");
    if (field.wide) label.className = "workspace-wide";
    const caption = document.createElement("span");
    caption.textContent = localized(field.label);
    const input = document.createElement("input");
    input.name = field.name;
    input.type = field.type || "text";
    input.placeholder = field.secret && workspaceValue[`${field.name}Configured`] ? localized("已配置；留空则保持不变") : localized(field.placeholder || "");
    if (!field.secret) input.value = workspaceValue[field.name] ?? "";
    input.required = Boolean(field.required && !(field.secret && workspaceValue[`${field.name}Configured`]));
    if (field.name === "localPath") input.readOnly = true;
    label.append(caption, input);
    container.append(label);
  });
}

function showWorkspaceForm(workspaceValue = null) {
  const editing = workspaceValue || {};
  state.editingWorkspaceId = editing.id || "";
  $("#workspace-form").hidden = false;
  $("#workspace-form-heading").textContent = localized(editing.id ? "配置工作区" : "新增工作区");
  $("#workspace-name").value = editing.name || "";
  $("#workspace-provider").value = editing.provider || "local";
  $("#workspace-provider").disabled = Boolean(editing.id);
  $("#workspace-remove").hidden = !editing.id;
  renderWorkspaceFields(editing.provider || "local", editing);
  $("#workspace-name").focus();
}

function hideWorkspaceForm() {
  state.editingWorkspaceId = "";
  $("#workspace-form").hidden = true;
}

function collectWorkspaceForm() {
  const provider = $("#workspace-provider").value;
  const workspaceValue = {
    id: state.editingWorkspaceId || undefined,
    name: $("#workspace-name").value.trim(),
    provider
  };
  $("#workspace-provider-fields").querySelectorAll("input[name]").forEach(input => {
    if (input.value !== "") workspaceValue[input.name] = input.type === "number" ? Number(input.value) : input.value;
  });
  return workspaceValue;
}

async function switchWorkspace(id) {
  if (id === state.activeWorkspaceId) return;
  if (state.documents.some(document => document.dirty) && !confirm("当前有未保存文稿。切换工作区会关闭这些文稿，是否继续？")) {
    $("#workspace-select").value = state.activeWorkspaceId;
    return;
  }
  try {
    const result = await hostRequest("activateWorkspace", { id });
    setWorkspaceState(result);
    toast("已切换工作区");
  } catch (error) {
    $("#workspace-select").value = state.activeWorkspaceId;
    toast(error.message);
  }
}

async function syncWorkspace(action) {
  const button = action === "push" ? $("#workspace-push") : $("#workspace-pull");
  const original = button.textContent;
  button.disabled = true;
  button.textContent = action === "push" ? "正在推送…" : "正在拉取…";
  try {
    const result = await hostRequest("syncWorkspace", { action });
    toast(`同步完成：${result.files || 0} 个文件`);
  } catch (error) {
    toast(`同步失败：${error.message}`);
  } finally {
    button.disabled = false;
    button.textContent = original;
  }
}

function graphDocuments(documents = state.workspaceDocuments) {
  const byPath = new Map(documents.map(document => [document.path || document.name, { ...document }]));
  for (const document of state.documents) {
    if (!document.path) continue;
    byPath.set(document.path, { name: state.files.find(file => file.path === document.path)?.name || document.name, path: document.path, markdown: document.markdown });
  }
  return [...byPath.values()];
}

function activeGraphNode() {
  const document = activeDocument();
  if (!document?.path) return null;
  return state.graph.nodes.find(node => node.path === document.path) || null;
}

function activateGraphDocument(node) {
  const document = state.documents.find(item => item.path && item.path === node.path);
  if (document) activateDocument(document.id, { announce: true, notifyHost: true });
  else if (node.path) bridge({ type: "openFile", path: node.path });
}

function updateDocumentBacklinks() {
  const node = activeGraphNode();
  const backlinks = node?.backlinks || [];
  $("#backlink-count").textContent = locale() === "en" ? `Backlinks ${backlinks.length}` : `反向链接 ${backlinks.length}`;
  const panel = $("#document-backlinks");
  panel.hidden = !node || backlinks.length === 0;
  $("#document-backlinks-title").textContent = localized("反向链接");
  $("#document-backlinks-summary").textContent = backlinks.length
    ? (locale() === "en" ? `${backlinks.length} notes link to this note` : `${backlinks.length} 篇文稿引用当前文稿`)
    : localized("没有文稿引用当前文稿");
  const list = $("#document-backlinks-list");
  list.innerHTML = "";
  for (const id of backlinks) {
    const source = state.graph.nodes.find(item => item.id === id);
    if (!source) continue;
    const button = document.createElement("button");
    button.type = "button";
    button.dataset.nodeId = source.id;
    const title = document.createElement("strong");
    title.textContent = source.title;
    const path = document.createElement("small");
    path.textContent = source.name;
    button.append(title, path);
    button.addEventListener("click", () => activateGraphDocument(source));
    list.append(button);
  }
}

function rebuildWorkspaceKnowledge({ renderGraph = false } = {}) {
  state.graph = buildKnowledgeGraph(graphDocuments());
  updateDocumentBacklinks();
  if (renderGraph || $("#knowledge-graph").classList.contains("is-open")) renderKnowledgeGraph(state.graph);
}

async function refreshWorkspaceKnowledge({ renderGraph = false } = {}) {
  const request = ++workspaceKnowledgeRequest;
  try {
    if (window.moryNative || nativeMacHost || nativeWailsHost()) {
      const documents = await hostRequest("workspaceDocuments");
      if (request !== workspaceKnowledgeRequest) return;
      state.workspaceDocuments = Array.isArray(documents) ? documents : [];
    }
    rebuildWorkspaceKnowledge({ renderGraph });
  } catch (error) {
    if (request !== workspaceKnowledgeRequest) return;
    // Preserve the last valid snapshot when a transient host error prevents a refresh.
    rebuildWorkspaceKnowledge({ renderGraph });
    if (renderGraph) toast(locale() === "en" ? `Unable to build graph: ${error.message}` : `无法生成知识图谱：${error.message}`);
  }
}

function updateGraphLabels() {
  const graph = state.graph;
  $("#graph-title").textContent = localized("知识图谱");
  $("#graph-stats").textContent = locale() === "en"
    ? `${graph.nodes.length} notes · ${graph.edges.length} links`
    : `${graph.nodes.length} 篇文稿 · ${graph.edges.length} 条链接`;
  $("#graph-refresh").textContent = localized("刷新");
  $("#graph-search").placeholder = localized("筛选文稿");
  $("#graph-empty").textContent = localized("当前工作区还没有可显示的文稿");
  const selected = graph.nodes.find(node => node.id === state.selectedGraphNodeId);
  if (selected) renderGraphRelations(selected);
}

function openGraphNode(node) {
  activateGraphDocument(node);
  toggleKnowledgeGraph(false);
}

function graphNodeID(value) {
  return typeof value === "object" ? value.id : value;
}

function renderGraphRelationList(container, ids, emptyLabel) {
  container.innerHTML = "";
  for (const id of ids) {
    const target = state.graph.nodes.find(node => node.id === id);
    if (!target) continue;
    const button = document.createElement("button");
    button.type = "button";
    button.dataset.nodeId = target.id;
    const title = document.createElement("span");
    title.textContent = target.title;
    const path = document.createElement("small");
    path.textContent = target.name;
    button.append(title, path);
    button.addEventListener("click", () => selectKnowledgeNode(target.id));
    button.addEventListener("dblclick", () => openGraphNode(target));
    container.append(button);
  }
  if (!container.children.length) {
    const empty = document.createElement("p");
    empty.textContent = localized(emptyLabel);
    container.append(empty);
  }
}

function renderGraphRelations(node) {
  const panel = $("#graph-relations");
  panel.hidden = false;
  $("#graph-relation-title").textContent = node.title;
  $("#graph-relations-close").setAttribute("aria-label", localized("收起关系"));
  $("#graph-forward-heading").textContent = `${localized("链接到")} · ${node.forwardLinks.length}`;
  $("#graph-backlink-heading").textContent = `${localized("被链接")} · ${node.backlinks.length}`;
  renderGraphRelationList($("#graph-forward-list"), node.forwardLinks, "没有正向链接");
  renderGraphRelationList($("#graph-backlink-list"), node.backlinks, "没有反向链接");
}

function updateGraphEmphasis() {
  const query = $("#graph-search").value.trim().toLocaleLowerCase();
  const selected = state.graph.nodes.find(node => node.id === state.selectedGraphNodeId);
  const forward = new Set(selected?.forwardLinks || []);
  const backlinks = new Set(selected?.backlinks || []);
  window.d3?.selectAll?.("#graph-svg .graph-node")
    .classed("is-selected", item => item.id === selected?.id)
    .classed("is-forward", item => forward.has(item.id))
    .classed("is-backlink", item => backlinks.has(item.id))
    .classed("is-mutual", item => forward.has(item.id) && backlinks.has(item.id))
    .classed("is-match", item => Boolean(query) && `${item.title} ${item.name}`.toLocaleLowerCase().includes(query))
    .classed("is-dimmed", item => {
      const queryMiss = Boolean(query) && !`${item.title} ${item.name}`.toLocaleLowerCase().includes(query);
      const relationMiss = Boolean(selected) && item.id !== selected.id && !forward.has(item.id) && !backlinks.has(item.id);
      return queryMiss || relationMiss;
    });
  window.d3?.selectAll?.("#graph-svg .graph-link")
    .classed("is-outgoing", item => graphNodeID(item.source) === selected?.id)
    .classed("is-incoming", item => graphNodeID(item.target) === selected?.id)
    .classed("is-mutual", item => Boolean(item.mutual))
    .classed("is-dimmed", item => Boolean(selected) && graphNodeID(item.source) !== selected.id && graphNodeID(item.target) !== selected.id);
}

function selectKnowledgeNode(id) {
  const node = state.graph.nodes.find(item => item.id === id);
  if (!node) return;
  state.selectedGraphNodeId = node.id;
  renderGraphRelations(node);
  updateGraphEmphasis();
}

function clearKnowledgeSelection() {
  state.selectedGraphNodeId = "";
  $("#graph-relations").hidden = true;
  updateGraphEmphasis();
}

function graphNodeRadius(node) {
  return Math.min(15, 6.5 + Math.sqrt(node.degree) * 2.2);
}

function handleGraphWheel(event) {
  if (!state.graphZoom || !$("#knowledge-graph").classList.contains("is-open")) return;
  const svgNode = $("#graph-svg");
  if (!svgNode || !window.d3) return;
  event.preventDefault();
  event.stopPropagation();
  // Match d3-zoom's default wheel delta across pixel, line, page, and pinch events.
  const delta = -event.deltaY
    * (event.deltaMode === 1 ? .05 : event.deltaMode ? 1 : .002)
    * (event.ctrlKey ? 10 : 1);
  state.graphZoom.scaleBy(
    window.d3.select(svgNode),
    Math.pow(2, delta),
    window.d3.pointer(event, svgNode),
    event
  );
}

function renderKnowledgeGraph(graph = state.graph) {
  state.graphSimulation?.stop();
  const svg = window.d3?.select?.("#graph-svg");
  if (!svg) {
    toast(locale() === "en" ? "Graph runtime failed to load" : "知识图谱运行时未加载");
    return;
  }
  svg.selectAll("*").remove();
  $("#graph-empty").hidden = graph.nodes.length > 0;
  updateGraphLabels();
  if (!graph.nodes.length) return;
  const bounds = $("#graph-canvas").getBoundingClientRect();
  const width = Math.max(420, bounds.width || 680);
  const height = Math.max(300, bounds.height || 480);
  const root = svg.append("g").attr("id", "graph-stage");
  svg.attr("viewBox", `0 0 ${width} ${height}`);
  const marker = svg.append("defs").append("marker")
    .attr("id", "graph-arrowhead").attr("viewBox", "0 -4 8 8").attr("refX", 7).attr("refY", 0)
    .attr("markerWidth", 6).attr("markerHeight", 6).attr("orient", "auto").attr("markerUnits", "userSpaceOnUse");
  marker.append("path").attr("d", "M0,-3.2L7,0L0,3.2Z").attr("fill", "context-stroke");
  const zoom = window.d3.zoom().scaleExtent([.35, 3.5]).on("zoom", event => {
    state.graphZoomScale = event.transform.k;
    root.attr("transform", event.transform);
    $("#graph-zoom").value = `${Math.round(event.transform.k * 100)}%`;
  });
  state.graphZoom = zoom;
  state.graphZoomScale = 1;
  $("#graph-zoom").value = "100%";
  // Keep D3 panning while the HTML canvas captures wheel input consistently across WKWebView versions.
  svg.call(zoom).on("wheel.zoom", null);
  const nodes = graph.nodes.map(node => ({ ...node }));
  const links = graph.edges.map(edge => ({ ...edge }));
  const current = activeDocument();
  const currentPath = current?.path || "";
  const link = root.append("g").selectAll("line").data(links).join("line").attr("class", "graph-link").attr("marker-end", "url(#graph-arrowhead)");
  const node = root.append("g").selectAll("g").data(nodes).join("g")
    .attr("class", item => `graph-node${item.path && item.path === currentPath ? " is-current" : ""}`)
    .attr("data-node-id", item => item.id)
    .attr("tabindex", "0").attr("role", "button")
    .attr("aria-label", item => item.title)
    .on("click", (_event, item) => selectKnowledgeNode(item.id))
    .on("dblclick", (event, item) => { event.stopPropagation(); openGraphNode(item); })
    .on("keydown", (event, item) => {
      if (event.key === "Enter") selectKnowledgeNode(item.id);
      if (event.key === " " && !event.repeat) { event.preventDefault(); openGraphNode(item); }
    });
  node.append("circle").attr("r", graphNodeRadius);
  node.append("text").attr("x", item => 11 + Math.min(8, Math.sqrt(item.degree) * 1.5)).attr("y", 3.5).text(item => item.title);
  const simulation = window.d3.forceSimulation(nodes)
    .force("link", window.d3.forceLink(links).id(item => item.id).distance(82).strength(.5))
    .force("charge", window.d3.forceManyBody().strength(-185))
    .force("center", window.d3.forceCenter(width / 2, height / 2))
    .force("collision", window.d3.forceCollide().radius(item => 26 + Math.min(20, item.title.length * 2)))
    .on("tick", () => {
      link
        .attr("x1", item => {
          const length = Math.hypot(item.target.x - item.source.x, item.target.y - item.source.y) || 1;
          return item.source.x + (item.target.x - item.source.x) * (graphNodeRadius(item.source) + 2) / length;
        })
        .attr("y1", item => {
          const length = Math.hypot(item.target.x - item.source.x, item.target.y - item.source.y) || 1;
          return item.source.y + (item.target.y - item.source.y) * (graphNodeRadius(item.source) + 2) / length;
        })
        .attr("x2", item => {
          const length = Math.hypot(item.target.x - item.source.x, item.target.y - item.source.y) || 1;
          return item.target.x - (item.target.x - item.source.x) * (graphNodeRadius(item.target) + 5) / length;
        })
        .attr("y2", item => {
          const length = Math.hypot(item.target.x - item.source.x, item.target.y - item.source.y) || 1;
          return item.target.y - (item.target.y - item.source.y) * (graphNodeRadius(item.target) + 5) / length;
        });
      node.attr("transform", item => `translate(${item.x},${item.y})`);
    });
  const drag = window.d3.drag()
    .on("start", (event, item) => { if (!event.active) simulation.alphaTarget(.2).restart(); item.fx = item.x; item.fy = item.y; })
    .on("drag", (event, item) => { item.fx = event.x; item.fy = event.y; })
    .on("end", (event, item) => { if (!event.active) simulation.alphaTarget(0); item.fx = null; item.fy = null; });
  node.call(drag);
  state.graphSimulation = simulation;
  state.selectedGraphNodeId = graph.nodes.some(item => item.id === state.selectedGraphNodeId) ? state.selectedGraphNodeId : "";
  if (state.selectedGraphNodeId) selectKnowledgeNode(state.selectedGraphNodeId);
  else clearKnowledgeSelection();
}

function filterKnowledgeGraph(value = "") {
  void value;
  updateGraphEmphasis();
}

async function refreshKnowledgeGraph() {
  $("#graph-stats").textContent = localized("正在读取工作区…");
  await refreshWorkspaceKnowledge({ renderGraph: true });
}

function toggleKnowledgeGraph(force) {
  const panel = $("#knowledge-graph");
  const open = typeof force === "boolean" ? force : !panel.classList.contains("is-open");
  panel.classList.toggle("is-open", open);
  panel.setAttribute("aria-hidden", String(!open));
  $("#graph-button").classList.toggle("is-active", open);
  if (open) {
    $("#graph-search").value = "";
    void refreshKnowledgeGraph();
  } else {
    state.graphSimulation?.stop();
  }
}

function mermaidTheme(theme, appearance = document.documentElement.dataset.appearance, colorTheme = "auto") {
  const lightPalettes = {
    "yuluo-css": { theme: "base", primaryColor: "#effaff", primaryTextColor: "#333333", primaryBorderColor: "#1a8f37", lineColor: "#4183c4", background: "#ffffff" },
    "lapis-cv": { theme: "base", primaryColor: "#eef3f9", primaryTextColor: "#353a42", primaryBorderColor: "#4870ad", lineColor: "#4870ad", background: "#ffffff" },
    github: { theme: "base", primaryColor: "#f6f8fa", primaryTextColor: "#1f2328", primaryBorderColor: "#8c959f", lineColor: "#59636e", background: "#ffffff" },
    whitey: { theme: "base", primaryColor: "#f5f5f3", primaryTextColor: "#2c2c2b", primaryBorderColor: "#b8b8b2", lineColor: "#74746f", background: "#ffffff" },
    newsprint: { theme: "neutral", primaryColor: "#eee9dd", primaryTextColor: "#191816", primaryBorderColor: "#8d877b", lineColor: "#5f5a52", background: "#f7f4ed" },
    pixyll: { theme: "base", primaryColor: "#f8eeea", primaryTextColor: "#333333", primaryBorderColor: "#d04f4a", lineColor: "#c14e4a", background: "#fffdf9" },
    gothic: { theme: "neutral", primaryColor: "#efede8", primaryTextColor: "#201f1d", primaryBorderColor: "#77716a", lineColor: "#55514d", background: "#f8f7f3" },
    night: { theme: "dark", primaryColor: "#292f36", primaryTextColor: "#e7ebef", primaryBorderColor: "#6f7b87", lineColor: "#9ca7b1", background: "#1f2328" }
  };
  const darkPalettes = {
    "yuluo-css": { theme: "dark", primaryColor: "#292e32", primaryTextColor: "#e7ecef", primaryBorderColor: "#58c978", lineColor: "#78b7e4", background: "#1f2224" },
    "lapis-cv": { theme: "dark", primaryColor: "#253142", primaryTextColor: "#e8edf4", primaryBorderColor: "#7ea2d8", lineColor: "#93b4e3", background: "#1f2732" },
    github: { theme: "dark", primaryColor: "#161b22", primaryTextColor: "#e6edf3", primaryBorderColor: "#6e7681", lineColor: "#8b949e", background: "#0d1117" },
    whitey: { theme: "dark", primaryColor: "#292a29", primaryTextColor: "#e5e5e0", primaryBorderColor: "#70716d", lineColor: "#a5a69f", background: "#1f2020" },
    newsprint: { theme: "dark", primaryColor: "#302d27", primaryTextColor: "#eee8da", primaryBorderColor: "#807768", lineColor: "#aaa193", background: "#24221e" },
    pixyll: { theme: "dark", primaryColor: "#302728", primaryTextColor: "#eee9e7", primaryBorderColor: "#b8625e", lineColor: "#e87872", background: "#211f20" },
    gothic: { theme: "dark", primaryColor: "#292927", primaryTextColor: "#f2f1ec", primaryBorderColor: "#85847f", lineColor: "#c2c1bc", background: "#1c1c1b" },
    night: { theme: "dark", primaryColor: "#22272e", primaryTextColor: "#f0f3f6", primaryBorderColor: "#6e7681", lineColor: "#9da7b1", background: "#171a1f" }
  };
  const colorPalettes = {
    ocean: {
      light: { theme: "base", primaryColor: "#e6f3f7", primaryTextColor: "#173b4a", primaryBorderColor: "#3f8ca5", lineColor: "#32728b", background: "#f8fcfd" },
      dark: { theme: "dark", primaryColor: "#18353f", primaryTextColor: "#e0f1f5", primaryBorderColor: "#58aac3", lineColor: "#80bfd1", background: "#14252c" }
    },
    forest: {
      light: { theme: "base", primaryColor: "#e9f2e8", primaryTextColor: "#263c2d", primaryBorderColor: "#5f8f67", lineColor: "#527a59", background: "#fbfdf9" },
      dark: { theme: "dark", primaryColor: "#24372a", primaryTextColor: "#e5f0e7", primaryBorderColor: "#70a778", lineColor: "#91ba97", background: "#1c2920" }
    },
    sunset: {
      light: { theme: "base", primaryColor: "#f8ebe3", primaryTextColor: "#4b3029", primaryBorderColor: "#c36f52", lineColor: "#ad6049", background: "#fffaf6" },
      dark: { theme: "dark", primaryColor: "#402b27", primaryTextColor: "#f7e8e1", primaryBorderColor: "#dc8366", lineColor: "#e5a087", background: "#2d211f" }
    },
    mono: {
      light: { theme: "neutral", primaryColor: "#f0f1f1", primaryTextColor: "#2e3133", primaryBorderColor: "#777d80", lineColor: "#62686b", background: "#fbfbfb" },
      dark: { theme: "dark", primaryColor: "#303335", primaryTextColor: "#eceeee", primaryBorderColor: "#8b9194", lineColor: "#a3a8aa", background: "#222527" }
    }
  };
  const normalizedTheme = normalizeMermaidColorTheme(colorTheme);
  if (normalizedTheme !== "auto") return colorPalettes[normalizedTheme][appearance === "dark" ? "dark" : "light"];
  const palettes = appearance === "dark" ? darkPalettes : lightPalettes;
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

function mermaidDiagramElements(root) {
  if (!root) return [];
  return [
    ...(root.matches?.(".mermaid-diagram") ? [root] : []),
    ...root.querySelectorAll(".mermaid-diagram")
  ];
}

function mermaidColorThemeName(value) {
  const names = locale() === "en"
    ? { auto: "Follow document", ocean: "Ocean", forest: "Forest", sunset: "Sunset", mono: "Monochrome" }
    : { auto: "跟随文档", ocean: "海洋", forest: "森林", sunset: "暖色", mono: "单色" };
  return names[normalizeMermaidColorTheme(value)];
}

function updateMermaidThemeControl(element) {
  const button = element.querySelector(".mermaid-theme-button");
  if (!button) return;
  const name = mermaidColorThemeName(element.dataset.mermaidTheme);
  const label = locale() === "en" ? `Diagram colors: ${name}` : `图表配色：${name}`;
  button.title = label;
  button.setAttribute("aria-label", label);
  button.dataset.mermaidTheme = normalizeMermaidColorTheme(element.dataset.mermaidTheme);
}

function updateMermaidLayoutControls(element) {
  const sourceButton = element.querySelector(".mermaid-source-toggle");
  const expandButton = element.querySelector(".mermaid-expand-button");
  const sourceCollapsed = element.classList.contains("is-source-collapsed");
  const workbenchExpanded = element.classList.contains("is-workbench-expanded");
  if (sourceButton) {
    const label = localized(sourceCollapsed ? "展开源码" : "收起源码");
    sourceButton.title = label;
    sourceButton.setAttribute("aria-label", label);
    sourceButton.setAttribute("aria-expanded", String(!sourceCollapsed));
    sourceButton.querySelector("use")?.setAttribute("href", sourceCollapsed ? "#i-arrow-right" : "#i-arrow-left");
  }
  if (expandButton) {
    const label = localized(workbenchExpanded ? "退出放大" : "放大编辑框");
    expandButton.title = label;
    expandButton.setAttribute("aria-label", label);
    expandButton.setAttribute("aria-pressed", String(workbenchExpanded));
    expandButton.querySelector("use")?.setAttribute("href", workbenchExpanded ? "#i-minimize-2" : "#i-maximize-2");
  }
}

function toggleMermaidSourcePane(element) {
  element.classList.toggle("is-source-collapsed");
  updateMermaidLayoutControls(element);
}

function toggleMermaidWorkbenchExpansion(element, force) {
  const shouldExpand = force ?? !element.classList.contains("is-workbench-expanded");
  if (shouldExpand && expandedMermaidDiagram && expandedMermaidDiagram !== element) {
    toggleMermaidWorkbenchExpansion(expandedMermaidDiagram, false);
  }
  if (shouldExpand) element.classList.remove("is-source-collapsed");
  element.classList.toggle("is-workbench-expanded", shouldExpand);
  expandedMermaidDiagram = shouldExpand ? element : (expandedMermaidDiagram === element ? null : expandedMermaidDiagram);
  document.body.classList.toggle("mermaid-workbench-open", Boolean(expandedMermaidDiagram));
  updateMermaidLayoutControls(element);
}

function closeExpandedMermaidWorkbench() {
  if (!expandedMermaidDiagram) return false;
  const element = expandedMermaidDiagram;
  toggleMermaidWorkbenchExpansion(element, false);
  element.querySelector(".mermaid-expand-button")?.focus();
  return true;
}

function updateMermaidWorkbenchLocale(root = write) {
  mermaidDiagramElements(root).forEach(element => {
    const input = element.querySelector(".mermaid-source-editor");
    if (input) input.setAttribute("aria-label", localized("Mermaid 源码"));
    const preview = element.querySelector(".mermaid-preview-pane");
    if (preview) preview.setAttribute("aria-label", localized("Mermaid 图表"));
    const heading = element.querySelector(".mermaid-preview-error strong");
    if (heading) heading.textContent = localized("Mermaid 无法渲染");
    updateMermaidThemeControl(element);
    updateMermaidLayoutControls(element);
  });
}

function nextMermaidRenderRequest(element) {
  const request = (mermaidRenderRequests.get(element) || 0) + 1;
  mermaidRenderRequests.set(element, request);
  return request;
}

function scheduleMermaidRender(element, delay = 180) {
  clearTimeout(mermaidInputTimers.get(element));
  const request = nextMermaidRenderRequest(element);
  element.dataset.mermaidState = element.dataset.mermaidSource?.trim() ? "rendering" : "empty";
  const timer = setTimeout(() => {
    mermaidInputTimers.delete(element);
    void renderMermaidDiagrams(element, state.documentTheme, document.documentElement.dataset.appearance, { interactive: true, requests: new Map([[element, request]]) });
  }, delay);
  mermaidInputTimers.set(element, timer);
}

function ensureMermaidWorkbench(element) {
  const existing = element.querySelector(".mermaid-workbench-grid");
  if (existing) {
    const input = existing.querySelector(".mermaid-source-editor");
    if (input && document.activeElement !== input && input.value !== (element.dataset.mermaidSource || "")) input.value = element.dataset.mermaidSource || "";
    updateMermaidWorkbenchLocale(element);
    return existing;
  }

  element.replaceChildren();
  const grid = document.createElement("div");
  grid.className = "mermaid-workbench-grid";
  const sourcePane = document.createElement("div");
  sourcePane.className = "mermaid-source-pane";
  const input = document.createElement("textarea");
  input.className = "mermaid-source-editor";
  input.value = element.dataset.mermaidSource || "";
  input.spellcheck = false;
  input.wrap = "off";
  input.setAttribute("aria-label", localized("Mermaid 源码"));
  input.addEventListener("beforeinput", event => event.stopPropagation());
  input.addEventListener("keydown", event => {
    event.stopPropagation();
    if (event.key !== "Tab" || event.metaKey || event.ctrlKey || event.altKey) return;
    event.preventDefault();
    const start = input.selectionStart;
    input.setRangeText("  ", start, input.selectionEnd, "end");
    input.dispatchEvent(new InputEvent("input", { bubbles: false, inputType: "insertText", data: "  " }));
  });
  input.addEventListener("input", event => {
    event.stopPropagation();
    element.dataset.mermaidSource = input.value;
    syncFromWrite();
    scheduleMermaidRender(element);
  });
  sourcePane.append(input);

  const preview = document.createElement("div");
  preview.className = "mermaid-preview-pane";
  preview.setAttribute("role", "region");
  preview.setAttribute("aria-label", localized("Mermaid 图表"));
  const expandButton = document.createElement("button");
  expandButton.type = "button";
  expandButton.className = "mermaid-expand-button";
  expandButton.innerHTML = '<svg aria-hidden="true"><use href="#i-maximize-2"/></svg>';
  expandButton.addEventListener("click", event => {
    event.stopPropagation();
    toggleMermaidWorkbenchExpansion(element);
  });
  const themeButton = document.createElement("button");
  themeButton.type = "button";
  themeButton.className = "mermaid-theme-button";
  themeButton.innerHTML = '<svg aria-hidden="true"><use href="#i-palette"/></svg>';
  themeButton.addEventListener("click", event => {
    event.stopPropagation();
    const current = normalizeMermaidColorTheme(element.dataset.mermaidTheme);
    element.dataset.mermaidTheme = mermaidColorThemes[(mermaidColorThemes.indexOf(current) + 1) % mermaidColorThemes.length];
    updateMermaidThemeControl(element);
    syncFromWrite();
    scheduleMermaidRender(element, 0);
  });
  const canvas = document.createElement("div");
  canvas.className = "mermaid-preview-canvas";
  const error = document.createElement("div");
  error.className = "mermaid-preview-error";
  error.hidden = true;
  error.setAttribute("role", "alert");
  const errorHeading = document.createElement("strong");
  errorHeading.textContent = localized("Mermaid 无法渲染");
  const errorDetail = document.createElement("small");
  error.append(errorHeading, errorDetail);
  preview.append(themeButton, canvas, error);
  grid.append(sourcePane, preview);
  const sourceButton = document.createElement("button");
  sourceButton.type = "button";
  sourceButton.className = "mermaid-source-toggle";
  sourceButton.innerHTML = '<svg aria-hidden="true"><use href="#i-arrow-left"/></svg>';
  sourceButton.addEventListener("click", event => {
    event.stopPropagation();
    toggleMermaidSourcePane(element);
  });
  element.append(grid, sourceButton, expandButton);
  updateMermaidThemeControl(element);
  updateMermaidLayoutControls(element);
  return grid;
}

function mermaidErrorMessage(error) {
  const message = error instanceof Error ? error.message : String(error || "Syntax error");
  return message.replace(/^Error:\s*/i, "").trim().slice(0, 500);
}

function showMermaidError(element, error, interactive) {
  element.dataset.mermaidState = "error";
  if (interactive) {
    ensureMermaidWorkbench(element);
    const canvas = element.querySelector(".mermaid-preview-canvas");
    const errorBox = element.querySelector(".mermaid-preview-error");
    canvas?.replaceChildren();
    if (errorBox) {
      errorBox.hidden = false;
      errorBox.querySelector("strong").textContent = localized("Mermaid 无法渲染");
      errorBox.querySelector("small").textContent = mermaidErrorMessage(error);
    }
    return;
  }
  element.replaceChildren();
  element.removeAttribute("data-mermaid-source");
  element.removeAttribute("data-mermaid-theme");
  element.removeAttribute("contenteditable");
  const heading = document.createElement("strong");
  heading.textContent = localized("Mermaid 无法渲染");
  const detail = document.createElement("small");
  detail.textContent = mermaidErrorMessage(error);
  element.append(heading, detail);
}

function renderMermaidDiagrams(root, theme = state.documentTheme, appearance = document.documentElement.dataset.appearance, options = {}) {
  const diagrams = mermaidDiagramElements(root);
  if (!diagrams.length) return Promise.resolve();
  const interactive = options.interactive ?? root === write;
  const requests = options.requests || new Map(diagrams.map(element => [element, nextMermaidRenderRequest(element)]));
  return enqueueMermaid(async () => {
    if (!window.mermaid?.initialize || !window.mermaid?.render) {
      diagrams.forEach(element => showMermaidError(element, localized("Mermaid 运行时未加载"), interactive));
      return;
    }
    for (const element of diagrams) {
      const source = element.dataset.mermaidSource || "";
      const colorTheme = normalizeMermaidColorTheme(element.dataset.mermaidTheme);
      const request = requests.get(element);
      if (interactive) ensureMermaidWorkbench(element);
      if (!source.trim()) {
        element.dataset.mermaidState = "empty";
        element.querySelector(".mermaid-preview-canvas")?.replaceChildren();
        const errorBox = element.querySelector(".mermaid-preview-error");
        if (errorBox) errorBox.hidden = true;
        continue;
      }
      element.dataset.mermaidState = "rendering";
      const palette = mermaidTheme(theme, appearance, colorTheme);
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
      try {
        const result = await window.mermaid.render(`mory-mermaid-${++mermaidSequence}`, source);
        if (interactive && (mermaidRenderRequests.get(element) !== request || element.dataset.mermaidSource !== source || normalizeMermaidColorTheme(element.dataset.mermaidTheme) !== colorTheme)) continue;
        const canvas = interactive ? element.querySelector(".mermaid-preview-canvas") : element;
        canvas.innerHTML = result.svg;
        const errorBox = element.querySelector(".mermaid-preview-error");
        if (errorBox) errorBox.hidden = true;
        element.dataset.mermaidState = "rendered";
        if (!interactive) {
          element.removeAttribute("data-mermaid-source");
          element.removeAttribute("data-mermaid-theme");
          element.removeAttribute("contenteditable");
        }
        result.bindFunctions?.(canvas);
      } catch (error) {
        if (!interactive || (mermaidRenderRequests.get(element) === request && element.dataset.mermaidSource === source)) showMermaidError(element, error, interactive);
      }
    }
  });
}

const calendarExportCSS = `
[data-calendar-color=red]{--calendar-color:#c9504b}[data-calendar-color=amber]{--calendar-color:#bd7b22}[data-calendar-color=green]{--calendar-color:#39825e}[data-calendar-color=blue]{--calendar-color:#397db5}[data-calendar-color=violet]{--calendar-color:#7659b4}[data-calendar-color=gray]{--calendar-color:#70777c}.calendar-block{margin:1.6em 0;border:1px solid rgba(127,127,127,.24);border-radius:6px;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;font-size:12px;line-height:1.35;break-inside:avoid}.calendar-block-header{min-height:50px;display:flex;align-items:center;padding:8px 14px;border-bottom:1px solid rgba(127,127,127,.2)}.calendar-block-header small{display:block;color:#397db5;font-size:7px;font-weight:700;letter-spacing:.16em}.calendar-block-header h3{margin:2px 0 0;font-size:16px}.calendar-block-weekdays,.calendar-block-days{display:grid;grid-template-columns:repeat(7,minmax(0,1fr))}.calendar-block-weekdays{height:25px;align-items:center;border-bottom:1px solid rgba(127,127,127,.2);opacity:.56;text-align:center;font-size:9px}.calendar-day-cell{position:relative;min-width:0;height:88px;padding:6px 0;border-right:1px solid rgba(127,127,127,.2);border-bottom:1px solid rgba(127,127,127,.2)}.calendar-day-cell:nth-child(7n){border-right:0}.calendar-day-cell:nth-last-child(-n+7){border-bottom:0}.calendar-day-cell.is-outside{background:rgba(127,127,127,.055);opacity:.55}.calendar-day-cell[data-marked=true]{box-shadow:inset 3px 0 var(--calendar-color)}.calendar-day-head{height:19px;display:flex;align-items:center;gap:4px;padding:0 7px}.calendar-date-number{font-size:9px}.calendar-mark-dot{width:6px;height:6px;display:block;border-radius:50%;background:var(--calendar-color)}.calendar-range-stack{display:grid;gap:2px;margin-top:3px}.calendar-range-bar{height:14px;display:block;margin:0 -1px;padding:0 5px;overflow:hidden;background:var(--calendar-color);color:#fff;font-size:8px;font-weight:650;line-height:14px;text-overflow:ellipsis;white-space:nowrap}.calendar-range-bar.is-segment-start{margin-left:4px;border-radius:3px 0 0 3px}.calendar-range-bar.is-segment-end{margin-right:4px;border-radius:0 3px 3px 0}.calendar-range-bar.is-segment-start.is-segment-end{border-radius:3px}.calendar-range-more{padding-left:7px;font-size:8px;opacity:.55}.calendar-mark-title{position:absolute;right:6px;bottom:7px;left:7px;overflow:hidden;color:var(--calendar-color);font-size:8px;font-weight:650;text-overflow:ellipsis;white-space:nowrap}.calendar-day-items{position:absolute;right:5px;bottom:4px}.calendar-day-items summary{padding:2px 5px;border:1px solid rgba(127,127,127,.25);border-radius:3px;font-size:8px;list-style:none}.calendar-day-items ul{display:none}.calendar-day-cell:has(.calendar-day-items) .calendar-mark-title{right:42px}
`;

const exportBaseCSS = `
*{box-sizing:border-box}html,body{margin:0;min-height:100%}body{background:#fff;color:#2c2c2b}.editor-scroll{min-height:100vh;padding:1px 0}.write{width:min(calc(100% - 72px),820px);margin:48px auto 72px;font-size:17px;line-height:1.8}.write h1,.write h2,.write h3,.write h4,.write h5,.write h6{margin:1.7em 0 .65em;line-height:1.35}.write h1{margin-top:.7em;padding-bottom:.28em;border-bottom:1px solid #ddd;font-size:2em}.write h2{padding-bottom:.24em;border-bottom:1px solid #e5e5e5;font-size:1.55em}.write h3{font-size:1.24em}.write p{margin:.75em 0}.write a{text-decoration:none}.write code{padding:.14em .35em;border-radius:3px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.84em}.write pre{position:relative;margin:1.1em 0;padding:16px 18px;overflow:auto;border-radius:4px;line-height:1.55}.write pre[data-title]:not([data-title=""]){padding-top:36px}.write pre[data-title]:not([data-title=""])::before{content:attr(data-title);position:absolute;top:8px;left:18px;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;font-size:10px;font-weight:600;opacity:.6}.write pre code{padding:0;background:transparent}.write .hljs-comment,.write .hljs-quote{color:#6a737d;font-style:italic}.write .hljs-keyword,.write .hljs-selector-tag,.write .hljs-type{color:#8250df;font-weight:600}.write .hljs-title,.write .hljs-section,.write .hljs-function{color:#0550ae}.write .hljs-string,.write .hljs-attr,.write .hljs-symbol{color:#0a3069}.write .hljs-number,.write .hljs-literal,.write .hljs-built_in{color:#953800}.write blockquote{margin:1.1em 0;padding:.1em 1.1em;border-left:3px solid}.write ul,.write ol{padding-left:1.6em}.write li{margin:.24em 0}.write hr{margin:2.2em 0;border:0;border-top:1px solid}.write table{width:100%;margin:1.2em 0;border-collapse:collapse;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;font-size:.88em}.write th,.write td{min-width:80px;padding:7px 10px;border:1px solid;text-align:left}.write img{max-width:100%}.write .task-item{list-style:none;margin-left:-1.4em}.write input[type=checkbox]{margin-right:.55em}.mermaid-diagram{margin:1.5em 0;padding:16px;overflow:auto;text-align:center}.mermaid-diagram svg{display:block;max-width:100%;height:auto;margin:auto}${calendarExportCSS}@page{margin:18mm 17mm}@media print{.editor-scroll{background:transparent!important}.write{width:auto;margin:0}.mermaid-diagram,.calendar-block{break-inside:avoid}}`;

async function readThemeCSS(theme) {
  if (state.themeCSS.has(theme)) return state.themeCSS.get(theme);
  const bundled = globalThis.__MORY_THEME_CSS__?.[theme];
  if (typeof bundled === "string") {
    state.themeCSS.set(theme, bundled);
    return bundled;
  }
  try {
    const response = await fetch(new URL(`themes/${theme}.css`, import.meta.url));
    if (!response.ok) throw new Error(String(response.status));
    const css = await response.text();
    state.themeCSS.set(theme, css);
    return css;
  } catch (error) {
    throw new Error(`无法读取主题 ${theme}：${error.message}`);
  }
}

async function bundledThemeAssetDataURL(filename) {
  if (bundledThemeAssetData.has(filename)) return bundledThemeAssetData.get(filename);
  const response = await fetch(new URL(`fonts/${filename}`, import.meta.url));
  if (!response.ok) throw new Error(String(response.status));
  const blob = await response.blob();
  // Windows WebView2 may expose local TTF files without a MIME type. Normalize
  // it so standalone exports have the same data URL on every desktop host.
  const exportBlob = blob.type === "font/ttf" ? blob : new Blob([blob], { type: "font/ttf" });
  const dataURL = await new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("load", () => resolve(String(reader.result || "")), { once: true });
    reader.addEventListener("error", () => reject(reader.error || new Error("Font asset could not be read")), { once: true });
    reader.readAsDataURL(exportBlob);
  });
  bundledThemeAssetData.set(filename, dataURL);
  return dataURL;
}

async function exportThemeCSS(theme, { inlineAssets = true } = {}) {
  let css = await readThemeCSS(theme);
  if (!inlineAssets) return css;
  for (const filename of bundledThemeAssets[theme] || []) {
    const dataURL = await bundledThemeAssetDataURL(filename);
    css = css.replaceAll(`../fonts/${filename}`, dataURL);
  }
  return css;
}

function syncThemeOptions() {
  for (const select of [$("#document-theme-select"), $("#export-theme")]) {
    select.querySelector('optgroup[data-custom-themes="true"]')?.remove();
    if (!state.customThemes.length) continue;
    const group = document.createElement("optgroup");
    group.dataset.customThemes = "true";
    group.label = localized("用户主题");
    for (const theme of state.customThemes) {
      const option = document.createElement("option");
      option.value = theme.id;
      option.textContent = theme.name;
      group.append(option);
    }
    select.append(group);
  }
  $("#custom-theme-summary").textContent = state.customThemes.length
    ? (locale() === "en" ? `${state.customThemes.length} custom theme(s) installed` : `已安装 ${state.customThemes.length} 个用户主题`)
    : localized("导入 CSS，或把主题与资源放入主题目录");
}

function registerCustomThemes(themes = []) {
  for (const theme of state.customThemes) state.themeCSS.delete(theme.id);
  state.customThemes = Array.isArray(themes) ? themes.filter(theme => theme && typeof theme.id === "string" && typeof theme.css === "string") : [];
  for (const theme of state.customThemes) state.themeCSS.set(theme.id, theme.css);
  syncThemeOptions();
  const selected = localStorage.getItem("mory.documentTheme") || state.documentTheme;
  if (state.customThemes.some(theme => theme.id === selected)) setDocumentTheme(selected);
  else if (!builtInThemes.includes(state.documentTheme)) setDocumentTheme("github");
}

async function refreshCustomThemes(announce = false) {
  try {
    const result = await hostRequest("listThemes");
    registerCustomThemes(result);
    if (announce) toast(locale() === "en" ? "Custom themes refreshed" : "用户主题已刷新");
  } catch {
    registerCustomThemes([]);
  }
}

function fontAvailable(family) {
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (!context) return false;
  const sample = "mmmmmmmmmmlli汉字 0123456789";
  return ["monospace", "serif", "sans-serif"].some(fallback => {
    context.font = `72px ${fallback}`;
    const baseline = context.measureText(sample).width;
    context.font = `72px "${family.replaceAll('"', '')}", ${fallback}`;
    return Math.abs(context.measureText(sample).width - baseline) > .1;
  });
}

async function updateThemeFontWarning({ announce = false } = {}) {
  const warning = $("#theme-font-warning");
  const theme = state.documentTheme;
  const fonts = bundledThemeFonts[theme] || [];
  const loaded = await Promise.all(fonts.map(async ([family, sample, weight]) => {
    await document.fonts.load(`${weight} 16px "${family}"`, sample);
    return document.fonts.check(`${weight} 16px "${family}"`, sample);
  })).catch(() => fonts.map(() => false));
  if (theme !== state.documentTheme) return false;
  const available = loaded.every(Boolean);
  const message = fonts.length && !available ? localized("内置主题字体加载失败，请重新启动 Mory 后再试。") : "";
  warning.hidden = !message;
  warning.textContent = message;
  if (theme === "yuluo-css") document.documentElement.dataset.yuluoFont = available ? "bundled" : "fallback";
  if (theme === "lapis-cv") document.documentElement.dataset.lapisCvFont = available ? "bundled" : "fallback";
  if (message && announce) toast(message, 5200);
  return available;
}

function setDocumentTheme(theme, { announceFontWarning = false } = {}) {
  const custom = state.customThemes.find(item => item.id === theme);
  const next = builtInThemes.includes(theme) || custom ? theme : "github";
  state.documentTheme = next;
  document.documentElement.dataset.docTheme = next;
  $("#document-theme").disabled = Boolean(custom);
  if (!custom) $("#document-theme").href = `themes/${next}.css`;
  $("#user-document-theme").textContent = custom?.css || "";
  $("#document-theme-select").value = next;
  $("#resume-template-button").hidden = next !== "lapis-cv";
  localStorage.setItem("mory.documentTheme", next);
  requestAnimationFrame(applyEditorZoom);
  readThemeCSS(next);
  void updateThemeFontWarning({ announce: announceFontWarning });
  void renderMermaidDiagrams(write, next);
}

function applyEditorZoom() {
  write.style.removeProperty("font-size");
  sourceEditor.style.removeProperty("font-size");
  if (state.zoom !== 1) {
    const writeBase = Number.parseFloat(getComputedStyle(write).fontSize) || 17;
    const sourceBase = Number.parseFloat(getComputedStyle(sourceEditor).fontSize) || 14;
    write.style.fontSize = `${writeBase * state.zoom}px`;
    sourceEditor.style.fontSize = `${sourceBase * state.zoom}px`;
  }
  document.documentElement.style.setProperty("--interface-scale", String(state.zoom));
  document.documentElement.style.setProperty("--status-font-size", `${12 * state.zoom}px`);
  document.documentElement.style.setProperty("--status-height", `${18 + 13 * state.zoom}px`);
}

window.addEventListener("resize", () => {
  cancelAnimationFrame(viewportTypographyFrame);
  viewportTypographyFrame = requestAnimationFrame(() => {
    viewportTypographyFrame = 0;
    applyEditorZoom();
  });
});

async function exportDocument(options = {}) {
  const title = $("#document-title").value || localized("未命名");
  if (options.format === "mindmap") return mindMapHTML(state.markdown, title, locale());
  const theme = options.theme && options.theme !== "current" ? options.theme : state.documentTheme;
  const themeCSS = await exportThemeCSS(theme, { inlineAssets: options.inlineThemeAssets !== false });
  const backgroundOverride = options.background === false ? ".editor-scroll{background:#fff!important}" : "";
  const exportRoot = document.createElement("article");
  exportRoot.className = "write";
  exportRoot.innerHTML = markdownToHTML(state.markdown);
  enhanceRawHTML(exportRoot, { interactive: false });
  enhanceCalendars(exportRoot, { interactive: false });
  applyDocumentAssets(exportRoot);
  highlightCodeBlocks(exportRoot, true);
  await renderMermaidDiagrams(exportRoot, theme, theme === "night" ? "dark" : "light");
  return `<!doctype html>\n<html lang="${locale()}" data-doc-theme="${escapeHTML(theme)}" data-export="true"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>${escapeHTML(title)}</title><style>${exportBaseCSS}\n${themeCSS}\n${backgroundOverride}</style></head><body><main class="editor-scroll"><article id="write" class="write">${exportRoot.innerHTML}</article></main></body></html>`;
}

async function exportToHost(options = {}) {
  try {
    bridge({ type: "export", options: await hostExportOptions(options) });
  } catch (error) {
    bridge({ type: "exportFailed", error: error instanceof Error ? error.message : String(error) });
  }
}

async function hostExportOptions(options) {
  const payload = { ...options, name: $("#document-title").value || "未命名" };
  if (options.format === "pptx") return { ...payload, markdown: state.markdown };
  if (nativeMacHost) return payload;
  return { ...payload, html: await exportDocument(options) };
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
  $("#export-theme-setting").hidden = format === "pptx";
  $("#export-background-setting").hidden = format === "pptx";
  $("#export-hint").textContent = format === "pptx"
    ? localized("PPTX 由官方 Slidev 生成，并保留演讲者备注")
    : format === "mindmap"
    ? (locale() === "en" ? "The mind map follows the document heading hierarchy" : "思维导图按文稿标题层级生成")
    : (["png", "jpeg"].includes(format) ? "长文档将在独立离屏页面中渲染" : localized("HTML、PDF 不需要 Pandoc"));
}

async function confirmExport() {
  const options = {
    format: $("#export-format").value,
    theme: $("#export-theme").value,
    paper: $("#export-paper").value,
    width: Number($("#export-width").value),
    background: $("#export-background").checked
  };
  if (window.webkit?.messageHandlers?.mory || window.moryNative || nativeWailsHost()) {
    // Wails cannot synchronously retrieve a JavaScript return value like Electron can.
    // Build the complete host payload before handing it to the native export implementation.
    bridge({ type: "export", options: await hostExportOptions(options) });
    toggleExportDialog(false);
    return;
  }
  if (options.format === "pptx") {
    toast(localized("PPTX 导出需要桌面版与 Slidev"));
    toggleExportDialog(false);
    return;
  }
  const html = await exportDocument(options);
  if (["html", "mindmap"].includes(options.format)) {
    const link = document.createElement("a");
    link.href = URL.createObjectURL(new Blob([html], { type: "text/html;charset=utf-8" }));
    const suffix = options.format === "mindmap" ? "-mind-map" : "";
    link.download = `${$("#document-title").value || localized("未命名")}${suffix}.html`;
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

function extractContentAfterCaret(block) {
  const selection = window.getSelection();
  if (!selection?.isCollapsed || !selection.rangeCount || !block.contains(selection.anchorNode)) return null;
  const range = document.createRange();
  range.selectNodeContents(block);
  try { range.setStart(selection.anchorNode, selection.anchorOffset); }
  catch { return null; }
  return range.extractContents();
}

function focusBlockStart(block) {
  write.focus({ preventScroll: true });
  const range = document.createRange();
  range.selectNodeContents(block);
  range.collapse(true);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
}

function appendEditableContent(block, fragment) {
  if (fragment?.childNodes.length) block.append(fragment);
  if (!block.childNodes.length) block.append(document.createElement("br"));
}

function splitHeadingAtCaret(block) {
  const tail = extractContentAfterCaret(block);
  if (!tail) return false;
  const paragraph = document.createElement("p");
  appendEditableContent(paragraph, tail);
  if (!block.childNodes.length) block.append(document.createElement("br"));
  block.after(paragraph);
  focusBlockStart(paragraph);
  syncFromWrite();
  updateFocusLine();
  return true;
}

function exitEmptyQuoteOrSplit(block) {
  beginEditorHistory("quote-enter", { force: true });
  if (!(block.textContent || "").replaceAll(caretMarker, "").trim()) {
    const paragraph = document.createElement("p");
    paragraph.append(document.createElement("br"));
    block.replaceWith(paragraph);
    focusBlockStart(paragraph);
  } else {
    const tail = extractContentAfterCaret(block);
    if (!tail) return false;
    const quote = document.createElement("blockquote");
    appendEditableContent(quote, tail);
    if (!block.childNodes.length) block.append(document.createElement("br"));
    block.after(quote);
    focusBlockStart(quote);
  }
  syncFromWrite();
  updateFocusLine();
  return true;
}

function paragraphizeHeading(block) {
  beginEditorHistory("heading-backspace", { force: true });
  const paragraph = document.createElement("p");
  while (block.firstChild) paragraph.append(block.firstChild);
  appendEditableContent(paragraph);
  block.replaceWith(paragraph);
  focusBlockStart(paragraph);
  syncFromWrite();
  updateFocusLine();
}

function handleEditorShortcut(event) {
  if (event.isComposing || event.keyCode === 229) return;
  if (handlePathSuggestionKey(event)) return;
  if (deleteTableBeforeCaret(event)) return;
  if (event.key !== "Enter") recentCompositionCommit = null;
  const command = event.metaKey || event.ctrlKey;
  if (command && event.key.toLowerCase() === "z") {
    event.preventDefault();
    if (event.shiftKey) redoEditor();
    else undoEditor();
    return;
  }
  if (command && event.key.toLowerCase() === "y") {
    event.preventDefault();
    redoEditor();
    return;
  }
  const tableCell = selectedTableCell(write);
  const selectedTable = tableCell?.closest("table");
  if (!command && event.key === "Tab" && selectedTable && tableCell) {
    event.preventDefault();
    const cells = [...selectedTable.querySelectorAll("th, td")];
    const index = cells.indexOf(tableCell);
    if (!event.shiftKey && index === cells.length - 1) {
      addTableRow(selectedTable);
    } else {
      focusTableCell(cells[Math.max(0, Math.min(cells.length - 1, index + (event.shiftKey ? -1 : 1)))]);
    }
    return;
  }
  if (command && event.shiftKey && event.key === "Backspace" && selectedTable) {
    event.preventDefault();
    deleteTableRow(selectedTable);
    return;
  }
  if (command && !event.shiftKey && event.key === "Enter" && selectedTable) {
    event.preventDefault();
    addTableRow(selectedTable);
    return;
  }
  if (command && event.key.toLowerCase() === "b") { event.preventDefault(); execute("bold"); }
  if (command && event.key.toLowerCase() === "i") { event.preventDefault(); execute("italic"); }
  if (command && event.key.toLowerCase() === "k") { event.preventDefault(); execute("link"); }
  if (command && event.key === "/") { event.preventDefault(); toggleSource(); }

  if (!command && event.key === "Backspace" && !state.sourceMode) {
    const block = currentWriteBlock();
    if (block?.matches("h1, h2, h3, h4, h5, h6") && selectionAtStart(block)) {
      event.preventDefault();
      paragraphizeHeading(block);
      return;
    }
  }

  if (!command && event.key === "Enter" && !state.sourceMode && rawTableCells(currentWriteBlock())) {
    event.preventDefault();
    if (renderRawTableAtCaret({ allowHeaderOnly: true })) {
      syncFromWrite();
      updateFocusLine();
    }
    return;
  }

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
      beginEditorHistory("heading-enter", { force: true });
      recentCompositionCommit = null;
      const template = document.createElement("template");
      template.innerHTML = markdownToHTML(block.textContent || "");
      const heading = template.content.firstElementChild;
      if (heading?.matches("h1, h2, h3, h4, h5, h6")) {
        exitHeadingToParagraph(block, heading);
        return;
      }
    }
    if (block.matches("blockquote")) {
      event.preventDefault();
      exitEmptyQuoteOrSplit(block);
      return;
    }
    const fence = atBlockEnd ? block.textContent?.match(/^(```|~~~)\s*(.*?)\s*$/) : null;
    if (fence && block.matches("p, div")) {
      event.preventDefault();
      const template = document.createElement("template");
      template.innerHTML = markdownToHTML(`${block.textContent}\n${fence[1]}`);
      const mermaid = template.content.querySelector(".mermaid-diagram");
      if (mermaid) {
        const paragraph = document.createElement("p");
        paragraph.append(document.createElement("br"));
        block.replaceWith(mermaid, paragraph);
        const workbench = ensureMermaidWorkbench(mermaid);
        workbench.querySelector(".mermaid-source-editor")?.focus();
        syncFromWrite();
        updateFocusLine();
        return;
      }
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
    if (/^H[1-6]$/.test(block.tagName)) {
      event.preventDefault();
      recentCompositionCommit = null;
      beginEditorHistory("heading-enter", { force: true });
      if (atBlockEnd) exitHeadingToParagraph(block);
      else splitHeadingAtCaret(block);
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
      beginEditorHistory("markdown-prefix", { force: true });
      const suffix = extractContentAfterCaret(block);
      const next = document.createElement(`h${heading[1].length}`);
      appendEditableContent(next, suffix);
      block.replaceWith(next);
      focusBlockStart(next);
      syncFromWrite();
      updateFocusLine();
    } else if (marker === ">") {
      event.preventDefault();
      beginEditorHistory("markdown-prefix", { force: true });
      const suffix = extractContentAfterCaret(block);
      const quote = document.createElement("blockquote");
      appendEditableContent(quote, suffix);
      block.replaceWith(quote);
      focusBlockStart(quote);
      syncFromWrite();
      updateFocusLine();
    } else if (/^(?:[-*+]|\d+[.)])$/.test(marker)) {
      event.preventDefault();
      beginEditorHistory("markdown-prefix", { force: true });
      const suffix = extractContentAfterCaret(block);
      const list = document.createElement(/^\d/.test(marker) ? "ol" : "ul");
      const item = document.createElement("li");
      appendEditableContent(item, suffix);
      list.append(item);
      block.replaceWith(list);
      focusBlockStart(item);
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

function fileAsBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error || new Error("无法读取图片。"));
    reader.onload = () => resolve(String(reader.result || "").split(",", 2)[1] || "");
    reader.readAsDataURL(file);
  });
}

async function importImages(files) {
  const activeDoc = activeDocument();
  if (!activeDoc) return;
  beginEditorHistory("import-image", { force: true });
  const selection = window.getSelection();
  const savedRange = selection?.rangeCount ? selection.getRangeAt(0).cloneRange() : null;
  const markdown = [];
  for (const file of files) {
    const result = await hostRequest("importImage", {
      documentPath: activeDoc.path || "",
      documentName: documentHostName(activeDoc),
      name: file.name || "图片",
      mime: file.type,
      data: await fileAsBase64(file)
    });
    activeDoc.assets ||= {};
    activeDoc.assets[result.relative] = result.dataURL;
    const alt = (file.name || "图片").replace(/\.[^.]+$/, "").replaceAll("]", "");
    markdown.push(`![${alt}](${result.relative})`);
  }
  if (savedRange && write.contains(savedRange.commonAncestorContainer)) {
    selection?.removeAllRanges();
    selection?.addRange(savedRange);
  }
  document.execCommand("insertText", false, markdown.join("\n\n"));
  renderMarkdownDocumentAtCaret();
  syncFromWrite();
  toast(`已归档 ${files.length} 张图片`);
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
  if (!event.isComposing && !event.inputType.startsWith("history")) {
    const coalesced = ["insertText", "deleteContentBackward", "deleteContentForward"].includes(event.inputType);
    beginEditorHistory(coalesced ? event.inputType : event.inputType || "input", { force: !coalesced });
  }
  if (state.sourceMode || event.isComposing || !["insertParagraph", "insertLineBreak"].includes(event.inputType)) return;
  const block = currentWriteBlock();
  if (rawTableCells(block)) {
    event.preventDefault();
    if (renderRawTableAtCaret({ allowHeaderOnly: true })) {
      syncFromWrite();
      updateFocusLine();
    }
    return;
  }
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
write.addEventListener("click", handleEditorLinkClick);
write.addEventListener("keydown", handleEditorShortcut);
write.addEventListener("change", event => {
  if (event.target.matches('input[type="checkbox"]')) syncFromWrite();
});
write.addEventListener("paste", event => {
  const images = [...event.clipboardData.files].filter(file => file.type.startsWith("image/"));
  if (images.length) {
    event.preventDefault();
    void importImages(images).catch(error => toast(`图片导入失败：${error.message}`));
    return;
  }
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
write.addEventListener("dragover", event => {
  if ([...event.dataTransfer.items].some(item => item.kind === "file" && item.type.startsWith("image/"))) event.preventDefault();
});
write.addEventListener("drop", event => {
  const images = [...event.dataTransfer.files].filter(file => file.type.startsWith("image/"));
  if (!images.length) return;
  event.preventDefault();
  write.focus();
  void importImages(images).catch(error => toast(`图片导入失败：${error.message}`));
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
    const converted = renderMarkdownBlockAtCaret();
    const normalized = normalizeInactiveRawHeadings(currentWriteBlock());
    if (converted || normalized) syncFromWrite();
  });
});
document.addEventListener("selectionchange", () => {
  if (document.activeElement !== write) return;
  const block = currentWriteBlock();
  if (block?.matches("pre[data-highlighted='true']")) clearCodeHighlight(block);
  highlightCodeBlocks(write);
  if ($("#path-suggestions").classList.contains("is-open")) updatePathSuggestions();
});
sourceEditor.addEventListener("beforeinput", event => {
  if (event.isComposing || event.inputType.startsWith("history")) return;
  const coalesced = ["insertText", "deleteContentBackward", "deleteContentForward"].includes(event.inputType);
  beginEditorHistory(coalesced ? `source-${event.inputType}` : `source-${event.inputType || "input"}`, { force: !coalesced });
});
sourceEditor.addEventListener("input", () => syncFromSource(false));
sourceEditor.addEventListener("keydown", handleEditorShortcut);

function handleWindowDragStart(event) {
  if (!nativeMacHost || event.button !== 0 || event.target.closest("button, input, select, a, [contenteditable='true']")) return;
  event.preventDefault();
  windowDragPointer = event.pointerId;
  event.currentTarget.setPointerCapture?.(event.pointerId);
  bridge({ type: "windowDragStart", screenX: event.screenX, screenY: event.screenY });
}

function handleWindowTitlebarDoubleClick(event) {
  if (!nativeMacHost || event.button !== 0 || event.target.closest("button, input, select, a, [contenteditable='true']")) return;
  event.preventDefault();
  bridge({ type: "windowTitlebarDoubleClick" });
}

// Apply identical drag and double-click zoom behavior to both native macOS titlebar regions.
for (const region of $$(".titlebar, .traffic-space")) {
  region.addEventListener("pointerdown", handleWindowDragStart);
  region.addEventListener("dblclick", handleWindowTitlebarDoubleClick);
}
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
$("#graph-button").addEventListener("click", () => toggleKnowledgeGraph());
$("#graph-canvas").addEventListener("wheel", handleGraphWheel, { passive: false });
$("#graph-close").addEventListener("click", () => toggleKnowledgeGraph(false));
$("#graph-relations-close").addEventListener("click", clearKnowledgeSelection);
$("#graph-refresh").addEventListener("click", () => void refreshKnowledgeGraph());
$("#graph-search").addEventListener("input", event => filterKnowledgeGraph(event.target.value));
$("#sidebar-toggle").addEventListener("click", () => $("#sidebar").classList.toggle("is-hidden"));
$("#new-file-button").addEventListener("click", () => void createDocumentInSelectedDirectory());
$("#new-folder-button").addEventListener("click", () => toggleNewFolderForm());
$("#new-folder-cancel").addEventListener("click", () => toggleNewFolderForm(false));
$("#new-folder-form").addEventListener("submit", event => {
  event.preventDefault();
  const value = $("#new-folder-input").value.trim();
  const parent = selectedDirectory()?.name || "";
  const relativePath = parent && value && !value.replaceAll("\\", "/").startsWith(`${parent}/`) ? `${parent}/${value}` : value;
  if (relativePath) void createWorkspaceFolder(relativePath);
});
$("#new-folder-input").addEventListener("keydown", event => {
  if (event.key === "Escape") { event.preventDefault(); toggleNewFolderForm(false); }
});
$("#file-context-menu").addEventListener("click", event => {
  const action = event.target.closest("button[data-entry-action]")?.dataset.entryAction;
  if (action) void handleFileContextAction(action);
});
$("#entry-operation-close").addEventListener("click", () => toggleEntryOperation(false));
$("#entry-operation-cancel").addEventListener("click", () => toggleEntryOperation(false));
$("#entry-operation-confirm").addEventListener("click", () => void confirmEntryOperation());
$("#entry-operation-name").addEventListener("keydown", event => {
  if (event.key === "Enter") { event.preventDefault(); void confirmEntryOperation(); }
  if (event.key === "Escape") { event.preventDefault(); toggleEntryOperation(false); }
});
$("#entry-operation-dialog").addEventListener("mousedown", event => {
  if (event.target === $("#entry-operation-dialog")) toggleEntryOperation(false);
});
$("#file-list").addEventListener("scroll", closeFileContextMenu);
$("#files-panel").addEventListener("click", event => {
  if (event.target !== $("#files-panel") && event.target !== $("#file-list")) return;
  state.selectedWorkspaceEntry = null;
  renderFiles();
});
$("#image-preview-close").addEventListener("click", closeImagePreview);
$("#image-preview").addEventListener("mousedown", event => {
  if (event.target === $("#image-preview")) closeImagePreview();
});
$("#settings-button").addEventListener("click", () => togglePreferences(true));
$("#workspace-button").addEventListener("click", () => { togglePreferences(true); showWorkspaceForm(activeWorkspace()); });
$("#workspace-add").addEventListener("click", () => showWorkspaceForm());
$("#workspace-form-close").addEventListener("click", hideWorkspaceForm);
$("#workspace-provider").addEventListener("change", event => renderWorkspaceFields(event.target.value));
$("#workspace-select").addEventListener("change", event => void switchWorkspace(event.target.value));
$("#workspace-pull").addEventListener("click", () => void syncWorkspace("pull"));
$("#workspace-push").addEventListener("click", () => void syncWorkspace("push"));
$("#workspace-open-local").addEventListener("click", async () => {
  try {
    const current = activeWorkspace();
    const result = await hostRequest("chooseLocalWorkspace", {
      id: current?.provider === "local" ? current.id : undefined,
      name: current?.provider === "local" ? current.name : undefined
    });
    if (!result?.canceled) { setWorkspaceState(result); hideWorkspaceForm(); toast("已设置本地工作目录"); }
  } catch (error) { toast(error.message); }
});
$("#workspace-form").addEventListener("submit", async event => {
  event.preventDefault();
  const workspaceValue = collectWorkspaceForm();
  try {
    const result = workspaceValue.provider === "local" && !workspaceValue.localPath
      ? await hostRequest("chooseLocalWorkspace", { id: workspaceValue.id, name: workspaceValue.name })
      : await hostRequest("saveWorkspace", { workspace: workspaceValue });
    if (!result?.canceled) { setWorkspaceState(result); hideWorkspaceForm(); toast("工作区配置已保存"); }
  } catch (error) { toast(`保存失败：${error.message}`); }
});
$("#workspace-remove").addEventListener("click", async () => {
  if (!state.editingWorkspaceId || !confirm("确定删除这个工作区配置吗？本地文件不会被删除。")) return;
  try {
    const result = await hostRequest("removeWorkspace", { id: state.editingWorkspaceId });
    setWorkspaceState(result);
    hideWorkspaceForm();
    toast("工作区配置已删除");
  } catch (error) { toast(error.message); }
});
$("#export-button").addEventListener("click", () => toggleExportDialog(true));
$("#export-close").addEventListener("click", () => toggleExportDialog(false));
$("#export-dialog").addEventListener("mousedown", event => { if (event.target === $("#export-dialog")) toggleExportDialog(false); });
$("#export-format").addEventListener("change", syncExportOptions);
$("#export-confirm").addEventListener("click", confirmExport);
initializeCalendarColors($("#calendar-mark-colors"), "mark");
initializeCalendarColors($("#calendar-range-colors"), "range");
$$('[data-calendar-mode]').forEach(button => button.addEventListener("click", () => {
  if (!calendarEditor) return;
  calendarEditor.mode = button.dataset.calendarMode;
  renderCalendarEditor();
}));
$("#calendar-mark-title").addEventListener("input", event => { if (calendarEditor) calendarEditor.markTitle = event.target.value; });
$("#calendar-range-title").addEventListener("input", event => { if (calendarEditor) calendarEditor.rangeTitle = event.target.value; });
$("#calendar-save-mark").addEventListener("click", () => {
  if (!calendarEditor) return;
  const marks = calendarEditor.calendar.marks.filter(mark => mark.date !== calendarEditor.activeDate);
  marks.push({ date: calendarEditor.activeDate, color: calendarEditor.markColor, title: calendarEditor.markTitle });
  calendarEditor.calendar = normalizeCalendarDocument({ ...calendarEditor.calendar, marks });
  setCalendarActiveDate(calendarEditor.activeDate);
  renderCalendarEditor();
});
$("#calendar-remove-mark").addEventListener("click", () => {
  if (!calendarEditor) return;
  calendarEditor.calendar.marks = calendarEditor.calendar.marks.filter(mark => mark.date !== calendarEditor.activeDate);
  setCalendarActiveDate(calendarEditor.activeDate);
  renderCalendarEditor();
});
$("#calendar-reset-range").addEventListener("click", () => { resetCalendarRangeSelection(); renderCalendarEditor(); });
$("#calendar-save-range").addEventListener("click", () => {
  if (!calendarEditor) return;
  const title = calendarEditor.rangeTitle.replace(/\s+/g, " ").trim();
  if (!calendarEditor.rangeStart || !calendarEditor.rangeEnd || !title) {
    toast(locale() === "en" ? "Select both dates and enter a title" : "请选择起止日期并填写标题");
    return;
  }
  const [start, end] = [calendarEditor.rangeStart, calendarEditor.rangeEnd].sort();
  const range = { start, end, color: calendarEditor.rangeColor, title };
  if (calendarEditor.editingRange >= 0) calendarEditor.calendar.ranges.splice(calendarEditor.editingRange, 1, range);
  else calendarEditor.calendar.ranges.push(range);
  calendarEditor.calendar = normalizeCalendarDocument(calendarEditor.calendar);
  resetCalendarRangeSelection();
  renderCalendarEditor();
});
$("#calendar-item-form").addEventListener("submit", event => {
  event.preventDefault();
  if (!calendarEditor) return;
  const input = $("#calendar-item-text");
  const text = input.value.replace(/\s+/g, " ").trim();
  if (!text) return;
  calendarEditor.calendar.items.push({ date: calendarEditor.activeDate, text, done: false });
  calendarEditor.calendar = normalizeCalendarDocument(calendarEditor.calendar);
  input.value = "";
  renderCalendarEditor();
  input.focus();
});
$("#calendar-previous-month").addEventListener("click", () => {
  if (!calendarEditor) return;
  calendarEditor.calendar.month = offsetCalendarMonth(calendarEditor.calendar.month, -1);
  setCalendarActiveDate(`${calendarEditor.calendar.month}-01`);
  renderCalendarEditor();
});
$("#calendar-next-month").addEventListener("click", () => {
  if (!calendarEditor) return;
  calendarEditor.calendar.month = offsetCalendarMonth(calendarEditor.calendar.month, 1);
  setCalendarActiveDate(`${calendarEditor.calendar.month}-01`);
  renderCalendarEditor();
});
$("#calendar-today").addEventListener("click", () => {
  if (!calendarEditor) return;
  const today = localDateKey();
  calendarEditor.calendar.month = today.slice(0, 7);
  setCalendarActiveDate(today);
  renderCalendarEditor();
});
$("#calendar-close").addEventListener("click", closeCalendarEditor);
$("#calendar-cancel").addEventListener("click", closeCalendarEditor);
$("#calendar-apply").addEventListener("click", applyCalendarEditor);
$("#calendar-delete").addEventListener("click", () => {
  if (confirm(locale() === "en" ? "Delete this calendar?" : "确定删除这个日历吗？")) deleteCalendarEditorBlock();
});
$("#calendar-dialog").addEventListener("mousedown", event => { if (event.target === $("#calendar-dialog")) closeCalendarEditor(); });
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
$("#backlink-count").addEventListener("click", () => {
  const panel = $("#document-backlinks");
  if (panel.hidden) {
    toast(locale() === "en" ? "No backlinks for this note" : "当前文稿没有反向链接");
    return;
  }
  panel.scrollIntoView({ behavior: "smooth", block: "center" });
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
  applyAppearanceTheme(event.target.value);
});
$("#document-theme-select").addEventListener("change", event => setDocumentTheme(event.target.value, { announceFontWarning: true }));
$("#resume-template-button").addEventListener("click", createResumeFromTemplate);
$("#document-theme").addEventListener("load", () => {
  applyEditorZoom();
  void updateThemeFontWarning();
});
$("#theme-import").addEventListener("click", async () => {
  try {
    const result = await hostRequest("importTheme");
    if (!result?.canceled) {
      registerCustomThemes(result.themes || []);
      toast(locale() === "en" ? "Theme imported" : "用户主题已导入");
    }
  } catch (error) { toast(error.message); }
});
$("#theme-choose-folder").addEventListener("click", async () => {
  try {
    const result = await hostRequest("chooseThemeFolder");
    if (!result?.canceled) {
      registerCustomThemes(result.themes || []);
      toast(localized("主题目录已更新"));
    }
  } catch (error) { toast(error.message); }
});
$("#theme-folder").addEventListener("click", async () => {
  try { await hostRequest("openThemeFolder"); }
  catch (error) { toast(error.message); }
});
$("#theme-refresh").addEventListener("click", () => void refreshCustomThemes(true));
$("#language-select").addEventListener("change", event => applyLocale(event.target.value));
$("#width-select").addEventListener("change", event => {
  document.documentElement.style.setProperty("--editor-width", `${event.target.value}px`);
  localStorage.setItem("mory.width", event.target.value);
});
$("#status-toggle").addEventListener("change", event => {
  $("#statusbar").hidden = !event.target.checked;
  localStorage.setItem("mory.status", String(event.target.checked));
});
$("#file-details-toggle").addEventListener("change", event => {
  state.showFileDetails = event.target.checked;
  localStorage.setItem("mory.fileDetails", String(event.target.checked));
  renderFiles();
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
  if (event.key !== "Escape" || !expandedMermaidDiagram) return;
  event.preventDefault();
  event.stopPropagation();
  closeExpandedMermaidWorkbench();
}, true);

document.addEventListener("keydown", event => {
  const command = event.metaKey || event.ctrlKey;
  const target = event.target instanceof Element ? event.target : null;
  const editing = target?.closest("input, textarea, select, [contenteditable='true']");
  const sidebarEntry = target?.closest(".file-item, .folder-item")
    || (document.activeElement instanceof Element ? document.activeElement.closest(".file-item, .folder-item") : null);
  if (!command && event.key === "Enter" && !editing && sidebarEntry && !$("#entry-operation-dialog").classList.contains("is-open") && beginSelectedEntryRename()) {
    event.preventDefault();
    return;
  }
  if (command && event.key.toLowerCase() === "p") { event.preventDefault(); openQuickOpen(); }
  if (command && event.key.toLowerCase() === "f") { event.preventDefault(); showFind(); }
  if (event.key === "Escape") {
    if (closeExpandedMermaidWorkbench()) {
      event.preventDefault();
      return;
    }
    closePathSuggestions(); closeFileContextMenu(); toggleEntryOperation(false); closeImagePreview(); closeQuickOpen(); closeFind(); closeCalendarQuickEditor(); closeCalendarEditor(); togglePreferences(false); toggleExportDialog(false); toggleKnowledgeGraph(false);
  }
});
document.addEventListener("pointermove", event => {
  if (!tableResize || event.pointerId !== tableResize.pointerId) return;
  resizeTableColumn(tableResize.table, tableResize.column, event.clientX - tableResize.originX, tableResize.widths);
  event.preventDefault();
});
function finishTableResize(event) {
  if (!tableResize || event.pointerId !== tableResize.pointerId) return;
  try { tableResize.handle.releasePointerCapture?.(event.pointerId); } catch { /* The pointer may already be released. */ }
  tableResize = null;
  document.body.classList.remove("is-resizing-table");
}
document.addEventListener("pointerup", finishTableResize);
document.addEventListener("pointercancel", finishTableResize);
document.addEventListener("pointermove", event => {
  if (!calendarDrag || event.pointerId !== calendarDrag.pointerId) return;
  const distance = Math.hypot(event.clientX - calendarDrag.originX, event.clientY - calendarDrag.originY);
  if (distance >= 5) calendarDrag.moved = true;
  const cell = document.elementFromPoint(event.clientX, event.clientY)?.closest(".calendar-day-cell");
  if (cell?.closest(".calendar-block") === calendarDrag.block) updateCalendarDragPreview(cell.dataset.date);
  event.preventDefault();
});
document.addEventListener("pointerup", event => {
  if (!calendarDrag || event.pointerId !== calendarDrag.pointerId) return;
  const drag = calendarDrag;
  try { drag.capture?.releasePointerCapture?.(event.pointerId); } catch { /* The pointer may already be released. */ }
  clearCalendarDragPreview();
  calendarDrag = null;
  drag.block.dataset.calendarSuppressClick = "true";
  setTimeout(() => delete drag.block.dataset.calendarSuppressClick, 0);
  if (drag.start !== drag.end) openCalendarRangeQuickEditor(drag.block, drag.start, drag.end);
  else openCalendarDateQuickEditor(drag.block, drag.start);
  event.preventDefault();
});
document.addEventListener("pointercancel", event => {
  if (!calendarDrag || event.pointerId !== calendarDrag.pointerId) return;
  try { calendarDrag.capture?.releasePointerCapture?.(event.pointerId); } catch { /* The pointer may already be released. */ }
  clearCalendarDragPreview();
  calendarDrag = null;
});
document.addEventListener("pointerdown", event => {
  if (!event.target.closest("#path-suggestions, #write")) closePathSuggestions();
  if (!event.target.closest("#file-context-menu")) closeFileContextMenu();
  if (!event.target.closest(".calendar-quick-editor, .calendar-day-cell")) closeCalendarQuickEditor();
});

function restorePreferences() {
  const savedLocale = localStorage.getItem("mory.locale") || "zh-CN";
  const theme = localStorage.getItem("mory.theme") || "system";
  const width = localStorage.getItem("mory.width") || "820";
  const showStatus = localStorage.getItem("mory.status") !== "false";
  const showFileDetails = localStorage.getItem("mory.fileDetails") === "true";
  const spellcheck = localStorage.getItem("mory.spell") !== "false";
  const savedDocumentTheme = localStorage.getItem("mory.documentTheme");
  const defaultThemeVersion = localStorage.getItem("mory.documentThemeDefaultVersion");
  const migrateBundledYuluoDefault = defaultThemeVersion === "yuluo-css-v1" && savedDocumentTheme === "yuluo-css";
  const documentTheme = migrateBundledYuluoDefault ? "github" : (savedDocumentTheme || "github");
  localStorage.setItem("mory.documentThemeDefaultVersion", "github-v1");
  $("#width-select").value = width;
  $("#status-toggle").checked = showStatus;
  $("#file-details-toggle").checked = showFileDetails;
  $("#spell-toggle").checked = spellcheck;
  applyAppearanceTheme(theme, { persist: false });
  document.documentElement.style.setProperty("--editor-width", `${width}px`);
  $("#statusbar").hidden = !showStatus;
  state.showFileDetails = showFileDetails;
  write.spellcheck = spellcheck;
  setDocumentTheme(builtInThemes.includes(documentTheme) ? documentTheme : "github");
  if (!builtInThemes.includes(documentTheme)) localStorage.setItem("mory.documentTheme", documentTheme);
  applyLocale(savedLocale);
}

window.Mory = {
  loadMarkdown: markdown => loadMarkdown(markdown, true),
  openDocument,
  newDocument: () => createUntitledDocument(),
  newFolder: () => toggleNewFolderForm(true),
  closeDocument,
  normalizeMarkdown: renderMarkdownDocumentAtCaret,
  getMarkdown: () => state.sourceMode ? sourceEditor.value : editorToMarkdown(write),
  setFiles: setWorkspaceFiles,
  setWorkspaceSnapshot,
  setWorkspaceDocuments: documents => {
    // An explicit snapshot supersedes pending host requests so stale responses cannot overwrite it.
    workspaceKnowledgeRequest += 1;
    state.workspaceDocuments = Array.isArray(documents) ? documents : [];
    rebuildWorkspaceKnowledge();
  },
  setCustomThemes: registerCustomThemes,
  fontAvailable,
  didSave: payload => {
    const document = activeDocument();
    if (document) {
      const previousOrderKey = fileEntryKey(document);
      document.path = typeof payload?.path === "string" ? payload.path : document.path;
      document.name = String(payload?.name || document.name);
      document.markdown = typeof payload?.markdown === "string" ? payload.markdown : (state.sourceMode ? sourceEditor.value : editorToMarkdown(write));
      if (payload?.assets && typeof payload.assets === "object") document.assets = payload.assets;
      document.dirty = false;
      state.documents = state.documents.filter(item => item === document || !document.path || item.path !== document.path);
      const orderIndex = state.manualFileOrder.indexOf(previousOrderKey);
      if (orderIndex >= 0) {
        state.manualFileOrder[orderIndex] = fileEntryKey(document);
        localStorage.setItem("mory.fileOrder", JSON.stringify(state.manualFileOrder));
      }
    }
    state.dirty = false;
    rebuildWorkspaceKnowledge();
    if (document && typeof payload?.markdown === "string" && payload.markdown !== state.markdown) renderDocument(document);
    renderFiles();
    $("#save-state").textContent = localized("已保存");
    setTimeout(() => $("#save-state").classList.remove("is-visible"), 900);
    toast(localized("已保存"));
  },
  exportStarted: format => toast(locale() === "en" ? `Exporting ${String(format).toUpperCase()}…` : `正在导出 ${String(format).toUpperCase()}…`, 5000),
  exportBusy: () => toast(locale() === "en" ? "An export is already in progress" : "已有导出任务正在进行"),
  didExport: format => toast(locale() === "en" ? `Exported ${String(format).toUpperCase()}` : `已导出 ${String(format).toUpperCase()}`),
  exportHTML: () => exportDocument({ theme: "current", background: true }),
  exportDocument,
  exportToHost,
  calendarMarkdown,
  optimizeTypography: optimizeActiveDocumentTypography,
  resolveHostRequest,
  setWorkspaceState,
  command: execute,
  undo: undoEditor,
  redo: redoEditor,
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
    applyEditorZoom();
    toast(`${Math.round(state.zoom * 100)}%`);
  }
};

restorePreferences();
const browserDraft = (window.webkit || window.moryNative || nativeWailsHost()) ? null : localStorage.getItem("mory.draft");
createUntitledDocument(browserDraft || defaultMarkdown, {
  announce: false,
  notifyHost: false,
  workspacePlaceholder: !browserDraft
});
bridge({ type: "ready" });
void refreshCustomThemes();
