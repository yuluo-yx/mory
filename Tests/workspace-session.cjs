const assert = require('node:assert/strict');
const path = require('node:path');
const { app, BrowserWindow } = require('electron');
app.disableHardwareAcceleration();
app.on('window-all-closed', () => {});
const index = path.join(__dirname, '../Sources/Mory/Web/index.html');
const read = (window, script) => window.webContents.executeJavaScript(script, true);
const snapshot = window => read(window, `({ markdown: window.Mory.getMarkdown(), files: [...document.querySelectorAll('#file-list .file-item')].map(item => window.Mory.getDocumentSnapshot(item.dataset.documentId)), workspace: document.querySelector('#workspace-select').value, events: window.__sessionEvents, recent: JSON.parse(localStorage.getItem('mory.recentWorkspaces')), recovery: JSON.parse(localStorage.getItem('mory.recovery')) })`);
let sequence = 0;
async function launch(fixture) {
  const window = new BrowserWindow({ show: false, webPreferences: {
    contextIsolation: false, nodeIntegration: false, sandbox: false,
    partition: `mory-startup-${process.pid}-${++sequence}`,
    preload: path.join(__dirname, 'workspace-session-preload.cjs'),
    additionalArguments: [`--mory-session-fixture=${JSON.stringify(fixture)}`]
  } });
  await window.loadFile(index);
  return window;
}
app.whenReady().then(async () => {
  const timeout = setTimeout(() => { process.stderr.write('Workspace startup regression timed out\n'); app.exit(1); }, 60000);
  try {
    const first = await launch({ existing: false });
    assert.match((await snapshot(first)).markdown, /Mory/);
    await first.loadFile(index);
    assert.equal((await snapshot(first)).markdown, '');
    await read(first, 'window.Mory.showIntroduction()');
    assert.match((await snapshot(first)).markdown, /Mory/);
    first.destroy();

    const upgraded = await launch({ existing: true });
    let state = await snapshot(upgraded);
    assert.equal(state.markdown, '');
    assert.equal(state.workspace, '');
    assert.equal(state.files.length, 1);
    assert.deepEqual(state.recent, ['alpha', 'beta', 'missing']);
    assert.equal(state.events.some(item => item.type === 'openFile'), false);
    await read(upgraded, `window.Mory.newFolder()`);
    assert.equal(await read(upgraded, `document.querySelector('#preferences').classList.contains('is-open')`), true);
    await read(upgraded, `window.Mory.openRecentWorkspace('alpha')`);
    state = await snapshot(upgraded);
    assert.equal(state.workspace, 'alpha');
    assert.equal(state.markdown, '', 'A first visit must not choose the first file');
    await read(upgraded, `window.Mory.openDocument({path:'/workspaces/alpha/last.md',markdown:'# Last read'}); window.Mory.openRecentWorkspace('beta')`);
    await read(upgraded, `window.Mory.openRecentWorkspace('alpha')`);
    state = await snapshot(upgraded);
    assert.equal(state.files.find(item => item?.path === '/workspaces/alpha/last.md').markdown, '# last.md');
    assert.equal(state.recent[0], 'alpha');
    await upgraded.loadFile(index);
    state = await snapshot(upgraded);
    assert.equal(state.workspace, '');
    assert.equal(state.markdown, '');
    await read(upgraded, `window.Mory.openRecentWorkspace('alpha')`);
    assert.equal((await snapshot(upgraded)).markdown, '# last.md');
    await read(upgraded, `window.Mory.setWorkspaceSnapshot({state:{activeId:'alpha',workspaces:[{id:'alpha',name:'Alpha',provider:'local',localPath:'/workspaces/alpha'}]},files:[{path:'/workspaces/alpha/first.md',name:'first.md'}]})`);
    assert.equal((await snapshot(upgraded)).markdown, '', 'A deleted last document must fall back to a blank draft');
    await read(upgraded, `window.Mory.openRecentWorkspace('missing')`);
    assert.equal((await snapshot(upgraded)).workspace, 'alpha', 'A missing workspace must not replace the current session');
    await read(upgraded, `window.Mory.removeRecentWorkspace('missing'); window.Mory.clearRecentWorkspaces()`);
    await upgraded.loadFile(index);
    assert.deepEqual((await snapshot(upgraded)).recent, []);
    upgraded.destroy();

    const external = await launch({ existing: true, openFile: true });
    state = await snapshot(external);
    assert.equal(state.files.length, 1);
    assert.equal(state.markdown, '# External note');
    assert.equal(state.workspace, '');
    external.destroy();

    const recovery = await launch({ existing: true });
    await read(recovery, `window.Mory.toggleSource(true); const source = document.querySelector('#source-editor'); source.value = 'Unsaved notes'; source.dispatchEvent(new InputEvent('input', {bubbles:true,inputType:'insertText'}))`);
    assert.equal((await snapshot(recovery)).recovery[0].markdown, 'Unsaved notes');
    await read(recovery, `window.Mory.openRecentWorkspace('beta')`);
    assert.ok((await snapshot(recovery)).files.some(item => item?.markdown === 'Unsaved notes'));
    await recovery.loadFile(index);
    state = await snapshot(recovery);
    assert.equal(state.markdown, '', 'Recovery must not replace the new blank document');
    const recovered = state.files.find(item => item?.markdown === 'Unsaved notes');
    assert.ok(recovered);
    await read(recovery, `window.Mory.didSave({documentId:${JSON.stringify(recovered.documentId)},path:'/saved/notes.md',markdown:'Unsaved notes',sourceMarkdown:'Unsaved notes'})`);
    assert.deepEqual((await snapshot(recovery)).recovery, []);
    await recovery.loadFile(index);
    assert.equal((await snapshot(recovery)).files.length, 1, 'Saved recovery buffers must not return');
    recovery.destroy();
    const limited = await launch({ existing: true });
    await read(limited, `
      const setItem = Storage.prototype.setItem;
      Storage.prototype.setItem = function(key, value) {
        if (key === 'mory.recovery') throw new DOMException('Storage full', 'QuotaExceededError');
        return setItem.call(this, key, value);
      };
      window.Mory.toggleSource(true);
      const source = document.querySelector('#source-editor');
      source.value = 'Save must still receive this text';
      source.dispatchEvent(new InputEvent('input', {bubbles:true,inputType:'insertText'}));
    `);
    state = await snapshot(limited);
    assert.equal(state.markdown, 'Save must still receive this text');
    assert.ok(state.events.some(event => event.type === 'changed' && event.markdown === state.markdown));
    assert.ok(await read(limited, `document.querySelector('#toast').textContent.length > 0`));
    limited.destroy();
    clearTimeout(timeout);
    process.stdout.write('Workspace startup passed: first use, relaunch, migration, external open, recent history, missing folders, last document, recovery and save\n');
    app.exit(0);
  } catch (error) { process.stderr.write(String(error.stack || error) + '\n'); app.exit(1); }
});
