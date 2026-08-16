import Foundation

struct WorkspaceConfig: Codable {
    var id: String
    var name: String
    var provider: String
    var localPath: String?
    var endpoint: String?
    var region: String?
    var bucket: String?
    var prefix: String?
    var accessKeyId: String?
    var accessKeySecret: String?
    var sessionToken: String?
    var repository: String?
    var branch: String?
    var token: String?
    var host: String?
    var port: Int?
    var username: String?
    var password: String?
    var privateKey: String?
    var knownHosts: String?
    var remotePath: String?

    init(dictionary: [String: Any], existing: WorkspaceConfig? = nil) {
        id = dictionary["id"] as? String ?? existing?.id ?? UUID().uuidString
        name = dictionary["name"] as? String ?? existing?.name ?? "工作区"
        provider = dictionary["provider"] as? String ?? existing?.provider ?? "local"
        localPath = dictionary["localPath"] as? String ?? existing?.localPath
        endpoint = dictionary["endpoint"] as? String ?? existing?.endpoint
        region = dictionary["region"] as? String ?? existing?.region
        bucket = dictionary["bucket"] as? String ?? existing?.bucket
        prefix = dictionary["prefix"] as? String ?? existing?.prefix
        accessKeyId = dictionary["accessKeyId"] as? String ?? existing?.accessKeyId
        accessKeySecret = dictionary["accessKeySecret"] as? String ?? existing?.accessKeySecret
        sessionToken = dictionary["sessionToken"] as? String ?? existing?.sessionToken
        repository = dictionary["repository"] as? String ?? existing?.repository
        branch = dictionary["branch"] as? String ?? existing?.branch
        token = dictionary["token"] as? String ?? existing?.token
        host = dictionary["host"] as? String ?? existing?.host
        port = (dictionary["port"] as? NSNumber)?.intValue ?? existing?.port
        username = dictionary["username"] as? String ?? existing?.username
        password = dictionary["password"] as? String ?? existing?.password
        privateKey = dictionary["privateKey"] as? String ?? existing?.privateKey
        knownHosts = dictionary["knownHosts"] as? String ?? existing?.knownHosts
        remotePath = dictionary["remotePath"] as? String ?? existing?.remotePath
    }

    func validate() throws {
        switch provider {
        case "local":
            guard let localPath, !localPath.isEmpty else { throw workspaceError("请选择本地工作目录。") }
        case "github":
            guard repository?.split(separator: "/").count == 2, !(token ?? "").isEmpty else {
                throw workspaceError("GitHub 工作区需要 owner/repo 格式的仓库和 Access Token。")
            }
        case "s3", "s4", "oss":
            guard !(region ?? "").isEmpty, !(bucket ?? "").isEmpty,
                  !(accessKeyId ?? "").isEmpty, !(accessKeySecret ?? "").isEmpty else {
                throw workspaceError("对象存储需要区域、Bucket、Access Key 和 Secret Key。")
            }
            if provider == "s4", (endpoint ?? "").isEmpty { throw workspaceError("S4 / S3 兼容存储需要 Endpoint。") }
        case "sftp":
            guard !(host ?? "").isEmpty, !(username ?? "").isEmpty, !(remotePath ?? "").isEmpty,
                  !(password ?? "").isEmpty || !(privateKey ?? "").isEmpty else {
                throw workspaceError("SFTP 需要服务器、用户名、远端目录，以及密码或私钥。")
            }
        default:
            throw workspaceError("不支持的存储插件：\(provider)")
        }
    }

    func publicDictionary(root: URL) -> [String: Any] {
        var result = dictionary(includeSecrets: false)
        result["localPath"] = root.path
        result["tokenConfigured"] = !(token ?? "").isEmpty
        result["accessKeySecretConfigured"] = !(accessKeySecret ?? "").isEmpty
        result["sessionTokenConfigured"] = !(sessionToken ?? "").isEmpty
        result["passwordConfigured"] = !(password ?? "").isEmpty
        result["privateKeyConfigured"] = !(privateKey ?? "").isEmpty
        return result
    }

    func dictionary(includeSecrets: Bool = true) -> [String: Any] {
        var result: [String: Any] = ["id": id, "name": name, "provider": provider]
        let values: [(String, Any?)] = [
            ("localPath", localPath), ("endpoint", endpoint), ("region", region), ("bucket", bucket),
            ("prefix", prefix), ("accessKeyId", accessKeyId), ("repository", repository), ("branch", branch),
            ("host", host), ("port", port), ("username", username), ("knownHosts", knownHosts), ("remotePath", remotePath)
        ]
        for (key, value) in values where value != nil { result[key] = value }
        if includeSecrets {
            let secrets: [(String, String?)] = [("accessKeySecret", accessKeySecret), ("sessionToken", sessionToken), ("token", token), ("password", password), ("privateKey", privateKey)]
            for (key, value) in secrets where !(value ?? "").isEmpty { result[key] = value }
        }
        return result
    }
}

