async function runDocumentLifecycleCases() {
  const passed = [];
  const failures = [];
  const check = (condition, message) => { if (!condition) throw new Error(message); };
  const currentId = () => document.querySelector('.file-item.is-active')?.dataset.documentId;
  const edit = markdown => {
    window.Mory.toggleSource(true);
    const editor = document.querySelector('#source-editor');
    editor.value = markdown;
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
  };
  const run = async (name, test) => {
    try { await test(); passed.push(name); }
    catch (error) { failures.push({ name, error: error.message }); }
  };
  await run('saving an older snapshot preserves text typed while saving', () => {
    window.Mory.openDocument({ path: '/lifecycle/edit.md', markdown: 'saved snapshot' });
    const id = currentId();
    edit('saved snapshot plus new text');
    window.Mory.didSave({ documentId: id, path: '/lifecycle/edit.md', markdown: 'saved snapshot', sourceMarkdown: 'saved snapshot' });
    check(window.Mory.getMarkdown() === 'saved snapshot plus new text', 'A save completion replaced newer text');
    check(document.querySelector('#save-state').classList.contains('is-visible'), 'Newer edits were marked saved');
  });
  await run('a delayed save updates its document without switching the active tab', () => {
    window.Mory.openDocument({ path: '/lifecycle/first.md', markdown: 'first' });
    const id = currentId();
    window.Mory.openDocument({ path: '/lifecycle/second.md', markdown: 'second' });
    const activeId = currentId();
    edit('second unsaved');
    window.Mory.didSave({ documentId: id, path: '/lifecycle/first.md', markdown: 'first', sourceMarkdown: 'first' });
    check(currentId() === activeId, 'A save completion changed the active tab');
    check(document.querySelector('.file-item.is-active').dataset.path === '/lifecycle/second.md', 'A save completion reassigned another document path');
    check(window.Mory.getMarkdown() === 'second unsaved', 'A save completion replaced another document');
  });
  await run('a save completion for a closed document does not modify its successor', () => {
    window.Mory.openDocument({ path: '/lifecycle/closed.md', markdown: 'closed' });
    const id = currentId();
    window.Mory.closeDocument(id);
    window.Mory.openDocument({ path: '/lifecycle/successor.md', markdown: 'successor' });
    window.Mory.didSave({ documentId: id, path: '/lifecycle/closed.md', markdown: 'closed', sourceMarkdown: 'closed' });
    check(window.Mory.getMarkdown() === 'successor', 'A closed document save replaced its successor');
  });
  await run('reopening an edited document preserves unsaved text and history', () => {
    window.Mory.openDocument({ path: '/lifecycle/reopen.md', markdown: 'disk' });
    edit('unsaved edits');
    window.Mory.openDocument({ path: '/lifecycle/reopen.md', markdown: 'disk' });
    check(window.Mory.getMarkdown() === 'unsaved edits', 'Reopening discarded unsaved edits');
  });
  await run('a matching save snapshot can apply relocated image paths', () => {
    window.Mory.openDocument({ path: '/lifecycle/old.md', markdown: '![cover](old/cover.png)' });
    const id = currentId();
    window.Mory.didSave({ documentId: id, path: '/lifecycle/new.md', sourceMarkdown: '![cover](old/cover.png)', markdown: '![cover](new/cover.png)' });
    check(window.Mory.getMarkdown() === '![cover](new/cover.png)', 'Saved image paths were not applied');
    check(document.querySelector('.file-item.is-active').dataset.path === '/lifecycle/new.md', 'Save As did not update the source path');
  });
  await run('Save As rebases image paths while preserving text typed during saving', () => {
    window.Mory.openDocument({ path: '/lifecycle/rebase.md', markdown: '![cover](rebase/cover.png)' });
    const original = window.Mory.getDocumentSnapshot();
    edit('New text ![cover](rebase/cover.png)');
    window.Mory.didSave({ documentId: original.documentId, path: '/elsewhere/copy.md', name: 'copy.md', sourceMarkdown: original.markdown, markdown: '![cover](copy/cover.png)', assetPathChanges: { 'rebase/': 'copy/' } });
    check(window.Mory.getMarkdown() === 'New text ![cover](copy/cover.png)', 'Save As lost newer text or left a broken image path');
    check(document.querySelector('#save-state').classList.contains('is-visible'), 'Unsaved edits were marked saved');
  });
  await run('saving a document does not discard another dirty document with the same path', () => {
    window.Mory.openDocument({ path: '/lifecycle/collision.md', markdown: 'target' });
    edit('unsaved target');
    const target = window.Mory.getDocumentSnapshot();
    window.Mory.openDocument({ path: '/lifecycle/from.md', markdown: 'source' });
    const source = window.Mory.getDocumentSnapshot();
    window.Mory.didSave({ documentId: source.documentId, path: target.path, sourceMarkdown: 'source', markdown: 'source' });
    check(window.Mory.getDocumentSnapshot(target.documentId)?.markdown === 'unsaved target', 'Save As discarded another dirty buffer');
  });
  await run('save snapshots preserve untouched raw Markdown in preview mode', () => {
    const source = 'A   paragraph\n\n\n<a href="https://example.com" title="Link">Link</a>\n';
    window.Mory.openDocument({ path: '/lifecycle/raw.md', markdown: source });
    window.Mory.toggleSource(false);
    check(window.Mory.getMarkdown() === source, 'Reading the document normalized its original syntax');
    check(window.Mory.getDocumentSnapshot().markdown === source, 'A save snapshot normalized the source');
  });
  for (const action of ['switch', 'edit', 'close', 'source mode', 'save as', 'batch save as', 'partial failure', 'undo']) {
    await run(`image import preserves the intended document after ${action}`, async () => {
      window.Mory.openDocument({ path: `/lifecycle/image-${action}.md`, markdown: 'Original paragraph' });
      const original = window.Mory.getDocumentSnapshot();
      window.Mory.toggleSource(false);
      const editor = document.querySelector('#write');
      const range = document.createRange();
      range.selectNodeContents(editor.querySelector('p'));
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      editor.focus();
      let finish;
      let started;
      const received = new Promise(resolve => { started = resolve; });
      const previousHost = window.moryNative;
      let requests = 0;
      window.moryNative = { send() {}, request(method, args) {
        if (method !== 'importImage') return Promise.resolve({});
        requests += 1;
        if (action === 'partial failure' && requests === 1) return Promise.reject(new Error('First image failed'));
        if (action === 'batch save as' && requests === 1) return Promise.resolve({ relative: 'assets/first.png', dataURL: 'data:image/png;base64,aW1hZ2U=' });
        if ((action === 'save as' && requests >= 2) || (action === 'batch save as' && requests >= 3)) {
          check(args.documentPath === '/lifecycle/import-copy.md', 'Retried import used the old document path');
          return Promise.resolve({ relative: `import-copy/${args.name}`, dataURL: 'data:image/png;base64,aW1hZ2U=' });
        }
        started();
        return new Promise(resolve => { finish = resolve; });
      } };
      try {
        const transfer = new DataTransfer();
        if (action === 'partial failure') transfer.items.add(new File(['bad'], 'bad.png', { type: 'image/png' }));
        if (action === 'batch save as') transfer.items.add(new File(['first'], 'first.png', { type: 'image/png' }));
        transfer.items.add(new File(['image'], 'cover.png', { type: 'image/png' }));
        editor.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: transfer }));
        await received;
        if (action === 'switch' || action === 'close') {
          if (action === 'close') window.Mory.closeDocument(original.documentId);
          window.Mory.openDocument({ path: `/lifecycle/other-${action}.md`, markdown: 'Other document' });
        } else if (action === 'edit') edit('Original paragraph plus edits');
        else if (action === 'source mode') window.Mory.toggleSource(true);
        else if (action.includes('save as')) window.Mory.didSave({ ...original, path: '/lifecycle/import-copy.md', name: 'import-copy.md', sourceMarkdown: original.markdown });
        finish({ relative: 'assets/cover.png', dataURL: 'data:image/png;base64,aW1hZ2U=' });
        await new Promise(resolve => {
          const channel = new MessageChannel();
          channel.port1.onmessage = () => { channel.port1.close(); channel.port2.close(); resolve(); };
          channel.port2.postMessage(null);
        });
        if (action === 'switch' || action === 'close') {
          check(window.Mory.getMarkdown() === 'Other document', 'Image import modified another document');
        }
        const saved = window.Mory.getDocumentSnapshot(original.documentId);
        if (action !== 'close') {
          check(saved.markdown.includes(action.includes('save as') ? 'import-copy/cover.png' : 'assets/cover.png'), 'Image import was lost from the original document');
          if (action === 'batch save as') check(saved.markdown.includes('import-copy/first.png') && !saved.markdown.includes('assets/first.png'), 'An earlier image in the batch retained its old location');
          if (action === 'edit') check(saved.markdown.includes('plus edits'), 'Image import lost concurrent edits');
          if (action === 'partial failure') check(!saved.markdown.includes('bad.png'), 'A failed image was inserted');
          if (action === 'undo') {
            window.Mory.undo();
            check(window.Mory.getMarkdown() === original.markdown, 'Image import could not be undone');
          }
        } else check(saved === null, 'Image import resurrected a closed document');
      } finally { window.moryNative = previousHost; }
    });
  }
  return { passed, failures };
}
