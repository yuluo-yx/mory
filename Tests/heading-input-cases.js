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