private struct WorkspaceStore: Codable {
    var version = 1
    var activeId: String
    var workspaces: [WorkspaceConfig]
}

final class WorkspaceManager: @unchecked Sendable {
    private let fileManager = FileManager.default
    private let supportRoot: URL
    private let configURL: URL
    private let cacheRoot: URL
    private(set) var workspaces: [WorkspaceConfig] = []
    private(set) var activeId = ""

    init() throws {
        let applicationSupport = try fileManager.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
        supportRoot = applicationSupport.appendingPathComponent("Mory", isDirectory: true)
        configURL = supportRoot.appendingPathComponent("workspaces.json")
        cacheRoot = supportRoot.appendingPathComponent("workspaces", isDirectory: true)
        try fileManager.createDirectory(at: cacheRoot, withIntermediateDirectories: true)
        try load()
    }

    var active: WorkspaceConfig { workspaces.first { $0.id == activeId } ?? workspaces[0] }

    func root(for workspace: WorkspaceConfig) -> URL {
        if workspace.provider == "local", let path = workspace.localPath { return URL(fileURLWithPath: path, isDirectory: true) }
        return cacheRoot.appendingPathComponent(workspace.id, isDirectory: true)
    }

    var activeRoot: URL { root(for: active) }

    func state() -> [String: Any] {
        ["activeId": activeId, "workspaces": workspaces.map { $0.publicDictionary(root: root(for: $0)) }]
    }

