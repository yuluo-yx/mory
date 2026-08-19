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
  assert.match(macOSHost, /case "createDirectory"/);
  assert.match(macOSHost, /case "documentAssets"/);
  assert.match(macOSWorkspace, /func createDirectory\(relativePath:/);
  assert.match(macOSWorkspace, /destination\.path\.hasPrefix\(rootPath \+ "\/"\)/);
  assert.match(web, /hostRequest\("createDirectory"/);
  assert.match(web, /hostRequest\("documentAssets"/);
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
  const plist = fs.readFileSync(path.join(root, "macOS", "Info.plist"), "utf8");
  const build = fs.readFileSync(path.join(root, "scripts", "build-macos.sh"), "utf8");
  const iconBuild = fs.readFileSync(path.join(root, "scripts", "build-macos-icons.sh"), "utf8");
  assert.match(plist, /<key>CFBundleIconFile<\/key><string>icon\.icns<\/string>/);
  assert.match(build, /build-macos-icons\.sh/);
  assert.match(build, /\.build\/icons\/icon\.icns/);
  assert.match(iconBuild, /icon_16x16\.png/);
  assert.match(iconBuild, /icon_512x512@2x\.png/);
  assert.match(iconBuild, /build-icns\.mjs/);
  assert.equal(fs.existsSync(path.join(root, "assets", "mory-icon.png")), true);
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

test("GitHub is the default theme and Yuluo warns when its preferred font is unavailable", () => {
  const html = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "index.html"), "utf8");
  const web = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "app.js"), "utf8");
  const yuluo = fs.readFileSync(path.join(root, "Sources", "Mory", "Web", "themes", "yuluo-css.css"), "utf8");
  assert.match(html, /data-doc-theme="github"/);
  assert.match(html, /href="themes\/github\.css"/);
  assert.match(web, /documentTheme:\s*"github"/);
  assert.match(web, /mory\.documentThemeDefaultVersion", "github-v1"/);
  assert.match(web, /function updateYuluoFontWarning/);
  assert.doesNotMatch(yuluo, /Segoe Print/);
});

test("every built-in document theme provides a readable dark appearance", () => {
  const themeDirectory = path.join(root, "Sources", "Mory", "Web", "themes");
  const themes = ["yuluo-css", "github", "whitey", "newsprint", "pixyll", "gothic", "night"];
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
