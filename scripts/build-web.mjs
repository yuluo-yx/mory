import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const projectDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const webDirectory = path.join(projectDirectory, "Sources", "Mory", "Web");
const markdownPath = path.join(webDirectory, "markdown.js");
const appPath = path.join(webDirectory, "app.js");
const outputPath = path.join(webDirectory, "app.bundle.js");
const vendorDirectory = path.join(webDirectory, "vendor");
const mermaidInputPath = path.join(projectDirectory, "node_modules", "mermaid", "dist", "mermaid.min.js");
const mermaidOutputPath = path.join(vendorDirectory, "mermaid.min.js");
const mermaidLicenseInputPath = path.join(projectDirectory, "node_modules", "mermaid", "LICENSE");
const mermaidLicenseOutputPath = path.join(vendorDirectory, "mermaid.LICENSE");
const highlightInputPath = path.join(projectDirectory, "node_modules", "@highlightjs", "cdn-assets", "highlight.min.js");
const highlightOutputPath = path.join(vendorDirectory, "highlight.min.js");
const highlightLicenseInputPath = path.join(projectDirectory, "node_modules", "@highlightjs", "cdn-assets", "LICENSE");
const highlightLicenseOutputPath = path.join(vendorDirectory, "highlight.LICENSE");
const themeNames = ["yuluo-css", "github", "whitey", "newsprint", "pixyll", "gothic", "night"];

const markdownSource = (await readFile(markdownPath, "utf8"))
  .replace(/^export\s+/gm, "");
const appSource = (await readFile(appPath, "utf8"))
  .replace(/^import\s+\{[^\n]+\}\s+from\s+["']\.\/markdown\.js["'];?\s*\n/, "")
  .replaceAll("import.meta.url", "document.baseURI");

const banner = `/* 此文件由 scripts/build-web.mjs 生成，请勿直接编辑。\n` +
  ` * 经典脚本用于兼容 file:// 下不执行 ES module 的 macOS WKWebView。 */\n`;
const mermaidRuntime = await readFile(mermaidInputPath, "utf8");
const mermaidLicense = await readFile(mermaidLicenseInputPath, "utf8");
const highlightRuntime = await readFile(highlightInputPath, "utf8");
const highlightLicense = await readFile(highlightLicenseInputPath, "utf8");
const themeCSS = Object.fromEntries(await Promise.all(themeNames.map(async name => [name, await readFile(path.join(webDirectory, "themes", `${name}.css`), "utf8")])));
const themeBootstrap = `globalThis.__MORY_THEME_CSS__ = ${JSON.stringify(themeCSS)};`;
const bundle = `${banner}\n${themeBootstrap}\n\n${markdownSource}\n\n${appSource}`;

if (process.argv.includes("--check")) {
  const current = await readFile(outputPath, "utf8").catch(() => "");
  const currentMermaid = await readFile(mermaidOutputPath, "utf8").catch(() => "");
  const currentLicense = await readFile(mermaidLicenseOutputPath, "utf8").catch(() => "");
  const currentHighlight = await readFile(highlightOutputPath, "utf8").catch(() => "");
  const currentHighlightLicense = await readFile(highlightLicenseOutputPath, "utf8").catch(() => "");
  if (current !== bundle || currentMermaid !== mermaidRuntime || currentLicense !== mermaidLicense
    || currentHighlight !== highlightRuntime || currentHighlightLicense !== highlightLicense) {
    console.error("Web 运行包已过期，请执行 npm run build:web");
    process.exit(1);
  }
} else {
  await mkdir(vendorDirectory, { recursive: true });
  await writeFile(outputPath, bundle, "utf8");
  await writeFile(mermaidOutputPath, mermaidRuntime, "utf8");
  await writeFile(mermaidLicenseOutputPath, mermaidLicense, "utf8");
  await writeFile(highlightOutputPath, highlightRuntime, "utf8");
  await writeFile(highlightLicenseOutputPath, highlightLicense, "utf8");
  console.log(`已生成 ${path.relative(projectDirectory, outputPath)}`);
  console.log(`已同步 Mermaid 运行时 ${path.relative(projectDirectory, mermaidOutputPath)}`);
  console.log(`已同步 Highlight.js 运行时 ${path.relative(projectDirectory, highlightOutputPath)}`);
}
