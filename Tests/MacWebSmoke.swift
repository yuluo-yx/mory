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
            delegate.finish(failure: "macOS export smoke test did not finish within 30 seconds")
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
        finish(failure: "Page load failed: \(error.localizedDescription)")
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        finish(failure: "Page provisional load failed: \(error.localizedDescription)")
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
          const folderSample = document.createElement('button');
          folderSample.className = 'folder-item';
          const fileSample = document.createElement('button');
          fileSample.className = 'file-item';
          document.body.append(folderSample, fileSample);
          const folderStyle = getComputedStyle(folderSample);
          const fileStyle = getComputedStyle(fileSample);
          result.folderTypographyCompensated = parseFloat(folderStyle.fontSize) >= parseFloat(fileStyle.fontSize) + 0.75
            && parseFloat(folderStyle.fontWeight) >= parseFloat(fileStyle.fontWeight) + 50
            && Math.abs(parseFloat(folderStyle.minHeight) - parseFloat(fileStyle.minHeight)) < 0.1;
          folderSample.remove();
          fileSample.remove();
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
                finish(failure: "JavaScript execution failed: \(error.localizedDescription)")
                return
            }
            guard let result = value as? [String: Any],
                  result["mory"] as? String == "object",
                  result["mermaid"] as? String == "object",
                  result["highlight"] as? String == "11.11.1",
                  result["defaultTheme"] as? String == "github",
                  result["codeHighlighted"] as? Bool == true,
                  result["host"] as? String == "mac-native",
                  result["exportOpen"] as? Bool == true,
                  result["preferencesOpen"] as? Bool == true,
                  result["sourceMode"] as? Bool == true,
                  result["statusbarHidden"] as? Bool == true,
                  result["folderTypographyCompensated"] as? Bool == true,
                  result["openDocuments"] as? Int == 3,
                  result["openDocumentsAfterClose"] as? Int == 2 else {
                finish(failure: "Interaction state is invalid: \(String(describing: value)); Renderer errors: \(errors.joined(separator: " | "))")
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
                          html.contains("data-doc-theme=\"github\"") else {
                        self.finish(failure: "macOS asynchronous HTML export is invalid: \(String(describing: exportValue))")
                        return
                    }
                    print("macOS WKWebView smoke test and asynchronous HTML export passed: defaultTheme=github, interactions=validated, HTML \(html.utf8.count) bytes")
                    let renderer = WKWebView(frame: NSRect(x: 0, y: 0, width: 900, height: 900))
                    renderer.navigationDelegate = self
                    self.exportWebView = renderer
                    renderer.loadHTMLString(html, baseURL: nil)
                } catch {
                    self.finish(failure: "macOS asynchronous export JavaScript failed: \(error.localizedDescription); Renderer errors: \(self.errors.joined(separator: " | "))")
                }
            }
        }
    }

    private func verifyRenderedFormats(_ renderer: WKWebView) {
        renderer.evaluateJavaScript("Math.max(document.documentElement.scrollHeight, document.body.scrollHeight)") { [weak self] value, error in
            guard let self else { return }
            if let error { finish(failure: "Failed to measure the export page: \(error.localizedDescription)"); return }
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
                                self.finish(failure: "macOS background PDF pagination returned an invalid result")
                                return
                            }
                            try? FileManager.default.removeItem(at: output)
                            self.verifyImages(
                                renderer,
                                pdfSize: paginated.count,
                                pageCount: document.numberOfPages
                            )
                        } catch {
                            self.finish(failure: "macOS asynchronous PDF export failed: \(error.localizedDescription)")
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
            if let error { finish(failure: "macOS image export failed: \(error.localizedDescription)"); return }
            guard let tiff = image?.tiffRepresentation,
                  let bitmap = NSBitmapImageRep(data: tiff),
                  let png = bitmap.representation(using: .png, properties: [:]),
                  let jpeg = bitmap.representation(using: .jpeg, properties: [.compressionFactor: 0.92]),
                  png.starts(with: Data([0x89, 0x50, 0x4e, 0x47])),
                  jpeg.starts(with: Data([0xff, 0xd8])) else {
                finish(failure: "macOS PNG or JPEG data has an invalid signature")
                return
            }
            print("macOS asynchronous PDF, PNG, and JPEG export passed: PDF \(pdfSize) bytes, \(pageCount) pages, PNG \(png.count), JPEG \(jpeg.count) bytes")
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
