# CI 与本地打包

## 目标

项目使用 Makefile 封装本地验证和打包命令。仓库默认不启用远端 CI/CD，只在 `docs/examples/build-binaries.yml` 保留可选模板。

## 本地命令

先安装锁定依赖：

```bash
make install
```

执行完整本地验证：

```bash
make verify
```

按平台生成制品：

```bash
make package-macos
make package-windows-x64
make package-windows-arm64
make package-windows
```

`package-macos` 生成当前 macOS 构建机架构的 `Mory.app`。`package-windows` 同时生成 x64 和 ARM64 的安装版与便携版。

## 可选的 GitHub Actions 模板

模板文件为 `docs/examples/build-binaries.yml`，不会被 GitHub 自动识别或执行。未来得到明确授权后，可将模板复制到 `.github/workflows/build-binaries.yml`。模板支持手动触发、`main` 分支推送和拉取请求，并执行以下任务：

- `macos-15` 生成 ARM64 应用包。
- `macos-15-intel` 生成 x64 应用包。
- `windows-2025` 生成 x64 和 ARM64 安装版、便携版。
- 每个任务先运行检查、单元测试和 Electron 端到端测试。
- macOS 任务额外运行 WKWebView 与窗口拖动冒烟测试。
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

本地 macOS 环境已验证 ARM64 应用和 Windows 双架构交叉打包。GitHub Actions 模板没有放入启用目录，因此不会产生远端运行记录。Windows 安装、文件关联和卸载仍需在真实 Windows 机器上核验。

## 迁移与回滚

本次变更不替换现有 npm 脚本。原命令仍然可用，Makefile 只提供统一入口，因此无需迁移。

如需启用远端构建，把 `docs/examples/build-binaries.yml` 复制到 `.github/workflows/build-binaries.yml`。如需移除模板，只删除示例文件即可；本地 Makefile 和已生成制品不受影响。
