import AppKit
import UniformTypeIdentifiers
import WebKit

@MainActor
private final class ExportRenderer: NSObject, WKNavigationDelegate {
    private let format: String
    private let destination: URL
    private let imageWidth: CGFloat
    private let paper: String
    private let webView: WKWebView
    private let completion: (Result<Void, Error>) -> Void

    init(format: String, destination: URL, imageWidth: CGFloat, paper: String, completion: @escaping (Result<Void, Error>) -> Void) {
        self.format = format
        self.destination = destination
        self.imageWidth = imageWidth
        self.paper = paper
        self.completion = completion
        self.webView = WKWebView(frame: NSRect(x: 0, y: 0, width: imageWidth, height: 900))
        super.init()
        self.webView.navigationDelegate = self
        self.webView.setValue(false, forKey: "drawsBackground")
    }

    func start(html: String) {
        webView.loadHTMLString(html, baseURL: nil)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        completion(.failure(error))
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        completion(.failure(error))
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        if format == "pdf" {
            let printInfo = NSPrintInfo()
            printInfo.paperSize = paperSize(for: paper)
            printInfo.topMargin = 28
            printInfo.bottomMargin = 28
            printInfo.leftMargin = 32
            printInfo.rightMargin = 32
            printInfo.horizontalPagination = .fit
            printInfo.verticalPagination = .automatic
            printInfo.jobDisposition = .save
            let savingURLKey = NSPrintInfo.AttributeKey(rawValue: "NSPrintJobSavingURL")
            printInfo.dictionary()[savingURLKey] = destination
            let operation = webView.printOperation(with: printInfo)
            operation.showsPrintPanel = false
            operation.showsProgressPanel = false
            if operation.run() {
                completion(.success(()))
            } else {
                completion(.failure(NSError(domain: "Mory.Export", code: 4, userInfo: [NSLocalizedDescriptionKey: "PDF 打印任务失败。"])) )
            }
            return
        }

        webView.evaluateJavaScript("Math.max(document.documentElement.scrollHeight, document.body.scrollHeight)") { [weak self] value, error in
            guard let self else { return }
            if let error { completion(.failure(error)); return }
            let height = CGFloat((value as? NSNumber)?.doubleValue ?? 0)
            guard height > 0, height <= 28_000 else {
                completion(.failure(NSError(domain: "Mory.Export", code: 1, userInfo: [NSLocalizedDescriptionKey: "文档超过 28000 像素，请降低图片宽度或改用 PDF 导出。"]))); return
            }
            webView.frame = NSRect(x: 0, y: 0, width: imageWidth, height: height)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
                self?.captureImage(height: height)
            }
        }
    }

    private func captureImage(height: CGFloat) {
        let configuration = WKSnapshotConfiguration()
        configuration.rect = NSRect(x: 0, y: 0, width: imageWidth, height: height)
        configuration.afterScreenUpdates = true
        webView.takeSnapshot(with: configuration) { [format, destination, completion] image, error in
            if let error { completion(.failure(error)); return }
            guard let image,
                  let tiff = image.tiffRepresentation,
                  let bitmap = NSBitmapImageRep(data: tiff) else {
                completion(.failure(NSError(domain: "Mory.Export", code: 2, userInfo: [NSLocalizedDescriptionKey: "无法生成图片数据。"]))); return
            }
            let fileType: NSBitmapImageRep.FileType = format == "jpeg" ? .jpeg : .png
            let properties: [NSBitmapImageRep.PropertyKey: Any] = format == "jpeg" ? [.compressionFactor: 0.92] : [:]
            guard let data = bitmap.representation(using: fileType, properties: properties) else {
                completion(.failure(NSError(domain: "Mory.Export", code: 3, userInfo: [NSLocalizedDescriptionKey: "无法编码图片。"]))); return
            }
            do {
                try data.write(to: destination, options: .atomic)
                completion(.success(()))
            } catch {
                completion(.failure(error))
            }
        }
    }

    private func paperSize(for value: String) -> NSSize {
        switch value.lowercased() {
        case "letter": return NSSize(width: 612, height: 792)
        case "legal": return NSSize(width: 612, height: 1008)
        default: return NSSize(width: 595.28, height: 841.89)
        }
    }
}

