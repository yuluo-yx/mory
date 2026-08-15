import AppKit
import WebKit

@MainActor
final class MacTypingSmoke: NSObject, NSApplicationDelegate, WKNavigationDelegate, WKScriptMessageHandler {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var errors: [String] = []

    static func main() {
        let application = NSApplication.shared
        let delegate = MacTypingSmoke()
        application.delegate = delegate
        application.setActivationPolicy(.prohibited)
        application.run()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        let controller = WKUserContentController()
        controller.add(self, name: "typingSmoke")
        controller.add(self, name: "mory")
        controller.addUserScript(WKUserScript(
            source: """
            window.addEventListener('error', event => window.webkit.messageHandlers.typingSmoke.postMessage(String(event.message)));
            window.addEventListener('unhandledrejection', event => window.webkit.messageHandlers.typingSmoke.postMessage(String(event.reason)));
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
        window.makeFirstResponder(webView)

        let defaultPath = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
            .appendingPathComponent("Sources/Mory/Web/index.html").path
        let source = URL(fileURLWithPath: ProcessInfo.processInfo.environment["MORY_WEB_INDEX"] ?? defaultPath)
        webView.loadFileURL(source, allowingReadAccessTo: source.deletingLastPathComponent())
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        focusEmptyEditor { [weak self] in
            guard let self else { return }
            for _ in 0..<2 {
                sendText("#")
                sendKey(" ", keyCode: 49)
                sendText("你好")
                sendKey("\r", keyCode: 36)
            }

            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                self.captureHeadingThenTestFence()
            }
        }
    }

    private func focusEmptyEditor(completion: @escaping () -> Void) {
        let script = """
        (() => {
          window.Mory.loadMarkdown('');
          const paragraph = document.querySelector('#write p');
          const range = document.createRange();
          range.setStart(paragraph, 0);
          range.collapse(true);
          const selection = getSelection();
          selection.removeAllRanges();
          selection.addRange(range);
          document.querySelector('#write').focus();
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] _, error in
            if let error {
                self?.finish(failure: "编辑器聚焦失败：\(error.localizedDescription)")
                return
            }
            self?.window.makeFirstResponder(self?.webView)
            completion()
        }
    }

    private func captureHeadingThenTestFence() {
        let script = """
        (() => {
          const result = {
            html: document.querySelector('#write').innerHTML,
            markdown: window.Mory.getMarkdown(),
            headings: [...document.querySelectorAll('#write > h1')].map(item => item.textContent),
            hasParagraphAfterHeadings: Boolean(document.querySelector('#write > h1:last-of-type + p')),
            rawHeading: document.querySelector('#write').textContent.includes('#')
          };
          window.Mory.loadMarkdown('');
          const paragraph = document.querySelector('#write > p');
          paragraph.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
          paragraph.textContent = '# 你好';
          const range = document.createRange();
          range.selectNodeContents(paragraph);
          range.collapse(false);
          getSelection().removeAllRanges();
          getSelection().addRange(range);
          paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', data: '你好', isComposing: true }));
          paragraph.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '你好' }));
          const enter = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' });
          result.immediateCompositionEnter = !paragraph.dispatchEvent(enter)
            && document.querySelector('#write > h1')?.textContent === '你好'
            && Boolean(document.querySelector('#write > h1 + p'));
          return result;
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "标题状态读取失败：\(error.localizedDescription)")
                return
            }
            guard let heading = value as? [String: Any] else {
                finish(failure: "标题状态格式异常：\(String(describing: value))")
                return
            }
            focusEmptyEditor {
                self.sendText("```go")
                self.sendKey("\r", keyCode: 36)
                self.sendText("fmt.Println(one)")
                self.sendKey("\r", keyCode: 36)
                self.sendText("fmt.Println(two)")
                self.sendKey("\r", keyCode: 36)
                self.sendKey("\r", keyCode: 36)
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                    self.verify(heading: heading)
                }
            }
        }
    }

    private func verify(heading: [String: Any]) {
        let script = """
        ({
          html: document.querySelector('#write').innerHTML,
          markdown: window.Mory.getMarkdown(),
          preCount: document.querySelectorAll('#write > pre').length,
          code: document.querySelector('#write > pre code')?.textContent || '',
          hasParagraphAfterCode: Boolean(document.querySelector('#write > pre + p')),
          activeTag: getSelection()?.anchorNode?.parentElement?.closest('#write > *')?.tagName || ''
        })
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "代码围栏状态读取失败：\(error.localizedDescription)")
                return
            }
            guard let fence = value as? [String: Any],
                  heading["headings"] as? [String] == ["你好", "你好"],
                  heading["hasParagraphAfterHeadings"] as? Bool == true,
                  heading["immediateCompositionEnter"] as? Bool == true,
                  heading["rawHeading"] as? Bool == false,
                  fence["preCount"] as? Int == 1,
                  fence["code"] as? String == "fmt.Println(one)\nfmt.Println(two)",
                  fence["hasParagraphAfterCode"] as? Bool == true else {
                finish(failure: "真实输入状态异常：heading=\(heading)；fence=\(String(describing: value))；页面错误=\(errors.joined(separator: " | "))")
                return
            }
            print("macOS WKWebView 真实输入通过：连续标题=\(heading)；代码块双回车=\(fence)")
            NSApplication.shared.terminate(nil)
        }
    }

    private func sendText(_ text: String) {
        for character in text {
            sendKey(String(character), keyCode: 0)
        }
    }

    private func sendKey(_ characters: String, keyCode: UInt16) {
        for type in [NSEvent.EventType.keyDown, .keyUp] {
            guard let event = NSEvent.keyEvent(
                with: type,
                location: .zero,
                modifierFlags: [],
                timestamp: ProcessInfo.processInfo.systemUptime,
                windowNumber: window.windowNumber,
                context: nil,
                characters: characters,
                charactersIgnoringModifiers: characters,
                isARepeat: false,
                keyCode: keyCode
            ) else { continue }
            window.sendEvent(event)
        }
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        if message.name == "typingSmoke" {
            errors.append(String(describing: message.body))
        }
    }

    private func finish(failure: String) {
        fputs("\(failure)\n", stderr)
        Darwin.exit(1)
    }
}

MainActor.assumeIsolated {
    MacTypingSmoke.main()
}
