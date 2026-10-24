const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { app, BrowserWindow, clipboard, ClipboardItem } = require('electron');
const { snapshotClipboard } = require('./clipboard-snapshot.cjs');

async function runNativeCodeInteractions(window) {
  const evaluate = script => window.webContents.executeJavaScript(script, true);
  const settle = () => evaluate('new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))');
  const source = 'const first = 1;\nconst second = 2;\nconst third = 3;';
  const start = source.indexOf('second') + 2;
  const end = source.indexOf('third') + 3;
  for (const backward of [false, true]) {
    const points = await evaluate(`(() => {
      window.Mory.toggleSource(false);
      window.Mory.loadMarkdown(${JSON.stringify('```js\n' + source + '\n```\n\nAfter')});
      const code = document.querySelector('#write pre code');
      code.scrollIntoView({ block: 'center' });
      const pointAt = offset => {
        const walker = document.createTreeWalker(code, NodeFilter.SHOW_TEXT);
        while (walker.nextNode()) {
          const node = walker.currentNode;
          if (offset < node.length) {
            const range = document.createRange();
            range.setStart(node, offset);
            range.setEnd(node, offset + 1);
            const rect = range.getBoundingClientRect();
            const point = { x: Math.round(rect.left), y: Math.round(rect.top + rect.height / 2) };
            if (!code.contains(document.elementFromPoint(point.x, point.y))) {
              throw new Error('Code selection point is obscured: ' + JSON.stringify(point));
            }
            return point;
          }
          offset -= node.length;
        }
        throw new Error('Code selection point was not found');
      };
      return [pointAt(${backward ? end : start}), pointAt(${backward ? start : end})];
    })()`);
    window.webContents.sendInputEvent({ type: 'mouseMove', ...points[0] });
    window.webContents.sendInputEvent({ type: 'mouseDown', ...points[0], button: 'left', clickCount: 1 });
    await settle();
    for (let step = 1; step <= 4; step += 1) {
      const fraction = step / 4;
      window.webContents.sendInputEvent({ type: 'mouseMove', button: 'left', modifiers: ['leftButtonDown'],
        x: Math.round(points[0].x + (points[1].x - points[0].x) * fraction),
        y: Math.round(points[0].y + (points[1].y - points[0].y) * fraction) });
      await settle();
    }
    window.webContents.sendInputEvent({ type: 'mouseUp', ...points[1], button: 'left', clickCount: 1 });
    await settle();
    assert.equal(await evaluate('getSelection().toString()'), source.slice(start, end),
      `${backward ? 'Backward' : 'Forward'} mouse drag selected text outside its endpoints`);
  }
  window.webContents.focus();
  await evaluate("document.querySelector('.code-fold-toggle').focus()");
  window.webContents.sendInputEvent({ type: 'keyDown', keyCode: 'Enter' });
  // sendInputEvent separates key transitions from the native character event.
  window.webContents.sendInputEvent({ type: 'char', keyCode: '\r' });
  window.webContents.sendInputEvent({ type: 'keyUp', keyCode: 'Enter' });
  await settle();
  assert.equal(await evaluate("document.querySelector('.code-fold-toggle').getAttribute('aria-expanded')"), 'false',
    'Enter did not collapse the focused code block');
  window.webContents.sendInputEvent({ type: 'keyDown', keyCode: 'Space' });
  window.webContents.sendInputEvent({ type: 'char', keyCode: ' ' });
  window.webContents.sendInputEvent({ type: 'keyUp', keyCode: 'Space' });
  await settle();
  assert.equal(await evaluate("document.querySelector('.code-fold-toggle').getAttribute('aria-expanded')"), 'true',
    'Space did not expand the focused code block');
  process.stdout.write('Native code interactions passed: forward/backward mouse drag and Enter/Space folding\n');

  // Restore every clipboard representation, not just plain text, after native paste.
  const savedClipboard = await snapshotClipboard(clipboard, ClipboardItem);
  const inserted = '\t# literal\n\n**literal** <span>\n';
  try {
    await evaluate(`(() => {
      window.Mory.loadMarkdown('Before\\n\\n\`\`\`js title="sample.js"\\nconst first = 1;\\nconst last = 2;\\n\`\`\`\\n\\nAfter');
      const code = document.querySelector('#write pre code');
      const text = code.querySelector('.hljs-keyword').firstChild;
      getSelection().setBaseAndExtent(text, 2, text, 2);
      document.querySelector('#write').focus();
    })()`);
    await clipboard.write([new ClipboardItem({
      'text/plain': inserted,
      'text/html': '<b>Rich clipboard</b>'
    })]);
    window.webContents.paste();
    await settle();
    assert.equal(await evaluate('window.Mory.getMarkdown()'),
      'Before\n\n```js title="sample.js"\nco\t# literal\n\n**literal** <span>\nnst first = 1;\nconst last = 2;\n```\n\nAfter',
      'Native clipboard paste split or reformatted literal code');
    assert.equal(await evaluate("document.querySelectorAll('#write pre').length"), 1,
      'Native clipboard paste created extra code blocks');
    process.stdout.write('Native multiline clipboard paste passed\n');
  } finally {
    // Preserve a newer copy if the user changed the clipboard while this test ran.
    if (await clipboard.readText() === inserted) {
      if (savedClipboard.length) await clipboard.write(savedClipboard);
      else clipboard.clear();
    }
  }
}

app.commandLine.appendSwitch('disable-gpu');
app.disableHardwareAcceleration();
app.whenReady().then(async () => {
  // Hidden Windows windows may stop producing animation frames unless rendered offscreen.
  const window = new BrowserWindow({ width: 1180, height: 790, show: false, webPreferences: { offscreen: true, contextIsolation: true, nodeIntegration: false, backgroundThrottling: false } });
  let phase = 'loading the editor';
  // Slow-frame runs spend about 35 seconds in 160 ms frames alone, so they receive a larger budget.
  const budget = process.argv.includes('--slow-frames') ? 90000 : 45000;
  const timeout = setTimeout(() => { process.stderr.write(`Heading input timed out while ${phase}\n`); app.exit(1); }, budget);
  try {
    await window.loadFile(process.env.MORY_WEB_INDEX || path.join(__dirname, '../Sources/Mory/Web/index.html'));
    await window.webContents.executeJavaScript('document.fonts.ready');
    phase = 'running editor interactions';
    process.stdout.write('Heading editor loaded; starting interaction cases\n');
    if (process.argv.includes('--slow-frames')) {
      // Exercise scheduling slower than the old 80 ms sleep, independently of the host OS.
      await window.webContents.executeJavaScript(`
        window.requestAnimationFrame = callback => setTimeout(() => callback(performance.now()), 160);
        window.cancelAnimationFrame = handle => clearTimeout(handle);
        void 0;
      `);
    }
    const cases = await fs.readFile(path.join(__dirname, 'heading-input-cases.js'), 'utf8');
    const result = await window.webContents.executeJavaScript(`${cases}\nrunHeadingInputCases()`, true);
    process.stdout.write(JSON.stringify(result, null, 2) + '\n');
    assert.deepEqual(result.failures, [], 'Heading input scenarios failed');
    phase = 'running native code interactions';
    await runNativeCodeInteractions(window);
    clearTimeout(timeout);
    app.exit(0);
  } catch (error) {
    process.stderr.write(String(error.stack || error) + '\n');
    app.exit(1);
  }
});