@main
final class MoryApp: NSObject, NSApplicationDelegate, WKNavigationDelegate, WKScriptMessageHandler {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var currentFileURL: URL?
    private var currentMarkdown = ""
    private var currentDocumentName = "未命名.md"
    private var workspaceManager: WorkspaceManager!
    private var themeManager: ThemeManager!
    private var editorReady = false
    private var pendingDocument: [String: Any]?
    private var exportRenderers: [ExportRenderer] = []
    private var dragStartPointer: NSPoint?
    private var dragStartWindowOrigin: NSPoint?
    private var interfaceLocale = "zh-CN"

    static func main() {
        let application = NSApplication.shared
        let delegate = MoryApp()
        application.delegate = delegate
        application.setActivationPolicy(.regular)
        application.run()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        if let iconURL = Bundle.main.resourceURL?.appendingPathComponent("icon.png"),
           let icon = NSImage(contentsOf: iconURL) {
            NSApp.applicationIconImage = icon
        }
        configureMenu()
        configureWindow()
        do {
            workspaceManager = try WorkspaceManager()
            themeManager = try ThemeManager()
        } catch {
            presentError("无法初始化工作区：\(error.localizedDescription)")
            NSApp.terminate(nil)
            return
        }
        loadEditor()
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }

    func application(_ sender: NSApplication, openFiles filenames: [String]) {
        guard let first = filenames.first else { return }
        openFile(at: URL(fileURLWithPath: first))
        sender.reply(toOpenOrPrint: .success)
    }

