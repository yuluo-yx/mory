const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { app, BrowserWindow } = require("electron");

app.disableHardwareAcceleration();
app.whenReady().then(async () => {
  const window = new BrowserWindow({ show: false, webPreferences: { contextIsolation: true, nodeIntegration: false, partition: `mory-lifecycle-${process.pid}` } });
  try {
    await window.loadFile(path.join(__dirname, "../Sources/Mory/Web/index.html"));
    const cases = await fs.readFile(path.join(__dirname, "document-lifecycle-cases.js"), "utf8");
    const result = await window.webContents.executeJavaScript(`${cases}\nrunDocumentLifecycleCases()`, true);
    process.stdout.write(JSON.stringify(result, null, 2) + "\n");
    assert.deepEqual(result.failures, [], "Document lifecycle scenarios failed");
    const draftWindow = new BrowserWindow({ show: false, webPreferences: { contextIsolation: true, nodeIntegration: false, partition: `mory-restored-draft-${process.pid}` } });
    const index = path.join(__dirname, "../Sources/Mory/Web/index.html");
    await draftWindow.loadFile(index);
    await draftWindow.webContents.executeJavaScript("localStorage.removeItem('mory.recovery'); localStorage.setItem('mory.draft', 'Recovered notes')");
    await draftWindow.loadFile(index);
    const preservedDraft = await draftWindow.webContents.executeJavaScript(`(() => {
      const draft = [...document.querySelectorAll('#file-list .file-item')].map(item => window.Mory.getDocumentSnapshot(item.dataset.documentId)).find(item => item.markdown === 'Recovered notes');
      window.Mory.openDocument({ path: '/lifecycle/external.md', markdown: 'External file' });
      return draft.markdown === 'Recovered notes'
        && window.Mory.getDocumentSnapshot(draft.documentId)?.markdown === 'Recovered notes'
        && document.querySelectorAll('#file-list .file-item').length === 2;
    })()`, true);
    assert.equal(preservedDraft, true, "Opening a file discarded a recovered browser draft");
    process.stdout.write("Recovered browser draft preserved after opening a file\n");
    app.exit(0);
  } catch (error) {
    process.stderr.write(String(error.stack || error) + "\n");
    app.exit(1);
  }
});
