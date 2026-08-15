# 按钮无响应修复记录

## 问题现象

macOS 原生应用能够显示编辑器页面，但侧栏、源码、导出和设置等按钮没有响应。Windows 版使用相同页面，因此需要同步完成回归验证。

## 根因

页面原先通过 `<script type="module">` 加载 `app.js`。macOS 原生宿主使用 WKWebView 读取本地 `file://` 页面。在该加载方式下，ES module 没有执行，导致 `window.Mory` 为 `undefined`，所有 JavaScript 监听器均未注册。

旧端到端测试直接调用 `window.Mory.exportDocument()`，没有发送鼠标事件，因此没有覆盖真实交互。

## 修复方式

- 新增 `scripts/build-web.mjs`，把 `markdown.js` 和 `app.js` 生成单文件经典脚本 `app.bundle.js`。
- 页面改为加载 `app.bundle.js`，macOS 和 Windows 使用同一运行产物。
- macOS 宿主在页面完成加载后检查 `window.Mory`。脚本加载失败时显示明确错误。
- 新增原生 WKWebView 冒烟测试和 Electron 真实鼠标点击测试。
- 测试失败改为返回非零退出码，避免产生假绿结果。

## 回滚方式

如需回滚，可恢复 `index.html` 的 ES module 引用，删除 Web 构建脚本及其 npm 生命周期命令，并恢复原测试退出逻辑。该回滚会重新引入 macOS 按钮无响应问题，仅适用于定位历史行为。

## 验证结果

- `npm run check`：通过，生成包与源码一致，JavaScript 语法检查无错误。
- `npm test`：14 项测试全部通过；行覆盖率 100%，分支覆盖率 92.96%，函数覆盖率 100%。
- `npm run test:e2e`：导出回归通过；19 项真实鼠标交互通过；渲染器错误为 0。
- `npm run test:mac-web`：WKWebView 中 `window.Mory` 初始化成功；导出、设置和源码按钮通过。
- macOS 打包资源冒烟：通过；应用临时签名校验通过。
- Windows ASAR 检查：`index.html` 已引用 `app.bundle.js`；Windows 真机安装、文件关联和卸载仍需单独验证。

本次无需迁移用户文档或配置，直接用新制品替换旧应用即可。

最后验证日期：2026-08-15。
