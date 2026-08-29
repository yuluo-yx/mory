const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const root = path.join(__dirname, "..");

test("the classic web bundle removes local module imports", () => {
  const bundle = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.bundle.js"), "utf8");
  assert.doesNotMatch(bundle, /^import\s/m);
  assert.match(bundle, /function parseCalendarSource/);
});

test("desktop hosts use atomic workspace snapshots to avoid file-list races", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  assert.match(electron, /window\.Mory\.setWorkspaceSnapshot/);
  assert.match(macOS, /window\.Mory\.setWorkspaceSnapshot/);
});

test("desktop hosts defer operating-system document opens until their editors are ready", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const windows = fs.readFileSync(path.join(root, "cmd", "mory-windows", "main_windows.go"), "utf8");
  assert.match(electron, /app\.on\("second-instance"/);
  assert.match(electron, /app\.on\("open-file"/);
  assert.match(electron, /pendingLaunchPath/);
  assert.match(macOS, /pendingOpenURL/);
  assert.match(macOS, /guard workspaceManager != nil, window != nil/);
  assert.match(windows, /SingleInstanceLock/);
  assert.match(windows, /OpenExternalFile/);
});

test("native workspace sidecars receive only storage provider fields", () => {
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "WorkspaceManager.swift"), "utf8");
  const electron = fs.readFileSync(path.join(root, "Electron", "workspaces.cjs"), "utf8");
  assert.match(macOS, /active\.storageDictionary\(\)/);
  assert.match(macOS, /removeValue\(forKey: "isImplicit"\)/);
  assert.match(electron, /workspace: storageWorkspace\(workspace\)/);
});

test("desktop hosts expose file timestamps and sizes for stable ordering and optional details", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "workspaces.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "WorkspaceManager.swift"), "utf8");
  assert.match(electron, /stat\.birthtimeMs/);
  assert.match(electron, /compareDocumentsByCreation/);
  assert.match(electron, /updatedAt: Number\(stat\.mtimeMs\)/);
  assert.match(electron, /size: Number\(stat\.size\)/);
  assert.match(macOS, /\.creationDateKey/);
  assert.match(macOS, /"createdAt"/);
  assert.match(macOS, /\.fileSizeKey/);
  assert.match(macOS, /"updatedAt"/);
});

