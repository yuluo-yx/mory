const { app, BrowserWindow } = require("electron");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");

app.commandLine.appendSwitch("disable-gpu");
app.disableHardwareAcceleration();

const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
let interactionCount = 0;

async function capturePNG(window) {
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    try {
      return (await window.webContents.capturePage()).toPNG();
    } catch (error) {
      const isTransientVizError = String(error?.message || error).includes("UnknownVizError");
      if (!isTransientVizError || attempt === 3) throw error;
      await wait(attempt * 150);
    }
  }
  throw new Error("Screenshot capture exhausted all attempts");
}

async function loadAndWait(window) {
  const loaded = new Promise((resolve, reject) => {
    window.webContents.once("did-finish-load", resolve);
    window.webContents.once("did-fail-load", (_event, code, message) => reject(new Error(`${code}: ${message}`)));
  });
  await window.loadFile(path.join(__dirname, "..", "Sources", "Mory", "Web", "index.html"));
  await loaded;
  await window.webContents.executeJavaScript("document.fonts.ready");
}

async function inspect(window, expression) {
  return window.webContents.executeJavaScript(expression, true);
}

async function click(window, selector) {
  let target = await inspect(window, `(() => {
    const element = document.querySelector(${JSON.stringify(selector)});
    if (!element) throw new Error(${JSON.stringify(`Element not found: ${selector}`)});
    element.scrollIntoView({ block: "center", inline: "center" });
    const rect = element.getBoundingClientRect();
    const x = Math.round(rect.left + rect.width / 2);
    const y = Math.round(rect.top + rect.height / 2);
    const hit = document.elementFromPoint(x, y);
    return { x, y, hit: hit?.id || hit?.dataset?.command || hit?.className || hit?.tagName };
  })()`);
  let hittable = false;
  for (let attempt = 0; attempt < 3 && !hittable; attempt += 1) {
    window.webContents.sendInputEvent({ type: "mouseMove", x: target.x, y: target.y });
    await wait(60);
    target = await inspect(window, `(() => {
      const expected = document.querySelector(${JSON.stringify(selector)});
      if (!expected) throw new Error(${JSON.stringify(`Element not found: ${selector}`)});
      const rect = expected.getBoundingClientRect();
      const x = Math.round(rect.left + rect.width / 2);
      const y = Math.round(rect.top + rect.height / 2);
      const actual = document.elementFromPoint(x, y);
      return {
        x,
        y,
        hittable: Boolean(actual && (expected === actual || expected.contains(actual))),
        hit: actual?.id || actual?.dataset?.command || actual?.className || actual?.tagName
      };
    })()`);
    hittable = target.hittable;
  }
  if (!hittable) throw new Error(`Click target is obscured: ${selector}; final hit ${target.hit}`);
  window.webContents.sendInputEvent({ type: "mouseDown", x: target.x, y: target.y, button: "left", clickCount: 1 });
  window.webContents.sendInputEvent({ type: "mouseUp", x: target.x, y: target.y, button: "left", clickCount: 1 });
  await wait(selector === "#sidebar-toggle" ? 280 : 80);
  return target;
}

async function hover(window, selector) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const point = await inspect(window, `(() => {
      const element = document.querySelector(${JSON.stringify(selector)});
      if (!element) throw new Error(${JSON.stringify(`Element not found: ${selector}`)});
      const rect = element.getBoundingClientRect();
      return { x: Math.round(rect.left + rect.width / 2), y: Math.round(rect.top + rect.height / 2) };
    })()`);
    // Hosted runners may retain stale pointer coordinates after resizing an offscreen window.
    window.webContents.sendInputEvent({ type: "mouseMove", x: 1, y: 1 });
    await wait(50);
    window.webContents.sendInputEvent({ type: "mouseMove", x: point.x, y: point.y });
    await wait(100);
    if (await inspect(window, "document.querySelector('#toolbar-tooltip').classList.contains('is-visible')")) return;
  }
  // Offscreen Intel renderers may not synthesize hardware pointer input; DOM events still cover event delegation.
  await inspect(window, `(() => {
    const element = document.querySelector(${JSON.stringify(selector)});
    return element.dispatchEvent(new MouseEvent('mouseover', { bubbles: true, relatedTarget: document.body }));
  })()`);
  await wait(50);
}

async function insertParagraph(window) {
  await inspect(window, `document.querySelector('#write').dispatchEvent(new InputEvent('beforeinput', {
    bubbles: true,
    cancelable: true,
    inputType: 'insertParagraph'
  }))`);
  await wait(40);
}

async function expect(window, label, expression) {
  const result = await inspect(window, expression);
  if (!result) throw new Error(`${label}: state did not change`);
  interactionCount += 1;
  process.stdout.write(`[button] ${label}\n`);
}

async function expectEventually(window, label, expression, timeout = 2000) {
  const deadline = Date.now() + timeout;
  do {
    if (await inspect(window, expression)) {
      interactionCount += 1;
      process.stdout.write(`[button] ${label}\n`);
      return;
    }
    await wait(50);
  } while (Date.now() < deadline);
  throw new Error(`${label}: state did not change within ${timeout} ms`);
}

