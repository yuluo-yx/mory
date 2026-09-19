const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const workspace = require('../Electron/workspaces.cjs');
const { inlineThemeAssets } = require('../Electron/themes.cjs');

async function fixture(t) {
  const base = await fs.mkdtemp(path.join(os.tmpdir(), 'mory-boundary-'));
  t.after(() => fs.rm(base, { recursive: true, force: true }));
  const root = path.join(base, 'root');
  const outside = path.join(base, 'outside');
  await fs.mkdir(path.join(root, 'inside'), { recursive: true });
  await fs.mkdir(outside);
  await fs.writeFile(path.join(outside, 'secret.png'), 'outside');
  await fs.writeFile(path.join(root, 'inside', 'safe.png'), 'inside');
  await fs.writeFile(path.join(root, 'note.md'), 'original');
  await fs.symlink(outside, path.join(root, 'escape'), 'junction');
  await fs.symlink(path.join(root, 'inside'), path.join(root, 'alias'), 'junction');
  return { root, outside };
}

test('workspace operations reject symlink escapes before modifying any file', async t => {
  const { root, outside } = await fixture(t);
  const escape = path.join(root, 'escape');
  const note = path.join(root, 'note.md');
  await assert.rejects(workspace.createWorkspaceDirectory(root, 'escape/new/nested'));
  await assert.rejects(workspace.createWorkspaceDocument(root, escape, 'new.md'));
  await assert.rejects(workspace.readDocumentImage(root, path.join(escape, 'secret.png')));
  await assert.rejects(workspace.copyWorkspaceEntry(root, note, escape));
  await assert.rejects(workspace.moveWorkspaceEntry(root, note, escape));
  assert.equal(await fs.readFile(note, 'utf8'), 'original');
  assert.deepEqual(await fs.readdir(outside), ['secret.png']);
});

test('document and theme resources cannot embed bytes through an external symlink', async t => {
  const { root } = await fixture(t);
  const assets = await workspace.loadDocumentAssets(path.join(root, 'note.md'), '![secret](escape/secret.png)', root);
  assert.deepEqual(assets, {});
  assert.equal(await inlineThemeAssets('p { background: url(escape/secret.png) }', root), 'p { background: url(escape/secret.png) }');
  await fs.symlink(path.join(root, 'escape'), path.join(root, 'note'), 'junction');
  assert.deepEqual(await workspace.listDocumentImages(path.join(root, 'note.md'), 'original'), []);
  await assert.rejects(workspace.importImage({ root, documentPath: path.join(root, 'note.md'), documentName: 'note.md', name: 'new.png', mime: 'image/png', data: Buffer.from('new').toString('base64') }));
});

test('internal symlinks and a symlinked workspace root remain usable', async t => {
  const { root } = await fixture(t);
  const rootAlias = path.join(path.dirname(root), 'root-alias');
  await fs.symlink(root, rootAlias, 'junction');
  for (const entryRoot of [root, rootAlias]) {
    const image = await workspace.readDocumentImage(entryRoot, path.join(entryRoot, 'alias', 'safe.png'));
    assert.ok(image.dataURL.endsWith(Buffer.from('inside').toString('base64')));
    await workspace.createWorkspaceDirectory(entryRoot, 'alias/new/nested');
    const document = await workspace.createWorkspaceDocument(entryRoot, path.join(entryRoot, 'alias', 'new'), 'new.md');
    assert.equal(await fs.readFile(document.path, 'utf8'), '');
  }
});

test('dangling and cyclic links cannot become new workspace destinations', async t => {
  const { root, outside } = await fixture(t);
  await fs.symlink(path.join(outside, 'missing'), path.join(root, 'dangling'), 'junction');
  await fs.symlink(path.join(root, 'cycle'), path.join(root, 'cycle'), 'junction');
  await assert.rejects(workspace.createWorkspaceDirectory(root, 'dangling/child'));
  await assert.rejects(workspace.createWorkspaceDirectory(root, 'cycle/child'));
  assert.deepEqual(await fs.readdir(outside), ['secret.png']);
});

test('unsafe companion assets reject copy, move and Save As before changing the note', async t => {
  const { root, outside } = await fixture(t);
  const note = path.join(root, 'note.md');
  await fs.mkdir(path.join(root, 'destination'));
  await fs.mkdir(path.join(root, 'note'));
  await fs.symlink(path.join(outside, 'secret.png'), path.join(root, 'note', 'secret.png'));
  await assert.rejects(workspace.copyWorkspaceEntry(root, note, path.join(root, 'destination')));
  await assert.rejects(workspace.moveWorkspaceEntry(root, note, path.join(root, 'destination')));
  await assert.rejects(workspace.relocateDocumentAssets({ root, oldPath: note, oldName: 'note.md', newPath: path.join(root, 'destination', 'copy.md'), markdown: '![x](note/secret.png)' }));
  assert.equal(await fs.readFile(note, 'utf8'), 'original');
  assert.deepEqual(await fs.readdir(path.join(root, 'destination')), []);
});

test('standalone document imports remain confined to their own directory', async t => {
  const { root, outside } = await fixture(t);
  const options = { root, documentPath: path.join(outside, 'standalone.md'), documentName: 'standalone.md', name: 'new.png', mime: 'image/png', data: Buffer.from('new').toString('base64') };
  const image = await workspace.importImage(options);
  assert.equal(await fs.readFile(path.join(outside, image.relative), 'utf8'), 'new');
  await fs.symlink(root, path.join(outside, 'escape-assets'), 'junction');
  await assert.rejects(workspace.importImage({ ...options, documentName: 'escape-assets.md' }));
});
