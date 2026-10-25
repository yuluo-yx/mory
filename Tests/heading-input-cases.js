async function runHeadingInputCases() {
  const editor = document.querySelector('#write');
  const passed = [];
  const failures = [];
  const check = (condition, message) => { if (!condition) throw new Error(`${message}; DOM=${editor.innerHTML}`); };
  const settle = () => new Promise((resolve, reject) => {
    let frame;
    const timeout = setTimeout(() => {
      cancelAnimationFrame(frame);
      reject(new Error('Editor animation frames did not settle within 5 seconds'));
    }, 5000);
    // Composition commits can queue normalization for the following frame.
    frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => {
        clearTimeout(timeout);
        resolve();
      });
    });
  });
  const caret = (node, offset) => {
    const range = document.createRange();
    range.setStart(node, offset);
    range.collapse(true);
    getSelection().removeAllRanges();
    getSelection().addRange(range);
    editor.focus();
  };
  const reset = () => {
    window.Mory.toggleSource(false);
    window.Mory.loadMarkdown('Old document\n\nSecond paragraph');
    editor.focus();
    document.execCommand('selectAll');
    document.execCommand('delete');
  };
  const run = async (name, test) => {
    try { await test(); passed.push(name); }
    catch (error) { failures.push({ name, error: error.message }); }
    await settle();
  };
  const paragraphBreak = () => {
    // execCommand emits input but does not emit the native beforeinput event in either engine.
    if (editor.dispatchEvent(new InputEvent('beforeinput', { bubbles: true, cancelable: true, inputType: 'insertParagraph' }))) {
      document.execCommand('insertParagraph');
    }
  };
  const longCode = Array.from({ length: 40 }, (_, index) => `const value${index} = ${index};`).join('\n');
  const paste = text => {
    const clipboard = new DataTransfer();
    clipboard.setData('text/plain', text);
    clipboard.setData('text/html', '<p>Untrusted rich clipboard content</p>');
    const event = new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: clipboard });
    // WebKit does not expose DataTransfer on constructed clipboard events.
    if (event.clipboardData !== clipboard) Object.defineProperty(event, 'clipboardData', { value: clipboard });
    editor.dispatchEvent(event);
  };
  for (const offset of [0, 16, 32]) {
    await run(`multiline paste at code offset ${offset} preserves a single literal code block`, async () => {
      const original = 'const first = 1;\nconst last = 2;\n';
      const inserted = '\t# literal\r\n\r\n**literal** <span>\r\n';
      window.Mory.loadMarkdown(`\`\`\`js title="sample.js"\n${original}\`\`\`\n\nAfter`);
      const block = editor.querySelector('pre');
      // Select by text offset across highlight spans, as a user clicking into code would.
      const walker = document.createTreeWalker(block.querySelector('code'), NodeFilter.SHOW_TEXT);
      let remaining = Math.min(offset, original.length - 1);
      while (walker.nextNode()) {
        if (remaining <= walker.currentNode.length) { caret(walker.currentNode, remaining); break; }
        remaining -= walker.currentNode.length;
      }
      paste(inserted);
      await settle();
      const position = Math.min(offset, original.length - 1);
      const expected = original.slice(0, position) + inserted.replaceAll('\r\n', '\n') + original.slice(position);
      check(editor.querySelectorAll('pre').length === 1, 'Pasting split one code block into several');
      check(editor.querySelector('pre') === block && block.querySelector('code').textContent === expected.replace(/\n$/, ''), 'Pasting changed code, indentation, or blank lines');
      check(!block.querySelector('strong, span:not([class])'), 'Rich or Markdown clipboard content was rendered');
      check(block.dataset.language === 'js' && block.dataset.title === 'sample.js', 'Pasting lost code metadata');
      check(editor.querySelector('p')?.textContent === 'After', 'Pasting changed the following paragraph');
      check(block.contains(getSelection().anchorNode), 'Pasting moved the caret outside code');
    });
  }
  await run('multiline paste into empty code preserves its block and caret', async () => {
    window.Mory.loadMarkdown('```txt\n\n```\n\nAfter');
    const block = editor.querySelector('pre');
    const code = block.querySelector('code');
    caret(code, 0);
    paste('first\n\tsecond\n\nlast');
    await settle();
    check(editor.querySelectorAll('pre').length === 1 && editor.querySelector('pre') === block, 'Empty code paste split or replaced its block');
    check(window.Mory.getMarkdown() === '```txt\nfirst\n\tsecond\n\nlast\n```\n\nAfter', 'Empty code paste lost literal lines');
    check(code.contains(getSelection().anchorNode), 'Empty code paste lost its caret');
  });
  await run('multiline paste replaces a backward code selection and supports undo and redo', async () => {
    const markdown = '```js\nfirst\nsecond\nthird\n```\n\nAfter';
    window.Mory.loadMarkdown(markdown);
    const code = editor.querySelector('pre code');
    // Plain code also occurs after typing; exercise a backward selection across lines.
    code.textContent = 'first\nsecond\nthird';
    editor.focus();
    getSelection().setBaseAndExtent(code.firstChild, 12, code.firstChild, 6);
    paste('new\n\tlines');
    await settle();
    const expected = '```js\nfirst\nnew\n\tlines\nthird\n```\n\nAfter';
    check(window.Mory.getMarkdown() === expected, 'Pasting replaced text outside the selected range');
    editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, bubbles: true, cancelable: true }));
    await settle();
    check(window.Mory.getMarkdown() === markdown, 'Undo did not restore code before pasting');
    editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, shiftKey: true, bubbles: true, cancelable: true }));
    await settle();
    check(window.Mory.getMarkdown() === expected, 'Redo did not restore the multiline paste');
  });
  await run('typed code line breaks survive a later paste', async () => {
    window.Mory.loadMarkdown('```js\nconst first = 1;\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    const walker = document.createTreeWalker(code, NodeFilter.SHOW_TEXT);
    let last = null;
    while (walker.nextNode()) last = walker.currentNode;
    caret(last, last.length);
    paragraphBreak();
    document.execCommand('insertText', false, 'const second = 2;');
    paragraphBreak();
    paste('const third = 3;\nconst fourth = 4;');
    await settle();
    check(window.Mory.getMarkdown() === '```js\nconst first = 1;\nconst second = 2;\nconst third = 3;\nconst fourth = 4;\n```\n\nAfter',
      'Pasting after typed line breaks merged code lines');
    check(editor.querySelectorAll('pre').length === 1, 'Pasting after typed line breaks split the code block');
  });
  await run('typed code line breaks survive highlighting after the caret leaves code', async () => {
    window.Mory.loadMarkdown('```js\nconst first = 1;\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    const walker = document.createTreeWalker(code, NodeFilter.SHOW_TEXT);
    let last = null;
    while (walker.nextNode()) last = walker.currentNode;
    caret(last, last.length);
    paragraphBreak();
    document.execCommand('insertText', false, 'const second = 2;');
    caret(editor.querySelector('p').firstChild, 0);
    document.dispatchEvent(new Event('selectionchange'));
    await settle();
    check(editor.querySelector('pre').dataset.highlighted === 'true', 'Leaving code did not restore highlighting');
    check(editor.querySelector('pre code').textContent === 'const first = 1;\nconst second = 2;', 'Highlighting merged typed code lines');
    check(window.Mory.getMarkdown() === '```js\nconst first = 1;\nconst second = 2;\n```\n\nAfter', 'Highlighting changed code Markdown');
  });
  await run('native line break elements in code are read as newlines', async () => {
    window.Mory.loadMarkdown('```txt\nfirst\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    // WebKit and Chromium can leave BR or DIV line nodes after native editing commands.
    code.innerHTML = 'first<br>second<div>third</div><div><br></div><div>fifth</div>';
    caret(code.lastChild.firstChild, 5);
    paste('!');
    await settle();
    check(code.textContent === 'first\nsecond\nthird\n\nfifth!', 'Native line nodes were dropped from code');
    check(window.Mory.getMarkdown() === '```txt\nfirst\nsecond\nthird\n\nfifth!\n```\n\nAfter', 'Native line nodes changed code Markdown');
  });
  const pressTab = (shiftKey = false) => {
    const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey, bubbles: true, cancelable: true });
    editor.dispatchEvent(event);
    return event;
  };
  const codeSelection = code => {
    const range = getSelection().getRangeAt(0);
    const offset = (node, at) => {
      const measure = document.createRange();
      measure.selectNodeContents(code);
      measure.setEnd(node, at);
      return measure.toString().replaceAll('​', '').length;
    };
    return [offset(range.startContainer, range.startOffset), offset(range.endContainer, range.endOffset)];
  };
  await run('Tab in code inserts indentation and keeps focus in the editor', async () => {
    window.Mory.loadMarkdown('```js\nif (ready) {\nrun();\n}\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    code.textContent = 'if (ready) {\nrun();\n}';
    caret(code.firstChild, 13);
    const event = pressTab();
    await settle();
    check(event.defaultPrevented, 'Tab in code moved focus instead of indenting');
    check(window.Mory.getMarkdown() === '```js\nif (ready) {\n    run();\n}\n```\n\nAfter', 'Tab did not insert one indentation unit');
    check(codeSelection(code).join() === '17,17', 'Tab did not place the caret after the indentation');
    const outdent = pressTab(true);
    await settle();
    check(outdent.defaultPrevented, 'Shift+Tab in code moved focus');
    check(window.Mory.getMarkdown() === '```js\nif (ready) {\nrun();\n}\n```\n\nAfter', 'Shift+Tab did not remove the line indentation');
    check(codeSelection(code).join() === '13,13', 'Shift+Tab did not keep the caret on its text');
    editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, bubbles: true, cancelable: true }));
    await settle();
    check(window.Mory.getMarkdown() === '```js\nif (ready) {\n    run();\n}\n```\n\nAfter', 'Undo did not restore the Tab indentation');
  });
  await run('Tab and Shift+Tab indent every selected code line and follow existing indentation', async () => {
    window.Mory.loadMarkdown('```js\nfunction run() {\n  first();\n  second();\n}\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    code.textContent = 'function run() {\n  first();\n  second();\n}';
    editor.focus();
    getSelection().setBaseAndExtent(code.firstChild, 19, code.firstChild, 31);
    pressTab();
    await settle();
    check(window.Mory.getMarkdown() === '```js\nfunction run() {\n    first();\n    second();\n}\n```\n\nAfter', 'Tab did not indent every selected line by the detected unit');
    check(codeSelection(code).join() === '21,35', 'Tab lost the multiline selection');
    pressTab(true);
    await settle();
    pressTab(true);
    await settle();
    check(window.Mory.getMarkdown() === '```js\nfunction run() {\nfirst();\nsecond();\n}\n```\n\nAfter', 'Shift+Tab did not outdent every selected line');
    check(editor.querySelectorAll('pre').length === 1 && editor.querySelector('pre').dataset.language === 'js', 'Indenting code changed its block');
  });
  await run('Tab in tab-indented code inserts a tab character', async () => {
    window.Mory.loadMarkdown('```go\nfunc main() {\n\tprintln()\n}\n```');
    const code = editor.querySelector('pre code');
    code.textContent = 'func main() {\n\tprintln()\n}';
    caret(code.firstChild, 24);
    pressTab();
    await settle();
    check(window.Mory.getMarkdown() === '```go\nfunc main() {\n\tprintln()\t\n}\n```', `Tab ignored the tab indentation style: ${JSON.stringify(window.Mory.getMarkdown())}`);
  });
  await run('Tab outside lists, tables, and code keeps focus in the editor', async () => {
    window.Mory.loadMarkdown('Plain paragraph');
    caret(editor.querySelector('p').firstChild, 5);
    const event = pressTab();
    await settle();
    check(event.defaultPrevented, 'Tab moved focus out of the editor');
    check(window.Mory.getMarkdown() === 'Plain paragraph', 'Tab changed a plain paragraph');
  });
  await run('Enter in code keeps the current line indentation', async () => {
    window.Mory.loadMarkdown('```js\nfunction run() {\n  first();\n}\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    code.textContent = 'function run() {\n  first();\n}';
    caret(code.firstChild, 27);
    paragraphBreak();
    document.execCommand('insertText', false, 'second();');
    await settle();
    check(window.Mory.getMarkdown() === '```js\nfunction run() {\n  first();\n  second();\n}\n```\n\nAfter',
      `Enter dropped the code indentation: ${JSON.stringify(window.Mory.getMarkdown())}`);
  });
  await run('double Enter after indented code exits without a whitespace line', async () => {
    window.Mory.loadMarkdown('```js\nif (ready) {\n  run();\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    code.textContent = 'if (ready) {\n  run();';
    caret(code.firstChild, code.textContent.length);
    paragraphBreak();
    paragraphBreak();
    await settle();
    check(window.Mory.getMarkdown().startsWith('```js\nif (ready) {\n  run();\n```\n\n'),
      `Exiting code kept an indentation-only line: ${JSON.stringify(window.Mory.getMarkdown())}`);
    check(!editor.querySelector('pre').contains(getSelection().anchorNode), 'Double Enter did not leave code');
  });
  await run('Backspace in code indentation removes one indentation unit', async () => {
    window.Mory.loadMarkdown('```js\nfunction run() {\n    first();\n}\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    code.textContent = 'function run() {\n    first();\n}';
    caret(code.firstChild, 21);
    const event = new KeyboardEvent('keydown', { key: 'Backspace', bubbles: true, cancelable: true });
    editor.dispatchEvent(event);
    await settle();
    check(event.defaultPrevented && window.Mory.getMarkdown() === '```js\nfunction run() {\nfirst();\n}\n```\n\nAfter',
      `Backspace did not remove one indentation unit: ${JSON.stringify(window.Mory.getMarkdown())}`);
    check(codeSelection(code).join() === '17,17', 'Backspace misplaced the caret');
    caret(code.firstChild, 20);
    const text = new KeyboardEvent('keydown', { key: 'Backspace', bubbles: true, cancelable: true });
    editor.dispatchEvent(text);
    check(!text.defaultPrevented, 'Backspace after code text must keep native character deletion');
  });
  await run('Select all inside code selects the code before the document', async () => {
    window.Mory.loadMarkdown('Before\n\n```js\nfirst();\nsecond();\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    caret(code.querySelector('*')?.firstChild || code.firstChild, 1);
    const first = new KeyboardEvent('keydown', { key: 'a', metaKey: true, bubbles: true, cancelable: true });
    editor.dispatchEvent(first);
    check(first.defaultPrevented && getSelection().toString() === 'first();\nsecond();', `Select all did not select the code: ${JSON.stringify(getSelection().toString())}`);
    const second = new KeyboardEvent('keydown', { key: 'a', metaKey: true, bubbles: true, cancelable: true });
    editor.dispatchEvent(second);
    check(!second.defaultPrevented, 'A second select all must select the whole document');
  });
  await run('Tab in source mode indents Markdown and supports undo', async () => {
    try {
      window.Mory.loadMarkdown('- first\n- second');
      window.Mory.toggleSource(true);
      const source = document.querySelector('#source-editor');
      source.focus();
      source.setSelectionRange(8, 8);
      const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
      source.dispatchEvent(event);
      await settle();
      check(event.defaultPrevented && document.activeElement === source, 'Tab moved focus out of the source editor');
      check(source.value === '- first\n    - second' && source.selectionStart === 12, `Tab did not indent the source line: ${JSON.stringify(source.value)}`);
      source.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true }));
      await settle();
      check(source.value === '- first\n- second', 'Shift+Tab did not outdent the source line');
      source.dispatchEvent(new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, bubbles: true, cancelable: true }));
      await settle();
      check(source.value === '- first\n    - second', 'Undo did not restore the source indentation');
    } finally {
      window.Mory.toggleSource(false);
    }
  });
  await run('Tab in code metadata fields keeps native field navigation', async () => {
    window.Mory.loadMarkdown('```js\nvalue\n```\n\nAfter');
    editor.querySelector('.code-language-button').click();
    await settle();
    const language = editor.querySelector('.code-language');
    check(language, 'Code metadata did not open');
    language.focus();
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    language.dispatchEvent(event);
    check(!event.defaultPrevented, 'The editor blocked Tab navigation inside code metadata fields');
  });
  await run('code copy button copies the complete literal code without editing the note', async () => {
    const source = 'const first = 1;\n\n  return `<b>${first}</b>`;';
    window.Mory.loadMarkdown(`\`\`\`js\n${source}\n\`\`\`\n\n\`\`\`txt\nSecond\n\`\`\``);
    const buttons = editor.querySelectorAll('.code-copy-button');
    check(buttons.length === 2, 'Code blocks have no copy buttons');
    check(buttons[0].getAttribute('aria-label') === '\u590D\u5236\u4EE3\u7801' || buttons[0].getAttribute('aria-label') === 'Copy code', 'Copy button has no accessible label');
    const copied = [];
    // Desktop hosts take precedence over the browser clipboard; capture the host request.
    const originalHost = window.moryNative;
    window.moryNative = { ...originalHost, request: async (method, args) => {
      if (method !== 'copyText') throw new Error(`Unexpected host request: ${method}`);
      copied.push(args.text);
      return { copied: true };
    } };
    try {
      const saveState = document.querySelector('#save-state');
      const wasUnsaved = saveState.classList.contains('is-visible');
      buttons[0].click();
      await settle();
      buttons[1].click();
      await settle();
      check(copied[0] === source, 'Copy button copied highlighted markup or partial code');
      check(copied[1] === 'Second', 'Copy button copied another block');
      check(buttons[0].classList.contains('is-copied'), 'Copy button has no copied feedback');
      check(saveState.classList.contains('is-visible') === wasUnsaved, 'Copying marked the note dirty');
      check(!window.Mory.getMarkdown().includes('Copy'), 'Copy control became document text');
    } finally {
      if (originalHost === undefined) delete window.moryNative;
      else window.moryNative = originalHost;
    }
  });
  await run('code folding changes only the view and keeps blocks independent', () => {
    const markdown = `\`\`\`js title="example.js"\n${longCode}\n\`\`\`\n\n\`\`\`txt\nSecond block\n\`\`\`\n\nAfter`;
    window.Mory.loadMarkdown(markdown);
    const blocks = editor.querySelectorAll('pre');
    const buttons = editor.querySelectorAll('.code-fold-toggle');
    check(buttons.length === 2, 'Code blocks have no fold controls');
    const height = blocks[0].getBoundingClientRect().height;
    const snapshot = window.Mory.getDocumentSnapshot();
    buttons[0].click();
    check(buttons[0].getAttribute('aria-expanded') === 'false', 'Fold control did not announce collapsed state');
    check(blocks[0].getBoundingClientRect().height < height / 2, 'Folding did not reduce code height');
    check(blocks[0].textContent === longCode, 'Folding removed code content');
    check(!blocks[0].isContentEditable, 'Collapsed code is still editable');
    check(buttons[1].getAttribute('aria-expanded') === 'true', 'Folding changed another block');
    check(window.Mory.getMarkdown() === markdown, 'Folding changed Markdown');
    check(window.Mory.getDocumentSnapshot().dirty === snapshot.dirty, 'Folding marked the note dirty');
    buttons[0].click();
    check(blocks[0].getBoundingClientRect().height === height && blocks[0].isContentEditable, 'Expanding did not restore editing');
    check(buttons[0].getAttribute('aria-expanded') === 'true', 'Expanding did not update accessibility state');
  });
  await run('editing beside folded code serializes all code without controls', () => {
    window.Mory.loadMarkdown(`\`\`\`js\n${longCode}\n\`\`\`\n\nAfter`);
    const button = editor.querySelector('.code-fold-toggle');
    check(button, 'Code fold control is missing');
    button.click();
    const paragraph = editor.querySelector('p');
    caret(paragraph.firstChild, 5);
    document.execCommand('insertText', false, '!');
    check(window.Mory.getMarkdown() === `\`\`\`js\n${longCode}\n\`\`\`\n\nAfter!`, 'Editing lost folded code or serialized controls');
    check(button.getAttribute('aria-expanded') === 'false', 'An unrelated edit expanded the code');
    window.Mory.toggleSource(true);
    check(document.querySelector('#source-editor').value.includes(longCode), 'Source mode lost folded code');
    window.Mory.toggleSource(false);
    check(editor.querySelectorAll('.code-fold-toggle').length === 1, 'Source round-trip duplicated or lost controls');
  });
  await run('folded code exports in full without editor controls', async () => {
    window.Mory.loadMarkdown(`\`\`\`txt\n${longCode}\n\`\`\``);
    const button = editor.querySelector('.code-fold-toggle');
    check(button, 'Code fold control is missing');
    button.click();
    const html = await window.Mory.exportHTML();
    const exported = new DOMParser().parseFromString(html, 'text/html');
    check(exported.querySelector('pre code')?.textContent === longCode, 'Export lost folded code');
    check(!exported.querySelector('.code-tools, .is-code-folded'), 'Export includes fold controls or collapsed state');
  });
  await run('code fold labels follow locale and expanded state', () => {
    window.Mory.loadMarkdown(`\`\`\`txt\n${longCode}\n\`\`\``);
    const button = editor.querySelector('.code-fold-toggle');
    check(button, 'Code fold control is missing');
    const language = document.querySelector('#language-select');
    const original = language.value;
    const changeLocale = value => { language.value = value; language.dispatchEvent(new Event('change', { bubbles: true })); };
    try {
      changeLocale('en');
      check(button.getAttribute('aria-label') === 'Collapse code block', 'Expanded label is not English');
      button.click();
      changeLocale('zh-CN');
      check(button.getAttribute('aria-label') === '\u5C55\u5F00\u4EE3\u7801\u5757', 'Collapsed label is not Chinese');
      changeLocale('en');
      check(button.textContent === 'Expand code block', 'Locale switch reset the collapsed label');
      button.click();
      check(button.textContent === 'Collapse code block', 'Expanding did not refresh the label');
    } finally { changeLocale(original); }
  });
  await run('code fold control appears only for code longer than six lines', async () => {
    const lines = count => Array.from({ length: count }, (_, index) => `line ${index + 1}`).join('\n');
    window.Mory.loadMarkdown(`\`\`\`txt\n${lines(6)}\n\`\`\`\n\n\`\`\`txt\n${lines(7)}\n\`\`\``);
    const [short, long] = editor.querySelectorAll('.code-fold-toggle');
    check(short.hidden && getComputedStyle(short).display === 'none', 'Six-line code shows a fold control');
    check(!long.hidden && getComputedStyle(long).display !== 'none', 'Seven-line code has no visible fold control');
    const code = editor.querySelector('pre code');
    caret(code.lastChild, code.lastChild.length);
    paragraphBreak();
    document.execCommand('insertText', false, 'line 7');
    check(!short.hidden, 'Typing a seventh line did not reveal the fold control');
  });
  await run('code language button edits language and refreshes highlighting', async () => {
    window.Mory.loadMarkdown('```\nfn main() { let value = 1; }\n```\n\nAfter');
    const block = editor.querySelector('pre');
    const button = editor.querySelector('.code-language-button');
    check(button && button.classList.contains('is-empty'), 'Code without a language has no language control');
    button.click();
    const meta = editor.querySelector('.code-meta');
    const input = meta.querySelector('.code-language');
    check(!meta.hidden && document.activeElement === input, 'Language control did not open the language field');
    input.value = 'rust';
    input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    check(block.querySelector('code').classList.contains('language-rust'), 'Changing language did not refresh highlighting');
    check(button.textContent === 'rust' && !button.classList.contains('is-empty'), 'Language control did not show the new language');
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    check(meta.hidden, 'Enter did not close the language field');
    check(window.Mory.getMarkdown() === '```rust\nfn main() { let value = 1; }\n```\n\nAfter', 'Language change was not serialized');
    button.click();
    meta.querySelector('.code-language').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    check(meta.hidden, 'Escape did not close the language field');
  });
  await run('code metadata labels follow the interface language', () => {
    window.Mory.loadMarkdown('```txt\nCode\n```');
    const language = document.querySelector('#language-select');
    const original = language.value;
    const changeLocale = value => { language.value = value; language.dispatchEvent(new Event('change', { bubbles: true })); };
    try {
      changeLocale('en');
      editor.querySelector('.code-language-button').click();
      const input = editor.querySelector('.code-meta .code-language');
      check(input.placeholder === 'Language' && editor.querySelector('.code-meta .code-title').placeholder === 'Name (optional)', 'Metadata fields are not English');
      check(editor.querySelector('.code-copy-button').getAttribute('aria-label') === 'Copy code', 'Copy label is not English');
      changeLocale('zh-CN');
      check(input.placeholder === '\u8BED\u8A00', 'Metadata fields are not Chinese');
    } finally { changeLocale(original); }
  });
  await run('newly typed fences receive fold controls and deleted blocks leave no controls', () => {
    window.Mory.loadMarkdown('');
    caret(editor.firstChild, 0);
    document.execCommand('insertText', false, '```js');
    editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    check(editor.querySelector('pre') && editor.querySelectorAll('.code-fold-toggle').length === 1, 'A new fence has no fold control');
    editor.querySelector('pre').remove();
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'deleteContentBackward' }));
    check(!editor.querySelector('.code-tools'), 'Deleted code left an orphan control');
    check(!window.Mory.getMarkdown().includes('code block'), 'Deleted controls became document text');
  });
  await run('highlighted code keeps its mouse selection anchor when entering the block', () => {
    window.Mory.loadMarkdown('```js\nconst first = 1;\nconst second = 2;\nconst third = 3;\n```\n\nAfter');
    const code = editor.querySelector('pre code');
    const keyword = code.querySelector('.hljs-keyword').firstChild;
    caret(keyword, 2);
    document.dispatchEvent(new Event('selectionchange'));
    check(keyword.isConnected && getSelection().anchorNode === keyword && getSelection().anchorOffset === 2,
      'Entering highlighted code replaced the mouse anchor');
  });
  await run('a forward or backward code selection is not rehighlighted while dragging', () => {
    for (const backward of [false, true]) {
      window.Mory.loadMarkdown('```js\nconst first = 1;\nconst second = 2;\nconst third = 3;\n```');
      const code = editor.querySelector('pre code');
      // Editing leaves plain text; dragging must not rebuild it into highlight spans.
      code.textContent = 'const first = 1;\nconst second = 2;\nconst third = 3;';
      delete code.dataset.moryHighlighted;
      delete code.parentElement.dataset.highlighted;
      const text = code.firstChild;
      editor.focus();
      getSelection().setBaseAndExtent(text, backward ? 43 : 22, text, backward ? 22 : 43);
      const selected = getSelection().toString();
      document.dispatchEvent(new Event('selectionchange'));
      check(text.isConnected && getSelection().toString() === selected, 'Dragging changed the selected code');
      check(getSelection().anchorOffset === (backward ? 43 : 22), 'Dragging moved the selection origin');
    }
  });
  await run('editing a highlighted code selection replaces only the selected text', () => {
    window.Mory.loadMarkdown('```js\nconst first = 1;\nconst second = 2;\n```');
    const code = editor.querySelector('pre code');
    const keyword = code.querySelectorAll('.hljs-keyword')[1].firstChild;
    editor.focus();
    getSelection().setBaseAndExtent(keyword, 5, keyword, 0);
    editor.dispatchEvent(new InputEvent('beforeinput', { bubbles: true, cancelable: true, inputType: 'insertText', data: 'let' }));
    document.execCommand('insertText', false, 'let');
    check(code.textContent === 'const first = 1;\nlet second = 2;', 'Code replacement changed unselected text');
  });
  await run('typing spaces inside a backtick pair keeps the caret in inline code', () => {
    window.Mory.loadMarkdown('');
    editor.innerHTML = '<p>``</p>';
    caret(editor.firstChild.firstChild, 1);
    document.execCommand('insertText', false, 'hello');
    check(editor.querySelector('code')?.contains(getSelection().anchorNode), 'Inline rendering moved the caret past the closing delimiter');
    document.execCommand('insertText', false, ' ');
    document.execCommand('insertText', false, 'world');
    check(editor.querySelector('code')?.textContent === 'hello world', 'Space exited inline code');
    check(window.Mory.getMarkdown() === '`hello world`', 'Inline code did not round-trip');
  });
  await run('a space as the first inline code character keeps the caret inside', () => {
    window.Mory.loadMarkdown('');
    editor.innerHTML = '<p>``</p>';
    caret(editor.firstChild.firstChild, 1);
    document.execCommand('insertText', false, ' ');
    check(editor.querySelector('code')?.contains(getSelection().anchorNode), 'Space moved the caret outside inline code');
    document.execCommand('insertText', false, 'value');
    check(editor.querySelector('code')?.textContent === ' value', 'Leading code space was lost');
  });
  await run('typing a closing backtick leaves following text outside inline code', () => {
    window.Mory.loadMarkdown('');
    caret(editor.firstChild, 0);
    document.execCommand('insertText', false, '`value');
    document.execCommand('insertText', false, '`');
    check(editor.querySelector('code')?.textContent === 'value', 'Closing delimiter did not render code');
    check(!editor.querySelector('code').contains(getSelection().anchorNode), 'Explicit closing delimiter trapped the caret');
    document.execCommand('insertText', false, ' after');
    check(window.Mory.getMarkdown() === '`value` after', 'Following text remained in code');
  });
  await run('inline code punctuation stays literal while editing', () => {
    window.Mory.loadMarkdown('Before `value` after');
    const code = editor.querySelector('code');
    caret(code.firstChild, 5);
    document.execCommand('insertText', false, ' **bold** <span>');
    check(editor.querySelector('code') === code && !code.querySelector('strong, span'), 'Inline code became Markdown formatting');
    check(window.Mory.getMarkdown() === 'Before `value **bold** <span>` after', 'Editing lost the inline code delimiters');
  });
  await run('rendering adjacent Markdown preserves existing inline code and formatting', () => {
    window.Mory.loadMarkdown('`a` and **bold** then');
    caret(editor.lastChild.lastChild, editor.lastChild.lastChild.textContent.length);
    document.execCommand('insertText', false, ' *italic*');
    check(editor.querySelector('code')?.textContent === 'a', 'Adjacent rendering removed inline code');
    check(editor.querySelector('strong')?.textContent === 'bold', 'Adjacent rendering removed bold formatting');
    check(editor.querySelector('em')?.textContent === 'italic', 'Adjacent Markdown was not rendered');
  });
  for (const level of [1, 2, 3, 4, 5, 6]) {
    await run(`heading level ${level} after select-all deletion`, async () => {
      reset();
      document.execCommand('insertText', false, '#'.repeat(level));
      document.execCommand('insertText', false, ' ');
      document.execCommand('insertText', false, 'hi');
      paragraphBreak();
      await settle();
      check(editor.querySelector(`h${level}`)?.textContent === 'hi', 'Heading was not rendered');
      check(window.Mory.getMarkdown() === `${'#'.repeat(level)} hi`, 'Heading Markdown changed');
      const anchor = getSelection().anchorNode;
      check(!editor.querySelector(`h${level}`).contains(anchor), 'Caret stayed in the heading');
      document.execCommand('insertText', false, 'Body');
      check(editor.querySelector(`h${level}`)?.textContent === 'hi', 'Body inherited the heading');
      check(window.Mory.getMarkdown() === `${'#'.repeat(level)} hi\n\nBody`, 'Body Markdown changed');
    });
  }
  await run('root text after replacing the entire document', async () => {
    reset();
    document.execCommand('insertText', false, '# hi');
    await settle();
    check(editor.querySelector('h1')?.textContent === 'hi', 'Replacement heading stayed raw');
  });
  for (const initialHTML of ['', '<br>', '<span></span>']) {
    await run(`typing into an unwrapped editor with initial HTML ${JSON.stringify(initialHTML)}`, async () => {
      reset();
      editor.innerHTML = initialHTML;
      caret(editor, 0);
      for (const text of ['#', ' ', 'h', 'i']) document.execCommand('insertText', false, text);
      paragraphBreak();
      await settle();
      check(editor.querySelector('h1')?.textContent === 'hi', 'Root heading stayed raw');
      check(window.Mory.getMarkdown() === '# hi', 'Root heading Markdown changed');
      document.execCommand('insertText', false, 'Body');
      check(window.Mory.getMarkdown() === '# hi\n\nBody', 'Root heading did not exit to body text');
    });
  }
  await run('root composition waits for commit and renders an editor-targeted commit', async () => {
    reset();
    editor.textContent = '# ';
    caret(editor.firstChild, 2);
    editor.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
    const text = getSelection().anchorNode;
    text.textContent = '# \u4F60\u597D';
    caret(text, text.textContent.length);
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', isComposing: true }));
    await settle();
    check(!editor.querySelector('h1'), 'Composition was rendered before commit');
    editor.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '\u4F60\u597D' }));
    await settle();
    check(editor.querySelector('h1')?.textContent === '\u4F60\u597D', 'Committed heading stayed raw');
  });
  await run('root heading normalization preserves the next paragraph and caret', async () => {
    reset();
    editor.innerHTML = '# hi<div>Body</div>';
    caret(editor.lastChild.firstChild, 2);
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: 'o' }));
    await settle();
    check(editor.querySelector('h1')?.textContent === 'hi', 'Inactive root heading stayed raw');
    check(getSelection().anchorNode.textContent === 'Body' && getSelection().anchorOffset === 2, 'Normalization moved the caret');
    check(window.Mory.getMarkdown() === '# hi\n\nBody', 'Normalization lost content');
  });
  await run('root line breaks and inline formatting survive normalization', async () => {
    reset();
    editor.innerHTML = '# hi<br><strong>Body</strong><br>Tail';
    caret(editor.lastChild, 4);
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    await settle();
    check(editor.querySelector('h1')?.textContent === 'hi', 'Root heading with a break stayed raw');
    check(editor.querySelector('strong')?.textContent === 'Body', 'Inline formatting was lost');
    check(window.Mory.getMarkdown() === '# hi\n\n**Body**\n\nTail', 'Line boundaries were lost');
    check(getSelection().anchorNode.textContent === 'Tail' && getSelection().anchorOffset === 4, 'Caret moved out of the last line');
  });
  await run('root normalization preserves a backward selection across inline nodes', async () => {
    reset();
    editor.innerHTML = 'One <strong>two</strong> three';
    editor.focus();
    getSelection().setBaseAndExtent(editor, 3, editor, 1);
    const selected = getSelection().toString();
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'deleteContentBackward' }));
    check(getSelection().toString() === selected, 'Selected text changed');
    check(getSelection().anchorOffset > getSelection().focusOffset, 'Selection direction changed');
    check(window.Mory.getMarkdown() === 'One **two** three', 'Selection normalization changed content');
  });
  await run('editor-targeted composition beginning in an empty paragraph renders on commit', async () => {
    reset();
    window.Mory.loadMarkdown('');
    const paragraph = editor.firstChild;
    caret(paragraph, 0);
    editor.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
    paragraph.textContent = '# \u4F60\u597D';
    caret(paragraph.firstChild, 4);
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', isComposing: true }));
    editor.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '\u4F60\u597D' }));
    await settle();
    check(editor.querySelector('h1')?.textContent === '\u4F60\u597D', 'Editor-targeted commit did not render');
  });
  await run('switching documents during composition does not block subsequent heading input', async () => {
    reset();
    editor.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
    window.Mory.loadMarkdown('');
    caret(editor.firstChild, 0);
    document.execCommand('insertText', false, '# hi');
    await settle();
    check(editor.querySelector('h1')?.textContent === 'hi', 'Old composition blocked the new document');
  });
  await run('native paragraph input splits the middle of a heading without losing text', () => {
    window.Mory.loadMarkdown('# Hello world');
    caret(editor.firstChild.firstChild, 5);
    paragraphBreak();
    check(editor.querySelector('h1')?.textContent === 'Hello', 'Heading prefix changed');
    check(editor.querySelector('h1 + p')?.textContent === ' world', 'Heading suffix changed');
    check(editor.querySelector('p').contains(getSelection().anchorNode), 'Caret did not follow the split');
  });
  await run('code and escaped heading text remain literal', async () => {
    window.Mory.loadMarkdown('```text\n# hi\n```\n\n\\# literal');
    const code = editor.querySelector('code');
    caret(code.firstChild, code.firstChild.textContent.length);
    document.execCommand('insertText', false, '!');
    await settle();
    check(!editor.querySelector('h1'), 'Code or escaped text became a heading');
    check(editor.querySelector('code')?.textContent === '# hi!', 'Code text changed');
    check(window.Mory.getMarkdown().endsWith('\\# literal'), 'Literal heading lost its escape');
    const paragraph = editor.querySelector('p');
    caret(paragraph.firstChild, paragraph.firstChild.textContent.length);
    document.execCommand('insertText', false, ' **bold**');
    await settle();
    check(!editor.querySelector('h1'), 'Formatting changed literal heading intent');
    check(editor.querySelector('p strong')?.textContent === 'bold', 'Literal paragraph formatting did not render');
    check(window.Mory.getMarkdown().endsWith('\\# literal **bold**'), 'Formatted literal heading did not round-trip');
  });
  await run('clearing literal heading text allows a new heading', () => {
    window.Mory.loadMarkdown('\\# literal');
    editor.focus();
    document.execCommand('selectAll');
    document.execCommand('delete');
    document.execCommand('insertText', false, '# New');
    check(editor.querySelector('h1')?.textContent === 'New', 'Cleared literal metadata blocked a new heading');
  });
  await run('repeated empty-root edits preserve heading undo and redo', async () => {
    for (let cycle = 0; cycle < 12; cycle += 1) {
      window.Mory.loadMarkdown('');
      editor.replaceChildren();
      caret(editor, 0);
      const title = `Heading ${cycle}`;
      editor.dispatchEvent(new InputEvent('beforeinput', { bubbles: true, cancelable: true, inputType: 'insertText', data: `# ${title}` }));
      document.execCommand('insertText', false, `# ${title}`);
      check(editor.querySelector('h1')?.textContent === title, 'Repeated input left a raw heading');
      window.Mory.undo();
      await settle();
      check(window.Mory.getMarkdown() === '', 'Undo did not restore the empty document');
      window.Mory.redo();
      await settle();
      check(editor.querySelector('h1')?.textContent === title, 'Redo lost the heading');
    }
  });
  return { passed, failures };
}
