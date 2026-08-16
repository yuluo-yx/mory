# Windows WebView2 迁移说明

## 目标

Windows 发布版从 Electron 迁移到 Wails v2 和 Microsoft Edge WebView2。迁移保留共享编辑器、Markdown 即时渲染、主题、Mermaid、知识图谱、工作区和导出行为，不改动 macOS Swift 宿主。

迁移后的 x64、ARM64 GUI 主程序分别约为 31 MB 和 28 MB。旧 Electron 安装版约为 100 MB。实际 NSIS 体积以 Windows 打包结果为准。

## 运行结构

```text
Sources/Mory/Web
    │ Wails 动态绑定
    ▼
cmd/mory-windows
    │
    ├── internal/windowshost：工作区、文稿、图片、主题、监听和宿主协议
    ├── internal/storage：GitHub、S3/S4、OSS 和 SFTP
    └── WebView2：编辑器窗口
```

前端继续使用 `bridge` 和 `hostRequest`。Windows 运行时通过 `window.go.main.WindowsHost` 接入 Go。Electron preload 与 macOS WKWebView 的既有协议保持兼容。

## 功能迁移

- 工作区配置保存在当前用户的应用数据目录。
- Windows 主程序直接调用 Go 存储后端，不再启动 `mory-storage.exe`。
- 文稿按 Windows 文件创建时间排序；时间相同按文件名自然排序。
- 工作区每 750 ms 比对一次文件路径、大小和修改时间。外部新增、重命名或删除会刷新原子快照。
- 图片按文稿名存放。打开文稿和即时输入图片路径时，宿主返回 Data URL。
- 用户 CSS 主题保存在可配置目录。相对字体和图片资源在载入时内联。
- Windows 菜单随中英文设置即时重建。
- 标准 Windows 标题栏负责拖动、双击最大化和还原。

## 导出

前端先生成完整 HTML。该页面已经包含主题 CSS、Mermaid SVG、代码高亮和内联图片。

- HTML：Go 宿主直接写入 UTF-8 文件。
- PDF：系统 Microsoft Edge 通过 DevTools `Page.printToPDF` 生成。
- PNG、JPEG：系统 Microsoft Edge 设置文档尺寸后，通过 `Page.captureScreenshot` 生成整页图片。

导出在独立 Edge 进程中运行，不阻塞编辑器 WebView2。单张图片高度上限为 28,000 px。Windows 缺少 Microsoft Edge 时，HTML 仍可导出，PDF 和图片会返回明确错误。

## 构建

在 macOS 或 Linux 上验证双架构 GUI：

```bash
make windows-build-x64 windows-build-arm64
```

在 Windows 上生成 NSIS 安装版和便携版：

```powershell
make package-windows
```

打包脚本固定使用 Wails v2.13.0，并内嵌 WebView2 Evergreen bootstrapper。系统已安装 WebView2 时，Mory 直接复用系统运行时，不把完整 Chromium 或 Fixed Version Runtime 放入安装包。

## 验证结果

最后验证日期：2026-08-16。

- Go 单元测试和 race detector 通过。
- 新 Windows 宿主语句覆盖率为 85.1%。未达到 90% 目标，未覆盖部分主要是文件系统故障注入分支。
- JavaScript 44 个单元测试全部通过，总语句覆盖率为 95.32%。
- Electron 共享前端 E2E 通过。导出、Mermaid、代码高亮和 86 个交互检查无 renderer error。
- Wails v2 官方构建器完成 Windows x64 production 构建。
- Go 交叉编译完成 Windows x64 和 ARM64 GUI 构建。
- `go vet`、Windows 专用 `go vet`、`go mod verify` 和 `git diff --check` 通过。

## 已接受风险

构建机当前使用 Go 1.26.5。`govulncheck` 报告 5 个可达的标准库漏洞，修复版本为 Go 1.26.6。用户在 2026-08-16 明确接受该风险，本次不升级 SDK。后续只需使用 Go 1.26.6 或更高修复版本重新构建，无需迁移数据或修改业务接口。

Windows NSIS 安装、文件关联、系统回收站、Edge PDF/图片导出和卸载仍需在真实 Windows x64、ARM64 环境完成最终回归。

## 回滚

回滚时恢复旧 Makefile 的 Electron Windows 打包目标，并移除 `cmd/mory-windows`、`internal/windowshost` 和 Wails 前端桥接。工作区配置仍使用 `workspaces.json`，本地目录和远端数据无需迁移。
