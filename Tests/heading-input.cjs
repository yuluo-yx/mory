const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { app, BrowserWindow } = require('electron');

app.disableHardwareAcceleration();
app.whenReady().then(async () => {
  const window = new BrowserWindow({ show: false, webPreferences: { contextIsolation: true, nodeIntegration: false } });
  const timeout = setTimeout(() => { process.stderr.write('Heading input timed out\n'); app.exit(1); }, 45000);
  try {
    await window.loadFile(process.env.MORY_WEB_INDEX || path.join(__dirname, '../Sources/Mory/Web/index.html'));
    const cases = await fs.readFile(path.join(__dirname, 'heading-input-cases.js'), 'utf8');
    const result = await window.webContents.executeJavaScript(`${cases}\nrunHeadingInputCases()`, true);
    process.stdout.write(JSON.stringify(result, null, 2) + '\n');
    assert.deepEqual(result.failures, [], 'Heading input scenarios failed');
    clearTimeout(timeout);
    app.exit(0);
  } catch (error) {
    process.stderr.write(String(error.stack || error) + '\n');
    app.exit(1);
  }
});
