import AppKit
import WebKit

@MainActor
@main
final class MacWebSmoke: NSObject, NSApplicationDelegate, WKNavigationDelegate, WKScriptMessageHandler {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var exportWebView: WKWebView?
    private var errors: [String] = []
    private var started = false

    static func main() {
        let application = NSApplication.shared
        let delegate = MacWebSmoke()
        application.delegate = delegate
        application.setActivationPolicy(.accessory)
        DispatchQueue.main.asyncAfter(deadline: .now() + 30) {
            delegate.finish(failure: "macOS 导出冒烟测试 30 秒内未完成")
        }
        application.finishLaunching()
        delegate.start()
        application.run()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        start()
    }

    private func start() {
        guard !started else { return }
        started = true
        let controller = WKUserContentController()
        controller.add(self, name: "smoke")
        controller.add(self, name: "mory")
        controller.addUserScript(WKUserScript(
            source: """
            window.addEventListener('error', event => window.webkit.messageHandlers.smoke.postMessage(String(event.message)));
            window.addEventListener('unhandledrejection', event => window.webkit.messageHandlers.smoke.postMessage(String(event.reason)));
            """,
            injectionTime: .atDocumentStart,
            forMainFrameOnly: true
        ))
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = controller
        webView = WKWebView(frame: NSRect(x: 0, y: 0, width: 1180, height: 790), configuration: configuration)
        webView.navigationDelegate = self
        window = NSWindow(contentRect: webView.frame, styleMask: [.borderless], backing: .buffered, defer: false)
        window.contentView = webView

        let defaultPath = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
            .appendingPathComponent("Sources/Mory/Web/index.html").path
        let source = URL(fileURLWithPath: ProcessInfo.processInfo.environment["MORY_WEB_INDEX"] ?? defaultPath)
        webView.loadFileURL(source, allowingReadAccessTo: source.deletingLastPathComponent())
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        finish(failure: "页面加载失败：\(error.localizedDescription)")
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        finish(failure: "页面预加载失败：\(error.localizedDescription)")
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        if webView === exportWebView {
            verifyRenderedFormats(webView)
            return
        }
        let script = """
        (() => {
          const result = { mory: typeof window.Mory, heading: document.querySelector('#write h1')?.textContent || '' };
          result.mermaid = typeof window.mermaid;
          result.highlight = window.hljs?.versionString || '';
          result.defaultTheme = document.documentElement.dataset.docTheme;
          result.host = document.documentElement.dataset.host;
          window.Mory.loadMarkdown('```go\\npackage main\\nfunc main() {}\\n```');
          result.codeHighlighted = Boolean(document.querySelector('#write pre[data-language="go"] .hljs-keyword'));
          document.querySelector('#export-button')?.click();
          result.exportOpen = document.querySelector('#export-dialog')?.classList.contains('is-open') || false;
          document.querySelector('#export-close')?.click();
          document.querySelector('#settings-button')?.click();
          result.preferencesOpen = document.querySelector('#preferences')?.classList.contains('is-open') || false;
          document.querySelector('#source-toggle')?.click();
          result.sourceMode = document.querySelector('.workspace')?.classList.contains('source-mode') || false;
          document.querySelector('#source-toggle')?.click();
          const statusToggle = document.querySelector('#status-toggle');
          statusToggle.checked = false;
          statusToggle.dispatchEvent(new Event('change', { bubbles: true }));
          result.statusbarHidden = document.querySelector('#statusbar')?.hidden === true
            && getComputedStyle(document.querySelector('#statusbar')).display === 'none';
          statusToggle.checked = true;
          statusToggle.dispatchEvent(new Event('change', { bubbles: true }));
          window.Mory.newDocument();
          window.Mory.newDocument();
          result.openDocuments = document.querySelectorAll('#file-list .file-item[data-document-id]').length;
          document.querySelector('.file-item.is-active')?.closest('.file-row')?.querySelector('.file-close')?.click();
          result.openDocumentsAfterClose = document.querySelectorAll('#file-list .file-item[data-document-id]').length;
          return result;
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "JavaScript 执行失败：\(error.localizedDescription)")
                return
            }
            guard let result = value as? [String: Any],
                  result["mory"] as? String == "object",
                  result["mermaid"] as? String == "object",
                  result["highlight"] as? String == "11.11.1",
                  result["defaultTheme"] as? String == "yuluo-css",
                  result["codeHighlighted"] as? Bool == true,
                  result["host"] as? String == "mac-native",
                  result["exportOpen"] as? Bool == true,
                  result["preferencesOpen"] as? Bool == true,
                  result["sourceMode"] as? Bool == true,
                  result["statusbarHidden"] as? Bool == true,
                  result["openDocuments"] as? Int == 3,
                  result["openDocumentsAfterClose"] as? Int == 2 else {
                finish(failure: "交互状态异常：\(String(describing: value))；页面错误：\(errors.joined(separator: " | "))")
                return
            }
            Task { @MainActor in
                do {
                    let exportValue = try await webView.callAsyncJavaScript(
                        """
                        const sections = Array.from({ length: 48 }, (_, index) =>
                          `## Section ${index + 1}\\n\\nThis is a multi-page PDF export regression paragraph with **theme styling**.`
                        );
                        window.Mory.loadMarkdown(`# Export regression\\n\\n${sections.join('\\n\\n')}`);
                        return await window.Mory.exportDocument(options);
                        """,
                        arguments: ["options": ["format": "html", "theme": "current", "paper": "A4", "width": 900, "background": true]],
                        in: nil,
                        contentWorld: .page
                    )
                    guard let html = exportValue as? String,
                          html.hasPrefix("<!doctype html>"),
                          html.contains("data-doc-theme=\"yuluo-css\"") else {
                        self.finish(failure: "macOS 异步导出 HTML 异常：\(String(describing: exportValue))")
                        return
                    }
                    print("macOS WKWebView 冒烟与异步导出 HTML 通过：\(result)，HTML \(html.utf8.count) 字节")
                    let renderer = WKWebView(frame: NSRect(x: 0, y: 0, width: 900, height: 900))
                    renderer.navigationDelegate = self
                    self.exportWebView = renderer
                    renderer.loadHTMLString(html, baseURL: nil)
                } catch {
                    self.finish(failure: "macOS 异步导出 JavaScript 失败：\(error.localizedDescription)；页面错误：\(self.errors.joined(separator: " | "))")
                }
            }
        }
    }

    private func verifyRenderedFormats(_ renderer: WKWebView) {
        renderer.evaluateJavaScript("Math.max(document.documentElement.scrollHeight, document.body.scrollHeight)") { [weak self] value, error in
            guard let self else { return }
            if let error { finish(failure: "测量导出页面失败：\(error.localizedDescription)"); return }
            let height = max(300, CGFloat((value as? NSNumber)?.doubleValue ?? 0))
            renderer.frame = NSRect(x: 0, y: 0, width: 900, height: height)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
                guard let self else { return }
                let configuration = WKPDFConfiguration()
                configuration.rect = renderer.bounds
                renderer.createPDF(configuration: configuration) { [weak self] result in
                    guard let self else { return }
                    Task { @MainActor in
                        do {
                            let sourcePDF = try result.get()
                            let output = FileManager.default.temporaryDirectory
                                .appendingPathComponent("mory-mac-paginated-\(ProcessInfo.processInfo.processIdentifier).pdf")
                            try await Task.detached(priority: .userInitiated) {
                                try PDFPaginator.write(
                                    sourcePDF,
                                    to: output,
                                    paperSize: CGSize(width: 595.28, height: 841.89)
                                )
                            }.value
                            guard let paginated = try? Data(contentsOf: output),
                                  paginated.starts(with: Data("%PDF".utf8)),
                                  let document = CGPDFDocument(output as CFURL),
                                  document.numberOfPages > 1 else {
                                self.finish(failure: "macOS 后台 PDF 分页结果无效")
                                return
                            }
                            try? FileManager.default.removeItem(at: output)
                            self.verifyImages(
                                renderer,
                                pdfSize: paginated.count,
                                pageCount: document.numberOfPages
                            )
                        } catch {
                            self.finish(failure: "macOS 异步 PDF 导出失败：\(error.localizedDescription)")
                        }
                    }
                }
            }
        }
    }

    private func verifyImages(_ renderer: WKWebView, pdfSize: Int, pageCount: Int) {
        let snapshot = WKSnapshotConfiguration()
        snapshot.rect = renderer.bounds
        renderer.takeSnapshot(with: snapshot) { [weak self] image, error in
            guard let self else { return }
            if let error { finish(failure: "macOS 图片导出失败：\(error.localizedDescription)"); return }
            guard let tiff = image?.tiffRepresentation,
                  let bitmap = NSBitmapImageRep(data: tiff),
                  let png = bitmap.representation(using: .png, properties: [:]),
                  let jpeg = bitmap.representation(using: .jpeg, properties: [.compressionFactor: 0.92]),
                  png.starts(with: Data([0x89, 0x50, 0x4e, 0x47])),
                  jpeg.starts(with: Data([0xff, 0xd8])) else {
                finish(failure: "macOS PNG/JPEG 数据签名无效")
                return
            }
            print("macOS 异步 PDF/PNG/JPEG 导出通过：PDF \(pdfSize) 字节、\(pageCount) 页，PNG \(png.count)，JPEG \(jpeg.count) 字节")
            NSApplication.shared.terminate(nil)
        }
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        if message.name == "smoke" {
            errors.append(String(describing: message.body))
        }
    }

    private func finish(failure: String) {
        fputs("\(failure)\n", stderr)
        Darwin.exit(1)
    }
}
