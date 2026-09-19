const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { writeAtomicFile } = require('../Electron/atomic-file.cjs');

async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'mory-atomic-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  return { root, target: path.join(root, 'note.md') };
}

test('completed saves replace content and preserve existing permissions and file links', async t => {
  const { root, target } = await fixture(t);
  await writeAtomicFile(target, 'first');
  await fs.chmod(target, 0o600);
  const originalMode = (await fs.stat(target)).mode & 0o777;
  const link = path.join(root, 'alias.md');
  await fs.symlink(target, link);
  await writeAtomicFile(link, 'second');
  assert.equal(await fs.readFile(target, 'utf8'), 'second');
  assert.equal((await fs.stat(target)).mode & 0o777, originalMode);
  assert.equal((await fs.lstat(link)).isSymbolicLink(), true);
  assert.deepEqual((await fs.readdir(root)).sort(), ['alias.md', 'note.md']);
});

test('write, flush, close and replacement failures preserve the original and remove temporary files', async t => {
  for (const phase of ['writeFile', 'chmod', 'sync', 'close', 'rename']) {
    const { root, target } = await fixture(t);
    await fs.writeFile(target, 'original', { mode: 0o600 });
    const failure = new Error(`Injected ${phase} failure`);
    const failingFS = { ...fs,
      rename: async (...args) => { if (phase === 'rename') throw failure; return fs.rename(...args); },
      open: async (...args) => {
        const file = await fs.open(...args);
        return new Proxy(file, { get(object, key) {
          const value = Reflect.get(object, key);
          if (key === phase) return async (...values) => {
            if (phase === 'writeFile') await file.writeFile('partial');
            if (phase === 'close') await file.close();
            throw failure;
          };
          return typeof value === 'function' ? value.bind(object) : value;
        } });
      }
    };
    await assert.rejects(writeAtomicFile(target, 'replacement', failingFS), failure);
    assert.equal(await fs.readFile(target, 'utf8'), 'original', phase);
    assert.deepEqual(await fs.readdir(root), ['note.md'], phase);
  }
});

test('invalid targets, denied access and dangling links fail before changing any entry', async t => {
  const { root, target } = await fixture(t);
  await assert.rejects(writeAtomicFile(root, 'replacement'), /regular file/);
  await fs.symlink(path.join(root, 'missing'), target);
  await assert.rejects(writeAtomicFile(target, 'replacement'), { code: 'ENOENT' });
  assert.equal((await fs.lstat(target)).isSymbolicLink(), true);
  const failure = Object.assign(new Error('Permission denied'), { code: 'EACCES' });
  await assert.rejects(writeAtomicFile(path.join(root, 'other'), '', { ...fs, realpath: async () => { throw failure; } }), failure);
  await assert.rejects(writeAtomicFile(path.join(root, 'other'), '', { ...fs, lstat: async () => { throw failure; } }), failure);
  await assert.rejects(writeAtomicFile(path.join(root, 'missing-parent/note.md'), ''), { code: 'ENOENT' });
});