    @discardableResult
    func save(_ dictionary: [String: Any]) throws -> [String: Any] {
        let existing = (dictionary["id"] as? String).flatMap { id in workspaces.first { $0.id == id } }
        var workspace = WorkspaceConfig(dictionary: dictionary, existing: existing)
        if workspace.name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            workspace.name = workspace.provider == "local" ? "本地工作区" : workspace.provider.uppercased()
        }
        try workspace.validate()
        if let index = workspaces.firstIndex(where: { $0.id == workspace.id }) { workspaces[index] = workspace }
        else { workspaces.append(workspace) }
        activeId = workspace.id
        try fileManager.createDirectory(at: root(for: workspace), withIntermediateDirectories: true)
        try persist()
        return state()
    }

    @discardableResult
    func activate(_ id: String) throws -> [String: Any] {
        guard let workspace = workspaces.first(where: { $0.id == id }) else { throw workspaceError("工作区不存在。") }
        activeId = id
        try fileManager.createDirectory(at: root(for: workspace), withIntermediateDirectories: true)
        try persist()
        return state()
    }

    @discardableResult
    func remove(_ id: String) throws -> [String: Any] {
        guard workspaces.count > 1 else { throw workspaceError("至少保留一个工作区。") }
        workspaces.removeAll { $0.id == id }
        if activeId == id { activeId = workspaces[0].id }
        try persist()
        return state()
    }

    func documents() throws -> [[String: Any]] {
        let keys: [URLResourceKey] = [.isRegularFileKey, .isDirectoryKey, .isHiddenKey, .creationDateKey, .contentModificationDateKey]
        guard let enumerator = fileManager.enumerator(at: activeRoot, includingPropertiesForKeys: keys, options: [.skipsHiddenFiles]) else { return [] }
        let extensions = Set(["md", "markdown", "mmd", "mdown", "mkd", "txt", "text"])
        var result: [[String: Any]] = []
        for case let url as URL in enumerator where extensions.contains(url.pathExtension.lowercased()) {
            let values = try url.resourceValues(forKeys: Set(keys))
            if values.isRegularFile == true {
                let relative = url.path.replacingOccurrences(of: activeRoot.path + "/", with: "")
                // creationDate 在 APFS、NTFS 等桌面文件系统可用；缺失时用元数据变更时间保持确定顺序。
                let createdAt = (values.creationDate ?? values.contentModificationDate ?? .distantFuture).timeIntervalSince1970 * 1_000
                result.append(["name": relative, "path": url.path, "createdAt": createdAt])
            }
        }
        return result.sorted {
            let leftTime = $0["createdAt"] as? Double ?? .greatestFiniteMagnitude
            let rightTime = $1["createdAt"] as? Double ?? .greatestFiniteMagnitude
            if leftTime != rightTime { return leftTime < rightTime }
            let leftName = $0["name"] as? String ?? ""
            let rightName = $1["name"] as? String ?? ""
            let nameOrder = leftName.localizedStandardCompare(rightName)
            if nameOrder != .orderedSame { return nameOrder == .orderedAscending }
            return ($0["path"] as? String ?? "") < ($1["path"] as? String ?? "")
        }
    }

    func documentContents() throws -> [[String: Any]] {
        try documents().compactMap { document in
            guard let path = document["path"] as? String, let attributes = try? fileManager.attributesOfItem(atPath: path),
                  ((attributes[.size] as? NSNumber)?.intValue ?? 0) <= 2 * 1024 * 1024,
                  let markdown = try? String(contentsOfFile: path, encoding: .utf8) else { return nil }
            var content = document
            content["name"] = document["name"] as? String ?? URL(fileURLWithPath: path).lastPathComponent
            content["markdown"] = markdown
            return content
        }
    }

    func importImage(arguments: [String: Any]) throws -> [String: String] {
        guard let mime = arguments["mime"] as? String, let fileExtension = imageExtension(for: mime),
              let encoded = arguments["data"] as? String, let data = Data(base64Encoded: encoded), !data.isEmpty, data.count <= 50 * 1024 * 1024 else {
            throw workspaceError("图片格式无效、为空或超过 50 MB。")
        }
        let documentName = arguments["documentName"] as? String ?? "未命名.md"
        let documentBase = sanitize(URL(fileURLWithPath: documentName).deletingPathExtension().lastPathComponent)
        let documentPath = arguments["documentPath"] as? String ?? ""
        let documentDirectory = documentPath.isEmpty ? activeRoot : URL(fileURLWithPath: documentPath).deletingLastPathComponent()
        let directory = documentDirectory.appendingPathComponent(documentBase, isDirectory: true)
        try fileManager.createDirectory(at: directory, withIntermediateDirectories: true)
        let sourceName = arguments["name"] as? String ?? "图片"
        let base = sanitize(URL(fileURLWithPath: sourceName).deletingPathExtension().lastPathComponent)
        var filename = base + fileExtension
        var serial = 2
        while fileManager.fileExists(atPath: directory.appendingPathComponent(filename).path) {
            filename = "\(base)-\(serial)\(fileExtension)"
            serial += 1
        }
        try data.write(to: directory.appendingPathComponent(filename), options: .atomic)
        return ["relative": "\(documentBase)/\(filename)", "dataURL": "data:\(mime);base64,\(data.base64EncodedString())"]
    }

    func relocateAssets(markdown: String, oldURL: URL?, oldName: String, newURL: URL) throws -> String {
        let oldBase = sanitize(URL(fileURLWithPath: oldName).deletingPathExtension().lastPathComponent)
        let newBase = sanitize(newURL.deletingPathExtension().lastPathComponent)
        let oldParent = oldURL?.deletingLastPathComponent() ?? activeRoot
        if oldBase == newBase && oldParent.standardizedFileURL == newURL.deletingLastPathComponent().standardizedFileURL { return markdown }
        let source = oldParent.appendingPathComponent(oldBase, isDirectory: true)
        let destination = newURL.deletingLastPathComponent().appendingPathComponent(newBase, isDirectory: true)
        guard fileManager.fileExists(atPath: source.path) else { return markdown }
        if !fileManager.fileExists(atPath: destination.path) {
            do { try fileManager.moveItem(at: source, to: destination) }
            catch {
                try fileManager.copyItem(at: source, to: destination)
            }
        } else if let enumerator = fileManager.enumerator(at: source, includingPropertiesForKeys: [.isRegularFileKey]) {
            for case let file as URL in enumerator {
                let values = try file.resourceValues(forKeys: [.isRegularFileKey])
                guard values.isRegularFile == true else { continue }
                let relative = file.path.replacingOccurrences(of: source.path + "/", with: "")
                let target = destination.appendingPathComponent(relative)
                if !fileManager.fileExists(atPath: target.path) {
                    try fileManager.createDirectory(at: target.deletingLastPathComponent(), withIntermediateDirectories: true)
                    try fileManager.copyItem(at: file, to: target)
                }
            }
        }
        return markdown.replacingOccurrences(of: "](\(oldBase)/", with: "](\(newBase)/")
    }

    func assets(for documentURL: URL, markdown: String) -> [String: String] {
        let expression = try? NSRegularExpression(pattern: #"!\[[^\]]*\]\(([^\s)]+)"#)
        let source = markdown as NSString
        var assets: [String: String] = [:]
        expression?.enumerateMatches(in: markdown, range: NSRange(location: 0, length: source.length)) { match, _, _ in
            guard let range = match?.range(at: 1), range.location != NSNotFound else { return }
            let relative = source.substring(with: range).removingPercentEncoding ?? source.substring(with: range)
            guard !relative.contains("://"), !relative.hasPrefix("data:"), !relative.hasPrefix("/") else { return }
            let fileURL = documentURL.deletingLastPathComponent().appendingPathComponent(relative)
            guard let data = try? Data(contentsOf: fileURL) else { return }
            assets[relative.replacingOccurrences(of: "\\", with: "/")] = "data:\(mimeType(for: fileURL));base64,\(data.base64EncodedString())"
        }
        return assets
    }

    func sync(action: String) throws -> [String: Any] {
        guard active.provider != "local" else { return ["files": 0, "bytes": 0, "local": true] }
        let executable = sidecarURL()
        guard fileManager.isExecutableFile(atPath: executable.path) else { throw workspaceError("存储插件侧车不存在，请重新安装 Mory。") }
        let request: [String: Any] = ["action": action, "root": activeRoot.path, "workspace": active.dictionary()]
        let input = try JSONSerialization.data(withJSONObject: request)
        let process = Process()
        let stdin = Pipe(), stdout = Pipe(), stderr = Pipe()
        process.executableURL = executable
        process.standardInput = stdin
        process.standardOutput = stdout
        process.standardError = stderr
        try process.run()
        try stdin.fileHandleForWriting.write(contentsOf: input)
        try stdin.fileHandleForWriting.close()
        process.waitUntilExit()
        let output = stdout.fileHandleForReading.readDataToEndOfFile()
        guard let response = try JSONSerialization.jsonObject(with: output) as? [String: Any], response["ok"] as? Bool == true else {
            let response = (try? JSONSerialization.jsonObject(with: output) as? [String: Any])?["error"] as? String
            throw workspaceError(response ?? "存储插件执行失败。")
        }
        return response["summary"] as? [String: Any] ?? [:]
    }

    private func load() throws {
        if let data = try? Data(contentsOf: configURL), let store = try? JSONDecoder().decode(WorkspaceStore.self, from: data), !store.workspaces.isEmpty {
            workspaces = store.workspaces
            activeId = workspaces.contains { $0.id == store.activeId } ? store.activeId : workspaces[0].id
        } else {
            let documents = fileManager.urls(for: .documentDirectory, in: .userDomainMask).first ?? URL(fileURLWithPath: NSHomeDirectory())
            let root = documents.appendingPathComponent("Mory", isDirectory: true)
            try fileManager.createDirectory(at: root, withIntermediateDirectories: true)
            let workspace = WorkspaceConfig(dictionary: ["name": "本地工作区", "provider": "local", "localPath": root.path])
            workspaces = [workspace]
            activeId = workspace.id
            try persist()
        }
        try fileManager.createDirectory(at: activeRoot, withIntermediateDirectories: true)
    }

    private func persist() throws {
        try fileManager.createDirectory(at: supportRoot, withIntermediateDirectories: true)
        let data = try JSONEncoder().encode(WorkspaceStore(activeId: activeId, workspaces: workspaces))
        try data.write(to: configURL, options: [.atomic])
        try fileManager.setAttributes([.posixPermissions: 0o600], ofItemAtPath: configURL.path)
    }

    private func sidecarURL() -> URL {
        let packaged = Bundle.main.resourceURL?.appendingPathComponent("storage/mory-storage")
        if let packaged, fileManager.fileExists(atPath: packaged.path) { return packaged }
        return URL(fileURLWithPath: fileManager.currentDirectoryPath).appendingPathComponent(".build/storage/mory-storage")
    }
}

func workspaceError(_ message: String) -> NSError {
    NSError(domain: "Mory.Workspace", code: 1, userInfo: [NSLocalizedDescriptionKey: message])
}

private func sanitize(_ value: String) -> String {
    let forbidden = CharacterSet(charactersIn: "<>:\"/\\|?* ").union(.controlCharacters).union(.whitespacesAndNewlines)
    let cleaned = value.components(separatedBy: forbidden).filter { !$0.isEmpty }.joined(separator: "-").trimmingCharacters(in: CharacterSet(charactersIn: ". "))
    return cleaned.isEmpty ? "未命名" : cleaned
}

private func imageExtension(for mime: String) -> String? {
    ["image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg", "image/bmp": ".bmp"][mime]
}

private func mimeType(for url: URL) -> String {
    ["png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg", "gif": "image/gif", "webp": "image/webp", "svg": "image/svg+xml", "bmp": "image/bmp"][url.pathExtension.lowercased()] ?? "application/octet-stream"
}
