# CI 与本地打包

## 目标

项目使用同一组 Makefile 目标封装本地命令和 GitHub Actions 命令。工作流负责构建、测试和上传 Actions 制品，不创建 GitHub Release，也不提交代码。构建环境同时安装 Node.js 22 和 Go 1.25，Go 用于生成跨平台存储插件侧车。

## 本地命令

先安装锁定依赖：

```bash
make install
```

执行完整本地验证：

```bash
make verify
make storage
```

按平台生成制品：

```bash
make package-macos
make package-windows-x64
make package-windows-arm64
make package-windows
```

`package-macos` 生成当前 macOS 构建机架构的 `Mory.app`、DMG、ZIP 与 SHA-256 校验文件。可分发制品位于 `dist/releases/`。`package-windows` 同时生成 x64 和 ARM64 的安装版与便携版。

## GitHub Actions

工作流文件为 `.github/workflows/build-binaries.yml`，Actions 页面显示名为 `Build Desktop Artifacts`。workflow、job、step 与 artifact 的可见名称统一使用英文。工作流支持手动触发、`main` 分支推送和拉取请求，并执行以下任务：

- `macos-15` 生成 ARM64 应用包。
- `macos-15-intel` 生成 x64 应用包。
- `windows-2025` 生成 x64 和 ARM64 安装版、便携版。
- 每个任务先运行检查、单元测试和 Electron 端到端测试。
- 每个应用包包含对应架构的 `mory-storage` 侧车。
- macOS 任务额外运行 WKWebView 与窗口拖动冒烟测试。
- Electron Builder 显式使用 `--publish never`，防止 CI 环境误触发发布并索取个人 Token。
- 制品保留 14 天。

工作流不会自动发布制品。需要发布时，应在验证现有 Actions 制品后另行增加签名、公证和 Release 流程。

## 调研记录

访问日期：2026-08-15。

检索关键词：

- `GitHub Actions setup-node v6 official`
- `GitHub hosted runners macos-15-intel macos-15 arm64 official`
- `GitHub Actions upload-artifact v7 official`

采用依据：

- GitHub 官方 `setup-node` 文档使用 `actions/checkout@v6`、`actions/setup-node@v6` 和显式 Node.js 版本。
- GitHub 托管运行器文档标明 `macos-15` 为 ARM64，`macos-15-intel` 为 Intel x64，`windows-2025` 为 x64。
- GitHub 官方 `upload-artifact` 文档使用 `actions/upload-artifact@v7`，并要求矩阵任务使用唯一制品名。

参考资料：

- [setup-node](https://github.com/actions/setup-node)
- [GitHub 托管运行器参考](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
- [upload-artifact](https://github.com/actions/upload-artifact)

## 验证范围与工具降级

当前会话没有提供 Sequential Thinking、Context7、Fetch 和 Playwright MCP。实施过程改用本地 `rg`、`apply_patch`、GNU Make、Electron 端到端测试、WKWebView 冒烟测试，以及 GitHub 官方公开资料检索。降级只影响调研和浏览器自动化工具入口，不影响生成的应用代码、工作流或制品。

本地 macOS 环境已验证 ARM64 应用和 Windows 双架构交叉打包。macOS 真实键盘测试按 20 ms 间隔发送输入事件，避免托管运行器负载较高时丢失连续按键。工作流推送后由 GitHub Actions 执行远端验证。Windows 安装、文件关联和卸载仍需在真实 Windows 机器上核验。

## 迁移与回滚

本次变更不替换现有 npm 脚本。原命令仍然可用，Makefile 只提供统一入口，因此无需迁移。

如需停用远端构建，删除 `.github/workflows/build-binaries.yml`，继续直接运行 Makefile 或 `package.json` 中的 npm 脚本。删除工作流不会影响本地命令和已经生成的制品。

## 已知风险

2026-08-16 使用本机 Go 1.26.5 执行 `govulncheck ./...`。扫描确认当前调用链可触达 5 个 Go 标准库漏洞，修复版本均为 Go 1.26.6。按当前项目决策，不安装或切换 Go 1.26.6；本地构建继续使用现有工具链，发布前由维护者决定是否接受这些风险。

扫描还发现依赖包与模块中各有 2 个不可达漏洞。`npm audit --omit=dev` 未发现生产依赖漏洞。
