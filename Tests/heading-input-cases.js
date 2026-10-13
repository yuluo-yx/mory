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
    window.Mory.loadMarkdown('```txt\nCode\n```');
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
