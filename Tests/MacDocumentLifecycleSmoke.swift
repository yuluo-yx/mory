import AppKit
import WebKit

@MainActor
@main
final class MacDocumentLifecycleSmoke: NSObject, WKNavigationDelegate, WKScriptMessageHandler {
    private var webView: WKWebView!
    private var window: NSWindow!

    static func main() {
        do { try verifyHostUtilities() }
        catch { fputs("Native save contract failed: \(error)\n", stderr); Darwin.exit(1) }
        let application = NSApplication.shared
        application.setActivationPolicy(.accessory)
        let runner = MacDocumentLifecycleSmoke()
        application.finishLaunching()
        runner.start()
        withExtendedLifetime(runner) { application.run() }
    }

    private static func verifyHostUtilities() throws {
        let manager = FileManager.default
        let root = URL(fileURLWithPath: manager.currentDirectoryPath).appendingPathComponent(".cache/mac-save-\(UUID().uuidString)", isDirectory: true)
        defer { try? manager.removeItem(at: root) }
        let support = root.appendingPathComponent("support", isDirectory: true)
        try manager.createDirectory(at: support, withIntermediateDirectories: true)
        let active = root.appendingPathComponent("active", isDirectory: true)
        let config = ["id": "test", "name": "Test", "provider": "local", "localPath": active.path]
        try JSONSerialization.data(withJSONObject: ["activeId": "test", "workspaces": [config]]).write(to: support.appendingPathComponent("workspaces.json"))
        let workspace = try WorkspaceManager(supportRoot: support)
        let original = root.appendingPathComponent("original", isDirectory: true)
        let destination = root.appendingPathComponent("output/copy.md")
        try manager.createDirectory(at: original, withIntermediateDirectories: true)
        try manager.createDirectory(at: destination.deletingLastPathComponent(), withIntermediateDirectories: true)
        let image = Data("image bytes".utf8)
        try image.write(to: original.appendingPathComponent("cover.png"))
        let markdown = "![cover](original/cover.png)"
        let saved = try workspace.relocateAssets(markdown: markdown, oldURL: nil, oldName: "original.md", newURL: destination, rootURL: root)
        guard saved == "![cover](copy/cover.png)",
              try Data(contentsOf: original.appendingPathComponent("cover.png")) == image,
              try Data(contentsOf: destination.deletingPathExtension().appendingPathComponent("cover.png")) == image else {
            throw workspaceError("Save As did not retain both image directories")
        }
        var collisionRejected = false
        do { _ = try workspace.relocateAssets(markdown: markdown, oldURL: nil, oldName: "original.md", newURL: destination, rootURL: root) }
        catch { collisionRejected = true }
        guard collisionRejected, try Data(contentsOf: original.appendingPathComponent("cover.png")) == image else {
            throw workspaceError("A conflicting Save As modified the source images")
        }
        guard workspace.savedAssetPathChanges(oldName: "original.md", newURL: destination) == ["original/": "copy/"] else {
            throw workspaceError("Save As returned an incorrect image path mapping")
        }
        var payload: [String: Any] = ["documentId": "draft", "path": "", "name": "original.md", "markdown": markdown]
        let snapshot = DocumentSaveSnapshot(dictionary: payload, rootURL: root)
        payload["markdown"] = "later edit"
        guard snapshot?.documentID == "draft", snapshot?.sourceURL == nil, snapshot?.markdown == markdown, snapshot?.rootURL == root,
              DocumentSaveSnapshot(dictionary: [:], rootURL: root) == nil else {
            throw workspaceError("Document save snapshots lost identity or captured content")
        }
        print("macOS native save contracts passed: snapshot, copy, conflict, captured root, path mapping")
    }

    private func start() {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController.add(self, name: "mory")
        webView = WKWebView(frame: NSRect(x: 0, y: 0, width: 1180, height: 790), configuration: configuration)
        webView.navigationDelegate = self
        window = NSWindow(contentRect: webView.frame, styleMask: [.borderless], backing: .buffered, defer: false)
        window.contentView = webView
        let index = URL(fileURLWithPath: FileManager.default.currentDirectoryPath).appendingPathComponent("Sources/Mory/Web/index.html")
        webView.loadFileURL(index, allowingReadAccessTo: index.deletingLastPathComponent())
        DispatchQueue.main.asyncAfter(deadline: .now() + 45) { self.finish("Document lifecycle smoke timed out") }
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {}

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        Task { @MainActor in
            do {
                let source = try String(contentsOfFile: "Tests/document-lifecycle-cases.js", encoding: .utf8)
                let value = try await webView.callAsyncJavaScript(source + "\nreturn await runDocumentLifecycleCases()", arguments: [:], in: nil, contentWorld: .page)
                guard let result = value as? [String: Any], let failures = result["failures"] as? [[String: Any]], failures.isEmpty else {
                    finish("Document lifecycle failed: \(String(describing: value))")
                }
                print("macOS document lifecycle passed: \((result["passed"] as? [String] ?? []).count) scenarios")
                Darwin.exit(0)
            } catch { finish(error.localizedDescription) }
        }
    }

    private func finish(_ error: String) -> Never {
        fputs(error + "\n", stderr)
        Darwin.exit(1)
    }
}
