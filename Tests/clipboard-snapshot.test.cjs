const assert = require('node:assert/strict');
const test = require('node:test');
const { snapshotClipboard } = require('./clipboard-snapshot.cjs');

class ClipboardItem {
  constructor(data) {
    assert.ok(Object.keys(data).length, 'Clipboard items require at least one MIME type');
    this.data = data;
  }
}

test('clipboard snapshots accept empty lists and empty system items', async () => {
  for (const items of [[], [{ types: [] }], [{ types: [] }, { types: [] }]]) {
    assert.deepEqual(await snapshotClipboard({ read: async () => items }, ClipboardItem), []);
  }
});

test('clipboard snapshots preserve all formats and materialize lazy data before replacement', async () => {
  let source = { 'text/plain': 'Original text', 'text/html': '<b>Original text</b>' };
  const items = [{ types: [] }, {
    types: Object.keys(source),
    getType: async type => source[type]
  }, { types: ['image/png'], getType: async () => Buffer.from([1, 2, 3]) }];
  const snapshot = await snapshotClipboard({ read: async () => items }, ClipboardItem);
  source = { 'text/plain': 'New text', 'text/html': '<i>New text</i>' };
  assert.deepEqual(snapshot.map(item => item.data), [
    { 'text/plain': 'Original text', 'text/html': '<b>Original text</b>' },
    { 'image/png': Buffer.from([1, 2, 3]) }
  ]);
});

test('clipboard snapshots propagate read failures before replacing user data', async () => {
  const error = new Error('Clipboard read failed');
  await assert.rejects(snapshotClipboard({ read: async () => { throw error; } }, ClipboardItem), error);
  await assert.rejects(snapshotClipboard({ read: async () => [{
    types: ['text/plain'], getType: async () => { throw error; }
  }] }, ClipboardItem), error);
});
