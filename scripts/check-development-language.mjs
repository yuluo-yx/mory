import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const projectDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const hanPattern = /\p{Script=Han}/u;
const textExtensions = new Set([".cjs", ".js", ".json", ".mjs", ".plist", ".ps1", ".sh", ".yaml", ".yml"]);
const sourceExtensions = new Set([".cjs", ".css", ".go", ".js", ".mjs", ".swift"]);

async function filesWithin(relativePath) {
  const absolutePath = path.join(projectDirectory, relativePath);
  const entries = await readdir(absolutePath, { withFileTypes: true });
  const files = await Promise.all(entries.map(entry => {
    const child = path.join(relativePath, entry.name);
    return entry.isDirectory() ? filesWithin(child) : [child];
  }));
  return files.flat();
}

function hasChineseComment(line) {
  if (!hanPattern.test(line)) return false;
  return /^\s*(?:\/\/|\/\*|\*)/.test(line) || /\/\*.*\*\//.test(line);
}

function hasEscapedDeveloperText(line, extension) {
  if (!/\\u(?:\{[0-9A-Fa-f]+\}|[0-9A-Fa-f]{4})/.test(line)) return false;
  if (extension === ".go") {
    return /\bname:\s*"\\u/.test(line)
      || /\bt\.(?:Fatal|Fatalf|Error|Errorf|Log|Logf)\(/.test(line);
  }
  return /\b(?:test|it|describe)\(\s*["'`]/.test(line)
    || /(?:throw new Error|finish\(failure:|\bprint\(|process\.(?:stdout|stderr)\.write)/.test(line);
}

const strictFiles = [
  "Makefile",
  "package.json",
  "macOS/Info.plist",
  ...(await filesWithin(".github")),
  ...(await filesWithin("scripts"))
].filter(file => path.basename(file) === "Makefile" || textExtensions.has(path.extname(file)));

const sourceFiles = [
  "webassets.go",
  ...(await filesWithin("Electron")),
  ...(await filesWithin("Sources")),
  ...(await filesWithin("Tests")),
  ...(await filesWithin("cmd")),
  ...(await filesWithin("internal"))
].filter(file => sourceExtensions.has(path.extname(file))
  && file !== "Sources/Mory/Web/app.bundle.js"
  && !file.startsWith("Sources/Mory/Web/vendor/"));

const testFiles = sourceFiles.filter(file => file.startsWith("Tests/") || file.endsWith("_test.go"));

const failures = new Set();
for (const file of strictFiles) {
  const contents = await readFile(path.join(projectDirectory, file), "utf8");
  contents.split("\n").forEach((line, index) => {
    if (hanPattern.test(line)) failures.add(`${file}:${index + 1}: ${line.trim()}`);
  });
}

for (const file of sourceFiles) {
  const contents = await readFile(path.join(projectDirectory, file), "utf8");
  contents.split("\n").forEach((line, index) => {
    if (hasChineseComment(line)) failures.add(`${file}:${index + 1}: ${line.trim()}`);
  });
}

for (const file of testFiles) {
  const contents = await readFile(path.join(projectDirectory, file), "utf8");
  contents.split("\n").forEach((line, index) => {
    if (hanPattern.test(line)) failures.add(`${file}:${index + 1}: ${line.trim()}`);
    if (hasEscapedDeveloperText(line, path.extname(file))) {
      failures.add(`${file}:${index + 1}: ${line.trim()}`);
    }
  });
}

if (failures.size) {
  console.error("Developer-facing metadata, build files, source comments, and test code must use English:\n");
  console.error([...failures].join("\n"));
  process.exit(1);
}

console.log("Development-language check passed.");
