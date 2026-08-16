# 工作区与存储插件

## 工作区模型

Mory 把工作目录作为工作区的核心。文稿、图片和其他附件都保存在当前工作目录中。切换工作区时，文件列表和编辑会话一起切换，避免不同项目的文稿混在同一个侧栏。

本地工作区直接读写用户选择的目录。远端工作区使用本地镜像目录编辑，再通过存储插件执行“拉取”或“推送”。编辑器只调用统一的工作区接口，不感知 GitHub、对象存储或 SFTP 的网络细节。

存储插件不是 WebAssembly。Go 代码会编译成独立的 `mory-storage` 侧车程序。Windows 使用 `mory-storage.exe`。Web 编辑器通过宿主桥发出请求，Electron 或 Swift 宿主再启动侧车程序。宿主通过标准输入写入 JSON，并从标准输出读取 JSON 结果。该结构避免把 Go 代码和存储凭证放入 JavaScript 页面。

```text
Web 编辑器
    │ 宿主请求
    ▼
Electron / Swift 宿主
    │ JSON：stdin / stdout
    ▼
mory-storage / mory-storage.exe
    │ 官方 SDK 或主流客户端库
    ├── GitHub
    ├── S3 / S4
    ├── OSS
    └── SFTP
```

```text
共享编辑器
    ↓
工作区接口：列出文稿、导入图片、拉取、推送
    ↓
本地镜像目录
    ├── Local：直接读写
    ├── GitHub：go-github + Git Database API
    ├── S3 / S4：S3 API
    ├── OSS：阿里云 OSS API
    └── SFTP：SSH File Transfer Protocol
```

远端同步只新增或更新文件，不自动删除远端对象。该规则防止一次误操作清空仓库或 Bucket。需要删除文件时，应在对应服务端确认后执行。

## 图片目录

图片按文稿名称建立目录。文稿和图片使用相对路径：

```text
工作区/
├── 发布说明.md
└── 发布说明/
    ├── 封面.png
    └── 架构图.webp
```

Markdown 中写入以下内容：

```markdown
![封面](发布说明/封面.png)
```

把图片粘贴或拖入编辑器时，宿主会完成以下操作：

1. 清理文件名中的路径字符和空白。
2. 把图片写入文稿同名目录。
3. 文件重名时追加数字序号。
4. 在 Markdown 中插入相对路径。
5. 导出时把本地图片转为 Data URL，保证 HTML、PDF 和长图可以独立使用。

未命名文稿先使用“未命名”目录。文稿首次保存后，Mory 会把图片目录改为正式文稿名，并同步更新 Markdown 路径。

## 设置存储插件

打开“偏好设置 > 工作区与存储”，可以新增多个工作区。同一种插件可以保存多套独立连接配置。

### 本地目录

本地工作区只需要选择工作目录。Mory 首次启动时默认创建“文稿/Mory”目录。用户可以随时改为其他目录。

### GitHub

GitHub 工作区包含以下配置：

| 字段 | 说明 |
| --- | --- |
| 仓库 | 使用 `owner/repository` 格式 |
| 分支 | 默认为 `main` |
| API 地址 | GitHub.com 默认为 `https://api.github.com`；GitHub Enterprise 可填写专用地址 |
| 仓库内目录 | 可选，只同步指定目录 |
| Access Token | 需要仓库 Contents 读写权限 |

Mory 使用 [`google/go-github`](https://github.com/google/go-github) 客户端和 Git Database API。拉取时先读取一次仓库树，只下载内容发生变化的 Blob。推送时先比较 Git Blob SHA，只上传新增或变化的文件，再把全部变化合并为一个 Tree 和一个 Commit。未变化的工作区只需要读取分支和树，不会逐文件提交。

该方式减少 Token 请求次数和提交数量，并保留 GitHub 客户端的主速率限制检测。遇到分支并发更新时，Mory 不强制覆盖远端分支，而是返回冲突。建议使用仅授予目标仓库 Contents 读写权限的 Fine-grained Access Token。GitHub 的主速率限制、次级速率限制和条件请求规则见 [REST API 最佳实践](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api)与[速率限制说明](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)。

### S3 和 S4

S3 与 S4 工作区包含 Endpoint、Region、Bucket、路径前缀、Access Key ID、Secret Access Key 和可选 Session Token。

S3 可以不填 Endpoint，此时使用 AWS 区域端点。S4 在当前实现中表示 S3 兼容服务，必须填写 Endpoint。实现使用 [AWS SDK for Go v2](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3)，支持标准 AWS S3 和使用路径式寻址的兼容服务。

### 阿里云 OSS

OSS 工作区包含 Endpoint、Region、Bucket、路径前缀、AccessKey ID、AccessKey Secret 和可选 Security Token。实现使用[阿里云 OSS SDK for Go v2](https://pkg.go.dev/github.com/aliyun/alibabacloud-oss-go-sdk-v2)。

### SFTP Server

SFTP 工作区包含服务器、端口、用户名、密码或私钥、远端目录和 `known_hosts` 文件路径。端口默认为 `22`。

私钥字段可以填写 PEM 私钥内容，也可以填写私钥文件路径。`known_hosts` 留空时，Mory 使用 `~/.ssh/known_hosts`。实现使用 [`pkg/sftp`](https://pkg.go.dev/github.com/pkg/sftp) 和 Go SSH 客户端。

## 凭证存储

每个工作区保存自己的凭证。渲染页面只能获得“已配置”状态，不能读取 Token、Secret、密码或私钥原文。凭证不会写入文稿目录、导出文件和同步日志。

宿主把工作区配置保存在应用数据目录中。macOS 文件权限设置为仅当前用户可读写；Windows 使用当前用户的应用数据目录。删除工作区配置不会删除本地工作目录或远端文件。

## 同步操作

“拉取”把远端文件更新到本地镜像。“推送”把本地文件新增或更新到远端。同步完成后，Mory 会重新扫描工作区并刷新侧栏。

建议先拉取，再编辑和推送。当前同步不自动处理同一文件的双向内容冲突，也不自动删除文件。GitHub 会返回提交冲突；对象存储和 SFTP 会以上传内容覆盖同名远端文件。

GitHub 同步按用户操作触发，不在后台轮询。重复推送未变化的工作区不会创建 Blob、Tree 或 Commit。

## 插件侧车

跨平台存储插件位于 `cmd/mory-storage` 和 `internal/storage`。macOS 与 Windows 打包时都会生成对应架构的 `mory-storage` 可执行文件。

侧车通过标准输入接收 JSON，通过标准输出返回结果。宿主不会把凭证放在命令行参数中，因此凭证不会出现在进程列表。新增存储后端时，需要实现统一的 `Pull` 和 `Push` 接口，并在设置页声明连接字段。

## GitHub 客户端选型记录

访问日期：2026-08-16。

检索关键词：

- `google go-github GitHub API client Go latest release`
- `GitHub REST Git trees create commit update reference`
- `GitHub REST API rate limits conditional requests`

采用 `google/go-github` v89.0.0。该版本提供 Git Blob、Tree、Commit、Reference 和速率限制错误模型，与项目的 Go 1.25 模块基线兼容。GitHub 官方 Git Database API 文档明确支持“创建树、创建提交、更新引用”的批量提交链路，因此用它替换逐文件 Contents API 写入。

最后验证日期：2026-08-16。
