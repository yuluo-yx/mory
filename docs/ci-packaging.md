# CI 与本地打包

## 目标

项目使用同一组 Makefile 目标封装本地命令和 GitHub Actions 命令。普通构建负责测试并上传 Actions 制品；带 `release_tag` 参数的手动构建还会创建 GitHub Release。构建环境同时安装 Node.js 22 和 Go 1.25，Go 用于生成跨平台存储插件侧车。

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

工作流文件为 `.github/workflows/build-binaries.yml`，Actions 页面显示名为 `Build and Release Desktop Apps`。workflow、job、step 与 artifact 的可见名称统一使用英文。工作流支持手动触发、`main` 分支推送和拉取请求，并执行以下任务：

- `macos-15` 生成 ARM64 DMG 和 ZIP。
- `macos-15-intel` 生成 x64 DMG 和 ZIP。
- `windows-2025` 生成 x64 和 ARM64 安装版、便携版。
- 每个任务先运行检查、单元测试和 Electron 端到端测试。
- 每个应用包包含对应架构的 `mory-storage` 侧车。
- macOS 任务额外运行 WKWebView、窗口交互、知识图谱滚轮和工作区监听冒烟测试。
- Electron Builder 显式使用 `--publish never`，防止 CI 环境误触发发布并索取个人 Token。
- 制品保留 14 天。

普通分支推送和拉取请求不会发布版本。需要发布时，手动传入与 `package.json` 完全一致的标签：

```bash
gh workflow run build-binaries.yml -f release_tag=v0.1.0
```

全部平台构建通过后，`Publish GitHub Release` 任务会执行以下操作：

- 校验标签与应用版本一致。
- 校验 2 个 macOS 架构和 2 个 Windows 架构的 8 个必需制品。
- 生成包含全部制品的 SHA-256 校验文件。
- 使用 runner 内置的 GitHub CLI 创建标签和 Release。
- 使用仓库临时 `GITHUB_TOKEN` 写入 Release，不读取个人 Token。

## 调研记录

访问日期：2026-08-16。

检索关键词：

- `GitHub Actions setup-node v6 official`
- `GitHub hosted runners macos-15-intel macos-15 arm64 official`
- `GitHub Actions upload-artifact v7 official`
- `GitHub Actions download-artifact v8 official`
- `GitHub CLI release create official`
- `GitHub Actions GITHUB_TOKEN permissions official`

采用依据：

- GitHub 官方仓库当前稳定版本为 `actions/checkout@v7`、`actions/setup-node@v7` 和 `actions/setup-go@v7`。
- GitHub 托管运行器文档标明 `macos-15` 为 ARM64，`macos-15-intel` 为 Intel x64，`windows-2025` 为 x64。
- GitHub 官方 `upload-artifact` 文档使用 `actions/upload-artifact@v7`，并要求矩阵任务使用唯一制品名。
- GitHub 官方 `download-artifact` 当前稳定版本为 v8，并支持把多个矩阵制品合并到同一目录。
- GitHub CLI 的 `gh release create` 支持指定目标提交、生成发布说明并上传多个制品。
- GitHub Actions 可以为单个发布任务授予 `contents: write`，其余任务继续保持只读权限。

参考资料：

- [setup-node](https://github.com/actions/setup-node)
- [GitHub 托管运行器参考](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
- [upload-artifact](https://github.com/actions/upload-artifact)
- [download-artifact](https://github.com/actions/download-artifact)
- [gh release create](https://cli.github.com/manual/gh_release_create)
- [控制 GITHUB_TOKEN 权限](https://docs.github.com/actions/how-tos/writing-workflows/choosing-what-your-workflow-does/controlling-permissions-for-github_token)

## 验证范围与工具降级

当前会话没有提供 Sequential Thinking、Context7、Fetch 和 Playwright MCP。实施过程改用本地 `rg`、`apply_patch`、GNU Make、Electron 端到端测试、WKWebView 冒烟测试，以及 GitHub 官方公开资料检索。降级只影响调研和浏览器自动化工具入口，不影响生成的应用代码、工作流或制品。

本地 macOS 环境已验证 ARM64 应用和 Windows 双架构交叉打包。macOS 真实键盘测试按 20 ms 间隔发送输入事件，避免托管运行器负载较高时丢失连续按键。工作流推送后由 GitHub Actions 执行远端验证。Windows 安装、文件关联和卸载仍需在真实 Windows 机器上核验。

## 迁移与回滚

本次变更不替换现有 npm 脚本和 Makefile 入口。普通 CI 行为保持不变；发布者改用 `workflow_dispatch` 的 `release_tag` 参数，因此无需迁移已有本地命令。

如需撤销未完成的发布，取消对应 Actions Run。Release 已创建时，执行 `gh release delete <标签> --cleanup-tag --yes` 可同时删除 Release 和标签。如需停用远端构建，删除 `.github/workflows/build-binaries.yml`；本地命令和已经生成的制品不受影响。

## 已知风险

2026-08-16 使用本机 Go 1.26.5 执行 `govulncheck ./...`。扫描确认当前调用链可触达 5 个 Go 标准库漏洞，修复版本均为 Go 1.26.6。按当前项目决策，不安装或切换 Go 1.26.6；本地构建继续使用现有工具链，发布前由维护者决定是否接受这些风险。

扫描还发现依赖包与模块中各有 2 个不可达漏洞。`npm audit --omit=dev` 未发现生产依赖漏洞。
