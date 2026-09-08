const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');

const mainPath = path.join(__dirname, '../Electron/main.cjs');
const mainRequire = createRequire(mainPath);

async function run() {
  const source = await fs.readFile(mainPath, 'utf8');
  const root = await fs.mkdtemp(path.join(__dirname, '../.cache/electron-save-'));
  let count = 0;
  try {
    for (const scenario of ['switch', 'edit', 'stale changed', 'cancel', 'closed', 'write failure', 'queued writes']) {
      const directory = path.join(root, scenario);
      await fs.mkdir(directory);
      const original = { documentId: 'original', path: path.join(directory, 'original.md'), name: 'original.md', markdown: 'captured text' };
      const destination = path.join(directory, 'saved.md');
      const callbacks = [];
      const errors = [];
      const handlers = new Map();
      let snapshot = original;
      let context;
      let firstWrite;
      let releaseWrite;
      const writeStarted = new Promise(resolve => { firstWrite = resolve; });
      const writeReleased = new Promise(resolve => { releaseWrite = resolve; });
      let writes = 0;
      const electron = {
        app: { on() {}, requestSingleInstanceLock: () => true, whenReady: () => new Promise(() => {}) },
        ipcMain: { on: (name, handler) => handlers.set(name, handler), handle() {} },
        dialog: {
          async showSaveDialog() {
            if (scenario === 'switch') await select({ documentId: 'other', path: path.join(directory, 'other.md'), name: 'other.md', markdown: 'other text' });
            if (scenario === 'edit') await handlers.get('mory:message')(null, { type: 'changed', documentId: original.documentId, markdown: 'newer text' });
            if (scenario === 'stale changed') await handlers.get('mory:message')(null, { type: 'changed', documentId: 'closed', markdown: 'stale text' });
            return { canceled: scenario === 'cancel', filePath: destination };
          },
          async showMessageBox(_window, options) { errors.push(options); return { response: 2 }; }
        }
      };
      const harness = {
        manager: { activeRoot: () => directory, active: () => ({ isImplicit: false }), state: () => ({}) },
        window: { isDestroyed: () => false, setTitle() {}, webContents: { async executeJavaScript(code) {
          if (code.startsWith('window.Mory.getDocumentSnapshot(')) return scenario === 'closed' ? null : { ...snapshot };
          if (code.startsWith('window.Mory.didSave(')) callbacks.push(JSON.parse(code.slice('window.Mory.didSave('.length, -1)));
          return null;
        } } }
      };
      context = vm.createContext({
        console, process, __dirname: path.dirname(mainPath), harness,
        require(name) {
          if (name === 'electron') return electron;
          if (name === './workspace-watcher.cjs') return { createWorkspaceWatcher: () => ({ start() {}, stop() {} }) };
          if (name === './recent-documents.cjs') return { addRecentDocument: () => false };
          if (name === 'node:fs/promises') return { ...fs, async writeFile(...args) {
            writes += 1;
            if (scenario === 'write failure' && writes === 1) throw new Error('Simulated disk failure');
            if (scenario === 'queued writes' && writes === 1) { firstWrite(); await writeReleased; }
            return fs.writeFile(...args);
          } };
          return mainRequire(name);
        }
      });
      vm.runInContext(source, context, { filename: mainPath });
      vm.runInContext('mainWindow = harness.window; workspaceManager = harness.manager;', context);
      async function select(document) {
        snapshot = document;
        await handlers.get('mory:message')(null, { type: 'documentSelected', ...document });
      }
      await select(original);
      if (scenario === 'queued writes') {
        const first = vm.runInContext('saveDocument()', context);
        await writeStarted;
        await select({ ...original, markdown: 'second text' });
        const second = vm.runInContext('saveDocument()', context);
        releaseWrite();
        await Promise.all([first, second]);
        assert.equal(await fs.readFile(original.path, 'utf8'), 'second text', 'An older save overtook a newer save');
        assert.deepEqual(callbacks.map(value => value.markdown), ['captured text', 'second text']);
      } else {
        await vm.runInContext('runSaveAction(saveAs)', context);
        if (scenario === 'cancel' || scenario === 'closed' || scenario === 'write failure') {
          await assert.rejects(fs.stat(destination), { code: 'ENOENT' });
          assert.equal(callbacks.length, 0, 'An unsuccessful save reported success');
          assert.equal(errors.length, scenario === 'cancel' ? 0 : 1);
          if (scenario === 'write failure') {
            await vm.runInContext('runSaveAction(saveAs)', context);
            assert.equal(await fs.readFile(destination, 'utf8'), original.markdown, 'The save queue did not recover');
          }
        } else {
          assert.equal(errors.length, 0, JSON.stringify(errors));
          assert.equal(await fs.readFile(destination, 'utf8'), original.markdown, 'Save As wrote a later or different document');
          assert.equal(callbacks[0].documentId, original.documentId);
          assert.equal(callbacks[0].sourceMarkdown, original.markdown);
          const current = vm.runInContext('({ id: currentDocumentId, markdown: currentMarkdown, path: currentFilePath })', context);
          assert.equal(current.id, scenario === 'switch' ? 'other' : original.documentId);
          assert.equal(current.markdown, scenario === 'switch' ? 'other text' : scenario === 'edit' ? 'newer text' : original.markdown);
          assert.equal(errors.length, 0);
        }
      }
      count += 1;
      process.stdout.write(`Electron save passed: ${scenario}\n`);
    }
    process.stdout.write(`Electron save lifecycle passed: ${count} scenarios\n`);
  } finally { await fs.rm(root, { recursive: true, force: true }); }
}

run().catch(error => { process.stderr.write(String(error.stack || error) + '\n'); process.exitCode = 1; });