app.whenReady().then(async () => {
  let exitCode = 0;
  const errors = [];
  const window = new BrowserWindow({
    show: false,
    width: 1180,
    height: 790,
    webPreferences: { sandbox: true, offscreen: true, partition: `mory-buttons-${process.pid}` }
  });
  window.webContents.on("console-message", event => {
    if (event.level === "error") errors.push(event.message);
  });
  window.webContents.on("unresponsive", () => errors.push("Renderer process became unresponsive"));

  try {
    await loadAndWait(window);
    await expect(window, "application script initializes", "typeof window.Mory === 'object' && document.querySelector('#write').textContent.includes('Mory')");
    await expect(window, "heading buttons are removed", "!document.querySelector('#toolbar [data-command=\"h1\"]') && !document.querySelector('#toolbar [data-command=\"h2\"]') && !document.querySelector('#toolbar [data-command=\"paragraph\"]')");
    await expect(window, "floating toolbar stays vertical in the lower-right corner", "(() => { const element = document.querySelector('#toolbar'); const bar = element.getBoundingClientRect(); return getComputedStyle(element).flexDirection === 'column' && bar.height > bar.width * 3 && innerWidth - bar.right < 24 && innerHeight - bar.bottom < 52 && document.querySelector('#toolbar #source-toggle') && document.querySelector('#toolbar #export-button'); })()");
    await expect(window, "toolbar shows icons only by default", "[...document.querySelectorAll('#toolbar button')].every(button => button.dataset.tooltip && button.getAttribute('aria-label') && !/\u5217\u8868|\u6E90\u7801|\u5BFC\u51FA/.test(button.textContent))");
    await inspect(window, "(() => { const read = selector => parseFloat(getComputedStyle(document.querySelector(selector)).fontSize); window.__normalTypography = { write: read('#write'), file: read('.file-item'), tab: read('.tab'), status: read('#statusbar') }; })()");
    window.setSize(2048, 1056);
    await wait(140);
    await expect(window, "wide viewports preserve CSS font sizes and only expand whitespace", "(() => { const read = selector => parseFloat(getComputedStyle(document.querySelector(selector)).fontSize); const stable = (selector, key) => Math.abs(read(selector) - window.__normalTypography[key]) < .1; return innerWidth >= 2000 && stable('#write', 'write') && stable('.file-item', 'file') && stable('.tab', 'tab') && stable('#statusbar', 'status') && document.querySelector('#write').getBoundingClientRect().width <= 821; })()");
    window.setSize(1180, 790);
    await wait(140);
    await hover(window, "#toolbar button[data-command='ul']");
    await expectEventually(window, "toolbar hover reveals a text tooltip", "document.querySelector('#toolbar-tooltip').classList.contains('is-visible') && document.querySelector('#toolbar-tooltip').textContent === '\u65E0\u5E8F\u5217\u8868'");
    await expect(window, "sidebar search tab is removed", "!document.querySelector('.tab[data-panel=\"search\"]') && !document.querySelector('#search-panel-side') && !document.querySelector('#side-search-input')");
    const screenshotPath = path.join(os.tmpdir(), "mory-ui-e2e.png");
    const preferencesScreenshotPath = path.join(os.tmpdir(), "mory-preferences-e2e.png");
    const codeMetaScreenshotPath = path.join(os.tmpdir(), "mory-code-meta-e2e.png");
    const lapisCVScreenshotPath = path.join(os.tmpdir(), "mory-lapis-cv-e2e.png");
    const darkThemeScreenshotPath = path.join(os.tmpdir(), "mory-dark-theme-e2e.png");
    const mermaidScreenshotPath = path.join(os.tmpdir(), "mory-mermaid-e2e.png");
    const mermaidExpandedScreenshotPath = path.join(os.tmpdir(), "mory-mermaid-expanded-e2e.png");
    const mermaidNarrowScreenshotPath = path.join(os.tmpdir(), "mory-mermaid-narrow-e2e.png");
    const calendarDirectScreenshotPath = path.join(os.tmpdir(), "mory-calendar-direct-e2e.png");
    await fs.writeFile(screenshotPath, await capturePNG(window));

    await click(window, ".tab[data-panel='outline']");
    await expect(window, "outline tab is clickable", "document.querySelector('#outline-panel').classList.contains('is-active')");
    await click(window, ".tab[data-panel='files']");
    await inspect(window, `(() => {
      window.moryNative = {
        send(payload) {
          window.__lastNativeMessage = payload;
          if (payload.type === 'openFile') window.__autoOpenedWorkspacePath = payload.path;
        },
        request(method, args) {
          window.__lastHostRequest = { method, args };
          if (method === 'deleteDocument') return Promise.resolve(window.__deleteDocumentResult || { deleted: true });
          if (method === 'deleteWorkspaceEntry') return Promise.resolve(window.__deleteWorkspaceEntryResult || { deleted: true });
          if (method === 'createDirectory') return Promise.resolve({ name: args.relativePath, path: '/virtual/' + args.relativePath, createdAt: Date.now() });
          if (method === 'createDocument') return Promise.resolve({ name: '\u672A\u547D\u540D.md', path: args.directoryPath + '/\u672A\u547D\u540D.md', markdown: '', images: [] });
          if (method === 'copyWorkspaceEntry') return Promise.resolve({ name: '\u526F\u672C', path: (args.destinationPath || '/opened') + '/\u526F\u672C', sourcePath: args.path, isDirectory: !args.path.endsWith('.md') });
          if (method === 'moveWorkspaceEntry') {
            window.__lastMoveRequest = { method, args };
            return Promise.resolve({ name: '\u79FB\u52A8\u9879', path: (args.destinationPath || '/opened') + '/\u79FB\u52A8\u9879.md', sourcePath: args.path, isDirectory: !args.path.endsWith('.md') });
          }
          if (method === 'renameWorkspaceEntry') return Promise.resolve({ name: args.name, path: args.path.replace(/[^/]+$/, args.name), sourcePath: args.path, isDirectory: !args.path.endsWith('.md') });
          if (method === 'importImage') return Promise.resolve({ relative: '\u6587\u7AE0/cover.png', dataURL: 'data:image/png;base64,iVBORw0KGgo=' });
          if (method === 'documentAssets') return Promise.resolve({ '\u6587\u7AE0/late.svg': 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjwvc3ZnPg==', './photo_1.svg': 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxIDEiPjxyZWN0IHdpZHRoPSIxIiBoZWlnaHQ9IjEiIGZpbGw9IiM0ODcwYWQiLz48L3N2Zz4=' });
          if (method === 'openExternal') {
            window.__openedExternalURL = args.url;
            return Promise.resolve({ opened: true });
          }
          if (method === 'documentImage') return Promise.resolve({ dataURL: 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjwvc3ZnPg==' });
          if (method === 'readDocument') return Promise.resolve({ name: args.path.split('/').at(-1), path: args.path, markdown: '# \u7B2C\u4E00\u7BC7', assets: {} });
          if (method === 'revealFile') return Promise.resolve({ revealed: true });
          if (method === 'chooseThemeFolder') return Promise.resolve({ directory: '/themes', themes: [{ id: 'user-folder-test', name: '\u76EE\u5F55\u4E3B\u9898', css: '#write{word-spacing:2px}' }] });
          return Promise.reject(new Error('The test host does not implement this request'));
        }
      };
      return true;
    })()`);

    await click(window, "#new-folder-button");
    await expect(window, "new-folder action reveals the inline path input", "!document.querySelector('#new-folder-form').hidden && document.activeElement === document.querySelector('#new-folder-input')");
    await window.webContents.insertText("\u8D44\u6599/\u9879\u76EE A");
    await click(window, "#new-folder-form button[type='submit']");
    await expectEventually(window, "active workspace creates and displays nested directories", "window.__lastHostRequest.method === 'createDirectory' && window.__lastHostRequest.args.relativePath === '\u8D44\u6599/\u9879\u76EE A' && document.querySelector('.folder-item[title=\"\u8D44\u6599/\u9879\u76EE A\"] .folder-name').textContent === '\u9879\u76EE A' && document.querySelector('#new-folder-form').hidden");

    const pastedMarkdown = '# \u7C98\u8D34\u6807\u9898\n\n**\u7C98\u8D34\u52A0\u7C97**\n\n```go\nfmt.Println("paste")\n```';
    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
    })()`);
    const firstDraftId = await inspect(window, "document.querySelector('.file-item.is-active').dataset.documentId");
    await window.webContents.insertText("\u8349\u7A3F\u4E00");
    await click(window, "#new-file-button");
    const secondDraftId = await inspect(window, "document.querySelector('.file-item.is-active').dataset.documentId");
    await window.webContents.insertText("\u8349\u7A3F\u4E8C");
    await expect(window, "multiple untitled documents are listed separately", "document.querySelectorAll('#file-list .file-item[data-document-id]').length === 2 && [...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '\u672A\u547D\u540D.md|\u672A\u547D\u540D 2.md'");
    await expect(window, "untitled documents use distinct identifiers", `${JSON.stringify(firstDraftId)} !== ${JSON.stringify(secondDraftId)} && document.querySelectorAll('#file-list .file-dirty:not([hidden])').length === 2`);
    await click(window, `[data-document-id=${JSON.stringify(firstDraftId)}]`);
    await expect(window, "switching to the first draft preserves its content", "document.querySelector('#write').textContent === '\u8349\u7A3F\u4E00' && document.querySelector('.file-item.is-active .file-name').textContent === '\u672A\u547D\u540D.md'");
    await click(window, `[data-document-id=${JSON.stringify(secondDraftId)}]`);
    await expect(window, "switching to the second draft preserves its content", "document.querySelector('#write').textContent === '\u8349\u7A3F\u4E8C' && document.querySelector('.file-item.is-active .file-name').textContent === '\u672A\u547D\u540D 2.md'");
    await inspect(window, "window.Mory.didSave({ path: '/tmp/\u8349\u7A3F\u4E8C.md', name: '\u8349\u7A3F\u4E8C.md' })");
    await expect(window, "saving renames only the active document and leaves drafts last", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '\u8349\u7A3F\u4E8C.md|\u672A\u547D\u540D.md' && document.querySelector('.file-item.is-active').dataset.path === '/tmp/\u8349\u7A3F\u4E8C.md'");
    await expect(window, "drafts and saved documents expose the correct remove and delete actions", `document.querySelector('.file-close[data-document-id=${JSON.stringify(firstDraftId)}]').getAttribute('aria-label') === '\u79FB\u9664\u8349\u7A3F' && document.querySelector('.file-close[data-document-id=${JSON.stringify(secondDraftId)}]').getAttribute('aria-label') === '\u5220\u9664\u6587\u6863'`);
    await click(window, `.file-close[data-document-id=${JSON.stringify(firstDraftId)}]`);
    await expect(window, "an inactive untitled draft can be removed", "document.querySelectorAll('#file-list .file-item[data-document-id]').length === 1 && document.querySelector('#write').textContent === '\u8349\u7A3F\u4E8C' && document.querySelector('.file-item.is-active .file-name').textContent === '\u8349\u7A3F\u4E8C.md'");
    await click(window, "#new-file-button");
    const thirdDraftId = await inspect(window, "document.querySelector('.file-item.is-active').dataset.documentId");
    await window.webContents.insertText("\u5F85\u79FB\u9664");
    await click(window, `.file-close[data-document-id=${JSON.stringify(thirdDraftId)}]`);
    await expect(window, "closing the active draft selects an adjacent document", "document.querySelectorAll('#file-list .file-item[data-document-id]').length === 1 && document.querySelector('#write').textContent === '\u8349\u7A3F\u4E8C' && document.querySelector('.file-item.is-active .file-name').textContent === '\u8349\u7A3F\u4E8C.md'");
    await inspect(window, "window.__deleteDocumentResult = { canceled: true }");
    await click(window, `.file-close[data-document-id=${JSON.stringify(secondDraftId)}]`);
    await expect(window, "cancelling deletion keeps the document on disk", "document.querySelector('.file-item.is-active .file-name').textContent === '\u8349\u7A3F\u4E8C.md'");
    await inspect(window, "window.__deleteDocumentResult = { deleted: true }");
    await click(window, `.file-close[data-document-id=${JSON.stringify(secondDraftId)}]`);
    await expectEventually(window, "confirming deletion removes the document and creates a blank draft", "window.__lastHostRequest.method === 'deleteDocument' && window.__lastHostRequest.args.path === '/tmp/\u8349\u7A3F\u4E8C.md' && document.querySelectorAll('#file-list .file-item[data-document-id]').length === 1 && document.querySelector('.file-item.is-active .file-name').textContent === '\u672A\u547D\u540D.md' && document.querySelector('#write').textContent === '' && document.querySelector('#toast').textContent === '\u6587\u6863\u5DF2\u79FB\u5230\u5E9F\u7EB8\u7BD3'");

    await inspect(window, `window.Mory.setWorkspaceSnapshot({
      state: { activeId: 'workspace-empty', workspaces: [{ id: 'workspace-empty', name: '\u7A7A\u76EE\u5F55', provider: 'local', localPath: '/empty' }] },
      files: []
    })`);
    await expect(window, "an empty workspace keeps an untitled placeholder", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '\u672A\u547D\u540D.md' && document.querySelector('.file-item.is-active')");
    await inspect(window, `window.Mory.setWorkspaceSnapshot({
      state: { activeId: 'workspace-opened', workspaces: [{ id: 'workspace-opened', name: '\u5DF2\u6253\u5F00\u76EE\u5F55', provider: 'local', localPath: '/opened' }] },
      files: [{ name: '\u5B50\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md', path: '/opened/\u5B50\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md', createdAt: 20 }, { name: '\u7B2C\u4E00\u7BC7.md', path: '/opened/\u7B2C\u4E00\u7BC7.md', createdAt: 10, images: [{ name: '\u5C01\u9762.svg', path: '/opened/\u7B2C\u4E00\u7BC7/\u5C01\u9762.svg', relative: '\u7B2C\u4E00\u7BC7/\u5C01\u9762.svg' }] }],
      directories: [{ name: '\u5B50\u76EE\u5F55', path: '/opened/\u5B50\u76EE\u5F55', createdAt: 5 }]
    })`);
    await expect(window, "a non-empty workspace removes the placeholder and opens the first sorted document", "document.querySelector('#folder-name').textContent === '\u5DF2\u6253\u5F00\u76EE\u5F55' && [...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '\u7B2C\u4E00\u7BC7.md|\u7B2C\u4E8C\u7BC7.md' && window.__autoOpenedWorkspacePath === '/opened/\u7B2C\u4E00\u7BC7.md'");
    await inspect(window, `window.Mory.openDocument({ name: '\u7B2C\u4E00\u7BC7.md', path: '/opened/\u7B2C\u4E00\u7BC7.md', markdown: '# \u7B2C\u4E00\u7BC7' })`);
    await expect(window, "the first sorted workspace document becomes active", "document.querySelector('.file-item.is-active .file-name').textContent === '\u7B2C\u4E00\u7BC7.md' && document.querySelector('#write h1').textContent === '\u7B2C\u4E00\u7BC7'");
    await inspect(window, "window.Mory.newDocument()");
    const orderedDraftId = await inspect(window, "document.querySelector('.file-item.is-active').dataset.documentId");
    await expect(window, "new untitled documents appear at the end of the tree", "document.querySelector('#file-list .file-row:last-child .file-item')?.dataset.documentId === " + JSON.stringify(orderedDraftId));
    await expect(window, "document rows support drag reordering", "document.querySelector('.file-item[data-document-id=\"" + orderedDraftId + "\"]').draggable === true");
    await inspect(window, `(() => {
      const source = document.querySelector('.file-item[data-document-id=${JSON.stringify(orderedDraftId)}]');
      const target = document.querySelector('.file-item[data-path="/opened/\u7B2C\u4E00\u7BC7.md"]');
      source.dispatchEvent(new DragEvent('dragstart', { bubbles: true }));
      target.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, clientY: target.getBoundingClientRect().top + 1 }));
      target.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, clientY: target.getBoundingClientRect().top + 1 }));
    })()`);
    await expect(window, "dragging moves an untitled document before the target", "document.querySelector('#file-list .file-item')?.dataset.documentId === " + JSON.stringify(orderedDraftId));
    await inspect(window, `window.Mory.closeDocument(${JSON.stringify(orderedDraftId)})`);
    await expect(window, "expanded directory trees preserve document creation order", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '\u7B2C\u4E00\u7BC7.md|\u7B2C\u4E8C\u7BC7.md'");
    await click(window, "#new-file-button");
    const freshDraftId = await inspect(window, "document.querySelector('.file-item.is-active').dataset.documentId");
    await expect(window, "new drafts remain last when a manual order exists", "document.querySelector('#file-list .file-row:last-child .file-item')?.dataset.documentId === " + JSON.stringify(freshDraftId));
    await inspect(window, `window.Mory.closeDocument(${JSON.stringify(freshDraftId)})`);
    await expect(window, "directories show disclosure controls and expand existing content", "document.querySelector('.folder-toggle[aria-expanded=\"true\"]') && document.querySelector('.folder-item[data-path=\"/opened/\u5B50\u76EE\u5F55\"]') && document.querySelector('.file-item[data-path=\"/opened/\u5B50\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md\"]')");
    await expect(window, "folder names receive visible size and weight compensation", "(() => { const folder = getComputedStyle(document.querySelector('.folder-item')); const file = getComputedStyle(document.querySelector('.file-item')); return parseFloat(folder.fontSize) >= parseFloat(file.fontSize) + 0.75 && parseFloat(folder.fontWeight) >= parseFloat(file.fontWeight) + 50 && Math.abs(parseFloat(folder.minHeight) - parseFloat(file.minHeight)) < 0.1; })()");
    await click(window, ".folder-toggle");
    await expect(window, "collapsing a directory hides child documents", "document.querySelector('.folder-toggle').getAttribute('aria-expanded') === 'false' && !document.querySelector('.file-item[data-path=\"/opened/\u5B50\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md\"]')");
    await click(window, ".folder-toggle");
    await click(window, ".folder-item[data-path='/opened/\u5B50\u76EE\u5F55']");
    await expect(window, "directories can be selected", "document.querySelector('.folder-item[data-path=\"/opened/\u5B50\u76EE\u5F55\"]').classList.contains('is-selected')");
    await expect(window, "directories and documents share a single selection state", "document.querySelectorAll('#file-list .folder-item.is-selected, #file-list .file-item.is-active, #file-list .file-item.is-selected').length === 1 && !document.querySelector('#file-list .file-item.is-active')");
    await inspect(window, "document.querySelector('.folder-item[data-path=\"/opened/\u5B50\u76EE\u5F55\"]').dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }))");
    await expect(window, "Enter opens rename for the selected directory", "document.querySelector('#entry-operation-dialog').classList.contains('is-open') && !document.querySelector('#entry-operation-name-row').hidden && document.querySelector('#entry-operation-name').value === '\u5B50\u76EE\u5F55'");
    await inspect(window, "document.querySelector('#entry-operation-name').value = '\u8D44\u6599\u76EE\u5F55'");
    await click(window, "#entry-operation-confirm");
    await expectEventually(window, "directory rename uses the shared workspace contract", "window.__lastHostRequest.method === 'renameWorkspaceEntry' && window.__lastHostRequest.args.path === '/opened/\u5B50\u76EE\u5F55' && window.__lastHostRequest.args.name === '\u8D44\u6599\u76EE\u5F55'");
    await expect(window, "renaming a directory immediately migrates child document paths", "document.querySelector('.folder-item[data-path=\"/opened/\u8D44\u6599\u76EE\u5F55\"]') && document.querySelector('.file-item[data-path=\"/opened/\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md\"]') && !document.querySelector('.folder-item[data-path=\"/opened/\u5B50\u76EE\u5F55\"]')");
    await click(window, "#new-file-button");
    await expectEventually(window, "the add button creates a document in the renamed directory", "window.__lastHostRequest.method === 'createDocument' && window.__lastHostRequest.args.directoryPath === '/opened/\u8D44\u6599\u76EE\u5F55' && document.querySelector('.file-item.is-active').dataset.path === '/opened/\u8D44\u6599\u76EE\u5F55/\u672A\u547D\u540D.md'");
    await inspect(window, "window.Mory.loadMarkdown('# \u5DE5\u4F5C\u533A\u6807\u9898')");
    await expectEventually(window, "untitled workspace documents follow the level-one heading like drafts", "document.querySelector('.file-item.is-active .file-name')?.textContent === '\u5DE5\u4F5C\u533A\u6807\u9898.md' && document.querySelector('.file-item.is-active')?.dataset.path === '/opened/\u8D44\u6599\u76EE\u5F55/\u672A\u547D\u540D.md'");
    await inspect(window, `window.Mory.openDocument({ name: '\u56FA\u5B9A\u6587\u4EF6\u540D.md', path: '/opened/\u8D44\u6599\u76EE\u5F55/\u56FA\u5B9A\u6587\u4EF6\u540D.md', markdown: '# \u4E0D\u8986\u76D6\u6587\u4EF6\u540D' })`);
    await expect(window, "named workspace documents follow the level-one heading without changing disk paths", "document.querySelector('.file-item.is-active .file-name')?.textContent === '\u4E0D\u8986\u76D6\u6587\u4EF6\u540D.md' && document.querySelector('.file-item.is-active')?.dataset.path === '/opened/\u8D44\u6599\u76EE\u5F55/\u56FA\u5B9A\u6587\u4EF6\u540D.md'");
    await inspect(window, "window.Mory.loadMarkdown('# \u66F4\u65B0\u540E\u7684\u6807\u9898')");
    await expectEventually(window, "changing a workspace heading immediately updates the sidebar name", "document.querySelector('.file-item.is-active .file-name')?.textContent === '\u66F4\u65B0\u540E\u7684\u6807\u9898.md' && document.querySelector('.file-item.is-active')?.dataset.path === '/opened/\u8D44\u6599\u76EE\u5F55/\u56FA\u5B9A\u6587\u4EF6\u540D.md'");
    await inspect(window, "window.Mory.closeDocument(document.querySelector('.file-item.is-active').dataset.documentId)");
    await inspect(window, `document.querySelector(".folder-item[data-path='/opened/\u8D44\u6599\u76EE\u5F55']").dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 150, clientY: 160 }))`);
    await expect(window, "directory context menu exposes create, reveal, rename, copy, move, and delete", "document.querySelector('#file-context-menu').classList.contains('is-open') && document.querySelectorAll('#file-context-menu [data-entry-action]:not([hidden])').length === 7 && document.querySelector('#file-context-menu [data-entry-action=\"delete\"]').textContent === '\u5220\u9664\u76EE\u5F55'");
    await expect(window, "context menu remains compact, legible, and free of backdrop blur", "(() => { const menu = document.querySelector('#file-context-menu'); const style = getComputedStyle(menu); return menu.getBoundingClientRect().width <= 160 && parseFloat(style.fontSize || getComputedStyle(menu.querySelector('button')).fontSize) >= 13 && (style.backdropFilter === 'none' || style.backdropFilter === ''); })()");
    await click(window, "#file-context-menu [data-entry-action='copy']");
    await expect(window, "copying a directory allows destination selection", "document.querySelector('#entry-operation-dialog').classList.contains('is-open') && document.querySelector('#entry-operation-destination option').textContent === '\u5DE5\u4F5C\u533A\u6839\u76EE\u5F55'");
    await click(window, "#entry-operation-confirm");
    await expectEventually(window, "directory copy uses the shared workspace contract", "window.__lastHostRequest.method === 'copyWorkspaceEntry' && window.__lastHostRequest.args.path === '/opened/\u8D44\u6599\u76EE\u5F55' && window.__lastHostRequest.args.destinationPath === ''");
    await inspect(window, `window.Mory.setWorkspaceSnapshot({
      state: { activeId: 'workspace-opened', workspaces: [{ id: 'workspace-opened', name: '\u5DF2\u6253\u5F00\u76EE\u5F55', provider: 'local', localPath: '/opened' }] },
      files: [{ name: '\u7B2C\u4E00\u7BC7.md', path: '/opened/\u7B2C\u4E00\u7BC7.md', createdAt: 10, images: [{ name: '\u5C01\u9762.svg', path: '/opened/\u7B2C\u4E00\u7BC7/\u5C01\u9762.svg', relative: '\u7B2C\u4E00\u7BC7/\u5C01\u9762.svg' }] }, { name: '\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md', path: '/opened/\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md', createdAt: 20 }, { name: '\u8D44\u6599\u76EE\u5F55/\u672A\u547D\u540D.md', path: '/opened/\u8D44\u6599\u76EE\u5F55/\u672A\u547D\u540D.md', createdAt: 30 }],
      directories: [{ name: '\u8D44\u6599\u76EE\u5F55', path: '/opened/\u8D44\u6599\u76EE\u5F55', createdAt: 5 }, { name: '\u76EE\u6807\u76EE\u5F55', path: '/opened/\u76EE\u6807\u76EE\u5F55', createdAt: 6 }]
    })`);
    await click(window, ".file-item[data-path='/opened/\u7B2C\u4E00\u7BC7.md']");
    await inspect(window, "document.querySelector('.file-item[data-path=\"/opened/\u7B2C\u4E00\u7BC7.md\"]').dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }))");
    await expect(window, "Enter renames the selected document", "document.querySelector('#entry-operation-dialog').classList.contains('is-open') && document.querySelector('#entry-operation-name').value === '\u7B2C\u4E00\u7BC7.md'");
    await click(window, "#entry-operation-cancel");
    await click(window, ".file-row:has(.file-item[data-path='/opened/\u7B2C\u4E00\u7BC7.md']) .file-expander");
    await expect(window, "document rows expand associated images", "document.querySelector('.file-assets .file-asset span')?.textContent === '\u5C01\u9762.svg'");
    await click(window, ".file-assets .file-asset");
    await expectEventually(window, "clicking an associated image opens its preview immediately", "document.querySelector('#image-preview').classList.contains('is-open') && document.querySelector('#image-preview-content').src.startsWith('data:image/svg+xml;base64,')");
    await click(window, "#image-preview-close");
    await inspect(window, `document.querySelector(".file-item[data-path='/opened/\u7B2C\u4E00\u7BC7.md']").dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 160, clientY: 180 }))`);
    await expect(window, "document context menu exposes open, reveal, rename, copy, move, export, and delete", "document.querySelector('#file-context-menu').classList.contains('is-open') && document.querySelectorAll('#file-context-menu [data-entry-action]:not([hidden])').length === 7");
    await click(window, "#file-context-menu [data-entry-action='reveal']");
    await expectEventually(window, "context menu reveals documents in the system file manager", "window.__lastHostRequest.method === 'revealFile' && window.__lastHostRequest.args.path === '/opened/\u7B2C\u4E00\u7BC7.md'");
    await inspect(window, `document.querySelector(".file-item[data-path='/opened/\u7B2C\u4E00\u7BC7.md']").dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 160, clientY: 180 }))`);
    await click(window, "#file-context-menu [data-entry-action='export']");
    await expectEventually(window, "context menu opens export for the current document", "document.querySelector('#export-dialog').classList.contains('is-open')");
    await click(window, "#export-close");
    await inspect(window, `window.Mory.openDocument({ name: '\u7B2C\u4E8C\u7BC7.md', path: '/opened/\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md', markdown: '# \u7B2C\u4E8C\u7BC7' })`);
    await expect(window, "opening a nested document does not promote it to the workspace root", "document.querySelectorAll('#file-list .file-item').length === 3 && document.querySelector('.file-item[data-path=\"/opened/\u7B2C\u4E00\u7BC7.md\"]') && document.querySelector('.file-item[data-path=\"/opened/\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md\"]')?.closest('.file-row').style.getPropertyValue('--tree-depth') === '1' && document.querySelector('.file-item[data-path=\"/opened/\u8D44\u6599\u76EE\u5F55/\u672A\u547D\u540D.md\"]')?.closest('.file-row').style.getPropertyValue('--tree-depth') === '1'");
    await click(window, "#new-file-button");
    await expectEventually(window, "the footer add button creates a sibling beside the selected document", "window.__lastHostRequest.method === 'createDocument' && window.__lastHostRequest.args.directoryPath === '/opened/\u8D44\u6599\u76EE\u5F55' && document.querySelector('.file-item.is-active')?.dataset.path === '/opened/\u8D44\u6599\u76EE\u5F55/\u672A\u547D\u540D.md'");
    await inspect(window, `window.Mory.openDocument({ name: '\u7B2C\u4E8C\u7BC7.md', path: '/opened/\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md', markdown: '# \u7B2C\u4E8C\u7BC7' })`);
    await inspect(window, `document.querySelector(".file-item[data-path='/opened/\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md']").dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 160, clientY: 180 }))`);
    await click(window, "#file-context-menu [data-entry-action='move']");
    await inspect(window, "document.querySelector('#entry-operation-destination').value = '/opened/\u76EE\u6807\u76EE\u5F55'");
    await click(window, "#entry-operation-confirm");
    await expectEventually(window, "documents move between workspace directories", "window.__lastMoveRequest.method === 'moveWorkspaceEntry' && window.__lastMoveRequest.args.path === '/opened/\u8D44\u6599\u76EE\u5F55/\u7B2C\u4E8C\u7BC7.md' && window.__lastMoveRequest.args.destinationPath === '/opened/\u76EE\u6807\u76EE\u5F55' && window.__lastHostRequest.method === 'readDocument' && document.querySelector('.file-item.is-active').dataset.path === '/opened/\u76EE\u6807\u76EE\u5F55/\u79FB\u52A8\u9879.md'");
    await inspect(window, "window.Mory.closeDocument(document.querySelector('.file-item.is-active').dataset.documentId)");
    await inspect(window, `window.Mory.setWorkspaceSnapshot({
      state: { activeId: 'workspace-opened', workspaces: [{ id: 'workspace-opened', name: '\u5DF2\u6253\u5F00\u76EE\u5F55', provider: 'local', localPath: '/opened' }] },
      files: [{ name: '\u7B2C\u4E00\u7BC7.md', path: '/opened/\u7B2C\u4E00\u7BC7.md', createdAt: 10 }]
    })`);
    await expect(window, "deleting the active document on disk removes it from the list", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '\u7B2C\u4E00\u7BC7.md' && window.__autoOpenedWorkspacePath === '/opened/\u7B2C\u4E00\u7BC7.md'");
    await inspect(window, `window.Mory.openDocument({ name: '\u7B2C\u4E00\u7BC7.md', path: '/opened/\u7B2C\u4E00\u7BC7.md', markdown: '# \u7B2C\u4E00\u7BC7' })`);
    await expect(window, "deleting the active document selects the first sorted document", "document.querySelector('.file-item.is-active .file-name').textContent === '\u7B2C\u4E00\u7BC7.md' && document.querySelector('#write h1').textContent === '\u7B2C\u4E00\u7BC7'");
    await inspect(window, `(() => {
      const ids = [...document.querySelectorAll('#file-list .file-item[data-path^="/opened/"][data-document-id]')].map(item => item.dataset.documentId);
      ids.forEach(id => window.Mory.closeDocument(id));
    })()`);

    await inspect(window, "document.documentElement.dataset.host = 'mac-native'");
    await expect(window, "files tab is clickable", "document.querySelector('#files-panel').classList.contains('is-active')");
    await click(window, "#sidebar-toggle");
    await expect(window, "sidebar toggle is clickable", "document.querySelector('#sidebar').classList.contains('is-hidden')");
    await expect(window, "macOS window controls do not overlap the sidebar toggle", "document.querySelector('#sidebar-toggle').getBoundingClientRect().left >= 74");
    await click(window, "#sidebar-toggle");
    await expect(window, "sidebar can be restored", "!document.querySelector('#sidebar').classList.contains('is-hidden')");

    await click(window, "#source-toggle");
    await expect(window, "source-mode button is clickable", "document.querySelector('.workspace').classList.contains('source-mode')");
    await click(window, "#source-toggle");
    await expect(window, "preview mode can be restored", "!document.querySelector('.workspace').classList.contains('source-mode')");

    await inspect(window, `(() => {
      window.Mory.didSave({ path: '/virtual/article.md', name: 'article.md', assets: { '\u6587\u7AE0/image.svg': 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjwvc3ZnPg==' } });
      const write = document.querySelector('#write');
      write.innerHTML = '<p>![image](\u6587\u7AE0/image.svg)</p>';
      const text = write.querySelector('p').firstChild;
      const range = document.createRange();
      range.setStart(text, text.nodeValue.length);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      window.Mory.normalizeMarkdown();
    })()`);
    await expect(window, "live Markdown rendering binds workspace image assets", "document.querySelector('#write img')?.src.startsWith('data:image/svg+xml;base64,') && window.Mory.getMarkdown().includes('(\u6587\u7AE0/image.svg)') && !window.Mory.getMarkdown().includes('data:image')");
    await inspect(window, `(() => {
      const write = document.querySelector('#write');
      write.innerHTML = '<p>![late](\u6587\u7AE0/late.svg)</p>';
      const text = write.querySelector('p').firstChild;
      const range = document.createRange();
      range.setStart(text, text.nodeValue.length);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      window.Mory.normalizeMarkdown();
    })()`);
    await expectEventually(window, "new relative image paths load on demand without reopening", "window.__lastHostRequest.method === 'documentAssets' && document.querySelector('#write img')?.src.startsWith('data:image/svg+xml;base64,') && window.Mory.getMarkdown().includes('(\u6587\u7AE0/late.svg)') && !window.Mory.getMarkdown().includes('data:image')");
    await inspect(window, "window.Mory.didSave({ path: '', name: '\u672A\u547D\u540D.md' })");

    await inspect(window, `window.Mory.setWorkspaceDocuments([
      { name: '\u5165\u53E3.md', path: '/virtual/\u5165\u53E3.md', markdown: '# \u5165\u53E3\\n[[\u4E13\u9898/\u8BBE\u8BA1]]' },
      { name: '\u4E13\u9898/\u8BBE\u8BA1.md', path: '/virtual/\u4E13\u9898/\u8BBE\u8BA1.md', markdown: '# \u8BBE\u8BA1\\n[\u8FD4\u56DE](../\u5165\u53E3.md)' },
      { name: '\u5F15\u7528\u8005.md', path: '/virtual/\u5F15\u7528\u8005.md', markdown: '# \u5F15\u7528\u8005\\n[[\u4E13\u9898/\u8BBE\u8BA1]]' },
      { name: '\u5B64\u7ACB.md', path: '/virtual/\u5B64\u7ACB.md', markdown: '# \u5B64\u7ACB' }
    ])`);
    await click(window, "#graph-button");
    await expect(window, "lower-right knowledge graph action opens the graph", "document.querySelector('#knowledge-graph').classList.contains('is-open')");
    await expectEventually(window, "knowledge graph renders workspace nodes and links", "document.querySelectorAll('#graph-svg .graph-node').length === 4 && document.querySelectorAll('#graph-svg .graph-link').length === 3");
    const graphWheelPoint = await inspect(window, `(() => {
      const editor = document.querySelector('#editor-scroll');
      editor.dataset.scrollBeforeGraphZoom = String(editor.scrollTop);
      const rect = document.querySelector('#graph-svg').getBoundingClientRect();
      return { x: Math.round(rect.left + rect.width / 2), y: Math.round(rect.top + rect.height / 2) };
    })()`);
    window.webContents.sendInputEvent({ type: "mouseMove", x: graphWheelPoint.x, y: graphWheelPoint.y });
    window.webContents.sendInputEvent({
      type: "mouseWheel",
      x: graphWheelPoint.x,
      y: graphWheelPoint.y,
      deltaX: 0,
      deltaY: -120,
      canScroll: true
    });
    await wait(180);
    await expectEventually(window, "graph wheel input zooms around the canvas without scrolling the document", "document.querySelector('#graph-stage').getAttribute('transform')?.includes('scale(') && document.querySelector('#graph-zoom').value !== '100%' && document.querySelector('#editor-scroll').scrollTop === Number(document.querySelector('#editor-scroll').dataset.scrollBeforeGraphZoom)");
    await click(window, "#graph-svg .graph-node[data-node-id='\u4E13\u9898/\u8BBE\u8BA1.md'] circle");
    await expectEventually(window, "selecting a node displays forward links and backlinks", "!document.querySelector('#graph-relations').hidden && document.querySelectorAll('#graph-forward-list button').length === 1 && document.querySelectorAll('#graph-backlink-list button').length === 2");
    await expectEventually(window, "graph distinguishes outgoing, incoming, and mutual references", "document.querySelectorAll('#graph-svg .graph-link.is-outgoing').length === 1 && document.querySelectorAll('#graph-svg .graph-link.is-incoming').length === 2 && document.querySelectorAll('#graph-svg .graph-node.is-mutual').length === 1 && document.querySelectorAll('#graph-svg .graph-node.is-backlink').length === 2");
    await click(window, "#graph-relations-close");
    await inspect(window, "(() => { const input = document.querySelector('#graph-search'); input.value = '\u5B64\u7ACB'; input.dispatchEvent(new Event('input', { bubbles: true })); })()");
    await expectEventually(window, "knowledge graph filters documents", "document.querySelectorAll('#graph-svg .graph-node.is-match').length === 1 && document.querySelectorAll('#graph-svg .graph-node.is-dimmed').length === 3");
    await click(window, "#graph-close");
    await expect(window, "knowledge graph can be closed", "!document.querySelector('#knowledge-graph').classList.contains('is-open')");

    await inspect(window, `window.Mory.openDocument({ name: '\u4E13\u9898/\u8BBE\u8BA1.md', path: '/virtual/\u4E13\u9898/\u8BBE\u8BA1.md', markdown: '# \u8BBE\u8BA1\\n[\u8FD4\u56DE](../\u5165\u53E3.md)' })`);
    await expect(window, "status bar shows the active document backlink count", "document.querySelector('#backlink-count').textContent === '\u53CD\u5411\u94FE\u63A5 2'");
    await expect(window, "document footer lists backlink sources", "!document.querySelector('#document-backlinks').hidden && document.querySelectorAll('#document-backlinks-list button').length === 2 && [...document.querySelectorAll('#document-backlinks-list strong')].map(item => item.textContent).join('|') === '\u5165\u53E3|\u5F15\u7528\u8005'");
    await click(window, "#backlink-count");
    await expect(window, "status-bar backlink action locates the document backlink section", "!document.querySelector('#document-backlinks').hidden");

    await click(window, "#export-button");
    await expect(window, "export button is clickable", "document.querySelector('#export-dialog').classList.contains('is-open')");
    await inspect(window, "(() => { const format = document.querySelector('#export-format'); format.value = 'pptx'; format.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "PowerPoint export explains the official Slidev path and hides irrelevant theme controls", "document.querySelector('#export-theme-setting').hidden && document.querySelector('#export-background-setting').hidden && document.querySelector('#export-hint').textContent.includes('Slidev')");
    await click(window, "#export-confirm");
    await expectEventually(window, "PowerPoint export sends raw Markdown without rendered HTML", "window.__lastNativeMessage?.type === 'export' && window.__lastNativeMessage.options.format === 'pptx' && typeof window.__lastNativeMessage.options.markdown === 'string' && !('html' in window.__lastNativeMessage.options)");
    await click(window, "#export-button");
    await click(window, "#export-close");
    await expect(window, "export dialog can be closed", "!document.querySelector('#export-dialog').classList.contains('is-open')");

    await click(window, "#settings-button");
    await expect(window, "settings button is clickable", "document.querySelector('#preferences').classList.contains('is-open')");
    await expect(window, "settings dialog presents readable grouped controls without horizontal overflow", "(() => { const card = document.querySelector('.preferences-card'); const scroll = document.querySelector('.preferences-scroll'); const title = document.querySelector('.preferences-card h2'); const rowTitle = document.querySelector('.settings-list .setting-row strong'); const select = document.querySelector('.settings-list select'); const cardRect = card.getBoundingClientRect(); return cardRect.width >= 780 && cardRect.height >= 680 && scroll.scrollWidth <= scroll.clientWidth && parseFloat(getComputedStyle(title).fontSize) >= 24 && parseFloat(getComputedStyle(rowTitle).fontSize) >= 14 && parseFloat(getComputedStyle(select).height) >= 36 && document.querySelectorAll('.settings-list .setting-row').length === 8; })()");
    await inspect(window, "(() => { const theme = document.querySelector('#document-theme-select'); theme.value = 'newsprint'; theme.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "resume template action stays hidden for document themes without templates", "document.querySelector('#resume-template-button').hidden && getComputedStyle(document.querySelector('#resume-template-button')).display === 'none'");
    await inspect(window, "(() => { const theme = document.querySelector('#document-theme-select'); theme.value = 'lapis-cv'; theme.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "resume template action appears only for Lapis CV", "!document.querySelector('#resume-template-button').hidden && getComputedStyle(document.querySelector('#resume-template-button')).display !== 'none'");
    await inspect(window, "(() => { const active = document.querySelector('#file-list .file-item.is-active'); window.__documentRowsBeforeResume = document.querySelectorAll('#file-list .file-item').length; window.__activeDocumentBeforeResume = active?.dataset.documentId || ''; })()");
    await click(window, "#resume-template-button");
    await expectEventually(window, "resume template creates a new Lapis CV draft without replacing the previous document", "(() => { const markdown = window.Mory.getMarkdown(); const active = document.querySelector('.file-item.is-active'); const headings = [...document.querySelectorAll('#write h2')].map(item => item.textContent); return document.documentElement.dataset.docTheme === 'lapis-cv' && markdown.startsWith('# \u516b\u722a\u732b') && document.querySelector('#write h1')?.textContent === '\u516b\u722a\u732b' && headings.some(title => title.includes('\u6559\u80b2\u7ecf\u5386')) && document.querySelector('#write div[alt=\"entry-title\"]') && document.querySelectorAll('#file-list .file-item').length === window.__documentRowsBeforeResume + 1 && active?.textContent.includes('\u516b\u722a\u732b.md') && document.querySelector('#save-state').textContent === '\u672a\u4fdd\u5b58' && !document.querySelector('#preferences').classList.contains('is-open'); })()");
    await inspect(window, "document.querySelector(`#file-list .file-item[data-document-id=\"${window.__activeDocumentBeforeResume}\"]`)?.click()");
    await click(window, "#settings-button");
    await inspect(window, "(() => { const appearance = document.querySelector('#theme-select'); appearance.value = 'light'; appearance.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expectEventually(window, "light sidebar uses high-contrast semantic colors", "(() => { const style = getComputedStyle(document.documentElement); return document.documentElement.dataset.appearance === 'light' && style.getPropertyValue('--sidebar-text').trim() === '#272a27' && style.getPropertyValue('--sidebar-muted').trim() === '#626762' && style.getPropertyValue('--sidebar-faint').trim() === '#7b817b'; })()");
    await wait(240);
    await fs.writeFile(preferencesScreenshotPath, await capturePNG(window));
    await inspect(window, `(() => {
      window.__openedExternalURL = '';
      window.Mory.loadMarkdown('Visit https://example.com/docs.');
      const text = document.querySelector('#write p').firstChild;
      const range = document.createRange();
      range.setStart(text, 12);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write p').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ctrlKey: true }));
    })()`);
    await expectEventually(window, "Ctrl-click opens bare URLs without rewriting Markdown", "window.__openedExternalURL === 'https://example.com/docs' && window.Mory.getMarkdown() === 'Visit https://example.com/docs.'");
    await inspect(window, `(() => {
      window.Mory.loadMarkdown(${JSON.stringify("# Ada Lovelace\n\n> <span alt=\"icon\">&#xe60f;</span> Engineer&emsp; <span alt=\"icon\">&#xe7ca;</span> ada@example.com&emsp; [Portfolio](https://example.com)\n\n<img alt=\"avatar\" src=\"./photo_1.svg\">\n\n## &#xe618; Experience\n\n<div alt=\"entry-title\" onclick=\"window.__unsafeRawHTML = true\">\n  <h3>Analytical Engine</h3>\n  <p>1842–1843</p>\n</div>\n**Data platform**\n- Designed a general-purpose algorithm.\n- Explained the work with precise technical notes.")});
      const documentTheme = document.querySelector('#document-theme-select');
      documentTheme.value = 'lapis-cv';
      documentTheme.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await expectEventually(window, "Lapis CV renders Typora-style HTML and adjacent Markdown as an A4 resume", "(() => { const write = document.querySelector('#write'); const h1 = write.querySelector('h1'); const h2 = write.querySelector('h2'); const entry = write.querySelector('div[alt=\"entry-title\"]'); const avatar = write.querySelector('img[alt=\"avatar\"]'); const icons = write.querySelectorAll('span[alt=\"icon\"]'); return document.documentElement.dataset.docTheme === 'lapis-cv' && getComputedStyle(h1).textAlign === 'center' && getComputedStyle(h2).color === 'rgb(72, 112, 173)' && getComputedStyle(entry).display === 'flex' && getComputedStyle(avatar).borderRadius === '50%' && avatar.src.startsWith('data:image/svg+xml;base64,') && write.querySelector('strong')?.textContent === 'Data platform' && icons.length === 2 && icons[0].textContent === '\ue60f' && !entry.hasAttribute('onclick') && window.__unsafeRawHTML !== true && getComputedStyle(write).minHeight !== '0px'; })()");
    await expect(window, "raw HTML survives editor-to-Markdown round trips with relative image sources", "(() => { const markdown = window.Mory.getMarkdown(); return markdown.includes('<span alt=\"icon\">&#xe60f;</span>') && markdown.includes('<img alt=\"avatar\" src=\"./photo_1.svg\">') && markdown.includes('<div alt=\"entry-title\" onclick=\"window.__unsafeRawHTML = true\">') && markdown.includes('**Data platform**') && !markdown.includes('data:image'); })()");
    await inspect(window, "document.querySelector('#write a[href=\"https://example.com\"]').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ctrlKey: true }))");
    await expectEventually(window, "Ctrl-click opens URL links through the desktop host", "window.__openedExternalURL === 'https://example.com'");
    await expectEventually(window, "Lapis CV loads its bundled text, code, and icon fonts", "document.documentElement.dataset.lapisCvFont === 'bundled' && document.querySelector('#theme-font-warning').hidden && document.fonts.check('400 16px \"Mory LapisCV Icon\"', '\ue60f') && getComputedStyle(document.querySelector('#write span[alt=\"icon\"]')).fontFamily.includes('Mory LapisCV Icon')");
    await expect(window, "Lapis CV CSS, fonts, and sanitized HTML are embedded in clean exports", "window.Mory.exportDocument({ theme: 'lapis-cv', background: true }).then(html => html.includes('data-doc-theme=\"lapis-cv\"') && html.includes('data-export=\"true\"') && html.includes('Mory Lapis CV document theme') && html.includes('data:font/ttf;base64,') && !html.includes('../fonts/') && html.includes('<div alt=\"entry-title\">') && !html.includes('onclick=') && !html.includes('mory-raw-html') && !html.includes('<script'))");
    await click(window, "#preferences-close");
    await inspect(window, "document.querySelector('#editor-scroll').scrollTop = 0");
    await wait(260);
    await fs.writeFile(lapisCVScreenshotPath, await capturePNG(window));
    await inspect(window, `window.Mory.loadMarkdown(${JSON.stringify("<section class=\"profile\" style=\"border-left: 3px solid #4870ad\" onclick=\"window.__unsafeHTMLBlock = true\">\n  <details open><summary>Portfolio</summary><p><mark>Selected work</mark></p></details>\n  <table><tbody><tr><th>Skill</th><td>Go</td></tr></tbody></table>\n  <script>window.__unsafeHTMLBlock = true</script>\n</section>")})`);
    await expect(window, "common block HTML renders while executable content is removed", "(() => { const section = document.querySelector('#write section.profile'); return section?.style.borderLeftWidth === '3px' && section.querySelector('details[open] summary')?.textContent === 'Portfolio' && section.querySelector('mark')?.textContent === 'Selected work' && section.querySelector('table td')?.textContent === 'Go' && !section.hasAttribute('onclick') && !section.querySelector('script') && window.__unsafeHTMLBlock !== true; })()");
    await expect(window, "generic HTML source round-trips and exports as sanitized markup", "window.Mory.exportDocument({ theme: 'lapis-cv' }).then(html => window.Mory.getMarkdown().includes('<details open>') && html.includes('<section class=\"profile\"') && html.includes('border-left:') && html.includes('<details open=\"\">') && !html.includes('<script') && !html.includes('onclick=') && !html.includes('mory-raw-html'))");
    await click(window, "#settings-button");
    await inspect(window, "(() => { const documentTheme = document.querySelector('#document-theme-select'); documentTheme.value = 'yuluo-css'; documentTheme.dispatchEvent(new Event('change', { bubbles: true })); const appearance = document.querySelector('#theme-select'); appearance.value = 'dark'; appearance.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expectEventually(window, "Yuluo keeps Hannotate first and provides a bundled handwriting fallback", "document.documentElement.dataset.yuluoFont === 'bundled' && document.querySelector('#theme-font-warning').hidden && document.fonts.check('400 16px \"Mory LXGW WenKai\"', '\u6c49\u5b57') && getComputedStyle(document.querySelector('#write')).fontFamily.startsWith('\"Hannotate SC\"')");
    const darkThemeColors = [
      ["yuluo-css", "rgb(31, 34, 36)", "rgb(221, 226, 229)"],
      ["lapis-cv", "rgb(31, 39, 50)", "rgb(232, 237, 244)"],
      ["github", "rgb(13, 17, 23)", "rgb(230, 237, 243)"],
      ["whitey", "rgb(31, 32, 32)", "rgb(222, 222, 217)"],
      ["newsprint", "rgb(36, 34, 30)", "rgb(222, 216, 202)"],
      ["pixyll", "rgb(33, 31, 32)", "rgb(222, 217, 216)"],
      ["gothic", "rgb(28, 28, 27)", "rgb(216, 215, 210)"],
      ["night", "rgb(23, 26, 31)", "rgb(220, 226, 232)"]
    ];
    for (const [theme, paper, text] of darkThemeColors) {
      await inspect(window, `(() => { const select = document.querySelector('#document-theme-select'); select.value = ${JSON.stringify(theme)}; select.dispatchEvent(new Event('change', { bubbles: true })); })()`);
      await expectEventually(window, `${theme} provides a coherent dark document canvas`, `(() => { const paper = getComputedStyle(document.querySelector('#editor-scroll')).backgroundColor; const text = getComputedStyle(document.querySelector('#write')).color; return document.documentElement.dataset.appearance === 'dark' && paper === ${JSON.stringify(paper)} && text === ${JSON.stringify(text)}; })()`);
      if (theme === "github") {
        await click(window, "#preferences-close");
        await fs.writeFile(darkThemeScreenshotPath, await capturePNG(window));
        await click(window, "#settings-button");
      }
    }
    await expect(window, "Mermaid uses a dark palette with the dark appearance", "(() => { const palette = mermaidTheme('github', 'dark'); return palette.theme === 'dark' && palette.background === '#0d1117' && palette.primaryTextColor === '#e6edf3'; })()");
    await expectEventually(window, "dark sidebar uses high-contrast semantic colors", "(() => { const style = getComputedStyle(document.documentElement); return style.getPropertyValue('--sidebar-text').trim() === '#f0f1ee' && style.getPropertyValue('--sidebar-muted').trim() === '#bdc1ba' && style.getPropertyValue('--sidebar-faint').trim() === '#9da39b'; })()");
    await inspect(window, "(() => { const documentTheme = document.querySelector('#document-theme-select'); documentTheme.value = 'yuluo-css'; documentTheme.dispatchEvent(new Event('change', { bubbles: true })); const appearance = document.querySelector('#theme-select'); appearance.value = 'light'; appearance.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expectEventually(window, "switching back to light appearance restores the default paper", "document.documentElement.dataset.appearance === 'light' && getComputedStyle(document.querySelector('#editor-scroll')).backgroundColor === 'rgb(255, 255, 255)'");
    await inspect(window, "window.Mory.setCustomThemes([{ id: 'user-paper-test', name: '\u7EB8\u5F20', css: '#write{letter-spacing:1px}' }])");
    await inspect(window, "(() => { const select = document.querySelector('#document-theme-select'); select.value = 'user-paper-test'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "custom CSS themes apply immediately", "document.documentElement.dataset.docTheme === 'user-paper-test' && document.querySelector('#user-document-theme').textContent.includes('letter-spacing')");
    await expect(window, "custom themes appear in export choices", "document.querySelector('#export-theme option[value=\"user-paper-test\"]')");
    await expect(window, "custom theme CSS is embedded in exported HTML", "window.Mory.exportDocument({ theme: 'current' }).then(html => html.includes('#write{letter-spacing:1px}'))");
    await click(window, "#theme-choose-folder");
    await expectEventually(window, "settings change the custom theme directory and refresh themes", "window.__lastHostRequest.method === 'chooseThemeFolder' && document.querySelector('#document-theme-select option[value=\"user-folder-test\"]')");
    await inspect(window, "(() => { const select = document.querySelector('#language-select'); select.value = 'en'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "settings switch the interface to English immediately", "document.documentElement.lang === 'en' && document.querySelector('#preferences h2').textContent === 'Preferences' && document.querySelector('#graph-button').getAttribute('aria-label') === 'Knowledge graph' && document.querySelector('#backlink-count').textContent === 'Backlinks 2'");
    await inspect(window, `window.Mory.openDocument({ name: 'English.md', path: '/virtual/English.md', markdown: '# English' })`);
    await expect(window, "dynamic status remains English when switching documents", "document.querySelector('#save-state').textContent === 'Saved' && document.querySelector('#toast').textContent === 'Document switched' && document.querySelector('.file-item.is-active + .file-close').getAttribute('aria-label') === 'Delete document'");
    await inspect(window, "(() => { const select = document.querySelector('#language-select'); select.value = 'zh-CN'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "settings can switch the interface back to Chinese", "document.documentElement.lang === 'zh-CN' && document.querySelector('#preferences h2').textContent === '\u504F\u597D\u8BBE\u7F6E'");
    await click(window, ".setting-row:has(#status-toggle) .switch span");
    await expect(window, "disabling the status bar takes effect immediately", "document.querySelector('#statusbar').hidden && getComputedStyle(document.querySelector('#statusbar')).display === 'none' && localStorage.getItem('mory.status') === 'false'");
    await click(window, ".setting-row:has(#status-toggle) .switch span");
    await expect(window, "re-enabling the status bar takes effect immediately", "!document.querySelector('#statusbar').hidden && getComputedStyle(document.querySelector('#statusbar')).display === 'flex' && localStorage.getItem('mory.status') === 'true'");
    await inspect(window, "window.Mory.setFiles([{ name: 'Details.md', path: '/virtual/Details.md', createdAt: 1, updatedAt: 1787070600000, size: 1536, images: [{ name: 'cover.png', path: '/virtual/Details/cover.png', relative: 'Details/cover.png', updatedAt: 1787070600000, size: 5242880 }] }])");
    await click(window, ".setting-row:has(#file-details-toggle) .switch span");
    await expect(window, "file details setting shows document size and update time", "document.querySelector('.file-item[data-path=\"/virtual/Details.md\"] .file-meta')?.textContent.includes('1.5 KB') && localStorage.getItem('mory.fileDetails') === 'true'");
    await inspect(window, "document.querySelector('.file-row:has(.file-item[data-path=\"/virtual/Details.md\"]) .file-expander').click()");
    await expect(window, "expanded image rows show image size and update time", "document.querySelector('.file-assets .file-meta')?.textContent.includes('5 MB')");
    await click(window, ".setting-row:has(#file-details-toggle) .switch span");
    await expect(window, "file details setting hides optional metadata", "!document.querySelector('#file-list .file-meta') && localStorage.getItem('mory.fileDetails') === 'false'");
    await inspect(window, "(() => { const bar = document.querySelector('#statusbar'); window.__statusBeforeZoom = { font: parseFloat(getComputedStyle(bar).fontSize), height: bar.getBoundingClientRect().height }; window.Mory.zoom(1); window.Mory.zoom(1); })()");
    await expect(window, "editor zoom scales status-bar text and height", "(() => { const bar = document.querySelector('#statusbar'); return parseFloat(getComputedStyle(bar).fontSize) >= window.__statusBeforeZoom.font * 1.19 && bar.getBoundingClientRect().height > window.__statusBeforeZoom.height; })()");
    await inspect(window, "window.Mory.zoom(0)");
    await expect(window, "actual-size zoom restores the status bar", "(() => { const bar = document.querySelector('#statusbar'); return parseFloat(getComputedStyle(bar).fontSize) === window.__statusBeforeZoom.font && Math.abs(bar.getBoundingClientRect().height - window.__statusBeforeZoom.height) < 0.5; })()");
    await click(window, "#workspace-add");
    await expect(window, "workspace plugin form opens", "!document.querySelector('#workspace-form').hidden");
    await inspect(window, "(() => { const select = document.querySelector('#workspace-provider'); select.value = 's3'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "S3 credential fields are complete", "['endpoint','region','bucket','prefix','accessKeyId','accessKeySecret','sessionToken'].every(name => document.querySelector(`#workspace-provider-fields [name=\"${name}\"]`))");
    await inspect(window, "(() => { const select = document.querySelector('#workspace-provider'); select.value = 'sftp'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "SFTP connection fields are complete", "['host','port','username','password','privateKey','knownHosts','remotePath'].every(name => document.querySelector(`#workspace-provider-fields [name=\"${name}\"]`))");
    await click(window, "#workspace-form-close");
    await click(window, "#preferences-close");
    await expect(window, "settings dialog can be closed", "!document.querySelector('#preferences').classList.contains('is-open')");
    await expect(window, "settings entry is unique", "!document.querySelector('#more-button') && document.querySelectorAll('#settings-button').length === 1");

    await inspect(window, "document.dispatchEvent(new KeyboardEvent('keydown', { key: 'p', metaKey: true, bubbles: true }))");
    await wait(80);
    await expect(window, "quick-open shortcut works after sidebar search removal", "!document.querySelector('#quick-open-button') && document.querySelector('#quick-open').classList.contains('is-open')");
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Escape" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Escape" });
    await wait(80);
    await expect(window, "quick-open dialog can be dismissed", "!document.querySelector('#quick-open').classList.contains('is-open')");

    await click(window, "#focus-button");
    await expect(window, "focus mode is clickable", "document.querySelector('.workspace').classList.contains('focus-mode')");
    await click(window, "#focus-button");
    await click(window, "#typewriter-button");

    await click(window, "#word-count");
    await expect(window, "word-count button is clickable", "document.querySelector('#toast').classList.contains('is-visible') && document.querySelector('#toast').textContent.includes('\u5B57\u7B26')");
    await expect(window, "typewriter mode is clickable", "document.querySelector('.workspace').classList.contains('typewriter-mode')");
    await click(window, "#typewriter-button");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      paragraph.focus?.();
      document.querySelector('#write').focus();
    })()`);
    await window.webContents.insertText("##");
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Space" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Space" });
    await wait(100);
    await expect(window, "a double-hash marker renders a level-two heading immediately", "document.querySelector('#write > h2') !== null && !document.querySelector('#write').textContent.includes('##')");
    await window.webContents.insertText("\u672A\u4FDD\u5B58\u6807\u9898");
    await wait(80);
    await click(window, ".tab[data-panel='outline']");
    await expect(window, "unsaved headings appear in the outline immediately", "document.querySelector('#outline-count').textContent === '1 \u9879' && document.querySelector('#outline-list .outline-item')?.textContent === '\u672A\u4FDD\u5B58\u6807\u9898'");
    await click(window, ".tab[data-panel='files']");
    await inspect(window, `(() => {
      const heading = document.querySelector('#write > h2');
      const range = document.createRange();
      range.selectNodeContents(heading);
      range.collapse(false);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
    })()`);
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Enter" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Enter" });
    await wait(30);
    await window.webContents.insertText("\u6B63\u6587\u5185\u5BB9");
    await expect(window, "pressing Enter after a heading restores paragraph text", "document.querySelector('#write > h2')?.textContent === '\u672A\u4FDD\u5B58\u6807\u9898' && document.querySelector('#write > p')?.textContent === '\u6B63\u6587\u5185\u5BB9'");
    await inspect(window, "window.Mory.newDocument(); window.Mory.loadMarkdown('# \u8349\u7A3F\u6807\u9898')");
    await expectEventually(window, "a level-one heading replaces an untitled draft name immediately", "document.querySelector('.file-item.is-active .file-name')?.textContent === '\u8349\u7A3F\u6807\u9898.md'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('# \u4E2D\u6587\u8F93\u5165\u6807\u9898');
      const heading = document.querySelector('#write > h1');
      const range = document.createRange();
      range.selectNodeContents(heading);
      range.collapse(false);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
      heading.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true, data: '' }));
      heading.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', data: '\u4E2D\u6587\u8F93\u5165\u6807\u9898', isComposing: true }));
      heading.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter', isComposing: true }));
    })()`);
    await expect(window, "IME confirmation Enter does not exit the heading early", "document.querySelectorAll('#write > h1').length === 1 && document.querySelectorAll('#write > p').length === 0");
    await inspect(window, "document.querySelector('#write > h1').dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '\u4E2D\u6587\u8F93\u5165\u6807\u9898' }))");
    await wait(80);
    await expect(window, "Chinese IME commits render a full-line heading immediately", "document.querySelector('#write > h1')?.textContent === '\u4E2D\u6587\u8F93\u5165\u6807\u9898' && !document.querySelector('#write').textContent.includes('#')");

    const immediateCompositionEnter = await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write > p');
      paragraph.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
      paragraph.textContent = '# \u4F60\u597D';
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
      paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', data: '\u4F60\u597D', isComposing: true }));
      paragraph.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '\u4F60\u597D' }));
      const enter = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' });
      const dispatched = paragraph.dispatchEvent(enter);
      return { prevented: !dispatched };
    })()`);
    await expect(window, "Enter immediately after IME commit creates a heading and paragraph", `(() => {
      const anchor = getSelection()?.anchorNode;
      const block = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('#write > *');
      return ${JSON.stringify(immediateCompositionEnter.prevented)} && document.querySelector('#write > h1')?.textContent === '\u4F60\u597D' && document.querySelector('#write > h1 + p') && block?.tagName === 'P';
    })()`);

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write > p');
      paragraph.textContent = '# \u4F60\u597D';
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      paragraph.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
      const next = document.createElement('p');
      next.append(document.createElement('br'));
      paragraph.after(next);
      const nextRange = document.createRange();
      nextRange.setStart(next, 0);
      nextRange.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(nextRange);
      paragraph.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '\u4F60\u597D' }));
    })()`);
    await expectEventually(window, "Chinese candidate confirmation with a line break preserves the heading", "document.querySelector('#write > h1')?.textContent === '\u4F60\u597D' && document.querySelector('#write > h1 + p') && !document.querySelector('#write').textContent.includes('#')");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
    })()`);
    for (let index = 0; index < 2; index += 1) {
      await window.webContents.insertText("#");
      window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Space" });
      window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Space" });
      await wait(30);
      await window.webContents.insertText("\u4F60\u597D");
      window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Enter" });
      window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Enter" });
      await wait(30);
    }
    await wait(80);
    await expect(window, "consecutive Chinese headings remain separate blocks", "document.querySelectorAll('#write > h1').length === 2 && [...document.querySelectorAll('#write > h1')].every(item => item.textContent === '\u4F60\u597D') && document.querySelector('#write > h1 + h1') !== null && document.querySelector('#write > h1:last-of-type + p') !== null");

    await inspect(window, `(() => {
      const editor = document.querySelector('#write');
      editor.innerHTML = '<p># 12 - test</p><h1>\u4F60\u597D\u554A</h1>';
      const activeHeading = editor.lastElementChild;
      const range = document.createRange();
      range.selectNodeContents(activeHeading);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      editor.focus();
      editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '\u554A' }));
    })()`);
    await expectEventually(window, "a missed first-line Markdown heading renders on the next frame", "document.querySelector('#write > h1:first-child')?.textContent === '12 - test' && document.querySelectorAll('#write > h1').length === 2");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
    })()`);
    await window.webContents.insertText("\u8FD9\u662F **\u5373\u65F6\u52A0\u7C97** \u6587\u672C");
    await expect(window, "paired asterisks convert to bold immediately", "document.querySelector('#write p strong')?.textContent === '\u5373\u65F6\u52A0\u7C97' && document.querySelector('#write p')?.textContent === '\u8FD9\u662F \u5373\u65F6\u52A0\u7C97 \u6587\u672C'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
      const clipboard = new DataTransfer();
      clipboard.setData('text/plain', ${JSON.stringify(pastedMarkdown)});
      document.querySelector('#write').dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: clipboard }));
    })()`);
    await expectEventually(window, "pasting a full block renders headings, bold text, and code immediately", "document.querySelector('#write > h1')?.textContent === '\u7C98\u8D34\u6807\u9898' && document.querySelector('#write strong')?.textContent === '\u7C98\u8D34\u52A0\u7C97' && document.querySelector('#write > pre[data-language=\"go\"] code')?.textContent === 'fmt.Println(\"paste\")'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
    })()`);
    await window.webContents.insertText("```go");
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Enter" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Enter" });
    await wait(80);
    await window.webContents.insertText("fmt.Println(\"hi\")");
    await expect(window, "code fences convert immediately and preserve the language", "document.querySelector('#write > pre')?.dataset.language === 'go' && document.querySelector('#write > pre code')?.textContent === 'fmt.Println(\"hi\")' && window.Mory.getMarkdown().includes('```go')");
    await insertParagraph(window);
    await window.webContents.insertText("fmt.Println(\"bye\")");
    await insertParagraph(window);
    await window.webContents.insertText("```");
    await wait(80);
    await expect(window, "multiline code stays in one fence and returns to text after closing", "document.querySelectorAll('#write > pre').length === 1 && document.querySelector('#write > pre code')?.textContent === 'fmt.Println(\"hi\")\\nfmt.Println(\"bye\")' && !document.querySelector('#write > pre code')?.textContent.includes('```') && document.querySelector('#write > pre + p') !== null");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
    })()`);
    await window.webContents.insertText("```mermaid");
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Enter" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Enter" });
    await wait(100);
    await expect(window, "Mermaid fences open a focused split workbench instead of a plain code block", "(() => { const grid = document.querySelector('#write > .mermaid-diagram .mermaid-workbench-grid'); const panes = grid ? [...grid.children].map(item => item.getBoundingClientRect().width) : []; return grid && !document.querySelector('#write > pre') && document.activeElement?.classList.contains('mermaid-source-editor') && panes.length === 2 && Math.abs(panes[0] - panes[1]) <= 2; })()");
    await inspect(window, `(() => {
      const input = document.querySelector('.mermaid-source-editor');
      input.value = 'flowchart TD\\n  A[Start] --> B{Ready?}\\n  B -->|Yes| C[Ship]';
      input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    await expectEventually(window, "Mermaid source edits render an SVG and update Markdown immediately", "document.querySelector('.mermaid-diagram[data-mermaid-state=\"rendered\"] .mermaid-preview-canvas svg') && window.Mory.getMarkdown().includes('A[Start] --> B{Ready?}')", 4000);
    await wait(120);
    await fs.writeFile(mermaidScreenshotPath, await capturePNG(window));
    await inspect(window, "(() => { const diagram = document.querySelector('.mermaid-diagram'); const button = diagram.querySelector('.mermaid-expand-button').getBoundingClientRect(); window.__mermaidMarkdownBeforeLayout = window.Mory.getMarkdown(); window.__mermaidCanvasHeight = diagram.querySelector('.mermaid-preview-canvas').getBoundingClientRect().height; window.__mermaidExpandOutside = button.bottom <= diagram.getBoundingClientRect().top; })()");
    await expect(window, "the enlarge control sits outside the Mermaid workbench and the divider uses one compact arrow", "(() => { const toggle = document.querySelector('.mermaid-source-toggle'); const size = toggle.getBoundingClientRect(); return window.__mermaidExpandOutside === true && toggle.querySelector('use').getAttribute('href') === '#i-arrow-left' && size.width <= 20 && size.height <= 28; })()");
    await click(window, ".mermaid-source-toggle");
    await expect(window, "the divider control collapses the source pane without changing Markdown", "(() => { const diagram = document.querySelector('.mermaid-diagram'); const grid = diagram.querySelector('.mermaid-workbench-grid').getBoundingClientRect(); const source = diagram.querySelector('.mermaid-source-pane').getBoundingClientRect(); const preview = diagram.querySelector('.mermaid-preview-pane').getBoundingClientRect(); const toggle = diagram.querySelector('.mermaid-source-toggle'); return diagram.classList.contains('is-source-collapsed') && source.width < 1 && Math.abs(preview.width - grid.width) <= 1 && window.Mory.getMarkdown() === window.__mermaidMarkdownBeforeLayout && toggle.getAttribute('aria-expanded') === 'false' && toggle.querySelector('use').getAttribute('href') === '#i-arrow-right'; })()");
    await click(window, ".mermaid-source-toggle");
    await expect(window, "the divider control restores an even source and preview split", "(() => { const diagram = document.querySelector('.mermaid-diagram'); const panes = [...diagram.querySelector('.mermaid-workbench-grid').children].map(item => item.getBoundingClientRect().width); return !diagram.classList.contains('is-source-collapsed') && Math.abs(panes[0] - panes[1]) <= 2 && diagram.querySelector('.mermaid-source-toggle').getAttribute('aria-expanded') === 'true'; })()");
    await click(window, ".mermaid-expand-button");
    await expect(window, "the enlarge control opens the complete Mermaid editor with source and preview visible", "(() => { const diagram = document.querySelector('.mermaid-diagram'); const grid = diagram.querySelector('.mermaid-workbench-grid').getBoundingClientRect(); const source = diagram.querySelector('.mermaid-source-pane').getBoundingClientRect(); const preview = diagram.querySelector('.mermaid-preview-pane').getBoundingClientRect(); const canvas = diagram.querySelector('.mermaid-preview-canvas').getBoundingClientRect(); const svg = diagram.querySelector('.mermaid-preview-canvas svg').getBoundingClientRect(); return diagram.classList.contains('is-workbench-expanded') && document.body.classList.contains('mermaid-workbench-open') && source.width > 0 && preview.width > 0 && Math.abs(source.width - preview.width) <= 2 && grid.height > innerHeight - 30 && canvas.height > window.__mermaidCanvasHeight + 200 && svg.width > 0 && svg.height > 0 && diagram.querySelector('.mermaid-expand-button').getAttribute('aria-pressed') === 'true'; })()");
    await inspect(window, `(() => {
      const input = document.querySelector('.mermaid-source-editor');
      input.focus();
      input.value = 'flowchart TD\\n  A[Start] --> B{Ready?}\\n  B -->|Yes| C[Ship]\\n  C --> D[Done]';
      input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    await expectEventually(window, "Mermaid remains editable with live rendering while the complete workbench is enlarged", "document.querySelector('.mermaid-diagram.is-workbench-expanded[data-mermaid-state=\"rendered\"] .mermaid-preview-canvas svg') && document.activeElement?.classList.contains('mermaid-source-editor') && window.Mory.getMarkdown().includes('C --> D[Done]')", 4000);
    await fs.writeFile(mermaidExpandedScreenshotPath, await capturePNG(window));
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Escape" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Escape" });
    await wait(80);
    await expect(window, "Escape exits the enlarged Mermaid editor", "(() => { const diagram = document.querySelector('.mermaid-diagram'); return !diagram.classList.contains('is-workbench-expanded') && !document.body.classList.contains('mermaid-workbench-open') && document.activeElement === diagram.querySelector('.mermaid-expand-button'); })()");
    window.setSize(720, 790);
    await wait(140);
    await expect(window, "narrow viewports stack Mermaid source above the preview", "(() => { const source = document.querySelector('.mermaid-source-pane').getBoundingClientRect(); const preview = document.querySelector('.mermaid-preview-pane').getBoundingClientRect(); return source.bottom <= preview.top + 1 && Math.abs(source.left - preview.left) <= 1 && Math.abs(source.width - preview.width) <= 1; })()");
    await fs.writeFile(mermaidNarrowScreenshotPath, await capturePNG(window));
    window.setSize(1180, 790);
    await wait(140);
    await click(window, ".mermaid-theme-button");
    await expectEventually(window, "the palette icon cycles diagram colors and persists the choice", "document.querySelector('.mermaid-diagram')?.dataset.mermaidTheme === 'ocean' && document.querySelector('.mermaid-theme-button')?.dataset.mermaidTheme === 'ocean' && window.Mory.getMarkdown().includes('```mermaid theme=ocean')", 4000);
    await inspect(window, `(() => {
      const input = document.querySelector('.mermaid-source-editor');
      input.value = 'flowchart TD\\n  A -->';
      input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    await expectEventually(window, "invalid Mermaid syntax reports an inline preview error without losing source", "(() => { const diagram = document.querySelector('.mermaid-diagram'); return diagram?.dataset.mermaidState === 'error' && !diagram.querySelector('.mermaid-preview-error').hidden && diagram.querySelector('.mermaid-preview-error small').textContent.length > 0 && window.Mory.getMarkdown().includes('A -->'); })()", 4000);
    await inspect(window, `(() => {
      const input = document.querySelector('.mermaid-source-editor');
      input.value = 'flowchart LR\\n  Source --> Preview';
      input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    await expectEventually(window, "corrected Mermaid syntax recovers the live preview", "document.querySelector('.mermaid-diagram[data-mermaid-state=\"rendered\"] .mermaid-preview-canvas svg')", 4000);
    await inspect(window, "window.Mory.loadMarkdown(window.Mory.getMarkdown())");
    await expectEventually(window, "Mermaid source and color survive a Markdown reload", "document.querySelector('.mermaid-source-editor')?.value.includes('Source --> Preview') && document.querySelector('.mermaid-diagram')?.dataset.mermaidTheme === 'ocean' && document.querySelector('.mermaid-preview-canvas svg')", 4000);

    await inspect(window, `(() => {
      window.Mory.loadMarkdown(${JSON.stringify('```go\nfmt.Println("double")\n```')});
      const code = document.querySelector('#write > pre code');
      const range = document.createRange();
      range.selectNodeContents(code);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
    })()`);
    await insertParagraph(window);
    await insertParagraph(window);
    await expect(window, "two Enter presses at the end of a code block return to text", "(() => { const anchor = getSelection()?.anchorNode; const block = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('#write > *'); return document.querySelector('#write > pre code')?.textContent === 'fmt.Println(\"double\")' && document.querySelector('#write > pre + p') !== null && block?.tagName === 'P'; })()");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown(${JSON.stringify('```go\nfmt.Println("meta")\n```')});
      const code = document.querySelector('#write > pre code');
      const range = document.createRange();
      range.selectNodeContents(code);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
    })()`);
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Down" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Down" });
    await wait(60);
    await expect(window, "ArrowDown on the last code line opens language and title fields", "!document.querySelector('#write > .code-meta')?.hidden && document.activeElement?.classList.contains('code-language') && document.querySelectorAll('#write > .code-meta input').length === 2");
    await fs.writeFile(codeMetaScreenshotPath, await capturePNG(window));
    await inspect(window, `(() => {
      const language = document.querySelector('.code-meta .code-language');
      language.value = 'rust';
      language.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Right" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Right" });
    await wait(30);
    await expect(window, "left and right arrows switch code metadata fields", "document.activeElement?.classList.contains('code-title')");
    await inspect(window, `(() => {
      const title = document.querySelector('.code-meta .code-title');
      title.value = 'main.rs';
      title.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Down" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Down" });
    await wait(60);
    await expect(window, "code language and title round-trip before ArrowDown exits", "(() => { const anchor = getSelection()?.anchorNode; const block = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('#write > *'); return document.querySelector('#write > pre')?.dataset.language === 'rust' && document.querySelector('#write > pre')?.dataset.title === 'main.rs' && document.querySelector('.code-meta')?.hidden && window.Mory.getMarkdown().includes('```rust title=\"main.rs\"') && block?.tagName === 'P'; })()");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
    })()`);
    await window.webContents.insertText("\u8FD9\u91CC\u662F `hi");
    await window.webContents.insertText("`");
    await window.webContents.insertText(" \u6B63\u6587");
    await expect(window, "paired backticks convert to inline code immediately", "document.querySelector('#write p code')?.textContent === 'hi' && document.querySelector('#write p')?.textContent === '\u8FD9\u91CC\u662F hi \u6B63\u6587' && window.Mory.getMarkdown().includes('`hi`')");

    await inspect(window, `(() => {
      const editor = document.querySelector('#write');
      editor.innerHTML = '<p>| \u540D\u79F0 | \u503C |</p><p>| --- | --- |</p>';
      const separator = editor.lastElementChild;
      const range = document.createRange();
      range.selectNodeContents(separator);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      editor.focus();
      editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '|' }));
    })()`);
    await expectEventually(window, "table headers and separators convert to an editable table immediately", "document.querySelector('#write > table thead th')?.textContent === '\u540D\u79F0' && document.querySelector('#write > table tbody td') && document.querySelector('#write > .table-tools')");
    await click(window, ".table-tools [data-table-action='add-row']");
    await click(window, ".table-tools [data-table-action='add-column']");
    await expect(window, "rendered tables can add rows and columns", "document.querySelectorAll('#write > table tbody tr').length === 2 && document.querySelectorAll('#write > table thead th').length === 3 && window.Mory.getMarkdown().includes('| --- | --- | --- |') && !window.Mory.getMarkdown().includes('\u6DFB\u52A0\u884C')");
    await click(window, ".table-tools [data-table-action='delete-row']");
    await click(window, ".table-tools [data-table-action='delete-column']");
    await expect(window, "table tools delete the current row and column", "document.querySelectorAll('#write > table tbody tr').length === 1 && document.querySelectorAll('#write > table thead th').length === 2 && window.Mory.getMarkdown().includes('| --- | --- |') && !window.Mory.getMarkdown().includes('\u5220\u9664\u884C')");
    await inspect(window, `(() => {
      const cells = [...document.querySelectorAll('#write > table tbody td')];
      const range = document.createRange();
      range.selectNodeContents(cells[0]);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      cells[0].dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Tab' }));
      const anchor = getSelection().anchorNode;
      window.__tableTabTarget = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('td, th') === cells[1];
      const first = document.querySelector('#write > table thead th');
      window.__tableWidthBefore = first.getBoundingClientRect().width;
      const handle = first.querySelector('.table-resize-handle');
      handle.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, button: 0, pointerId: 7, clientX: 100 }));
      document.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, cancelable: true, pointerId: 7, clientX: 124 }));
      document.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, pointerId: 7, clientX: 124 }));
      window.__tableWidthAfter = first.getBoundingClientRect().width;
    })()`);
    await expect(window, "Tab moves between table cells and column separators resize the table", "window.__tableTabTarget && document.querySelector('#write > table colgroup') && window.__tableWidthAfter > window.__tableWidthBefore");

    await inspect(window, `(() => {
      const editor = document.querySelector('#write');
      editor.innerHTML = '<p>| \u8868\u5934 | \u8868\u5934 |</p>';
      const paragraph = editor.firstElementChild;
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      editor.focus();
      paragraph.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }));
    })()`);
    await expect(window, "Enter after a pipe header creates a table", "document.querySelectorAll('#write > table thead th').length === 2 && document.querySelectorAll('#write > table tbody td').length === 2 && document.querySelector('#write > .table-tools')");
    await inspect(window, `(() => {
      const tools = document.querySelector('#write > .table-tools');
      const paragraph = document.createElement('p');
      paragraph.append(document.createElement('br'));
      tools.after(paragraph);
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      paragraph.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Backspace' }));
    })()`);
    await expect(window, "Delete beside a table removes the entire table", "!document.querySelector('#write > table') && !document.querySelector('#write > .table-tools') && document.querySelector('#write > p')");

    await inspect(window, `(() => {
      const editor = document.querySelector('#write');
      editor.innerHTML = '<p>- \u65E0\u5E8F\u9879\u76EE</p><p>1. \u6709\u5E8F\u9879\u76EE</p>';
      for (const paragraph of [...editor.children]) {
        const range = document.createRange();
        range.selectNodeContents(paragraph);
        range.collapse(false);
        getSelection().removeAllRanges();
        getSelection().addRange(range);
        paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '\u76EE' }));
      }
    })()`);
    await expect(window, "dash and numeric markers convert to toolbar-equivalent lists", "document.querySelector('#write > ul > li')?.textContent === '\u65E0\u5E8F\u9879\u76EE' && document.querySelector('#write > ol > li')?.textContent === '\u6709\u5E8F\u9879\u76EE' && window.Mory.getMarkdown().includes('- \u65E0\u5E8F\u9879\u76EE') && window.Mory.getMarkdown().includes('1. \u6709\u5E8F\u9879\u76EE')");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('\u4FDD\u7559\u7684\u539F\u6587');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph.firstChild, 0);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
      document.execCommand('insertText', false, '-');
      paragraph.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: ' ' }));
    })()`);
    await expect(window, "list conversion preserves text after a marker inserted at the start", "document.querySelector('#write > ul > li')?.textContent === '\u4FDD\u7559\u7684\u539F\u6587' && window.Mory.getMarkdown() === '- \u4FDD\u7559\u7684\u539F\u6587'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('> \u5F15\u7528\u5185\u5BB9');
      const quote = document.querySelector('#write blockquote');
      const range = document.createRange();
      range.selectNodeContents(quote);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      quote.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }));
      const emptyQuote = document.querySelectorAll('#write > blockquote')[1];
      emptyQuote.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }));
    })()`);
    await expect(window, "a second Enter exits consecutive blockquotes", "document.querySelectorAll('#write > blockquote').length === 1 && document.querySelector('#write > blockquote')?.textContent === '\u5F15\u7528\u5185\u5BB9' && document.querySelector('#write > blockquote + p')");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('## AlphaBeta');
      const heading = document.querySelector('#write h2');
      const range = document.createRange();
      range.setStart(heading.firstChild, 5);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      heading.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }));
    })()`);
    await expect(window, "Enter in the middle of a heading keeps the prefix heading and moves the suffix to a paragraph", "document.querySelector('#write > h2')?.textContent === 'Alpha' && document.querySelector('#write > h2 + p')?.textContent === 'Beta' && window.Mory.getMarkdown() === '## Alpha\\n\\nBeta'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('# Title');
      const heading = document.querySelector('#write h1');
      const range = document.createRange();
      range.setStart(heading.firstChild, 0);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      heading.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Backspace' }));
    })()`);
    await expect(window, "Backspace at a heading start removes heading formatting without merging blocks", "!document.querySelector('#write > h1') && document.querySelector('#write > p')?.textContent === 'Title' && window.Mory.getMarkdown() === 'Title'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('# Undo');
      const heading = document.querySelector('#write h1');
      const range = document.createRange();
      range.selectNodeContents(heading);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
    })()`);
    await window.webContents.insertText('X');
    await inspect(window, "document.querySelector('#write').dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'z', ctrlKey: true }))");
    await expectEventually(window, "custom history undoes edits inside rendered headings", "window.Mory.getMarkdown() === '# Undo' && document.querySelector('#write h1')?.textContent === 'Undo'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('# Draft Image');
      const paragraph = document.querySelector('#write h1');
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      const file = new File([new Uint8Array([137, 80, 78, 71])], 'cover.png', { type: 'image/png' });
      const paste = new Event('paste', { bubbles: true, cancelable: true });
      Object.defineProperty(paste, 'clipboardData', { value: { files: [file], getData: () => '' } });
      document.querySelector('#write').dispatchEvent(paste);
    })()`);
    await expectEventually(window, "pasted images use the displayed document name and render from the archived asset path", "window.__lastHostRequest.args.documentName === 'Draft Image.md' && window.Mory.getMarkdown().includes('![cover](\\u6587\\u7AE0/cover.png)') && document.querySelector('#write img')");
    await inspect(window, "document.querySelector('#write').dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'z', ctrlKey: true }))");
    await expectEventually(window, "custom history undoes asynchronous image insertion", "window.Mory.getMarkdown() === '# Draft Image' && !document.querySelector('#write img')");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.setStart(paragraph, 0);
      range.collapse(true);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
    })()`);
    await click(window, "#toolbar [data-command='calendar']");
    await expect(window, "calendar toolbar action opens the structured calendar editor", "document.querySelector('#calendar-dialog').classList.contains('is-open') && document.querySelectorAll('#calendar-editor-days .calendar-editor-day').length === 42 && !document.querySelector('#write > table')");
    await inspect(window, `(() => {
      const currentDays = [...document.querySelectorAll('#calendar-editor-days .calendar-editor-day:not(.is-outside)')];
      window.__calendarTestDates = {
        day24: currentDays.find(day => day.querySelector(':scope > span')?.textContent === '24').dataset.date,
        day25: currentDays.find(day => day.querySelector(':scope > span')?.textContent === '25').dataset.date,
        day27: currentDays.find(day => day.querySelector(':scope > span')?.textContent === '27').dataset.date
      };
    })()`);
    await inspect(window, "document.querySelector(`#calendar-editor-days [data-date=\"${window.__calendarTestDates.day24}\"]`).click()");
    await inspect(window, `(() => { const input = document.querySelector('#calendar-mark-title'); input.value = 'Important day'; input.dispatchEvent(new InputEvent('input', { bubbles: true, data: 'Important day' })); })()`);
    await click(window, "#calendar-mark-colors [data-calendar-color='red']");
    await click(window, "#calendar-save-mark");
    await click(window, "[data-calendar-mode='range']");
    await inspect(window, "document.querySelector(`#calendar-editor-days [data-date=\"${window.__calendarTestDates.day24}\"]`).click()");
    await inspect(window, "document.querySelector(`#calendar-editor-days [data-date=\"${window.__calendarTestDates.day27}\"]`).click()");
    await expect(window, "calendar ranges use a closed interval with an explicit day count", "document.querySelector('#calendar-range-selection').textContent.includes('4 \u5929')");
    await inspect(window, `(() => { const input = document.querySelector('#calendar-range-title'); input.value = 'Frontend delivery'; input.dispatchEvent(new InputEvent('input', { bubbles: true, data: 'Frontend delivery' })); })()`);
    await click(window, "#calendar-range-colors [data-calendar-color='green']");
    await click(window, "#calendar-save-range");
    await click(window, "[data-calendar-mode='items']");
    await inspect(window, "document.querySelector(`#calendar-editor-days [data-date=\"${window.__calendarTestDates.day25}\"]`).click()");
    await inspect(window, `(() => { const input = document.querySelector('#calendar-item-text'); input.value = 'Review interaction'; })()`);
    await click(window, "#calendar-item-form button[type='submit']");
    await click(window, "#calendar-apply");
    await expect(window, "calendar saves marks, continuous ranges, and collapsed date items", "(() => { const block = document.querySelector('#write > .calendar-block'); const markdown = window.Mory.getMarkdown(); return block && block.querySelectorAll('.calendar-day-cell').length === 42 && block.querySelector('.calendar-day-cell[data-calendar-color=\"red\"]') && block.querySelectorAll('.calendar-range-bar[data-calendar-color=\"green\"]').length === 4 && block.querySelector('.calendar-day-items:not([open]) summary').textContent.includes('1') && markdown.startsWith('```calendar') && markdown.includes('Important day') && markdown.includes('Frontend delivery') && markdown.includes('Review interaction') && !markdown.includes('| --- |'); })()");
    await inspect(window, `(() => {
      const day24 = document.querySelector('#write .calendar-day-cell[data-date="' + window.__calendarTestDates.day24 + '"]');
      day24.click();
    })()`);
    await expectEventually(window, "clicking a rendered date opens its compact title and item editor", "document.querySelector('.calendar-date-quick-editor') && !document.querySelector('#calendar-dialog').classList.contains('is-open') && document.activeElement === document.querySelector('.calendar-date-quick-editor .calendar-quick-field input')");
    await inspect(window, `(() => {
      const title = document.querySelector('.calendar-date-quick-editor .calendar-quick-field input');
      title.value = 'Launch day';
      title.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
      document.querySelector('.calendar-date-quick-editor .calendar-quick-colors [data-calendar-color="amber"]').click();
      const item = document.querySelector('.calendar-date-quick-editor .calendar-quick-item-form input');
      item.value = 'Publish release notes';
      item.closest('form').requestSubmit();
    })()`);
    await expect(window, "date items can be added without opening the full calendar editor", "document.querySelectorAll('.calendar-date-quick-editor .calendar-quick-item').length === 1 && document.querySelector('.calendar-date-quick-editor .calendar-quick-item span').textContent === 'Publish release notes'");
    await click(window, ".calendar-date-quick-editor > footer .primary-button");
    await expect(window, "the compact date editor saves its title, color, and item to Markdown", "(() => { const cell = document.querySelector(`#write .calendar-day-cell[data-date=\"${window.__calendarTestDates.day24}\"]`); const markdown = window.Mory.getMarkdown(); return !document.querySelector('.calendar-quick-editor') && cell.dataset.calendarColor === 'amber' && cell.textContent.includes('Launch day') && markdown.includes('Launch day') && markdown.includes('Publish release notes'); })()");
    await inspect(window, `(() => {
      const currentDays = [...document.querySelectorAll('#write .calendar-day-cell:not(.is-outside)')];
      window.__calendarTestDates.day28 = currentDays.find(day => day.querySelector('.calendar-date-number')?.textContent === '28').dataset.date;
      window.__calendarTestDates.day30 = currentDays.find(day => day.querySelector('.calendar-date-number')?.textContent === '30').dataset.date;
      const start = document.querySelector('#write .calendar-day-cell[data-date="' + window.__calendarTestDates.day30 + '"]');
      const end = document.querySelector('#write .calendar-day-cell[data-date="' + window.__calendarTestDates.day28 + '"]');
      const startRect = start.getBoundingClientRect();
      const endRect = end.getBoundingClientRect();
      const pointer = (type, target, rect) => target.dispatchEvent(new PointerEvent(type, {
        bubbles: true,
        cancelable: true,
        pointerId: 47,
        pointerType: 'mouse',
        button: 0,
        clientX: rect.left + rect.width / 2,
        clientY: rect.top + rect.height / 2
      }));
      pointer('pointerdown', start, startRect);
      pointer('pointermove', document, endRect);
      window.__calendarDragPreviewCount = document.querySelectorAll('#write .calendar-day-cell.is-drag-selected').length;
      pointer('pointerup', document, endRect);
    })()`);
    await expect(window, "dragging backward previews and opens a normalized closed range", "(() => { const panel = document.querySelector('.calendar-range-quick-editor'); return window.__calendarDragPreviewCount === 3 && panel && panel.querySelector('header strong').textContent.includes('3 \u5929') && panel.querySelector('header strong').textContent.indexOf('28') < panel.querySelector('header strong').textContent.indexOf('30'); })()");
    await fs.writeFile(calendarDirectScreenshotPath, await capturePNG(window));
    await inspect(window, `(() => {
      const title = document.querySelector('.calendar-range-quick-editor .calendar-quick-field input');
      title.value = 'Direct planning';
      title.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
      document.querySelector('.calendar-range-quick-editor .calendar-quick-colors [data-calendar-color="blue"]').click();
    })()`);
    await click(window, ".calendar-range-quick-editor > footer .primary-button");
    await expect(window, "drag-created ranges render continuously and persist without the full editor", "(() => { const markdown = window.Mory.getMarkdown(); return document.querySelectorAll('#write .calendar-range-bar[data-calendar-color=\"blue\"]').length === 3 && markdown.includes('Direct planning') && markdown.includes(window.__calendarTestDates.day28) && markdown.includes(window.__calendarTestDates.day30); })()");
    await inspect(window, `(() => { const cell = document.querySelector('.calendar-day-cell:has(.calendar-day-items)'); window.__calendarCellHeight = cell.getBoundingClientRect().height; cell.querySelector('summary').click(); })()`);
    await expect(window, "expanded date items do not resize the calendar grid", "(() => { const cell = document.querySelector('.calendar-day-cell:has(.calendar-day-items)'); return cell.querySelector('details').open && cell.getBoundingClientRect().height === window.__calendarCellHeight; })()");
    await inspect(window, `(() => { const markdown = window.Mory.getMarkdown(); window.Mory.loadMarkdown(markdown); })()`);
    await expect(window, "calendar data survives a Markdown reload", "document.querySelectorAll('#write > .calendar-block .calendar-range-bar[data-calendar-color=\"green\"]').length === 4 && window.Mory.getMarkdown().includes('```calendar')");
    await inspect(window, "window.Mory.exportDocument({ theme: 'current' }).then(html => { window.__calendarExportHTML = html; })");
    await expectEventually(window, "calendar exports as a rendered month without embedded JSON", "window.__calendarExportHTML.includes('class=\"calendar-block\"') && window.__calendarExportHTML.includes('Frontend delivery') && !window.__calendarExportHTML.includes('data-calendar-source') && !window.__calendarExportHTML.includes('&quot;version&quot;')");

    await inspect(window, `window.Mory.loadMarkdown(${JSON.stringify("# Mory\u7F16\u8F91\u5668\n\n**\u4E2D\u6587English**\n\n\`const value=1\`")})`);
    await click(window, "#toolbar [data-command='typography']");
    await expect(window, "typography optimization adds CJK spacing without changing inline code", "window.Mory.getMarkdown().includes('Mory \u7F16\u8F91\u5668') && window.Mory.getMarkdown().includes('\u4E2D\u6587 English') && window.Mory.getMarkdown().includes('`const value=1`')");
    await inspect(window, "window.Mory.exportDocument({ format: 'mindmap' }).then(html => { window.__mindMapHTML = html; })");
    await expect(window, "mind-map export produces a standalone heading map", "window.__mindMapHTML.startsWith('<!doctype html>') && window.__mindMapHTML.includes('<svg') && window.__mindMapHTML.includes('Mory') && !window.__mindMapHTML.includes('<script')");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown(${JSON.stringify("```go\nfunc main() {\n  fmt.Println(\"hello\")\n}\n```\n\n\u6B63\u6587")});
      const editor = document.querySelector('#write');
      const code = editor.querySelector('pre code');
      const paragraph = editor.querySelector('p');
      editor.focus();
      let range = document.createRange();
      range.selectNodeContents(code);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.dispatchEvent(new Event('selectionchange'));
      range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.dispatchEvent(new Event('selectionchange'));
    })()`);
    await expect(window, "syntax highlighting returns immediately after leaving a code block", "document.querySelector('#write pre[data-highlighted=\"true\"] code .hljs-keyword')?.textContent === 'func'");

    await inspect(window, `window.Mory.loadMarkdown(${JSON.stringify("**\u6587\u672C\u52A0\u7C97**  \\*\\* \u6B63\u5E38\u663E\u793A\u661F\u53F7 \\*\\*")})`);
    await expect(window, "backslash-escaped asterisks remain plain text through round trips", "document.querySelectorAll('#write strong').length === 1 && document.querySelector('#write').textContent.includes('** \u6B63\u5E38\u663E\u793A\u661F\u53F7 **') && !document.querySelector('#write').textContent.includes('\\\\') && window.Mory.getMarkdown() === " + JSON.stringify("**\u6587\u672C\u52A0\u7C97**  \\*\\* \u6B63\u5E38\u663E\u793A\u661F\u53F7 \\*\\*"));

    await inspect(window, `(() => {
      window.Mory.setWorkspaceSnapshot({
        state: { activeId: 'workspace-links', workspaces: [{ id: 'workspace-links', name: '\u94FE\u63A5\u6D4B\u8BD5', provider: 'local', localPath: '/links' }] },
        files: [
          { name: '\u5F53\u524D.md', path: '/links/\u5F53\u524D.md', createdAt: 1, images: [{ name: '\u5C01\u9762.png', path: '/links/\u5F53\u524D/\u5C01\u9762.png', relative: '\u5F53\u524D/\u5C01\u9762.png' }] },
          { name: '\u8D44\u6599/\u76EE\u6807.md', path: '/links/\u8D44\u6599/\u76EE\u6807.md', createdAt: 2 }
        ],
        directories: [{ name: '\u8D44\u6599', path: '/links/\u8D44\u6599', createdAt: 1 }]
      });
      window.Mory.openDocument({ name: '\u5F53\u524D.md', path: '/links/\u5F53\u524D.md', markdown: '' });
    })()`);
    await wait(60);
    await inspect(window, `(() => {
      const paragraph = document.querySelector('#write p');
      paragraph.textContent = '![Alt](./\u8D44\u6599/\u76EE';
      document.querySelector('#write').focus();
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '\u76EE' }));
    })()`);
    await expect(window, "workspace-relative paths show document suggestions", "document.querySelector('#path-suggestions').classList.contains('is-open') && document.querySelector('#path-suggestions button.is-selected')?.dataset.path === './\u8D44\u6599/\u76EE\u6807.md'");
    await inspect(window, "document.querySelector('#write p').dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }))");
    await expect(window, "Enter selects a document suggestion and normalizes the link", "document.querySelector('#write a.document-link')?.textContent === 'Alt' && document.querySelector('#write a.document-link')?.getAttribute('href') === './\u8D44\u6599/\u76EE\u6807.md' && !document.querySelector('#write img') && window.Mory.getMarkdown() === '[Alt](./\u8D44\u6599/\u76EE\u6807.md)'");

    await inspect(window, `(() => {
      const paragraph = document.querySelector('#write p');
      paragraph.textContent = '![\u5C01\u9762](./\u5F53\u524D/\u5C01';
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '\u5C01' }));
      paragraph.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' }));
    })()`);
    await expect(window, "image suggestions preserve image syntax and render immediately", "document.querySelector('#write img')?.getAttribute('src') === './\u5F53\u524D/\u5C01\u9762.png' && window.Mory.getMarkdown() === '![\u5C01\u9762](./\u5F53\u524D/\u5C01\u9762.png)'");

    await inspect(window, `(() => {
      const paragraph = document.querySelector('#write p');
      paragraph.textContent = '[\u5F15\u7528](./';
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '/' }));
      const before = document.querySelector('#path-suggestions button.is-selected')?.dataset.path;
      paragraph.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'ArrowDown' }));
      window.__pathSuggestionMoved = before !== document.querySelector('#path-suggestions button.is-selected')?.dataset.path;
      paragraph.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Escape' }));
    })()`);
    await expect(window, "suggestions support arrow navigation and Escape dismissal", "window.__pathSuggestionMoved && !document.querySelector('#path-suggestions').classList.contains('is-open')");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('[Target](./\u8D44\u6599/\u76EE\u6807.md)');
      document.querySelector('#write a.document-link').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, metaKey: true }));
    })()`);
    await expectEventually(window, "Command-click opens workspace-relative Markdown links in Mory", "window.__lastHostRequest.method === 'readDocument' && window.__lastHostRequest.args.path === '/links/\u8D44\u6599/\u76EE\u6807.md'");

    await inspect(window, `window.Mory.loadMarkdown('# \u683C\u5F0F\u6D4B\u8BD5\\n\\n\u9700\u8981\u52A0\u7C97\u7684\u6BB5\u843D')`);
    await inspect(window, `(() => {
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
    })()`);
    await click(window, "#toolbar button[data-command='bold']");
    await expect(window, "format button is clickable", "document.querySelector('#write strong, #write b') !== null");

    if (errors.length) throw new Error(`Renderer errors: ${errors.join(" | ")}`);
    process.stdout.write(JSON.stringify({ status: "passed", interactions: interactionCount, rendererErrors: 0, dpiStableTypography: true, tableRowColumnDeletion: true, bundledThemeFonts: true, lapisCVResumeTheme: true, multipleUntitledDocuments: true, draftSwitchingPreservesContent: true, removableUntitledDocuments: true, savedDocumentTrash: true, deleteCancellation: true, activeDocumentCloseFallback: true, lastCloseCreatesBlankDocument: true, emptyWorkspacePlaceholder: true, nonEmptyWorkspaceAutoOpen: true, deletedWorkspaceFileReconciled: true, headingEnterCreatesParagraph: true, compositionHeadingRendering: true, immediateCompositionEnter: true, separateConsecutiveHeadings: true, staleHeadingRecovery: true, liveBold: true, pastedMarkdownRendering: true, liveFencedCode: true, multiLineFencedCode: true, fencedCodeExit: true, doubleEnterCodeExit: true, codeMetadataNavigation: true, liveInlineCode: true, instantHeading: true, liveUnsavedOutline: true, workspaceCreationOrder: true, stableOpenedFilePosition: true, statusbarSetting: true, zoomedStatusbar: true, readableSidebarContrast: true, coherentDarkDocument: true, readableGroupedPreferences: true, iconOnlyToolbar: true, hoverTooltip: true, sidebarSearchRemoved: true, verticalFloatingToolbar: true, singleSettingsEntry: true, macTrafficLightSafeArea: true, screenshot: screenshotPath, preferencesScreenshot: preferencesScreenshotPath, codeMetaScreenshot: codeMetaScreenshotPath, lapisCVScreenshot: lapisCVScreenshotPath, darkThemeScreenshot: darkThemeScreenshotPath, mermaidScreenshot: mermaidScreenshotPath, mermaidExpandedScreenshot: mermaidExpandedScreenshotPath, mermaidNarrowScreenshot: mermaidNarrowScreenshotPath, calendarDirectScreenshot: calendarDirectScreenshotPath }, null, 2) + "\n");
  } catch (error) {
    process.stderr.write(`${error.stack || error}${errors.length ? `\nRenderer errors: ${errors.join(" | ")}` : ""}\n`);
    exitCode = 1;
  } finally {
    window.destroy();
    app.exit(exitCode);
  }
});
