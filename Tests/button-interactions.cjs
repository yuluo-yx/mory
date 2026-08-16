const { app, BrowserWindow } = require("electron");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");

app.commandLine.appendSwitch("disable-gpu");
app.disableHardwareAcceleration();

const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));

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
    if (!element) throw new Error(${JSON.stringify(`找不到元素：${selector}`)});
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
      if (!expected) throw new Error(${JSON.stringify(`找不到元素：${selector}`)});
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
  if (!hittable) throw new Error(`点击目标被遮挡：${selector}；最终命中 ${target.hit}`);
  window.webContents.sendInputEvent({ type: "mouseDown", x: target.x, y: target.y, button: "left", clickCount: 1 });
  window.webContents.sendInputEvent({ type: "mouseUp", x: target.x, y: target.y, button: "left", clickCount: 1 });
  await wait(selector === "#sidebar-toggle" ? 280 : 80);
  return target;
}

async function hover(window, selector) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const point = await inspect(window, `(() => {
      const element = document.querySelector(${JSON.stringify(selector)});
      if (!element) throw new Error(${JSON.stringify(`找不到元素：${selector}`)});
      const rect = element.getBoundingClientRect();
      return { x: Math.round(rect.left + rect.width / 2), y: Math.round(rect.top + rect.height / 2) };
    })()`);
    // 托管 runner 缩放离屏窗口后可能保留旧指针坐标；每次重新取坐标并先移出目标。
    window.webContents.sendInputEvent({ type: "mouseMove", x: 1, y: 1 });
    await wait(50);
    window.webContents.sendInputEvent({ type: "mouseMove", x: point.x, y: point.y });
    await wait(100);
    if (await inspect(window, "document.querySelector('#toolbar-tooltip').classList.contains('is-visible')")) return;
  }
  // macOS Intel 的离屏渲染器偶尔不合成物理指针事件；标准 DOM 事件仍覆盖产品实际的事件委托路径。
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
  if (!result) throw new Error(`${label}：状态未变化`);
  process.stdout.write(`[button] ${label}\n`);
}

