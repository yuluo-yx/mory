# Typora 本机应用包审计记录

## 审计目的

本记录用于说明 Mory 的架构依据。审计对象是用户本机安装的 Typora 0.11.18。审计仅读取应用包、公开前端资源、菜单文案和二进制依赖，不反编译或复制闭源实现。

审计日期：2026-08-15。

## 输入来源

| 编号 | 来源 | 观察结果 | Mory 对应实现 |
| --- | --- | --- | --- |
| 1 | `/Applications/Typora.app/Contents/Info.plist` | 版本为 0.11.18，构建号为 5941；注册 Markdown 与纯文本类型 | macOS `Info.plist` 和 Windows `fileAssociations` 注册 Markdown 文件 |
| 2 | `/Applications/Typora.app/Contents/MacOS/Typora` | Mach-O 通用二进制依赖 Cocoa、WebKit、Sparkle 和 Sentry | macOS 使用 AppKit 与 WKWebView；更新和遥测不在本次范围内 |
| 3 | `/Applications/Typora.app/Contents/Resources/TypeMark/index.html` | 主界面包含侧边栏、文件、大纲、搜索、源码区和 `#write` 编辑区 | 共享 Web 内核保留相同信息分区 |
| 4 | `/Applications/Typora.app/Contents/Resources/TypeMark/style/themes/` | 每套主题对应一个 CSS 文件 | 六套主题分别存放在 `Sources/Mory/Web/themes/` |
| 5 | `/Applications/Typora.app/Contents/Resources/TypeMark/Docs/Custom Themes.md` | 主题目录支持基础主题、`base.user.css` 和主题专属覆盖文件 | 当前版本实现独立主题 CSS；用户覆盖层列入后续扩展接口 |
| 6 | `/Applications/Typora.app/Contents/Resources/TypeMark/appsrc/main.js` | `theme_css` 动态切换主题；导出时收集基础 CSS、主题 CSS 和用户 CSS | Mory 动态切换 `document-theme`，导出时读取并内联主题 CSS |
| 7 | 同上，`exportPDF` 和 `printToPDF` 调用附近 | PDF 先生成主题化 HTML，再交给宿主打印后端 | Electron 使用 `printToPDF`；WKWebView 使用无界面打印操作 |
| 8 | 同上，`exportToImage` 调用附近 | 图片导出创建独立页面，测量总高度，再截图并组合 | Mory 创建离屏页面、按宽度重排、测量高度并输出 PNG 或 JPEG |
| 9 | `/Applications/Typora.app/Contents/Resources/TypeMark/Docs/Install and Use Pandoc.md` | HTML 和 PDF 不需要 Pandoc；其他高级格式依赖 Pandoc 2.0 或更高版本 | 当前 HTML、PDF 和图片导出均不依赖 Pandoc |
| 10 | `/Applications/Typora.app/Contents/Resources/TypeMark/Docs/Markdown Reference.md` | 输入围栏并按回车创建独立代码块，可在围栏后提供语言 | Mory 把围栏转换为独立 `pre` 块，并在 Markdown 往返时保留语言 |
| 11 | `/Applications/Typora.app/Contents/Resources/TypeMark/Docs/Code Fences Language Support.md` | 语言标识不区分大小写，编辑层与语法高亮层分离 | Mory 当前保留原始语言标识；语法高亮仍作为后续独立能力处理 |
| 12 | `/Applications/Typora.app/Contents/Resources/TypeMark/Docs/Change Log.md` | 代码块的上下边界、退出和保存曾作为专门缺陷持续修复 | Mory 为代码块建立独立的回车/方向键状态转换，并加入自动化边界回归 |

## 架构结论

Typora 0.11.18 采用原生桌面宿主与 Web 编辑器结合的结构。原生层负责窗口、菜单、文件系统和导出。Web 层负责 Markdown 文档模型、编辑器 DOM、主题和导出 HTML。

Mory 采用相同的职责边界，但使用独立代码：

```text
Markdown 文本
    ↓
共享解析与编辑器 DOM
    ↓
主题 CSS + 导出基础 CSS
    ↓
主题化 HTML
    ├── 直接写入 HTML
    ├── 宿主打印为 PDF
    └── 离屏页面截图为 PNG/JPEG
```

该结构确保编辑区与导出结果使用同一份 Markdown 解析结果和主题样式。新增主题时，不需要修改 PDF 或图片导出代码。

## 差异说明

- Mory 未复制 Typora 的 `main.js`、原生二进制、图标和主题 CSS。
- Mory 不包含激活、许可证绕过或 Typora 在线服务。
- Mory 不接入 Sparkle、Sentry、AWS SDK、MathJax 和 Pandoc；Mermaid 使用锁定版本的官方运行时并离线打包。
- 双回车退出和可选代码名称是按用户需求独立实现的交互扩展，不声称来自 Typora 的内部代码。
- Mory 允许不兼容替换，无旧数据迁移要求。Markdown 文件仍是直接读写的 UTF-8 文本。

## 验证证据

2026-08-15 的本地验证结果如下：

- JavaScript 语法检查通过。
- Markdown 单元测试 16 项全部通过。
- `markdown.js` 行覆盖率为 100%，函数覆盖率为 100%，分支覆盖率为 90.12%。
- Electron 41 项交互回归连续两轮通过，渲染进程错误为 0；其中包含输入法提交后零等待 Enter。
- macOS 原生 `NSEvent → WKWebView` 测试实际输入两个连续中文标题，并验证代码块双回车退出。
- Electron 端到端测试成功生成 HTML、PNG 和 PDF，代码名称完成 Markdown 与导出往返。
- macOS 构建、WKWebView 冒烟、原生键盘和窗口拖动测试通过。
- Windows x64 与 ARM64 的 NSIS 安装版和便携版均已生成。
