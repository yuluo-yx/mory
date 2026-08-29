# Mory Markdown 编辑器

Mory 是一款面向 macOS 和 Windows 的跨平台 Markdown 编辑器。它使用原生桌面宿主和共享编辑内核，在保留纯 Markdown 文件的同时提供所见即所得写作体验。

## 名称

`Mory` 由 `M` 和 `ory` 组成。`M` 代表 Markdown 与 Memory，`ory` 取自 Story。这个名称表达了产品目标：用 Markdown 记录内容，并把分散文稿组织成可检索、可引用、可长期保存的个人记忆与故事。

项目正式名称为 `Mory`。应用名称、包名和发布制品均使用这一拼写。

## 优势

- 所见即所得：直接编辑排版后的 Markdown，同时保留源码模式。
- 原生桌面体验：macOS 使用 Swift、AppKit 和 WKWebView；Windows 使用 Go、Wails 和 WebView2。
- 本地优先：工作区是普通目录，文稿和图片不依赖专有数据库。
- 知识组织：支持目录树、知识图谱、双向链接和反向链接。
- 完整 Markdown 能力：支持表格、代码高亮、Mermaid、图片和自定义主题。
- 同源导出：HTML、PDF、PNG 和 JPEG 与编辑区使用同一份主题化内容。
- 双语界面：可在设置中即时切换简体中文和 English。

## 构建

构建环境需要 Node.js 22、npm 10 和 Go 1.26.6。macOS 构建还需要 Swift 6；Windows 安装包需要 NSIS。

安装依赖：

```bash
npm ci
```

启动 Electron 开发壳：

```bash
npm run dev
```

执行完整验证：

```bash
make verify
```

### macOS

生成当前架构的 Mory.app、DMG、ZIP 和 SHA-256 校验文件：

```bash
make package-macos
```

输出目录：

```text
dist/macos/Mory.app
dist/releases/Mory-<版本>-macos-<架构>.dmg
dist/releases/Mory-<版本>-macos-<架构>.zip
dist/releases/Mory-<版本>-macos-<架构>-SHA256SUMS.txt
```

### Windows

在 Windows PowerShell 中生成 x64 安装版和便携版：

```powershell
powershell -ExecutionPolicy Bypass `
  -File .\scripts\package-windows-wails.ps1 `
  -Architecture amd64
```

ARM64 使用 `-Architecture arm64`。安装 GNU Make 后，也可以同时生成两种架构：

```powershell
make package-windows
```

输出目录：

```text
dist\windows\Mory-Setup-<版本>-<架构>.exe
dist\windows\Mory-Portable-<版本>-<架构>.exe
```

在 macOS 或 Linux 上只能交叉编译 Windows GUI 可执行文件，不生成 NSIS 安装器：

```bash
make windows-build-x64 windows-build-arm64
```

## Command line

Release packages include a Go-based `mory` client. It delegates rendering to the installed native app, so CLI exports use the same Markdown engine, themes, Mermaid renderer, and syntax highlighting as the editor.

Open a document:

```bash
mory guide.md
```

Export HTML, PDF, PNG, or JPEG into a directory:

```bash
mory export --format=pdf --path=./ guide.md
```

Existing output files are preserved by default. Pass `--force` to replace the derived output file. Use `--app=/path/to/Mory.app` on macOS or `--app=C:\\path\\to\\Mory.exe` on Windows when the app is not installed in its standard location.

On macOS, copy the release `*-cli` artifact to a directory on `PATH`, for example `/usr/local/bin/mory`. On Windows, rename the matching `Mory-CLI-<version>-<arch>.exe` artifact to `mory.exe`, place it on `PATH`, and keep the Mory application installed. The CLI also ships inside `Mory.app/Contents/Resources/bin/mory` for managed deployments.
