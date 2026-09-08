const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { app, BrowserWindow } = require("electron");

app.disableHardwareAcceleration();
app.whenReady().then(async () => {
  const window = new BrowserWindow({ show: false, webPreferences: { contextIsolation: true, nodeIntegration: false } });
  try {
    await window.loadFile(path.join(__dirname, "../Sources/Mory/Web/index.html"));
    const cases = await fs.readFile(path.join(__dirname, "document-lifecycle-cases.js"), "utf8");
    const result = await window.webContents.executeJavaScript(`${cases}\nrunDocumentLifecycleCases()`, true);
    process.stdout.write(JSON.stringify(result, null, 2) + "\n");
    assert.deepEqual(result.failures, [], "Document lifecycle scenarios failed");
    app.exit(0);
  } catch (error) {
    process.stderr.write(String(error.stack || error) + "\n");
    app.exit(1);
  }
});
