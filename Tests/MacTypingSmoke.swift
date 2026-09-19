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
        // Animation-frame normalization must run as it does in a visible editor window.
        window.alphaValue = 0.01
        window.orderFront(nil)
        window.makeFirstResponder(webView)

        let defaultPath = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
            .appendingPathComponent("Sources/Mory/Web/index.html").path
        let source = URL(fileURLWithPath: ProcessInfo.processInfo.environment["MORY_WEB_INDEX"] ?? defaultPath)
        webView.loadFileURL(source, allowingReadAccessTo: source.deletingLastPathComponent())
        DispatchQueue.main.asyncAfter(deadline: .now() + 60) { self.finish(failure: "Typing smoke timed out") }
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        Task { @MainActor in
            do {
                let source = try String(contentsOfFile: "Tests/heading-input-cases.js", encoding: .utf8)
                let value = try await webView.callAsyncJavaScript(source + "\nreturn await runHeadingInputCases()", arguments: [:], in: nil, contentWorld: .page)
                guard let result = value as? [String: Any], let failures = result["failures"] as? [[String: Any]], failures.isEmpty else {
                    finish(failure: "Heading input failed: \(String(describing: value))")
                    return
                }
                print("macOS heading input passed: \((result["passed"] as? [String] ?? []).count) scenarios")
                testNativeTyping()
            } catch { finish(failure: error.localizedDescription) }
        }
    }

    private func testNativeTyping() {
        focusEmptyEditor(unwrapped: true) { [weak self] in
            guard let self else { return }

            var actions: [() -> Void] = []
            for _ in 0..<2 {
                appendText("#", to: &actions)
                actions.append { self.sendKey(" ", keyCode: 49) }
                appendText("\u{4F60}\u{597D}", to: &actions)
                actions.append { self.sendKey("\r", keyCode: 36) }
            }
            perform(actions) {
                self.captureHeadingThenTestFence()
            }
        }
    }

    private func focusEmptyEditor(unwrapped: Bool = false, completion: @escaping () -> Void) {
        let script = """
        (() => {
          window.Mory.loadMarkdown('');
          const editor = document.querySelector('#write');
          if (\(unwrapped)) editor.replaceChildren();
          const paragraph = editor.firstElementChild || editor;
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
                self?.finish(failure: "Failed to focus the editor: \(error.localizedDescription)")
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
          paragraph.textContent = '# \u{4F60}\u{597D}';
          const range = document.createRange();
          range.selectNodeContents(paragraph);
          range.collapse(false);
          getSelection().removeAllRanges();
          getSelection().addRange(range);
          paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText', data: '\u{4F60}\u{597D}', isComposing: true }));
          paragraph.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: '\u{4F60}\u{597D}' }));
          const enter = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter' });
          const committedBlock = document.querySelector('#write > h1') || paragraph;
          result.immediateCompositionEnter = !committedBlock.dispatchEvent(enter)
            && document.querySelector('#write > h1')?.textContent === '\u{4F60}\u{597D}'
            && Boolean(document.querySelector('#write > h1 + p'));
          return result;
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "Failed to read heading state: \(error.localizedDescription)")
                return
            }
            guard let heading = value as? [String: Any] else {
                finish(failure: "Heading state has an invalid shape: \(String(describing: value))")
                return
            }
            focusEmptyEditor {
                var actions: [() -> Void] = []
                self.appendText("```go", to: &actions)
                actions.append { self.sendKey("\r", keyCode: 36) }
                self.appendText("fmt.Println(one)", to: &actions)
                actions.append { self.sendKey("\r", keyCode: 36) }
                self.appendText("fmt.Println(two)", to: &actions)
                actions.append { self.sendKey("\r", keyCode: 36) }
                actions.append { self.sendKey("\r", keyCode: 36) }
                self.perform(actions) {
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
          code: document.querySelector('#write > pre code')?.innerText || '',
          hasParagraphAfterCode: Boolean(document.querySelector('#write > pre + p')),
          activeTag: getSelection()?.anchorNode?.parentElement?.closest('#write > *')?.tagName || ''
        })
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "Failed to read code-fence state: \(error.localizedDescription)")
                return
            }
            guard let fence = value as? [String: Any],
                  heading["headings"] as? [String] == ["\u{4F60}\u{597D}", "\u{4F60}\u{597D}"],
                  heading["hasParagraphAfterHeadings"] as? Bool == true,
                  heading["immediateCompositionEnter"] as? Bool == true,
                  heading["rawHeading"] as? Bool == false,
                  fence["preCount"] as? Int == 1,
                  fence["code"] as? String == "fmt.Println(one)\nfmt.Println(two)",
                  fence["hasParagraphAfterCode"] as? Bool == true else {
                finish(failure: "Native input state is invalid: heading=\(heading); fence=\(String(describing: value)); Renderer errors=\(errors.joined(separator: " | "))")
                return
            }
            print("macOS WKWebView native input passed: consecutiveHeadings=\(heading); doubleEnterCodeExit=\(fence)")
            testEmptyNestedHeading()
        }
    }

    private func testEmptyNestedHeading() {
        let setup = """
        (() => {
          window.Mory.loadMarkdown('## First\\n\\n## Second');
          const heading = document.querySelectorAll('#write > h2')[1];
          const range = document.createRange();
          range.selectNodeContents(heading);
          range.collapse(true);
          const selection = getSelection();
          selection.removeAllRanges();
          selection.addRange(range);
          document.querySelector('#write').focus();
        })()
        """
        webView.evaluateJavaScript(setup) { [weak self] _, error in
            guard let self else { return }
            if let error {
                finish(failure: "Failed to prepare the empty nested heading test: \(error.localizedDescription)")
                return
            }
            window.makeFirstResponder(webView)
            var actions: [() -> Void] = []
            actions.append { self.sendKey("\r", keyCode: 36) }
            appendText("###", to: &actions)
            actions.append { self.sendKey(" ", keyCode: 49) }
            perform(actions) { self.verifyEmptyNestedHeading() }
        }
    }

    private func verifyEmptyNestedHeading() {
        let script = """
        (() => {
          const capture = () => ({
            html: document.querySelector('#write').innerHTML,
            markdown: window.Mory.getMarkdown(),
            h2Count: document.querySelectorAll('#write > h2').length,
            h3Count: document.querySelectorAll('#write > h3').length,
            h3Text: document.querySelector('#write > h3')?.textContent || '',
            h3HasPlaceholder: Boolean(document.querySelector('#write > h3')?.firstChild),
            h3Height: document.querySelector('#write > h3')?.getBoundingClientRect().height || 0,
            activeTag: getSelection()?.anchorNode?.parentElement?.closest('#write > *')?.tagName
              || (getSelection()?.anchorNode?.nodeType === Node.ELEMENT_NODE ? getSelection().anchorNode.closest('#write > *')?.tagName : '')
          });
          const live = capture();
          window.Mory.toggleSource(true);
          const source = document.querySelector('#source-editor').value;
          window.Mory.toggleSource(false);
          return { live, source, roundTrip: capture() };
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "Failed to read the empty nested heading state: \(error.localizedDescription)")
                return
            }
            let expected = "## First\n\n###\n\n## Second"
            guard let result = value as? [String: Any],
                  let live = result["live"] as? [String: Any],
                  let roundTrip = result["roundTrip"] as? [String: Any],
                  live["markdown"] as? String == expected,
                  live["h2Count"] as? Int == 2,
                  live["h3Count"] as? Int == 1,
                  live["h3Text"] as? String == "",
                  live["h3HasPlaceholder"] as? Bool == true,
                  (live["h3Height"] as? Double ?? 0) > 0,
                  live["activeTag"] as? String == "H3",
                  result["source"] as? String == expected,
                  roundTrip["markdown"] as? String == expected,
                  roundTrip["h2Count"] as? Int == 2,
                  roundTrip["h3Count"] as? Int == 1,
                  roundTrip["h3Text"] as? String == "" else {
                finish(failure: "Empty nested heading state is invalid: \(String(describing: value)); Renderer errors=\(errors.joined(separator: " | "))")
                return
            }
            print("macOS WKWebView empty nested heading passed: \(result)")
            NSApplication.shared.terminate(nil)
        }
    }

    private func appendText(_ text: String, to actions: inout [() -> Void]) {
        // Submit each semantic segment in one native input event; headless Intel WebViews may drop dense IPC keystrokes.
        actions.append { [weak self] in self?.sendKey(text, keyCode: 0) }
    }

    private func perform(_ actions: [() -> Void], index: Int = 0, completion: @escaping () -> Void) {
        guard index < actions.count else {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                completion()
            }
            return
        }
        actions[index]()
        // Intel hosted runners relayout code blocks asynchronously after punctuation; preserve a realistic typing interval.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.05) { [weak self] in
            self?.perform(actions, index: index + 1, completion: completion)
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
