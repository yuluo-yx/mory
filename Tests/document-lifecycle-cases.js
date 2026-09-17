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
  const startupMarkdown = window.Mory.getMarkdown();
  const visibleFiles = () => [...document.querySelectorAll('#file-list .file-item')];
  let workspaceSequence = 0;
  const emptyWorkspace = () => {
    for (const item of visibleFiles()) {
      if (item.dataset.documentId) window.Mory.closeDocument(item.dataset.documentId);
    }
    const id = `lifecycle-workspace-${++workspaceSequence}`;
    window.Mory.setWorkspaceSnapshot({ state: { activeId: id, workspaces: [{ id, name: 'Test', provider: 'local' }] }, files: [], directories: [] });
  };
  await run('opening a file replaces the untouched startup introduction', () => {
    const introduction = window.Mory.getDocumentSnapshot();
    check(visibleFiles().length === 1 && startupMarkdown.includes('Mory'), 'The startup introduction was not present');
    window.Mory.openDocument({ path: '/lifecycle/startup.md', markdown: '# Opened file' });
    check(visibleFiles().length === 1, 'Opening a file retained an extra introduction entry');
    check(window.Mory.getDocumentSnapshot(introduction.documentId) === null, 'The introduction remained in the document session');
    check(window.Mory.getMarkdown() === '# Opened file', 'The requested file was not activated');
  });
  // Initialize the workspace identity; subsequent changes create fresh empty-workspace placeholders.
  emptyWorkspace();
  for (const order of ['before opening', 'after opening']) {
    await run(`empty workspace refresh ${order} does not retain a placeholder`, () => {
      emptyWorkspace();
      const placeholder = window.Mory.getDocumentSnapshot();
      if (order === 'before opening') window.Mory.setFiles([]);
      window.Mory.openDocument({ path: `/lifecycle/${order}.md`, markdown: '# External file' });
      if (order === 'after opening') window.Mory.setFiles([]);
      check(visibleFiles().length === 1, 'An empty workspace left an extra placeholder');
      check(window.Mory.getDocumentSnapshot(placeholder.documentId) === null, 'Placeholder cleanup only hid the entry');
      check(window.Mory.getMarkdown() === '# External file', 'Workspace refresh changed the requested file');
    });
  }
  await run('opening files preserves an edited introduction and its undo history', () => {
    emptyWorkspace();
    window.Mory.loadMarkdown(startupMarkdown);
    const introduction = window.Mory.getDocumentSnapshot();
    window.Mory.toggleSource(true);
    document.querySelector('#source-editor').dispatchEvent(new InputEvent('beforeinput', { bubbles: true, inputType: 'insertText' }));
    edit('My notes in the introduction');
    window.Mory.openDocument({ path: '/lifecycle/keep-introduction.md', markdown: 'Opened' });
    check(visibleFiles().length === 2, 'Opening a file removed the edited introduction');
    check(window.Mory.getDocumentSnapshot(introduction.documentId)?.markdown === 'My notes in the introduction', 'Opening discarded introduction edits');
    document.querySelector(`#file-list .file-item[data-document-id="${introduction.documentId}"]`).click();
    window.Mory.undo();
    check(window.Mory.getMarkdown() === startupMarkdown, 'Opening discarded introduction undo history');
  });
  await run('opening a file preserves an explicitly created empty draft', () => {
    emptyWorkspace();
    window.Mory.newDocument();
    const draft = window.Mory.getDocumentSnapshot();
    window.Mory.openDocument({ path: '/lifecycle/keep-draft.md', markdown: 'Opened' });
    check(visibleFiles().length === 2, 'Opening removed an explicit draft or retained the placeholder');
    check(window.Mory.getDocumentSnapshot(draft.documentId)?.markdown === '', 'The explicit draft was discarded');
  });
  await run('opening another file preserves a saved introduction', () => {
    emptyWorkspace();
    window.Mory.loadMarkdown(startupMarkdown);
    const introduction = window.Mory.getDocumentSnapshot();
    window.Mory.didSave({ ...introduction, path: '/lifecycle/introduction.md', name: 'introduction.md', sourceMarkdown: introduction.markdown });
    window.Mory.openDocument({ path: '/lifecycle/keep-saved.md', markdown: 'Opened' });
    check(visibleFiles().length === 2, 'Opening removed the saved introduction');
    check(window.Mory.getDocumentSnapshot(introduction.documentId)?.path === '/lifecycle/introduction.md', 'The saved introduction was discarded');
  });
  await run('repeated external opens keep only the requested documents', () => {
    emptyWorkspace();
    window.Mory.openDocument({ path: '/lifecycle/one.md', markdown: 'One' });
    window.Mory.openDocument({ path: '/lifecycle/two.md', markdown: 'Two' });
    window.Mory.openDocument({ path: '/lifecycle/one.md', markdown: 'One' });
    check(visibleFiles().length === 2, 'Opening files added a placeholder or duplicate entry');
    check(window.Mory.getMarkdown() === 'One', 'Reopening did not activate the requested file');
    window.Mory.closeDocument(currentId());
    window.Mory.closeDocument(currentId());
    check(visibleFiles().length === 1 && window.Mory.getMarkdown() === '', 'Closing files restored the introduction');
  });
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
