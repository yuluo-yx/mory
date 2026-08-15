# Mermaid 渲染实现记录

## 功能范围

Mory 支持把语言标记为 `mermaid` 的 Markdown 代码围栏渲染为 SVG。编辑区、HTML、PDF、PNG 和 JPEG 使用同一渲染结果。

当前锁定 Mermaid 11.16.1。构建脚本从 npm 包复制浏览器运行时和许可证。应用运行时不访问外部网络。

## 渲染流程

1. Markdown 解析器把 Mermaid 围栏转换为待渲染节点，并保存原始图表代码。
2. 编辑器调用官方 `initialize` 和 `render` API，把节点替换为 SVG。
3. 切换文档主题时，编辑器按主题调色板重新渲染图表。
4. 导出前创建独立正文节点，并按导出主题生成 SVG。
5. 导出的 HTML 直接内联 SVG。PDF 和图片从该 HTML 渲染。

图表使用 `securityLevel: "strict"`，关闭图内点击脚本和未转义 HTML。语法错误会显示错误摘要和原始代码，不会丢失 Markdown 源文。

## macOS 界面修复

- 侧边栏收起后，标题栏左侧保留 78 px 安全区，避免按钮与红黄绿窗口控制重叠。
- 原生 WKWebView 通过指针事件向 Swift 宿主发送屏幕坐标。宿主按坐标增量移动窗口。
- 按钮、输入框、链接和可编辑元素不属于拖动区域。
- Dock 图标主体缩放到画布的约 78%，四周保留 56 px 透明边距。
- 格式、源码和导出操作合并为右下角固定纵向浮动栏，顶部标题栏只保留窗口级操作。
- 删除正文、H1 和 H2 按钮；在空白行输入 `# `～`###### ` 后即时转换为标题。

## 外部资料记录

- 检索关键词：`site:mermaid.js.org configuring mermaid initialize startOnLoad run API securityLevel official`。
- 筛选条件：仅采用 Mermaid 官方文档；不采用博客或第三方 CDN 示例。
- 访问日期：2026-08-15。
- 采用依据：[Mermaid Usage](https://mermaid.js.org/config/usage.html) 推荐通过 `initialize` 配置并调用渲染 API；[Mermaid Config Schema](https://mermaid.js.org/config/schema-docs/config) 说明 `startOnLoad` 和 `securityLevel` 配置。

## 验收标准

- Mermaid 围栏可以渲染为 SVG。
- 从预览切换到源码后仍保留原始围栏。
- Newsprint 和 Night 等文档主题使用对应图表配色。
- HTML、PDF、PNG 和 JPEG 包含图表，不引用 CDN。
- 语法错误不会破坏编辑器。
- 侧边栏收起后，标题栏按钮不进入窗口控制区。
- macOS 标题栏空白区域可以拖动，交互控件保持可点击。
- 原生拖动冒烟通过页面桥接发送坐标，窗口位置应准确变化 `dx=36, dy=18`。
- 右下角工具栏保持可见，`## ` 可以通过真实键盘事件即时转换为二级标题。

## 验证结果

- Mermaid 11.16.1 已锁定在 `package-lock.json`，离线运行时和 MIT 许可证已写入双平台制品。
- 15 项 Markdown 单元测试全部通过；行覆盖率 100%，分支覆盖率 92.67%，函数覆盖率 100%。
- Electron E2E 已验证 Mermaid 正常渲染、语法错误恢复、Newsprint/Night 双主题、源码往返和 SVG 内联导出。
- 19 项真实交互通过，包括 `## ` 即时标题、纵向浮动栏、单一设置入口和 macOS 窗口控制安全区。
- HTML、PNG 和 PDF 导出通过，渲染器错误为 0。
- macOS WKWebView 包内资源冒烟通过；原生窗口拖动冒烟通过，位移为 `dx=36, dy=18`。
- Windows ASAR 已验证包含最终 Mermaid 运行时、许可证、纵向工具栏和单一设置入口。
- 依赖离线审计未发现漏洞。

本次无需迁移 Markdown 文档或配置，直接替换旧应用即可。Windows 真机安装、文件关联和卸载仍需单独验证。

最后验证日期：2026-08-15。
