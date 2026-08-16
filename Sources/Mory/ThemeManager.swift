import AppKit
import CryptoKit
import Foundation

final class ThemeManager {
    private let fileManager = FileManager.default
    let directory: URL

    init() throws {
        let support = try fileManager.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
        directory = support.appendingPathComponent("Mory/themes", isDirectory: true)
        try fileManager.createDirectory(at: directory, withIntermediateDirectories: true)
    }

    func list() throws -> [[String: String]] {
        let files = try fileManager.contentsOfDirectory(at: directory, includingPropertiesForKeys: [.fileSizeKey, .isRegularFileKey])
            .filter { $0.pathExtension.lowercased() == "css" }
            .sorted { $0.lastPathComponent.localizedStandardCompare($1.lastPathComponent) == .orderedAscending }
        return try files.compactMap { file in
            let values = try file.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey])
            guard values.isRegularFile == true, (values.fileSize ?? 0) <= 1024 * 1024 else { return nil }
            let css = try inlineAssets(in: String(contentsOf: file, encoding: .utf8), base: directory)
            return ["id": themeID(file.lastPathComponent), "name": file.deletingPathExtension().lastPathComponent,
                    "filename": file.lastPathComponent, "css": css]
        }
    }

    func importFile(_ source: URL) throws -> [[String: String]] {
        guard source.pathExtension.lowercased() == "css" else { throw workspaceError("请选择 CSS 主题文件。") }
        let values = try source.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey])
        guard values.isRegularFile == true, (values.fileSize ?? 0) <= 1024 * 1024 else {
            throw workspaceError("主题文件无效或超过 1 MB。")
        }
        let destination = directory.appendingPathComponent(source.lastPathComponent)
        if source.standardizedFileURL != destination.standardizedFileURL {
            if fileManager.fileExists(atPath: destination.path) { try fileManager.removeItem(at: destination) }
            try fileManager.copyItem(at: source, to: destination)
        }
        return try list()
    }

    private func themeID(_ filename: String) -> String {
        let base = URL(fileURLWithPath: filename).deletingPathExtension().lastPathComponent
            .lowercased().replacingOccurrences(of: #"[^\p{L}\p{N}-]+"#, with: "-", options: .regularExpression)
            .trimmingCharacters(in: CharacterSet(charactersIn: "-"))
        let digest = SHA256.hash(data: Data(filename.utf8)).prefix(4).map { String(format: "%02x", $0) }.joined()
        return "user-\(base.isEmpty ? "theme" : String(base.prefix(48)))-\(digest)"
    }

    private func inlineAssets(in css: String, base: URL) throws -> String {
        let expression = try NSRegularExpression(pattern: #"url\(\s*([\"']?)([^\"')]+)\1\s*\)"#, options: .caseInsensitive)
        var result = css
        var total = 0
        for match in expression.matches(in: css, range: NSRange(css.startIndex..., in: css)).reversed() {
            guard let referenceRange = Range(match.range(at: 2), in: css), let fullRange = Range(match.range, in: css) else { continue }
            let reference = String(css[referenceRange]).trimmingCharacters(in: .whitespacesAndNewlines)
            if reference.range(of: #"^(data:|https?:|file:|#|/)"#, options: [.regularExpression, .caseInsensitive]) != nil { continue }
            let asset = base.appendingPathComponent(reference.removingPercentEncoding ?? reference).standardizedFileURL
            guard asset.path.hasPrefix(base.standardizedFileURL.path + "/"), fileManager.fileExists(atPath: asset.path) else { continue }
            let data = try Data(contentsOf: asset)
            total += data.count
            if total > 5 * 1024 * 1024 { throw workspaceError("主题资源总大小不能超过 5 MB。") }
            let mimeTypes: [String: String] = [
                "png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg", "gif": "image/gif", "webp": "image/webp",
                "svg": "image/svg+xml", "woff": "font/woff", "woff2": "font/woff2", "ttf": "font/ttf", "otf": "font/otf"
            ]
            let mime = mimeTypes[asset.pathExtension.lowercased()] ?? "application/octet-stream"
            result.replaceSubrange(fullRange, with: "url(\"data:\(mime);base64,\(data.base64EncodedString())\")")
        }
        return result
    }
}
