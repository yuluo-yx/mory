const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const test = require('node:test');
const { hostMessage } = require('../Electron/host-localization.cjs');
const messages = require('../Sources/Mory/Web/host-messages.json');

test('shared host messages translate English labels and preserve Chinese and external details', () => {
  for (const [chinese, english] of Object.entries(messages)) {
    assert.equal(hostMessage(chinese, 'en'), english);
    assert.equal(hostMessage(chinese, 'zh-CN'), chinese);
    assert.equal(hostMessage(chinese + '\u3002', 'en'), english);
  }
  assert.equal(hostMessage('\u5BFC\u51FA\u5931\u8D25\uFF1AENOENT /notes/file.md', 'en'), 'Export failed: ENOENT /notes/file.md');
  assert.equal(hostMessage('ENOENT /notes/file.md', 'en'), 'ENOENT /notes/file.md');
  assert.notEqual(hostMessage('Path must remain inside the selected directory.', 'zh-CN'), 'Path must remain inside the selected directory.');
});

test('Electron About and folder error dialogs follow the selected interface language', async () => {
  const mainPath = path.join(__dirname, '../Electron/main.cjs');
  const mainRequire = createRequire(mainPath);
  const dialogs = [];
  let menu;
  const context = vm.createContext({
    console, process, __dirname: path.dirname(mainPath),
    require(name) {
      if (name === 'electron') return {
        app: { on() {}, requestSingleInstanceLock: () => true, whenReady: () => new Promise(() => {}), getVersion: () => 'test' },
        ipcMain: { on() {}, handle() {} },
        Menu: { buildFromTemplate: value => value, setApplicationMenu: value => { menu = value; } },
        dialog: { showMessageBox: async (_window, options) => { dialogs.push(options); return { response: 0 }; } }
      };
      if (name === './recent-documents.cjs') return { listRecentDocuments: () => [] };
      return mainRequire(name);
    }
  });
  vm.runInContext(fs.readFileSync(mainPath, 'utf8'), context);
  for (const locale of ['en', 'zh-CN']) {
    vm.runInContext(`interfaceLocale = '${locale}'; buildMenu()`, context);
    const about = menu.at(-1).submenu[1];
    await about.click();
    assert.equal(dialogs.at(-1).title, locale === 'en' ? 'About Mory' : '\u5173\u4E8E Mory');
    assert.equal(dialogs.at(-1).detail, locale === 'en' ? 'A focused, cross-platform Markdown editor' : '\u4E00\u4E2A\u8DE8\u5E73\u53F0\u3001\u4E13\u6CE8\u7684 Markdown \u7F16\u8F91\u5668\u3002');
    await vm.runInContext("openWorkspaceFolder('/nonexistent-mory-audit-folder')", context);
    assert.equal(dialogs.at(-1).message, locale === 'en' ? 'Unable to read folder' : '\u65E0\u6CD5\u8BFB\u53D6\u6587\u4EF6\u5939');
  }
});