async function expectEventually(window, label, expression, timeout = 2000) {
  const deadline = Date.now() + timeout;
  do {
    if (await inspect(window, expression)) {
      process.stdout.write(`[button] ${label}\n`);
      return;
    }
    await wait(50);
  } while (Date.now() < deadline);
  throw new Error(`${label}：${timeout} ms 内状态未变化`);
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
  window.webContents.on("unresponsive", () => errors.push("渲染进程无响应"));

  try {
    await loadAndWait(window);
    await expect(window, "脚本已初始化", "typeof window.Mory === 'object' && document.querySelector('#write').textContent.includes('Mory')");
    await expect(window, "标题按钮已移除", "!document.querySelector('#toolbar [data-command=\"h1\"]') && !document.querySelector('#toolbar [data-command=\"h2\"]') && !document.querySelector('#toolbar [data-command=\"paragraph\"]')");
    await expect(window, "浮动栏纵向固定在右下角", "(() => { const element = document.querySelector('#toolbar'); const bar = element.getBoundingClientRect(); return getComputedStyle(element).flexDirection === 'column' && bar.height > bar.width * 3 && innerWidth - bar.right < 24 && innerHeight - bar.bottom < 52 && document.querySelector('#toolbar #source-toggle') && document.querySelector('#toolbar #export-button'); })()");
    await expect(window, "工具栏默认仅显示图标", "[...document.querySelectorAll('#toolbar button')].every(button => button.dataset.tooltip && button.getAttribute('aria-label') && !/列表|源码|导出/.test(button.textContent))");
    await inspect(window, "(() => { const read = selector => parseFloat(getComputedStyle(document.querySelector(selector)).fontSize); window.__normalTypography = { write: read('#write'), file: read('.file-item'), tab: read('.tab'), status: read('#statusbar') }; })()");
    window.setSize(2048, 1056);
    await wait(140);
    await expect(window, "全屏宽视口自动放大正文和关键界面文字", "(() => { const read = selector => parseFloat(getComputedStyle(document.querySelector(selector)).fontSize); return innerWidth >= 2000 && read('#write') >= window.__normalTypography.write + 2 && read('.file-item') >= window.__normalTypography.file + 1 && read('.tab') >= window.__normalTypography.tab + 1 && read('#statusbar') >= window.__normalTypography.status + 2; })()");
    window.setSize(1180, 790);
    await wait(140);
    await hover(window, "#toolbar button[data-command='ul']");
    await expectEventually(window, "工具栏悬停显示文字提示", "document.querySelector('#toolbar-tooltip').classList.contains('is-visible') && document.querySelector('#toolbar-tooltip').textContent === '无序列表'");
    await expect(window, "左侧搜索标签已移除", "!document.querySelector('.tab[data-panel=\"search\"]') && !document.querySelector('#search-panel-side') && !document.querySelector('#side-search-input')");
    const screenshotPath = path.join(os.tmpdir(), "mory-ui-e2e.png");
    const codeMetaScreenshotPath = path.join(os.tmpdir(), "mory-code-meta-e2e.png");
    await fs.writeFile(screenshotPath, (await window.webContents.capturePage()).toPNG());

    await click(window, ".tab[data-panel='outline']");
    await expect(window, "大纲标签可点击", "document.querySelector('#outline-panel').classList.contains('is-active')");
    await click(window, ".tab[data-panel='files']");
    await inspect(window, `(() => {
      window.moryNative = {
        send(payload) {
          if (payload.type === 'openFile') window.__autoOpenedWorkspacePath = payload.path;
        },
        request(method, args) {
          window.__lastHostRequest = { method, args };
          if (method === 'deleteDocument') return Promise.resolve(window.__deleteDocumentResult || { deleted: true });
          return Promise.reject(new Error('测试宿主未实现该请求'));
        }
      };
      return true;
    })()`);

    const pastedMarkdown = '# 粘贴标题\n\n**粘贴加粗**\n\n```go\nfmt.Println("paste")\n```';
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
    await window.webContents.insertText("草稿一");
    await click(window, "#new-file-button");
    const secondDraftId = await inspect(window, "document.querySelector('.file-item.is-active').dataset.documentId");
    await window.webContents.insertText("草稿二");
    await expect(window, "多个未命名文档逐个列出", "document.querySelectorAll('#file-list .file-item[data-document-id]').length === 2 && [...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '未命名.md|未命名 2.md'");
    await expect(window, "未命名文档使用独立标识", `${JSON.stringify(firstDraftId)} !== ${JSON.stringify(secondDraftId)} && document.querySelectorAll('#file-list .file-dirty:not([hidden])').length === 2`);
    await click(window, `[data-document-id=${JSON.stringify(firstDraftId)}]`);
    await expect(window, "切回第一个草稿保留内容", "document.querySelector('#write').textContent === '草稿一' && document.querySelector('.file-item.is-active .file-name').textContent === '未命名.md'");
    await click(window, `[data-document-id=${JSON.stringify(secondDraftId)}]`);
    await expect(window, "切回第二个草稿保留内容", "document.querySelector('#write').textContent === '草稿二' && document.querySelector('.file-item.is-active .file-name').textContent === '未命名 2.md'");
    await inspect(window, "window.Mory.didSave({ path: '/tmp/草稿二.md', name: '草稿二.md' })");
    await expect(window, "保存后只重命名当前文档", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '未命名.md|草稿二.md' && document.querySelector('.file-item.is-active').dataset.path === '/tmp/草稿二.md'");
    await expect(window, "草稿与已保存文档提供对应移除和删除动作", `document.querySelector('.file-close[data-document-id=${JSON.stringify(firstDraftId)}]').getAttribute('aria-label') === '移除草稿' && document.querySelector('.file-close[data-document-id=${JSON.stringify(secondDraftId)}]').getAttribute('aria-label') === '删除文档'`);
    await click(window, `.file-close[data-document-id=${JSON.stringify(firstDraftId)}]`);
    await expect(window, "可移除非活动未命名草稿", "document.querySelectorAll('#file-list .file-item[data-document-id]').length === 1 && document.querySelector('#write').textContent === '草稿二' && document.querySelector('.file-item.is-active .file-name').textContent === '草稿二.md'");
    await click(window, "#new-file-button");
    const thirdDraftId = await inspect(window, "document.querySelector('.file-item.is-active').dataset.documentId");
    await window.webContents.insertText("待移除");
    await click(window, `.file-close[data-document-id=${JSON.stringify(thirdDraftId)}]`);
    await expect(window, "关闭活动草稿后回退相邻文档", "document.querySelectorAll('#file-list .file-item[data-document-id]').length === 1 && document.querySelector('#write').textContent === '草稿二' && document.querySelector('.file-item.is-active .file-name').textContent === '草稿二.md'");
    await inspect(window, "window.__deleteDocumentResult = { canceled: true }");
    await click(window, `.file-close[data-document-id=${JSON.stringify(secondDraftId)}]`);
    await expect(window, "取消删除时保留磁盘文稿", "document.querySelector('.file-item.is-active .file-name').textContent === '草稿二.md'");
    await inspect(window, "window.__deleteDocumentResult = { deleted: true }");
    await click(window, `.file-close[data-document-id=${JSON.stringify(secondDraftId)}]`);
    await expectEventually(window, "确认删除后移除文稿并创建空白文档", "window.__lastHostRequest.method === 'deleteDocument' && window.__lastHostRequest.args.path === '/tmp/草稿二.md' && document.querySelectorAll('#file-list .file-item[data-document-id]').length === 1 && document.querySelector('.file-item.is-active .file-name').textContent === '未命名.md' && document.querySelector('#write').textContent === '' && document.querySelector('#toast').textContent === '文档已移到废纸篓'");

    await inspect(window, `window.Mory.setWorkspaceSnapshot({
      state: { activeId: 'workspace-empty', workspaces: [{ id: 'workspace-empty', name: '空目录', provider: 'local', localPath: '/empty' }] },
      files: []
    })`);
    await expect(window, "空工作区保留未命名占位文稿", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '未命名.md' && document.querySelector('.file-item.is-active')");
    await inspect(window, `window.Mory.setWorkspaceSnapshot({
      state: { activeId: 'workspace-opened', workspaces: [{ id: 'workspace-opened', name: '已打开目录', provider: 'local', localPath: '/opened' }] },
      files: [{ name: '子目录/第二篇.md', path: '/opened/子目录/第二篇.md', createdAt: 20 }, { name: '第一篇.md', path: '/opened/第一篇.md', createdAt: 10 }]
    })`);
    await expect(window, "非空工作区移除占位文稿并请求打开排序首篇", "document.querySelector('#folder-name').textContent === '已打开目录' && [...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '第一篇.md|子目录/第二篇.md' && window.__autoOpenedWorkspacePath === '/opened/第一篇.md'");
    await inspect(window, `window.Mory.openDocument({ name: '第一篇.md', path: '/opened/第一篇.md', markdown: '# 第一篇' })`);
    await expect(window, "工作区排序首篇自动成为当前文稿", "document.querySelector('.file-item.is-active .file-name').textContent === '第一篇.md' && document.querySelector('#write h1').textContent === '第一篇'");
    await expect(window, "工作区文稿按创建时间升序排列", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '第一篇.md|子目录/第二篇.md'");
    await inspect(window, `window.Mory.openDocument({ name: '子目录/第二篇.md', path: '/opened/子目录/第二篇.md', markdown: '# 第二篇' })`);
    await expect(window, "打开文稿不会改变文件列表位置", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '第一篇.md|子目录/第二篇.md'");
    await inspect(window, `window.Mory.setWorkspaceSnapshot({
      state: { activeId: 'workspace-opened', workspaces: [{ id: 'workspace-opened', name: '已打开目录', provider: 'local', localPath: '/opened' }] },
      files: [{ name: '第一篇.md', path: '/opened/第一篇.md', createdAt: 10 }]
    })`);
    await expect(window, "磁盘删除的当前文稿立即从列表移除", "[...document.querySelectorAll('#file-list .file-name')].map(item => item.textContent).join('|') === '第一篇.md' && window.__autoOpenedWorkspacePath === '/opened/第一篇.md'");
    await inspect(window, `window.Mory.openDocument({ name: '第一篇.md', path: '/opened/第一篇.md', markdown: '# 第一篇' })`);
    await expect(window, "删除当前文稿后自动切换到排序首篇", "document.querySelector('.file-item.is-active .file-name').textContent === '第一篇.md' && document.querySelector('#write h1').textContent === '第一篇'");
    await inspect(window, `(() => {
      const ids = [...document.querySelectorAll('#file-list .file-item[data-path^="/opened/"][data-document-id]')].map(item => item.dataset.documentId);
      ids.forEach(id => window.Mory.closeDocument(id));
    })()`);

    await inspect(window, "document.documentElement.dataset.host = 'mac-native'");
    await expect(window, "文件标签可点击", "document.querySelector('#files-panel').classList.contains('is-active')");
    await click(window, "#sidebar-toggle");
    await expect(window, "侧栏按钮可点击", "document.querySelector('#sidebar').classList.contains('is-hidden')");
    await expect(window, "macOS 窗口控制区无重叠", "document.querySelector('#sidebar-toggle').getBoundingClientRect().left >= 74");
    await click(window, "#sidebar-toggle");
    await expect(window, "侧栏可恢复", "!document.querySelector('#sidebar').classList.contains('is-hidden')");

    await click(window, "#source-toggle");
    await expect(window, "源码按钮可点击", "document.querySelector('.workspace').classList.contains('source-mode')");
    await click(window, "#source-toggle");
    await expect(window, "预览模式可恢复", "!document.querySelector('.workspace').classList.contains('source-mode')");

    await inspect(window, `window.Mory.setWorkspaceDocuments([
      { name: '入口.md', path: '/virtual/入口.md', markdown: '# 入口\\n[[专题/设计]]' },
      { name: '专题/设计.md', path: '/virtual/专题/设计.md', markdown: '# 设计\\n[返回](../入口.md)' },
      { name: '引用者.md', path: '/virtual/引用者.md', markdown: '# 引用者\\n[[专题/设计]]' },
      { name: '孤立.md', path: '/virtual/孤立.md', markdown: '# 孤立' }
    ])`);
    await click(window, "#graph-button");
    await expect(window, "右下角知识图谱入口可打开", "document.querySelector('#knowledge-graph').classList.contains('is-open')");
    await expectEventually(window, "知识图谱渲染工作区节点和链接", "document.querySelectorAll('#graph-svg .graph-node').length === 4 && document.querySelectorAll('#graph-svg .graph-link').length === 3");
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
    await expectEventually(window, "知识图谱滚轮以画布为中心缩放且不滚动正文", "document.querySelector('#graph-stage').getAttribute('transform')?.includes('scale(') && document.querySelector('#graph-zoom').value !== '100%' && document.querySelector('#editor-scroll').scrollTop === Number(document.querySelector('#editor-scroll').dataset.scrollBeforeGraphZoom)");
    await click(window, "#graph-svg .graph-node[data-node-id='专题/设计.md'] circle");
    await expectEventually(window, "单击节点展示正向链接与反向链接", "!document.querySelector('#graph-relations').hidden && document.querySelectorAll('#graph-forward-list button').length === 1 && document.querySelectorAll('#graph-backlink-list button').length === 2");
    await expectEventually(window, "图谱区分正向、反向和互相引用", "document.querySelectorAll('#graph-svg .graph-link.is-outgoing').length === 1 && document.querySelectorAll('#graph-svg .graph-link.is-incoming').length === 2 && document.querySelectorAll('#graph-svg .graph-node.is-mutual').length === 1 && document.querySelectorAll('#graph-svg .graph-node.is-backlink').length === 2");
    await click(window, "#graph-relations-close");
    await inspect(window, "(() => { const input = document.querySelector('#graph-search'); input.value = '孤立'; input.dispatchEvent(new Event('input', { bubbles: true })); })()");
    await expectEventually(window, "知识图谱支持文稿筛选", "document.querySelectorAll('#graph-svg .graph-node.is-match').length === 1 && document.querySelectorAll('#graph-svg .graph-node.is-dimmed').length === 3");
    await click(window, "#graph-close");
    await expect(window, "知识图谱可关闭", "!document.querySelector('#knowledge-graph').classList.contains('is-open')");

    await inspect(window, `window.Mory.openDocument({ name: '专题/设计.md', path: '/virtual/专题/设计.md', markdown: '# 设计\\n[返回](../入口.md)' })`);
    await expect(window, "状态栏显示当前文稿反向链接数量", "document.querySelector('#backlink-count').textContent === '反向链接 2'");
    await expect(window, "文稿底部列出反向链接来源", "!document.querySelector('#document-backlinks').hidden && document.querySelectorAll('#document-backlinks-list button').length === 2 && [...document.querySelectorAll('#document-backlinks-list strong')].map(item => item.textContent).join('|') === '入口|引用者'");
    await click(window, "#backlink-count");
    await expect(window, "状态栏反链入口可定位文章反链区", "!document.querySelector('#document-backlinks').hidden");

    await click(window, "#export-button");
    await expect(window, "导出按钮可点击", "document.querySelector('#export-dialog').classList.contains('is-open')");
    await click(window, "#export-close");
    await expect(window, "导出窗口可关闭", "!document.querySelector('#export-dialog').classList.contains('is-open')");

    await click(window, "#settings-button");
    await expect(window, "设置按钮可点击", "document.querySelector('#preferences').classList.contains('is-open')");
    await inspect(window, "(() => { const appearance = document.querySelector('#theme-select'); appearance.value = 'light'; appearance.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expectEventually(window, "浅色侧栏使用高对比语义颜色", "(() => { const style = getComputedStyle(document.documentElement); return document.documentElement.dataset.appearance === 'light' && style.getPropertyValue('--sidebar-text').trim() === '#272a27' && style.getPropertyValue('--sidebar-muted').trim() === '#626762' && style.getPropertyValue('--sidebar-faint').trim() === '#7b817b'; })()");
    await inspect(window, "(() => { const documentTheme = document.querySelector('#document-theme-select'); documentTheme.value = 'yuluo-css'; documentTheme.dispatchEvent(new Event('change', { bubbles: true })); const appearance = document.querySelector('#theme-select'); appearance.value = 'dark'; appearance.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expectEventually(window, "深色外观与默认文稿画布保持一致", "document.documentElement.dataset.appearance === 'dark' && getComputedStyle(document.querySelector('#editor-scroll')).backgroundColor === 'rgb(31, 34, 36)' && getComputedStyle(document.querySelector('#write')).color === 'rgb(221, 226, 229)'");
    await expectEventually(window, "深色侧栏使用高对比语义颜色", "(() => { const style = getComputedStyle(document.documentElement); return style.getPropertyValue('--sidebar-text').trim() === '#f0f1ee' && style.getPropertyValue('--sidebar-muted').trim() === '#bdc1ba' && style.getPropertyValue('--sidebar-faint').trim() === '#9da39b'; })()");
    await inspect(window, "(() => { const appearance = document.querySelector('#theme-select'); appearance.value = 'light'; appearance.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expectEventually(window, "切回浅色外观恢复默认文稿纸张", "document.documentElement.dataset.appearance === 'light' && getComputedStyle(document.querySelector('#editor-scroll')).backgroundColor === 'rgb(255, 255, 255)'");
    await inspect(window, "window.Mory.setCustomThemes([{ id: 'user-paper-test', name: '纸张', css: '#write{letter-spacing:1px}' }])");
    await inspect(window, "(() => { const select = document.querySelector('#document-theme-select'); select.value = 'user-paper-test'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "用户 CSS 主题即时应用", "document.documentElement.dataset.docTheme === 'user-paper-test' && document.querySelector('#user-document-theme').textContent.includes('letter-spacing')");
    await expect(window, "用户主题进入导出选择", "document.querySelector('#export-theme option[value=\"user-paper-test\"]')");
    await expect(window, "用户主题写入导出 HTML", "window.Mory.exportDocument({ theme: 'current' }).then(html => html.includes('#write{letter-spacing:1px}'))");
    await inspect(window, "(() => { const select = document.querySelector('#language-select'); select.value = 'en'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "设置可即时切换英文", "document.documentElement.lang === 'en' && document.querySelector('#preferences h2').textContent === 'Preferences' && document.querySelector('#graph-button').getAttribute('aria-label') === 'Knowledge graph' && document.querySelector('#backlink-count').textContent === 'Backlinks 2'");
    await inspect(window, `window.Mory.openDocument({ name: 'English.md', path: '/virtual/English.md', markdown: '# English' })`);
    await expect(window, "英文模式切换文档时动态状态保持英文", "document.querySelector('#save-state').textContent === 'Saved' && document.querySelector('#toast').textContent === 'Document switched' && document.querySelector('.file-item.is-active + .file-close').getAttribute('aria-label') === 'Delete document'");
    await inspect(window, "(() => { const select = document.querySelector('#language-select'); select.value = 'zh-CN'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "设置可切回中文", "document.documentElement.lang === 'zh-CN' && document.querySelector('#preferences h2').textContent === '偏好设置'");
    await click(window, ".setting-row:has(#status-toggle) .switch span");
    await expect(window, "关闭状态栏设置即时生效", "document.querySelector('#statusbar').hidden && getComputedStyle(document.querySelector('#statusbar')).display === 'none' && localStorage.getItem('mory.status') === 'false'");
    await click(window, ".setting-row:has(#status-toggle) .switch span");
    await expect(window, "重新开启状态栏设置即时生效", "!document.querySelector('#statusbar').hidden && getComputedStyle(document.querySelector('#statusbar')).display === 'flex' && localStorage.getItem('mory.status') === 'true'");
    await inspect(window, "(() => { const bar = document.querySelector('#statusbar'); window.__statusBeforeZoom = { font: parseFloat(getComputedStyle(bar).fontSize), height: bar.getBoundingClientRect().height }; window.Mory.zoom(1); window.Mory.zoom(1); })()");
    await expect(window, "正文放大同步放大状态栏文字和高度", "(() => { const bar = document.querySelector('#statusbar'); return parseFloat(getComputedStyle(bar).fontSize) >= window.__statusBeforeZoom.font * 1.19 && bar.getBoundingClientRect().height > window.__statusBeforeZoom.height; })()");
    await inspect(window, "window.Mory.zoom(0)");
    await expect(window, "恢复实际大小同步恢复状态栏", "(() => { const bar = document.querySelector('#statusbar'); return parseFloat(getComputedStyle(bar).fontSize) === window.__statusBeforeZoom.font && Math.abs(bar.getBoundingClientRect().height - window.__statusBeforeZoom.height) < 0.5; })()");
    await click(window, "#workspace-add");
    await expect(window, "工作区插件表单可打开", "!document.querySelector('#workspace-form').hidden");
    await inspect(window, "(() => { const select = document.querySelector('#workspace-provider'); select.value = 's3'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "S3 凭证字段完整", "['endpoint','region','bucket','prefix','accessKeyId','accessKeySecret','sessionToken'].every(name => document.querySelector(`#workspace-provider-fields [name=\"${name}\"]`))");
    await inspect(window, "(() => { const select = document.querySelector('#workspace-provider'); select.value = 'sftp'; select.dispatchEvent(new Event('change', { bubbles: true })); })()");
    await expect(window, "SFTP 连接字段完整", "['host','port','username','password','privateKey','knownHosts','remotePath'].every(name => document.querySelector(`#workspace-provider-fields [name=\"${name}\"]`))");
    await click(window, "#workspace-form-close");
    await click(window, "#preferences-close");
    await expect(window, "设置窗口可关闭", "!document.querySelector('#preferences').classList.contains('is-open')");
    await expect(window, "设置入口不重复", "!document.querySelector('#more-button') && document.querySelectorAll('#settings-button').length === 1");

    await inspect(window, "document.dispatchEvent(new KeyboardEvent('keydown', { key: 'p', metaKey: true, bubbles: true }))");
    await wait(80);
    await expect(window, "侧栏搜索移除后快速打开快捷键仍可用", "!document.querySelector('#quick-open-button') && document.querySelector('#quick-open').classList.contains('is-open')");
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Escape" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Escape" });
    await wait(80);
    await expect(window, "快速打开可退出", "!document.querySelector('#quick-open').classList.contains('is-open')");

    await click(window, "#focus-button");
    await expect(window, "专注模式可点击", "document.querySelector('.workspace').classList.contains('focus-mode')");
    await click(window, "#focus-button");
    await click(window, "#typewriter-button");

    await click(window, "#word-count");
    await expect(window, "字数按钮可点击", "document.querySelector('#toast').classList.contains('is-visible') && document.querySelector('#toast').textContent.includes('字符')");
    await expect(window, "打字机模式可点击", "document.querySelector('.workspace').classList.contains('typewriter-mode')");
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
    await expect(window, "## 空格即时渲染二级标题", "document.querySelector('#write > h2') !== null && !document.querySelector('#write').textContent.includes('##')");
    await window.webContents.insertText("未保存标题");
    await wait(80);
    await click(window, ".tab[data-panel='outline']");
    await expect(window, "未保存标题实时进入大纲", "document.querySelector('#outline-count').textContent === '1 项' && document.querySelector('#outline-list .outline-item')?.textContent === '未保存标题'");
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
    await window.webContents.insertText("正文内容");
    await expect(window, "标题换行后恢复正文", "document.querySelector('#write > h2')?.textContent === '未保存标题' && document.querySelector('#write > p')?.textContent === '正文内容'");

    await inspect(window, `(() => {
      window.Mory.loadMarkdown('# 中文输入标题');
      const heading = document.querySelector('#write > h1');
      const range = document.createRange();
      range.selectNodeContents(heading);
      range.collapse(false);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      document.querySelector('#write').focus();
      heading.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true, data: '' }));
      heading.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', data: '中文输入标题', isComposing: true }));
      heading.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter', isComposing: true }));
    })()`);
    await expect(window, "输入法确认回车不会提前退出标题", "document.querySelectorAll('#write > h1').length === 1 && document.querySelectorAll('#write > p').length === 0");
    await inspect(window, "document.querySelector('#write > h1').dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '中文输入标题' }))");
    await wait(80);
    await expect(window, "中文输入法提交整行标题后即时渲染", "document.querySelector('#write > h1')?.textContent === '中文输入标题' && !document.querySelector('#write').textContent.includes('#')");

    const immediateCompositionEnter = await inspect(window, `(() => {
      window.Mory.loadMarkdown('');
      const paragraph = document.querySelector('#write > p');
      paragraph.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
      paragraph.textContent = '# 你好';
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      document.querySelector('#write').focus();
      paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', data: '你好', isComposing: true }));
      paragraph.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '你好' }));
      const enter = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' });
      const dispatched = paragraph.dispatchEvent(enter);
      return { prevented: !dispatched };
    })()`);
    await expect(window, "输入法提交后立即回车同步生成标题和正文", `(() => {
      const anchor = getSelection()?.anchorNode;
      const block = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('#write > *');
      return ${JSON.stringify(immediateCompositionEnter.prevented)} && document.querySelector('#write > h1')?.textContent === '你好' && document.querySelector('#write > h1 + p') && block?.tagName === 'P';
    })()`);

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
      await window.webContents.insertText("你好");
      window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Enter" });
      window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Enter" });
      await wait(30);
    }
    await wait(80);
    await expect(window, "连续中文标题保持两个独立块", "document.querySelectorAll('#write > h1').length === 2 && [...document.querySelectorAll('#write > h1')].every(item => item.textContent === '你好') && document.querySelector('#write > h1 + h1') !== null && document.querySelector('#write > h1:last-of-type + p') !== null");

    await inspect(window, `(() => {
      const editor = document.querySelector('#write');
      editor.innerHTML = '<p># 12 - test</p><h1>你好啊</h1>';
      const activeHeading = editor.lastElementChild;
      const range = document.createRange();
      range.selectNodeContents(activeHeading);
      range.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(range);
      editor.focus();
      editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '啊' }));
    })()`);
    await expectEventually(window, "遗漏的首行 Markdown 标题会在下一帧补偿渲染", "document.querySelector('#write > h1:first-child')?.textContent === '12 - test' && document.querySelectorAll('#write > h1').length === 2");

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
    await window.webContents.insertText("这是 **即时加粗** 文本");
    await expect(window, "成对星号即时转换加粗", "document.querySelector('#write p strong')?.textContent === '即时加粗' && document.querySelector('#write p')?.textContent === '这是 即时加粗 文本'");

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
    await expectEventually(window, "整段粘贴即时渲染标题加粗和代码块", "document.querySelector('#write > h1')?.textContent === '粘贴标题' && document.querySelector('#write strong')?.textContent === '粘贴加粗' && document.querySelector('#write > pre[data-language=\"go\"] code')?.textContent === 'fmt.Println(\"paste\")'");

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
    await expect(window, "代码围栏即时转换并保留语言", "document.querySelector('#write > pre')?.dataset.language === 'go' && document.querySelector('#write > pre code')?.textContent === 'fmt.Println(\"hi\")' && window.Mory.getMarkdown().includes('```go')");
    await insertParagraph(window);
    await window.webContents.insertText("fmt.Println(\"bye\")");
    await insertParagraph(window);
    await window.webContents.insertText("```");
    await wait(80);
    await expect(window, "多行代码保持同一围栏并在闭合后回到正文", "document.querySelectorAll('#write > pre').length === 1 && document.querySelector('#write > pre code')?.textContent === 'fmt.Println(\"hi\")\\nfmt.Println(\"bye\")' && !document.querySelector('#write > pre code')?.textContent.includes('```') && document.querySelector('#write > pre + p') !== null");

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
    await expect(window, "代码块末尾连续两次回车退出到正文", "(() => { const anchor = getSelection()?.anchorNode; const block = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('#write > *'); return document.querySelector('#write > pre code')?.textContent === 'fmt.Println(\"double\")' && document.querySelector('#write > pre + p') !== null && block?.tagName === 'P'; })()");

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
    await expect(window, "代码块末行下键打开语言和名称字段", "!document.querySelector('#write > .code-meta')?.hidden && document.activeElement?.classList.contains('code-language') && document.querySelectorAll('#write > .code-meta input').length === 2");
    await fs.writeFile(codeMetaScreenshotPath, (await window.webContents.capturePage()).toPNG());
    await inspect(window, `(() => {
      const language = document.querySelector('.code-meta .code-language');
      language.value = 'rust';
      language.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Right" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Right" });
    await wait(30);
    await expect(window, "代码信息左右键切换字段", "document.activeElement?.classList.contains('code-title')");
    await inspect(window, `(() => {
      const title = document.querySelector('.code-meta .code-title');
      title.value = 'main.rs';
      title.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }));
    })()`);
    window.webContents.sendInputEvent({ type: "keyDown", keyCode: "Down" });
    window.webContents.sendInputEvent({ type: "keyUp", keyCode: "Down" });
    await wait(60);
    await expect(window, "代码语言和名称写回 Markdown 后再次下键退出", "(() => { const anchor = getSelection()?.anchorNode; const block = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('#write > *'); return document.querySelector('#write > pre')?.dataset.language === 'rust' && document.querySelector('#write > pre')?.dataset.title === 'main.rs' && document.querySelector('.code-meta')?.hidden && window.Mory.getMarkdown().includes('```rust title=\"main.rs\"') && block?.tagName === 'P'; })()");

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
    await window.webContents.insertText("这里是 `hi");
    await window.webContents.insertText("`");
    await window.webContents.insertText(" 正文");
    await expect(window, "成对反引号即时转换行内代码", "document.querySelector('#write p code')?.textContent === 'hi' && document.querySelector('#write p')?.textContent === '这里是 hi 正文' && window.Mory.getMarkdown().includes('`hi`')");

    await inspect(window, `window.Mory.loadMarkdown('# 格式测试\\n\\n需要加粗的段落')`);
    await inspect(window, `(() => {
      const paragraph = document.querySelector('#write p');
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
    })()`);
    await click(window, "#toolbar button[data-command='bold']");
    await expect(window, "格式按钮可点击", "document.querySelector('#write strong, #write b') !== null");

    if (errors.length) throw new Error(`页面错误：${errors.join(" | ")}`);
    process.stdout.write(JSON.stringify({ status: "passed", interactions: 86, rendererErrors: 0, responsiveFullscreenTypography: true, multipleUntitledDocuments: true, draftSwitchingPreservesContent: true, removableUntitledDocuments: true, savedDocumentTrash: true, deleteCancellation: true, activeDocumentCloseFallback: true, lastCloseCreatesBlankDocument: true, emptyWorkspacePlaceholder: true, nonEmptyWorkspaceAutoOpen: true, deletedWorkspaceFileReconciled: true, headingEnterCreatesParagraph: true, compositionHeadingRendering: true, immediateCompositionEnter: true, separateConsecutiveHeadings: true, staleHeadingRecovery: true, liveBold: true, pastedMarkdownRendering: true, liveFencedCode: true, multiLineFencedCode: true, fencedCodeExit: true, doubleEnterCodeExit: true, codeMetadataNavigation: true, liveInlineCode: true, instantHeading: true, liveUnsavedOutline: true, workspaceCreationOrder: true, stableOpenedFilePosition: true, statusbarSetting: true, zoomedStatusbar: true, readableSidebarContrast: true, coherentDarkDocument: true, iconOnlyToolbar: true, hoverTooltip: true, sidebarSearchRemoved: true, verticalFloatingToolbar: true, singleSettingsEntry: true, macTrafficLightSafeArea: true, screenshot: screenshotPath, codeMetaScreenshot: codeMetaScreenshotPath }, null, 2) + "\n");
  } catch (error) {
    process.stderr.write(`${error.stack || error}${errors.length ? `\n页面错误：${errors.join(" | ")}` : ""}\n`);
    exitCode = 1;
  } finally {
    window.destroy();
    app.exit(exitCode);
  }
});