    private func configureWindow() {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController.add(self, name: "mory")

        webView = WKWebView(frame: .zero, configuration: configuration)
        webView.navigationDelegate = self
        webView.setValue(false, forKey: "drawsBackground")

        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1180, height: 790),
            styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
            backing: .buffered,
            defer: false
        )
        window.title = "未命名 — Mory"
        window.titlebarAppearsTransparent = true
        window.titleVisibility = .hidden
        window.minSize = NSSize(width: 760, height: 520)
        window.contentView = webView
        window.center()
        window.makeKeyAndOrderFront(nil)
    }

    private func loadEditor() {
        let packagedURL = Bundle.main.resourceURL?
            .appendingPathComponent("Web", isDirectory: true)
            .appendingPathComponent("index.html", isDirectory: false)
        let resourceURL = packagedURL.flatMap { FileManager.default.fileExists(atPath: $0.path) ? $0 : nil }
            ?? Bundle.module.url(forResource: "index", withExtension: "html", subdirectory: "Web")
        guard let resourceURL else {
            presentError("找不到编辑器资源 Web/index.html")
            return
        }
        webView.loadFileURL(resourceURL, allowingReadAccessTo: resourceURL.deletingLastPathComponent())
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        webView.evaluateJavaScript("typeof window.Mory === 'object'") { [weak self] value, error in
            guard let self else { return }
            if error != nil || value as? Bool != true {
                presentError("编辑器脚本加载失败，请重新安装当前版本。")
                return
            }
            runWindowDragSmokeIfRequested()
        }
    }

    private func runWindowDragSmokeIfRequested() {
        guard ProcessInfo.processInfo.environment["MORY_DRAG_SMOKE"] == "1" else { return }
        // 托管运行器会把居中的窗口贴近屏幕上缘。先下移窗口，为向上拖动预留空间。
        let initial = window.frame.origin
        window.setFrameOrigin(NSPoint(x: initial.x, y: initial.y - 40))
        let origin = window.frame.origin
        let script = """
        window.webkit.messageHandlers.mory.postMessage({type:'windowDragStart',screenX:100,screenY:100});
        window.webkit.messageHandlers.mory.postMessage({type:'windowDragMove',screenX:136,screenY:82});
        window.webkit.messageHandlers.mory.postMessage({type:'windowDragEnd'});
        true;
        """
        webView.evaluateJavaScript(script) { [weak self] _, error in
            guard let self else { return }
            if let error {
                fputs("macOS 窗口拖动冒烟失败：\(error.localizedDescription)\n", stderr)
                Darwin.exit(1)
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) { [weak self] in
                guard let self else { return }
                let actual = window.frame.origin
                let passed = abs(actual.x - origin.x - 36) < 0.5 && abs(actual.y - origin.y - 18) < 0.5
                if passed {
                    print("macOS 窗口拖动冒烟通过：dx=36, dy=18")
                    NSApplication.shared.terminate(nil)
                } else {
                    fputs("macOS 窗口拖动冒烟失败：origin=\(origin), actual=\(actual)\n", stderr)
                    Darwin.exit(1)
                }
            }
        }
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        presentError("编辑器页面加载失败：\(error.localizedDescription)")
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        presentError("编辑器页面加载失败：\(error.localizedDescription)")
    }

    private func localizeMenu(_ menu: NSMenu) {
        guard interfaceLocale == "en" else { return }
        let translations = [
            "关于 Mory": "About Mory", "偏好设置…": "Preferences…", "退出 Mory": "Quit Mory", "文件": "File", "新建": "New",
            "打开…": "Open…", "打开文件夹…": "Open Folder…", "保存": "Save", "另存为…": "Save As…", "导出…": "Export…",
            "编辑": "Edit", "撤销": "Undo", "重做": "Redo", "剪切": "Cut", "复制": "Copy", "粘贴": "Paste", "全选": "Select All",
            "查找和替换": "Find and Replace", "格式": "Format", "加粗": "Bold", "斜体": "Italic", "删除线": "Strikethrough", "行内代码": "Inline Code",
            "显示": "View", "显示／隐藏侧边栏": "Show/Hide Sidebar", "源代码模式": "Source Mode", "专注模式": "Focus Mode",
            "打字机模式": "Typewriter Mode", "实际大小": "Actual Size", "放大": "Zoom In", "缩小": "Zoom Out"
        ]
        for item in menu.items {
            if let translated = translations[item.title] { item.title = translated }
            else if item.title.hasSuffix(" 级标题"), let level = item.title.split(separator: " ").first { item.title = "Heading \(level)" }
            if let submenu = item.submenu { localizeMenu(submenu) }
        }
    }

    private func configureMenu() {
        let main = NSMenu()

        let appItem = NSMenuItem()
        main.addItem(appItem)
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "关于 Mory", action: #selector(showAbout), keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "偏好设置…", action: #selector(showPreferences), keyEquivalent: ",")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "退出 Mory", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        appItem.submenu = appMenu

        let fileItem = NSMenuItem()
        main.addItem(fileItem)
        let fileMenu = NSMenu(title: "文件")
        fileMenu.addItem(withTitle: "新建", action: #selector(newDocument), keyEquivalent: "n")
        fileMenu.addItem(withTitle: "打开…", action: #selector(openDocument), keyEquivalent: "o")
        fileMenu.addItem(withTitle: "打开文件夹…", action: #selector(openFolder), keyEquivalent: "O").keyEquivalentModifierMask = [.command, .shift]
        fileMenu.addItem(.separator())
        fileMenu.addItem(withTitle: "保存", action: #selector(saveDocument), keyEquivalent: "s")
        fileMenu.addItem(withTitle: "另存为…", action: #selector(saveDocumentAs), keyEquivalent: "S").keyEquivalentModifierMask = [.command, .shift]
        fileMenu.addItem(.separator())
        fileMenu.addItem(withTitle: "导出…", action: #selector(showExportDialog), keyEquivalent: "e")
        fileItem.submenu = fileMenu

        let editItem = NSMenuItem()
        main.addItem(editItem)
        let editMenu = NSMenu(title: "编辑")
        editMenu.addItem(withTitle: "撤销", action: Selector(("undo:")), keyEquivalent: "z")
        editMenu.addItem(withTitle: "重做", action: Selector(("redo:")), keyEquivalent: "Z").keyEquivalentModifierMask = [.command, .shift]
        editMenu.addItem(.separator())
        editMenu.addItem(withTitle: "剪切", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        editMenu.addItem(withTitle: "复制", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "粘贴", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editMenu.addItem(withTitle: "全选", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        editMenu.addItem(.separator())
        editMenu.addItem(withTitle: "查找和替换", action: #selector(showFind), keyEquivalent: "f")
        editItem.submenu = editMenu

        let formatItem = NSMenuItem()
        main.addItem(formatItem)
        let formatMenu = NSMenu(title: "格式")
        formatMenu.addItem(withTitle: "加粗", action: #selector(toggleBold), keyEquivalent: "b")
        formatMenu.addItem(withTitle: "斜体", action: #selector(toggleItalic), keyEquivalent: "i")
        formatMenu.addItem(withTitle: "删除线", action: #selector(toggleStrike), keyEquivalent: "d").keyEquivalentModifierMask = [.command, .shift]
        formatMenu.addItem(withTitle: "行内代码", action: #selector(toggleCode), keyEquivalent: "`")
        formatMenu.addItem(.separator())
        for level in 1...6 {
            let item = NSMenuItem(title: "\(level) 级标题", action: #selector(applyHeading(_:)), keyEquivalent: String(level))
            item.tag = level
            item.keyEquivalentModifierMask = [.command, .option]
            formatMenu.addItem(item)
        }
        formatItem.submenu = formatMenu

        let viewItem = NSMenuItem()
        main.addItem(viewItem)
        let viewMenu = NSMenu(title: "显示")
        viewMenu.addItem(withTitle: "显示／隐藏侧边栏", action: #selector(toggleSidebar), keyEquivalent: "l").keyEquivalentModifierMask = [.command, .shift]
        viewMenu.addItem(withTitle: "源代码模式", action: #selector(toggleSource), keyEquivalent: "/")
        viewMenu.addItem(withTitle: "专注模式", action: #selector(toggleFocus), keyEquivalent: "f").keyEquivalentModifierMask = [.command, .shift]
        viewMenu.addItem(withTitle: "打字机模式", action: #selector(toggleTypewriter), keyEquivalent: "t").keyEquivalentModifierMask = [.command, .shift]
        viewMenu.addItem(.separator())
        viewMenu.addItem(withTitle: "实际大小", action: #selector(resetZoom), keyEquivalent: "0")
        viewMenu.addItem(withTitle: "放大", action: #selector(zoomIn), keyEquivalent: "+")
        viewMenu.addItem(withTitle: "缩小", action: #selector(zoomOut), keyEquivalent: "-")
        viewItem.submenu = viewMenu

        localizeMenu(main)
        NSApp.mainMenu = main
    }

    @objc private func newDocument() {
        currentFileURL = nil
        currentMarkdown = ""
        currentDocumentName = "未命名.md"
        window.representedURL = nil
        window.title = "未命名 — Mory"
        runJavaScript("window.Mory.newDocument()")
    }

    @objc private func openDocument() {
        let panel = NSOpenPanel()
        panel.allowedContentTypes = [.plainText, UTType(filenameExtension: "md") ?? .plainText]
        panel.allowsMultipleSelection = false
        guard panel.runModal() == .OK, let url = panel.url else { return }
        openFile(at: url)
    }

    private func openFile(at url: URL) {
        do {
            let markdown = try String(contentsOf: url, encoding: .utf8)
            currentFileURL = url
            currentMarkdown = markdown
            currentDocumentName = url.lastPathComponent
            window.representedURL = url
            window.title = url.lastPathComponent
            sendDocument(markdown: markdown, url: url)
        } catch {
            presentError("无法打开文件：\(error.localizedDescription)")
        }
    }

    @objc private func saveDocument() {
        guard let url = currentFileURL else {
            saveDocumentAs()
            return
        }
        writeDocument(to: url)
    }

    @objc private func saveDocumentAs() {
        let panel = NSSavePanel()
        panel.allowedContentTypes = [UTType(filenameExtension: "md") ?? .plainText]
        panel.nameFieldStringValue = currentFileURL?.lastPathComponent ?? currentDocumentName
        if currentFileURL == nil { panel.directoryURL = workspaceManager.activeRoot }
        guard panel.runModal() == .OK, let url = panel.url else { return }
        writeDocument(to: url)
    }

    private func writeDocument(to url: URL) {
        fetchMarkdown { [weak self] markdown in
            guard let self else { return }
            do {
                let updatedMarkdown = try workspaceManager.relocateAssets(markdown: markdown, oldURL: currentFileURL, oldName: currentDocumentName, newURL: url)
                try updatedMarkdown.write(to: url, atomically: true, encoding: .utf8)
                currentMarkdown = updatedMarkdown
                currentFileURL = url
                currentDocumentName = url.lastPathComponent
                window.representedURL = url
                window.title = url.lastPathComponent
                window.isDocumentEdited = false
                sendJSON(function: "window.Mory.didSave", value: [
                    "path": url.path,
                    "name": url.lastPathComponent,
                    "markdown": updatedMarkdown,
                    "assets": workspaceManager.assets(for: url, markdown: updatedMarkdown)
                ])
                refreshWorkspace()
            } catch {
                presentError("无法保存文件：\(error.localizedDescription)")
            }
        }
    }

    @objc private func openFolder() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        guard panel.runModal() == .OK, let folder = panel.url else { return }

        do {
            _ = try workspaceManager.save(["name": folder.lastPathComponent, "provider": "local", "localPath": folder.path])
            refreshWorkspace()
        } catch {
            presentError("无法设置工作区：\(error.localizedDescription)")
        }
    }

    @objc private func showExportDialog() {
        runJavaScript("window.Mory.toggleExport()")
    }

    private func exportDocument(options: [String: Any]) {
        let format = options["format"] as? String ?? "html"
        let fileExtension = format == "jpeg" ? "jpg" : format
        let panel = NSSavePanel()
        panel.allowedContentTypes = [UTType(filenameExtension: fileExtension) ?? .data]
        panel.nameFieldStringValue = (currentFileURL?.deletingPathExtension().lastPathComponent ?? "未命名") + ".\(fileExtension)"
        guard panel.runModal() == .OK, let url = panel.url else { return }

        Task { @MainActor [weak self] in
            guard let self else { return }
            let value: Any?
            do {
                value = try await webView.callAsyncJavaScript(
                    "return await window.Mory.exportDocument(options)",
                    arguments: ["options": options],
                    in: nil,
                    contentWorld: .page
                )
            } catch {
                presentError("无法生成导出文档：\(error.localizedDescription)")
                return
            }
            guard let html = value as? String else {
                presentError("无法生成导出文档")
                return
            }

            if format == "html" {
                do {
                    try html.write(to: url, atomically: true, encoding: .utf8)
                    runJavaScript("window.Mory.didExport('html')")
                } catch {
                    presentError("无法导出 HTML：\(error.localizedDescription)")
                }
                return
            }

            let width = CGFloat(options["width"] as? Double ?? 900)
            var renderer: ExportRenderer!
            renderer = ExportRenderer(format: format, destination: url, imageWidth: width, paper: options["paper"] as? String ?? "A4") { [weak self] result in
                guard let self else { return }
                exportRenderers.removeAll { $0 === renderer }
                switch result {
                case .success:
                    runJavaScript("window.Mory.didExport('\(format)')")
                case .failure(let error):
                    presentError("导出失败：\(error.localizedDescription)")
                }
            }
            exportRenderers.append(renderer)
            renderer.start(html: html)
        }
    }

    private func sendDocument(markdown: String, url: URL) {
        let document: [String: Any] = [
            "markdown": markdown,
            "path": url.path,
            "name": url.lastPathComponent,
            "assets": workspaceManager.assets(for: url, markdown: markdown)
        ]
        guard editorReady else {
            pendingDocument = document
            return
        }
        sendJSON(function: "window.Mory.openDocument", value: document)
    }

    private func fetchMarkdown(completion: @escaping (String) -> Void) {
        webView.evaluateJavaScript("window.Mory.getMarkdown()") { [weak self] value, _ in
            let markdown = value as? String ?? self?.currentMarkdown ?? ""
            completion(markdown)
        }
    }

    private func sendJSON(function: String, value: Any) {
        guard let data = try? JSONSerialization.data(withJSONObject: [value]),
              let encoded = String(data: data, encoding: .utf8) else { return }
        let argument = String(encoded.dropFirst().dropLast())
        runJavaScript("\(function)(\(argument))")
    }

    private func runJavaScript(_ source: String) {
        webView.evaluateJavaScript(source, completionHandler: nil)
    }

    private func refreshWorkspace() {
        do {
            sendJSON(function: "window.Mory.setFiles", value: try workspaceManager.documents())
            sendJSON(function: "window.Mory.setWorkspaceState", value: workspaceManager.state())
        } catch {
            presentError("无法读取工作区：\(error.localizedDescription)")
        }
    }

    private func answerHostRequest(id: String, result: Any? = nil, error: Error? = nil) {
        var payload: [String: Any] = ["requestId": id]
        if let error { payload["error"] = error.localizedDescription }
        else { payload["result"] = result ?? NSNull() }
        sendJSON(function: "window.Mory.resolveHostRequest", value: payload)
    }

    private func handleHostRequest(id: String, method: String, arguments: [String: Any]) {
        do {
            switch method {
            case "workspaceState":
                answerHostRequest(id: id, result: workspaceManager.state())
            case "chooseLocalWorkspace":
                let panel = NSOpenPanel()
                panel.canChooseFiles = false
                panel.canChooseDirectories = true
                panel.canCreateDirectories = true
                guard panel.runModal() == .OK, let folder = panel.url else {
                    answerHostRequest(id: id, result: ["canceled": true])
                    return
                }
                var value: [String: Any] = [
                    "name": arguments["name"] as? String ?? folder.lastPathComponent,
                    "provider": "local",
                    "localPath": folder.path
                ]
                if let workspaceId = arguments["id"] as? String { value["id"] = workspaceId }
                answerHostRequest(id: id, result: try workspaceManager.save(value))
                refreshWorkspace()
            case "saveWorkspace":
                guard let workspace = arguments["workspace"] as? [String: Any] else { throw workspaceError("工作区配置无效。") }
                answerHostRequest(id: id, result: try workspaceManager.save(workspace))
                refreshWorkspace()
            case "activateWorkspace":
                answerHostRequest(id: id, result: try workspaceManager.activate(arguments["id"] as? String ?? ""))
                refreshWorkspace()
            case "removeWorkspace":
                answerHostRequest(id: id, result: try workspaceManager.remove(arguments["id"] as? String ?? ""))
                refreshWorkspace()
            case "importImage":
                answerHostRequest(id: id, result: try workspaceManager.importImage(arguments: arguments))
            case "workspaceDocuments":
                answerHostRequest(id: id, result: try workspaceManager.documentContents())
            case "listThemes":
                answerHostRequest(id: id, result: try themeManager.list())
            case "importTheme":
                let panel = NSOpenPanel()
                panel.canChooseFiles = true
                panel.canChooseDirectories = false
                panel.allowsMultipleSelection = false
                panel.allowedContentTypes = [UTType(filenameExtension: "css") ?? .text]
                guard panel.runModal() == .OK, let source = panel.url else {
                    answerHostRequest(id: id, result: ["canceled": true])
                    return
                }
                answerHostRequest(id: id, result: ["themes": try themeManager.importFile(source)])
            case "openThemeFolder":
                NSWorkspace.shared.open(themeManager.directory)
                answerHostRequest(id: id, result: ["opened": true])
            case "syncWorkspace":
                let action = arguments["action"] as? String == "push" ? "push" : "pull"
                let manager = workspaceManager!
                DispatchQueue.global(qos: .userInitiated).async { [weak self] in
                    do {
                        let summary = try manager.sync(action: action)
                        DispatchQueue.main.async { self?.answerHostRequest(id: id, result: summary); self?.refreshWorkspace() }
                    } catch {
                        DispatchQueue.main.async { self?.answerHostRequest(id: id, error: error) }
                    }
                }
            default:
                throw workspaceError("未知宿主请求：\(method)")
            }
        } catch {
            answerHostRequest(id: id, error: error)
        }
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard let payload = message.body as? [String: Any], let type = payload["type"] as? String else { return }
        switch type {
        case "ready":
            editorReady = true
            refreshWorkspace()
            if let pendingDocument {
                sendJSON(function: "window.Mory.openDocument", value: pendingDocument)
                self.pendingDocument = nil
            }
        case "changed":
            currentMarkdown = payload["markdown"] as? String ?? currentMarkdown
            currentDocumentName = payload["name"] as? String ?? currentDocumentName
            window.isDocumentEdited = true
        case "documentSelected":
            if let path = payload["path"] as? String, !path.isEmpty {
                currentFileURL = URL(fileURLWithPath: path)
                window.representedURL = currentFileURL
            } else {
                currentFileURL = nil
                window.representedURL = nil
            }
            currentMarkdown = payload["markdown"] as? String ?? ""
            currentDocumentName = payload["name"] as? String ?? "未命名.md"
            window.title = currentFileURL?.lastPathComponent ?? "\(currentDocumentName.replacingOccurrences(of: ".md", with: "")) — Mory"
            window.isDocumentEdited = payload["dirty"] as? Bool ?? false
        case "openFile":
            if let path = payload["path"] as? String { openFile(at: URL(fileURLWithPath: path)) }
        case "title":
            if currentFileURL == nil, let title = payload["value"] as? String, !title.isEmpty {
                window.title = "\(title) — Mory"
            }
        case "export":
            if let options = payload["options"] as? [String: Any] { exportDocument(options: options) }
        case "localeChanged":
            interfaceLocale = payload["locale"] as? String == "en" ? "en" : "zh-CN"
            configureMenu()
        case "hostRequest":
            if let requestId = payload["requestId"] as? String,
               let method = payload["method"] as? String {
                handleHostRequest(id: requestId, method: method, arguments: payload["args"] as? [String: Any] ?? [:])
            }
        case "windowDragStart":
            if let x = (payload["screenX"] as? NSNumber)?.doubleValue,
               let y = (payload["screenY"] as? NSNumber)?.doubleValue {
                dragStartPointer = NSPoint(x: x, y: y)
                dragStartWindowOrigin = window.frame.origin
            }
        case "windowDragMove":
            if let start = dragStartPointer,
               let origin = dragStartWindowOrigin,
               let x = (payload["screenX"] as? NSNumber)?.doubleValue,
               let y = (payload["screenY"] as? NSNumber)?.doubleValue {
                window.setFrameOrigin(NSPoint(x: origin.x + x - start.x, y: origin.y - y + start.y))
            }
        case "windowDragEnd":
            dragStartPointer = nil
            dragStartWindowOrigin = nil
        default:
            break
        }
    }

    @objc private func showAbout() {
        NSApp.orderFrontStandardAboutPanel(options: [
            .applicationName: "Mory",
            .applicationVersion: "0.1.0",
            .credits: NSAttributedString(string: "一个原生、专注的 Markdown 编辑器。")
        ])
    }

    @objc private func showPreferences() { runJavaScript("window.Mory.togglePreferences()") }
    @objc private func showFind() { runJavaScript("window.Mory.showFind()") }
    @objc private func toggleBold() { runJavaScript("window.Mory.command('bold')") }
    @objc private func toggleItalic() { runJavaScript("window.Mory.command('italic')") }
    @objc private func toggleStrike() { runJavaScript("window.Mory.command('strike')") }
    @objc private func toggleCode() { runJavaScript("window.Mory.command('code')") }
    @objc private func toggleSidebar() { runJavaScript("window.Mory.toggleSidebar()") }
    @objc private func toggleSource() { runJavaScript("window.Mory.toggleSource()") }
    @objc private func toggleFocus() { runJavaScript("window.Mory.toggleFocus()") }
    @objc private func toggleTypewriter() { runJavaScript("window.Mory.toggleTypewriter()") }
    @objc private func resetZoom() { runJavaScript("window.Mory.zoom(0)") }
    @objc private func zoomIn() { runJavaScript("window.Mory.zoom(1)") }
    @objc private func zoomOut() { runJavaScript("window.Mory.zoom(-1)") }
    @objc private func applyHeading(_ sender: NSMenuItem) { runJavaScript("window.Mory.heading(\(sender.tag))") }

    private func presentError(_ message: String) {
        let alert = NSAlert()
        alert.alertStyle = .warning
        alert.messageText = "Mory"
        alert.informativeText = message
        alert.runModal()
    }
}