test("macOS and Windows hosts watch workspaces recursively and refresh atomic snapshots", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "workspace-watcher.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "WorkspaceWatcher.swift"), "utf8");
  assert.match(electron, /fs\.watch\(nextRoot, \{ recursive: true \}/);
  assert.match(electron, /setInterval\(scheduleRefresh, pollIntervalMs\)/);
  assert.match(electron, /Promise\.resolve\(onChange\(\)\)/);
  assert.match(macOS, /FSEventStreamCreate\(/);
  assert.match(macOS, /kFSEventStreamCreateFlagFileEvents/);
  assert.match(macOS, /kFSEventStreamCreateFlagWatchRoot/);
});

test("macOS and Windows move deleted documents to the system trash", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(electron, /case "deleteDocument"/);
  assert.match(electron, /shell\.trashItem\(filePath\)/);
  assert.match(macOS, /case "deleteDocument"/);
  assert.match(macOS, /FileManager\.default\.trashItem/);
  assert.match(web, /hostRequest\("deleteDocument"/);
  assert.match(web, /localized\("\u6587\u6863\u5DF2\u79FB\u5230\u5E9F\u7EB8\u7BD3"\)/);
});

test("macOS and Windows constrain directory creation and lazy image loading to the workspace", () => {
  const electronHost = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const electronWorkspace = fs.readFileSync(path.join(root, "Electron", "workspaces.cjs"), "utf8");
  const macOSHost = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const macOSWorkspace = fs.readFileSync(path.join(root, "Sources", "Mory", "WorkspaceManager.swift"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(electronHost, /case "createDirectory"/);
  assert.match(electronHost, /case "documentAssets"/);
  assert.match(electronWorkspace, /function resolveWorkspaceDirectory/);
  assert.match(electronWorkspace, /local\.startsWith\(`\.\./);
  assert.ok(electronWorkspace.includes("<img\\b[^>]*\\bsrc"));
  assert.match(macOSHost, /case "createDirectory"/);
  assert.match(macOSHost, /case "documentAssets"/);
  assert.match(macOSWorkspace, /func createDirectory\(relativePath:/);
  assert.match(macOSWorkspace, /destination\.path\.hasPrefix\(rootPath \+ "\/"\)/);
  assert.ok(macOSWorkspace.includes("<img\\b[^>]*\\bsrc"));
  assert.match(web, /hostRequest\("createDirectory"/);
  assert.match(web, /hostRequest\("documentAssets"/);
});

test("desktop hosts open modified-click web links outside the editor", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const windows = fs.readFileSync(path.join(root, "internal", "windowshost", "host.go"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(electron, /case "openExternal"/);
  assert.match(electron, /shell\.openExternal\(url\)/);
  assert.match(macOS, /case "openExternal"/);
  assert.match(macOS, /NSWorkspace\.shared\.open\(url\)/);
  assert.match(windows, /case "openExternal"/);
  assert.ok(web.includes("(!event.ctrlKey && !event.metaKey)"));
  assert.match(web, /workspaceFileForLink\(href\)/);
});

test("macOS and Windows save drafts into the active workspace when possible", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  assert.match(electron, /workspaceManager\?\.active\(\)\?\.isImplicit !== true/);
  assert.match(electron, /availableDocumentPath\(workspaceManager\.activeRoot\(\), suggestedDocumentName\(markdown\)\)/);
  assert.match(electron, /else await saveAs\(\)/);
  assert.match(macOS, /workspaceManager\.active\.isImplicit != true else \{ saveDocumentAs\(\); return \}/);
  assert.match(macOS, /availableDocumentURL\(markdown: markdown\)/);
});

test("both desktop hosts provide document actions, image previews, and theme-folder selection", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  for (const method of ["readDocument", "revealFile", "documentImage", "chooseThemeFolder"]) {
    assert.match(electron, new RegExp(`case "${method}"`));
    assert.match(macOS, new RegExp(`case "${method}"`));
  }
  assert.match(web, /showFileContextMenu/);
  assert.match(web, /previewDocumentImage/);
  assert.match(web, /firstLevelHeading/);
});

test("all three hosts share create, copy, move, rename, and delete contracts", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const windows = fs.readFileSync(path.join(root, "internal", "windowshost", "host.go"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  for (const method of ["createDocument", "copyWorkspaceEntry", "moveWorkspaceEntry", "renameWorkspaceEntry", "deleteWorkspaceEntry"]) {
    assert.match(electron, new RegExp(`"${method}"`));
    assert.match(macOS, new RegExp(`"${method}"`));
    assert.match(windows, new RegExp(`"${method}"`));
    assert.match(web, new RegExp(method));
  }
  assert.match(web, /expandedDirectoryPaths/);
  assert.match(web, /folder-toggle/);
  assert.match(web, /selectedWorkspaceEntry/);
});

test("macOS PDF export uses asynchronous WebKit generation and background pagination", () => {
  const source = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const paginator = fs.readFileSync(path.join(root, "Sources", "Mory", "PDFPaginator.swift"), "utf8");
  assert.match(source, /webView\.createPDF\(/);
  assert.match(source, /Task\.detached\(priority: \.userInitiated\)/);
  assert.match(source, /PDFPaginator\.write\(/);
  assert.match(paginator, /context\.beginPDFPage/);
  assert.match(paginator, /context\.drawPDFPage/);
  assert.doesNotMatch(source, /operation\.run\(\)/);
});

test("desktop hosts save mind maps as standalone HTML files", () => {
  const electron = fs.readFileSync(path.join(root, "Electron", "main.cjs"), "utf8");
  const macOS = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const windows = fs.readFileSync(path.join(root, "cmd", "mory-windows", "platform_windows.go"), "utf8");
  assert.match(electron, /format === "mindmap" \? "html"/);
  assert.match(electron, /format === "html" \|\| format === "mindmap"/);
  assert.match(macOS, /format == "mindmap" \? "html"/);
  assert.match(macOS, /format == "html" \|\| format == "mindmap"/);
  assert.match(windows, /extension == "mindmap"/);
  assert.match(windows, /request\.Format == "html" \|\| request\.Format == "mindmap"/);
});

test("macOS rebuilds and localizes top-level application menus after locale changes", () => {
  const host = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const localizer = fs.readFileSync(path.join(root, "Sources", "Mory", "MenuLocalizer.swift"), "utf8");
  assert.match(host, /case "localeChanged":/);
  assert.match(host, /configureMenu\(\)/);
  assert.match(host, /MenuLocalizer\.localize\(main, locale: interfaceLocale\)/);
  assert.match(localizer, /menu\.title = englishTitle\(for: menu\.title\)/);
  assert.match(localizer, /item\.title = submenu\.title/);
});

test("macOS sidebar and document title bars share native window zoom behavior", () => {
  const host = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const typingSmoke = fs.readFileSync(path.join(root, "Tests", "MacTypingSmoke.swift"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(web, /\$\$\("\.titlebar, \.traffic-space"\)/);
  assert.match(web, /region\.addEventListener\("dblclick", handleWindowTitlebarDoubleClick\)/);
  assert.match(web, /windowTitlebarDoubleClick/);
  assert.match(host, /case "windowTitlebarDoubleClick"/);
  assert.match(host, /window\.performZoom\(nil\)/);
  assert.match(host, /let constrainedToVisibleFrame = visibleFrame\.map/);
  assert.match(host, /let restored = sizeRestored && \(!window\.isZoomed \|\| sizeWasAlreadyMaximal\) \|\| constrainedToVisibleFrame/);
  assert.match(host, /document\.querySelector\('\.traffic-space'\)/);
  assert.match(host, /new MouseEvent\('dblclick'/);
  assert.match(typingSmoke, /code: document\.querySelector\('#write > pre code'\)\?\.innerText/);
});

test("the macOS bundle declares and generates a complete multi-size ICNS icon", () => {
  const host = fs.readFileSync(path.join(root, "Sources", "Mory", "MoryApp.swift"), "utf8");
  const plist = fs.readFileSync(path.join(root, "macOS", "Info.plist"), "utf8");
  const build = fs.readFileSync(path.join(root, "scripts", "build-macos.sh"), "utf8");
  const iconBuild = fs.readFileSync(path.join(root, "scripts", "build-macos-icons.sh"), "utf8");
  const windowsPackage = fs.readFileSync(path.join(root, "scripts", "package-windows-wails.ps1"), "utf8");
  const iconSVG = fs.readFileSync(path.join(root, "assets", "mory-icon.svg"), "utf8");
  const iconPNG = fs.readFileSync(path.join(root, "assets", "mory-icon.png"));
  assert.match(plist, /<key>CFBundleIconFile<\/key><string>icon\.icns<\/string>/);
  assert.match(build, /build-macos-icons\.sh/);
  assert.match(build, /\.build\/icons\/icon\.icns/);
  assert.match(iconBuild, /icon_16x16\.png/);
  assert.match(iconBuild, /icon_512x512@2x\.png/);
  assert.match(iconBuild, /build-icns\.mjs/);
  assert.match(windowsPackage, /assets[\\/]mory-icon\.png/);
  assert.match(host, /options\[\.applicationIcon\] = icon/);
  assert.match(iconSVG, /viewBox="0 0 1024 1024"/);
  assert.match(iconSVG, /A calm short-haired writer illustrated in charcoal, ivory, and teal/);
  assert.doesNotMatch(iconSVG, /geometric M|Möbius|infinity/i);
  assert.equal(iconPNG.subarray(1, 4).toString(), "PNG");
  assert.equal(iconPNG.readUInt32BE(16), 1024);
  assert.equal(iconPNG.readUInt32BE(20), 1024);
  assert.equal(iconPNG[25], 6);
});

test("the knowledge graph captures wheel input on HTML and normalizes it with the D3 formula", () => {
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(web, /#graph-canvas"\)\.addEventListener\("wheel", handleGraphWheel, \{ passive: false \}\)/);
  assert.match(web, /event\.deltaMode === 1 \? \.05 : event\.deltaMode \? 1 : \.002/);
  assert.match(web, /state\.graphZoom\.scaleBy/);
  assert.match(web, /svg\.call\(zoom\)\.on\("wheel\.zoom", null\)/);
});

test("desktop hosts map DPI without scaling typography again on wide viewports", () => {
  const styles = [
    fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "styles.css"), "utf8"),
    ...fs.readdirSync(path.join(root, "Sources", "Mory", "Web", "themes"))
      .filter(name => name.endsWith(".css"))
      .map(name => fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "themes", name), "utf8"))
  ].join("\n");
  assert.doesNotMatch(styles, /font-size:\s*clamp\([^;]*vw/);
  assert.match(styles, /--editor-gutter:\s*68px/);
  assert.match(styles, /width:\s*min\(calc\(100% - var\(--editor-gutter\) - var\(--editor-gutter\)\), var\(--editor-width\)\)/);
});

test("tables add and delete the current row or column and support Typora-style shortcuts", () => {
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(web, /function deleteTableRow\(table\)/);
  assert.match(web, /function deleteTableColumn\(table\)/);
  assert.match(web, /\["delete-row", "\u5220\u9664\u884C", deleteTableRow\]/);
  assert.match(web, /\["delete-column", "\u5220\u9664\u5217", deleteTableColumn\]/);
  assert.match(web, /command && event\.shiftKey && event\.key === "Backspace"/);
});

test("GitHub remains the default while bundled themes warn when preferred fonts are unavailable", () => {
  const html = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "index.html"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  const yuluo = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "themes", "yuluo-css.css"), "utf8");
  const lapis = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "themes", "lapis-cv.css"), "utf8");
  const license = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "themes", "lapis-cv.LICENSE"), "utf8");
  assert.match(html, /data-doc-theme="github"/);
  assert.match(html, /href="themes\/github\.css"/);
  assert.match(html, /option value="lapis-cv">Lapis CV<\/option>/);
  assert.match(web, /documentTheme:\s*"github"/);
  assert.match(web, /mory\.documentThemeDefaultVersion", "github-v1"/);
  assert.match(web, /function updateThemeFontWarning/);
  assert.match(web, /const lapisCVFonts = \["Source Han Sans CN", "JetBrains Mono", "LapisCV Icon"\]/);
  assert.doesNotMatch(yuluo, /Segoe Print/);
  assert.match(lapis, /Mory Lapis CV document theme/);
  assert.match(lapis, /img\[alt="avatar"\]/);
  assert.match(lapis, /@page \{ size: A4; \}/);
  assert.match(license, /MIT License/);
  assert.match(license, /Copyright \(c\) 2024 YiNN/);
});

test("raw Markdown HTML uses the bundled DOMPurify runtime and preserves source wrappers", () => {
  const html = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "index.html"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  const markdown = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "markdown.js"), "utf8");
  const build = fs.readFileSync(path.join(root, "scripts", "build-web.mjs"), "utf8");
  assert.match(html, /vendor\/dompurify\.min\.js/);
  assert.match(web, /function enhanceRawHTML/);
  assert.match(web, /purifier\.sanitize\(source/);
  assert.match(web, /FORBID_TAGS: \["script", "style", "iframe"/);
  assert.match(markdown, /mory-raw-html-placeholder/);
  assert.match(markdown, /element\.dataset\.rawHtml/);
  assert.match(build, /dompurify\.LICENSE/);
});

test("every built-in document theme provides a readable dark appearance", () => {
  const themeDirectory = path.join(root, "Sources", "Mory", "Web", "themes");
  const themes = ["yuluo-css", "lapis-cv", "github", "whitey", "newsprint", "pixyll", "gothic", "night"];
  const channel = value => {
    const normalized = value / 255;
    return normalized <= .04045 ? normalized / 12.92 : ((normalized + .055) / 1.055) ** 2.4;
  };
  const luminance = hex => {
    const values = hex.match(/[0-9a-f]{2}/gi).map(value => Number.parseInt(value, 16));
    return .2126 * channel(values[0]) + .7152 * channel(values[1]) + .0722 * channel(values[2]);
  };
  const contrast = (foreground, background) => {
    const values = [luminance(foreground), luminance(background)].sort((left, right) => right - left);
    return (values[0] + .05) / (values[1] + .05);
  };

  for (const theme of themes) {
    const css = fs.readFileSync(path.join(themeDirectory, `${theme}.css`), "utf8");
    const selector = `:root[data-appearance="dark"][data-doc-theme="${theme}"]`;
    const paper = css.match(new RegExp(`${selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")} \\.editor-scroll \\{ background: (#[0-9a-f]{6}); \\}`, "i"));
    const text = css.match(new RegExp(`${selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")} \\.write \\{ color: (#[0-9a-f]{6}); \\}`, "i"));
    assert.ok(paper, `${theme} must define a dark paper color`);
    assert.ok(text, `${theme} must define a dark text color`);
    assert.ok(contrast(text[1], paper[1]) >= 4.5, `${theme} dark text must meet WCAG AA contrast`);
  }
});

test("Mermaid follows editor appearance while exports retain document paper colors", () => {
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  assert.match(web, /function mermaidTheme\(theme, appearance = document\.documentElement\.dataset\.appearance, colorTheme = "auto"\)/);
  assert.match(web, /appearance === "dark" \? darkPalettes : lightPalettes/);
  assert.match(web, /colorPalettes\[normalizedTheme\]\[appearance === "dark" \? "dark" : "light"\]/);
  assert.match(web, /function ensureMermaidWorkbench/);
  assert.match(web, /className = "mermaid-source-editor"/);
  assert.match(web, /className = "mermaid-preview-canvas"/);
  assert.match(web, /renderMermaidDiagrams\(write, state\.documentTheme\)/);
  assert.match(web, /renderMermaidDiagrams\(exportRoot, theme, theme === "night" \? "dark" : "light"\)/);
});
