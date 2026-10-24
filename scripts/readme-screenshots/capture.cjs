// Captures the README screenshots from the shared web editor in an offscreen Electron window.
// Usage: npx electron scripts/readme-screenshots/capture.cjs
// Output: assets/screenshots/<locale>-<scene>.png for the zh-CN and en locales.
const { app, BrowserWindow } = require("electron");
const fs = require("node:fs/promises");
const path = require("node:path");

const projectDirectory = path.resolve(__dirname, "..", "..");
const outputDirectory = path.join(projectDirectory, "assets", "screenshots");
const samplesDirectory = path.join(__dirname, "samples");
const workspaceRoot = "/Notes";
const locales = [
  { id: "zh-CN", samples: "zh", entry: "\u5199\u4f5c\u6307\u5357.md", prefix: "zh" },
  { id: "en", samples: "en", entry: "Writing-Guide.md", prefix: "en" }
];

const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));

async function readSamples(directory) {
  const names = (await fs.readdir(directory)).filter(name => name.endsWith(".md")).sort();
  return Promise.all(names.map(async (name, index) => ({
    name,
    path: `${workspaceRoot}/${name}`,
    markdown: await fs.readFile(path.join(directory, name), "utf8"),
    createdAt: index + 1,
    updatedAt: index + 1,
    size: 0
  })));
}

async function load(window, { locale, theme }) {
  await window.loadFile(path.join(projectDirectory, "Sources", "Mory", "Web", "index.html"));
  // Preferences are read at startup, so they are stored first and applied by a reload.
  await window.webContents.executeJavaScript(`
    localStorage.setItem("mory.locale", ${JSON.stringify(locale)});
    localStorage.setItem("mory.theme", ${JSON.stringify(theme)});
    localStorage.setItem("mory.introductionSeen", "true");
    localStorage.removeItem("mory.recovery");
  `);
  await window.webContents.reload();
  await new Promise(resolve => window.webContents.once("did-finish-load", resolve));
  await window.webContents.executeJavaScript("document.fonts.ready.then(() => true)");
}

async function openWorkspace(window, documents, entry) {
  const files = documents.map(({ markdown, ...file }) => file);
  const opened = documents.find(document => document.name === entry);
  await window.webContents.executeJavaScript(`(() => {
    window.Mory.setWorkspaceSnapshot({
      state: { activeId: "readme", workspaces: [{ id: "readme", name: "Notes", provider: "local", localPath: ${JSON.stringify(workspaceRoot)} }] },
      files: ${JSON.stringify(files)},
      directories: []
    });
    window.Mory.setWorkspaceDocuments(${JSON.stringify(documents)});
    window.Mory.openDocument(${JSON.stringify({ name: opened.name, path: opened.path, markdown: opened.markdown })});
    return true;
  })()`);
  // Mermaid and syntax highlighting render asynchronously after the document opens.
  await wait(1200);
}

async function verify(window, label, expression) {
  const result = await window.webContents.executeJavaScript(`(() => { try { return Boolean(${expression}); } catch { return false; } })()`);
  if (!result) throw new Error(`Screenshot precondition failed: ${label}`);
}

async function capture(window, name) {
  const image = await window.webContents.capturePage();
  await fs.writeFile(path.join(outputDirectory, `${name}.png`), image.toPNG());
  console.log(`Captured assets/screenshots/${name}.png`);
}

async function hoverToolbarDock(window) {
  const point = await window.webContents.executeJavaScript(`(() => {
    const rect = document.querySelector("#toolbar-dock-trigger").getBoundingClientRect();
    return { x: Math.round(rect.left + rect.width / 2), y: Math.round(rect.top + rect.height / 2) };
  })()`);
  window.webContents.sendInputEvent({ type: "mouseMove", x: point.x, y: point.y });
  await wait(400);
  const open = await window.webContents.executeJavaScript(`document.querySelector("#toolbar-dock").classList.contains("is-open")`);
  if (!open) {
    await window.webContents.executeJavaScript(`document.querySelector("#toolbar-dock").dispatchEvent(new MouseEvent("mouseenter"))`);
    await wait(400);
  }
}

async function main() {
  await fs.mkdir(outputDirectory, { recursive: true });
  const window = new BrowserWindow({
    show: false,
    width: 1280,
    height: 800,
    webPreferences: { sandbox: true, offscreen: true, partition: `mory-readme-${process.pid}` }
  });
  for (const locale of locales) {
    const documents = await readSamples(path.join(samplesDirectory, locale.samples));

    await load(window, { locale: locale.id, theme: "light" });
    await openWorkspace(window, documents, locale.entry);
    await verify(window, "editor content", `document.querySelector("#write h1")?.textContent.includes("Mory")
      && document.querySelector("#write table") && document.querySelector("#write .mermaid svg, #write svg")
      && document.querySelectorAll("#file-list .file-item[data-path]").length === ${documents.length}`);
    await capture(window, `${locale.prefix}-editor`);
    await hoverToolbarDock(window);
    await verify(window, "expanded toolbar", `getComputedStyle(document.querySelector("#toolbar")).visibility === "visible"`);
    await capture(window, `${locale.prefix}-toolbar`);

    await load(window, { locale: locale.id, theme: "dark" });
    await openWorkspace(window, documents, locale.entry);
    await verify(window, "dark appearance", `document.documentElement.dataset.appearance === "dark"`);
    await capture(window, `${locale.prefix}-dark`);

    await load(window, { locale: locale.id, theme: "light" });
    await openWorkspace(window, documents, locale.entry);
    await window.webContents.executeJavaScript(`document.querySelector("#graph-button").click()`);
    await wait(1800);
    await verify(window, "knowledge graph", `document.querySelector("#knowledge-graph").classList.contains("is-open")
      && document.querySelectorAll("#knowledge-graph svg circle, #knowledge-graph .graph-node").length >= ${documents.length}`);
    await capture(window, `${locale.prefix}-graph`);
  }
  window.destroy();
}

app.whenReady()
  .then(main)
  .then(() => app.quit())
  .catch(error => {
    console.error(error);
    app.exit(1);
  });
