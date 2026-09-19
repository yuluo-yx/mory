const { app, BrowserWindow } = require('electron');
const assert = require('node:assert/strict');
const path = require('node:path');
app.disableHardwareAcceleration();
app.commandLine.appendSwitch('disable-gpu');

async function runWorkspaceMessages() {
  const results = [];
  const workspace = { activeId: 'test', workspaces: [{ id: 'test', name: 'Test', provider: 's3', localPath: '/virtual/test' }] };
  let failure = false;
  window.moryNative = { send() {}, request: async method => {
    if (failure) throw new Error('Fixture failure');
    if (method === 'syncWorkspace') return { files: 2 };
    return workspace;
  } };
  const settle = async predicate => {
    for (let attempts = 0; attempts < 100; attempts++) {
      if (predicate()) return;
      await new Promise(resolve => setTimeout(resolve, 10));
    }
    throw new Error('Workspace message did not settle');
  };
  const toast = document.querySelector('#toast');
  for (const language of ['en', 'zh-CN']) {
    window.Mory.setWorkspaceState(workspace);
    const select = document.querySelector('#language-select');
    select.value = language;
    select.dispatchEvent(new Event('change'));
    const english = language === 'en';
    const check = (name, actual, expected) => results.push({ language, name, actual, expected });
    for (const action of ['pull', 'push']) {
      for (const fails of [false, true]) {
        failure = fails;
        toast.textContent = '';
        const button = document.querySelector('#workspace-' + action);
        button.click();
        check(action + ' progress', button.textContent, english ? (action === 'push' ? 'Pushing\u2026' : 'Pulling\u2026') : (action === 'push' ? '\u6B63\u5728\u63A8\u9001\u2026' : '\u6B63\u5728\u62C9\u53D6\u2026'));
        await settle(() => !button.disabled);
        check(action + ' result', toast.textContent, english
          ? (fails ? 'Sync failed: Fixture failure' : 'Sync complete: 2 files')
          : (fails ? '\u540C\u6B65\u5931\u8D25\uFF1AFixture failure' : '\u540C\u6B65\u5B8C\u6210\uFF1A2 \u4E2A\u6587\u4EF6'));
      }
    }
    for (const fails of [false, true]) {
      failure = fails;
      document.querySelector('#workspace-button').click();
      toast.textContent = '';
      document.querySelector('#workspace-form').dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
      await settle(() => toast.textContent !== '');
      check('save configuration', toast.textContent, english
        ? (fails ? 'Save failed: Fixture failure' : 'Workspace configuration saved')
        : (fails ? '\u4FDD\u5B58\u5931\u8D25\uFF1AFixture failure' : '\u5DE5\u4F5C\u533A\u914D\u7F6E\u5DF2\u4FDD\u5B58'));
    }
    failure = false;
    document.querySelector('#workspace-button').click();
    let confirmation;
    window.confirm = value => { confirmation = value; return true; };
    toast.textContent = '';
    document.querySelector('#workspace-remove').click();
    await settle(() => toast.textContent !== '');
    check('remove confirmation', confirmation, english ? 'Delete this workspace configuration? Local files will be kept.' : '\u786E\u5B9A\u5220\u9664\u8FD9\u4E2A\u5DE5\u4F5C\u533A\u914D\u7F6E\u5417\uFF1F\u672C\u5730\u6587\u4EF6\u4E0D\u4F1A\u88AB\u5220\u9664\u3002');
    check('remove result', toast.textContent, english ? 'Workspace configuration deleted' : '\u5DE5\u4F5C\u533A\u914D\u7F6E\u5DF2\u5220\u9664');
    toast.textContent = '';
    document.querySelector('#workspace-open-local').click();
    await settle(() => toast.textContent !== '');
    check('choose local folder', toast.textContent, english ? 'Local working folder set' : '\u5DF2\u8BBE\u7F6E\u672C\u5730\u5DE5\u4F5C\u76EE\u5F55');
  }
  return results;
}

app.whenReady().then(async () => {
  const timeout = setTimeout(() => { console.error('Workspace localization timed out'); app.exit(1); }, 30000);
  try {
    const window = new BrowserWindow({ show: false, webPreferences: { partition: `workspace-locale-${process.pid}`, backgroundThrottling: false } });
    await window.loadFile(path.join(__dirname, '../Sources/Mory/Web/index.html'));
    const results = await window.webContents.executeJavaScript(`(${runWorkspaceMessages.toString()})()`, true);
    for (const result of results) assert.equal(result.actual, result.expected, `${result.language}: ${result.name}`);
    console.log(`Workspace localization passed: ${results.length} bilingual interaction assertions`);
    clearTimeout(timeout);
    app.exit(0);
  } catch (error) { console.error(error); app.exit(1); }
});
