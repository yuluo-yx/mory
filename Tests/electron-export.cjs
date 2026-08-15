const { app, BrowserWindow } = require("electron");
const fs = require("node:fs/promises");
const path = require("node:path");
const os = require("node:os");

app.commandLine.appendSwitch("disable-gpu");
app.disableHardwareAcceleration();
process.stdout.write(`[e2e] boot electron=${process.versions.electron || "missing"}\n`);

async function loadAndWait(window, load) {
  const finished = new Promise((resolve, reject) => {
    window.webContents.once("did-finish-load", resolve);
    window.webContents.once("did-fail-load", (_event, code, message) => reject(new Error(`${code}: ${message}`)));
  });
  load();
  await Promise.race([
    finished,
    new Promise((_, reject) => setTimeout(() => reject(new Error("页面加载超过 10 秒")), 10_000))
  ]);
}

app.whenReady().then(async () => {
  let exitCode = 0;
  process.stdout.write("[e2e] app-ready\n");
  let editor;
  let exportView;
  try {
    editor = new BrowserWindow({ show: false, width: 1280, height: 800, webPreferences: { sandbox: true, offscreen: true, partition: `mory-export-${process.pid}` } });
    editor.webContents.on("console-message", (_event, details) => process.stdout.write(`[editor:${details.level}] ${details.message}\n`));
    editor.webContents.on("render-process-gone", (_event, details) => process.stdout.write(`[editor-gone] ${JSON.stringify(details)}\n`));
    await loadAndWait(editor, () => editor.loadFile(path.join(__dirname, "..", "Sources", "Mory", "Web", "index.html")));
    process.stdout.write("[e2e] editor-loaded\n");

    const mermaidMarkdown = "# Mory Markdown 编辑器\n\n## Mermaid\n\n```mermaid\nflowchart LR\n  A[Markdown] --> B[SVG]\n  B --> C[PDF / HTML / 图片]\n```\n\n## 命名代码\n\n```go title=\"main.go\"\npackage main\nfunc main() { fmt.Println(\"export\") }\n```\n\n## 导出验证";
    const invalidMermaidMarkdown = "```mermaid\nthis is not a diagram\n```";
    const result = await editor.webContents.executeJavaScript(`(async () => {
      window.Mory.loadMarkdown(${JSON.stringify(mermaidMarkdown)});
      const defaultTheme = document.documentElement.dataset.docTheme;
      for (let index = 0; index < 80 && !document.querySelector('.mermaid-diagram svg'); index += 1) {
        await new Promise(resolve => setTimeout(resolve, 50));
      }
      const theme = document.querySelector('#document-theme-select');
      theme.value = 'newsprint';
      theme.dispatchEvent(new Event('change'));
      document.querySelector('#write').dispatchEvent(new Event('input', { bubbles: true }));
      for (let index = 0; index < 80 && document.querySelector('.mermaid-diagram')?.dataset.mermaidState !== 'rendered'; index += 1) {
        await new Promise(resolve => setTimeout(resolve, 50));
      }
      const html = await window.Mory.exportDocument({ theme: 'newsprint', background: true });
      const snapshot = {
        html,
        heading: document.querySelector('#write h1')?.textContent,
        outlineCount: document.querySelectorAll('#outline-list .outline-item').length,
        theme: document.documentElement.dataset.docTheme,
        defaultTheme,
        highlightRuntime: window.hljs?.versionString || '',
        codeHighlighted: Boolean(document.querySelector('#write pre[data-language="go"] .hljs-keyword')),
        exportCodeHighlighted: html.includes('hljs-keyword'),
        sourceRoundTrip: window.Mory.getMarkdown().includes(${JSON.stringify("```mermaid")}),
        namedCodeRoundTrip: window.Mory.getMarkdown().includes(${JSON.stringify('```go title="main.go"')}) && html.includes('data-title="main.go"'),
        mermaidRendered: document.querySelector('.mermaid-diagram')?.dataset.mermaidState === 'rendered'
      };
      const nightHTML = await window.Mory.exportDocument({ theme: 'night', background: true });
      snapshot.nightExport = nightHTML.includes('data-doc-theme="night"') && nightHTML.includes('<svg');
      window.Mory.loadMarkdown(${JSON.stringify(invalidMermaidMarkdown)});
      for (let index = 0; index < 80 && document.querySelector('.mermaid-diagram')?.dataset.mermaidState !== 'error'; index += 1) {
        await new Promise(resolve => setTimeout(resolve, 50));
      }
      snapshot.mermaidError = document.querySelector('.mermaid-diagram')?.dataset.mermaidState === 'error';
      return snapshot;
    })()`);

    if (!result.html.includes('[data-doc-theme="newsprint"]') || !result.html.includes('Mory Markdown 编辑器') || !result.html.includes('<svg') || result.html.includes('<script')) {
      throw new Error("导出 HTML 未内联 Newsprint 主题、正文或 Mermaid SVG");
    }
    if (!result.mermaidRendered) throw new Error("编辑区 Mermaid 未渲染");
    if (result.defaultTheme !== "yuluo-css") throw new Error("Yuluo CSS 未成为默认文档主题");
    if (result.highlightRuntime !== "11.11.1" || !result.codeHighlighted || !result.exportCodeHighlighted) throw new Error("Highlight.js 未覆盖编辑区和导出 HTML");
    if (!result.namedCodeRoundTrip) throw new Error("代码片段名称未完成 Markdown 与导出往返");
    if (!result.nightExport) throw new Error("Night 主题 Mermaid 导出失败");
    if (!result.mermaidError) throw new Error("Mermaid 语法错误未显示可恢复状态");
    process.stdout.write("[e2e] html-generated\n");

    exportView = new BrowserWindow({ show: false, width: 900, height: 1000, webPreferences: { sandbox: true, offscreen: true, partition: `mory-export-view-${process.pid}` } });
    await loadAndWait(exportView, () => exportView.loadURL(`data:text/html;charset=UTF-8,${encodeURIComponent(result.html)}`));
    process.stdout.write("[e2e] export-view-loaded\n");
    const height = await exportView.webContents.executeJavaScript("Math.max(document.documentElement.scrollHeight, document.body.scrollHeight)");
    exportView.setContentSize(900, Math.ceil(height));
    await new Promise(resolve => setTimeout(resolve, 160));

    const image = await exportView.webContents.capturePage({ x: 0, y: 0, width: 900, height: Math.ceil(height) });
    const png = image.toPNG();
    const pdf = await exportView.webContents.printToPDF({ printBackground: true, pageSize: "A4" });
    const base = path.join(os.tmpdir(), "mory-e2e");
    await fs.writeFile(`${base}.html`, result.html, "utf8");
    await fs.writeFile(`${base}.png`, png);
    await fs.writeFile(`${base}.pdf`, pdf);

    if (png.subarray(1, 4).toString("ascii") !== "PNG") throw new Error("PNG 签名无效");
    if (pdf.subarray(0, 4).toString("ascii") !== "%PDF") throw new Error("PDF 签名无效");

    process.stdout.write(JSON.stringify({
      status: "passed",
      heading: result.heading,
      outlineCount: result.outlineCount,
      theme: result.theme,
      defaultTheme: result.defaultTheme,
      highlightRuntime: result.highlightRuntime,
      codeHighlighted: result.codeHighlighted,
      sourceRoundTrip: result.sourceRoundTrip,
      namedCodeRoundTrip: result.namedCodeRoundTrip,
      mermaidRendered: result.mermaidRendered,
      mermaidErrorHandled: result.mermaidError,
      nightMermaidExport: result.nightExport,
      exportHeight: height,
      htmlBytes: Buffer.byteLength(result.html),
      pngBytes: png.length,
      pdfBytes: pdf.length,
      artifacts: [`${base}.html`, `${base}.png`, `${base}.pdf`]
    }, null, 2) + "\n");
  } catch (error) {
    process.stderr.write(`${error.stack || error}\n`);
    exitCode = 1;
  } finally {
    exportView?.destroy();
    editor?.destroy();
    app.exit(exitCode);
  }
});
