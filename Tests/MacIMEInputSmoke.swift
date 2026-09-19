import AppKit
import WebKit

final class IMEKeyWindow: NSWindow {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }
}

@MainActor
final class MacIMEInputSmoke: NSObject, NSApplicationDelegate, WKNavigationDelegate {
    private struct Stroke {
        let keyCode: UInt16
    }

    private var window: NSWindow!
    private var webView: WKWebView!
    private let headingStrokes = [
        Stroke(keyCode: 45),
        Stroke(keyCode: 34),
        Stroke(keyCode: 4),
        Stroke(keyCode: 0),
        Stroke(keyCode: 31),
        Stroke(keyCode: 49),
        Stroke(keyCode: 36)
    ]
    private let boldStrokes = [
        Stroke(keyCode: 38),
        Stroke(keyCode: 32),
        Stroke(keyCode: 14),
        Stroke(keyCode: 8),
        Stroke(keyCode: 14),
        Stroke(keyCode: 49),
        Stroke(keyCode: 38),
        Stroke(keyCode: 34),
        Stroke(keyCode: 40),
        Stroke(keyCode: 14),
        Stroke(keyCode: 49)
    ]

    static func main() {
        let application = NSApplication.shared
        let delegate = MacIMEInputSmoke()
        application.delegate = delegate
        application.setActivationPolicy(.regular)
        application.run()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        let configuration = WKWebViewConfiguration()
        webView = WKWebView(frame: NSRect(x: 0, y: 0, width: 900, height: 620), configuration: configuration)
        webView.navigationDelegate = self
        window = IMEKeyWindow(
            contentRect: webView.frame,
            styleMask: [.borderless],
            backing: .buffered,
            defer: false
        )
        window.alphaValue = 0.02
        window.setFrameOrigin(NSPoint(x: -1_800, y: -1_200))
        window.contentView = webView
        window.makeKeyAndOrderFront(nil)
        NSApplication.shared.activate(ignoringOtherApps: true)
        window.makeFirstResponder(webView)

        let defaultPath = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
            .appendingPathComponent("Sources/Mory/Web/index.html").path
        let source = URL(fileURLWithPath: ProcessInfo.processInfo.environment["MORY_WEB_INDEX"] ?? defaultPath)
        webView.loadFileURL(source, allowingReadAccessTo: source.deletingLastPathComponent())
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        let script = """
        (() => {
          window.Mory.loadMarkdown('');
          window.__moryIMETrace = [];
          const write = document.querySelector('#write');
          const record = event => {
            const selection = getSelection();
            window.__moryIMETrace.push({
              type: event.type,
              key: event.key || '',
              inputType: event.inputType || '',
              data: event.data ?? '',
              isComposing: Boolean(event.isComposing),
              cancelable: event.cancelable,
              defaultPrevented: event.defaultPrevented,
              html: write.innerHTML,
              anchor: selection?.anchorNode?.nodeName || '',
              offset: selection?.anchorOffset ?? -1
            });
          };
          ['keydown', 'beforeinput', 'input', 'compositionstart', 'compositionupdate', 'compositionend'].forEach(type => {
            write.addEventListener(type, record, true);
          });
          const paragraph = write.querySelector('p');
          paragraph.textContent = '#';
          const range = document.createRange();
          const text = paragraph.firstChild;
          range.setStart(text, text.data.length);
          range.collapse(true);
          getSelection().removeAllRanges();
          getSelection().addRange(range);
          write.focus();
          // Follow the same keydown path as a typed space, create the empty H1 immediately,
          // then hand subsequent Chinese text to the system Simplified Pinyin input method.
          paragraph.dispatchEvent(new KeyboardEvent('keydown', {
            key: ' ', code: 'Space', bubbles: true, cancelable: true
          }));
          return Boolean(write.querySelector(':scope > h1'));
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] _, error in
            guard let self else { return }
            if let error {
                finish(failure: "Native IME test initialization failed: \(error.localizedDescription)")
                return
            }
            window.makeKeyAndOrderFront(nil)
            NSApplication.shared.activate(ignoringOtherApps: true)
            window.makeFirstResponder(webView)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) {
                print("Native IME host: active=\(NSApplication.shared.isActive), key=\(self.window.isKeyWindow), firstResponder=\(String(describing: self.window.firstResponder))")
                self.clearPendingComposition()
            }
        }
    }

    private func clearPendingComposition() {
        for keyDown in [true, false] {
            guard let event = CGEvent(
                keyboardEventSource: nil,
                virtualKey: 53,
                keyDown: keyDown
            ) else { continue }
            event.flags = []
            event.post(tap: .cghidEventTap)
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
            guard let self else { return }
            sendStrokes(headingStrokes) { self.verifyHeading() }
        }
    }

    private func sendStrokes(_ strokes: [Stroke], at index: Int = 0, completion: @escaping () -> Void) {
        guard index < strokes.count else {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) { completion() }
            return
        }
        let stroke = strokes[index]
        for keyDown in [true, false] {
            guard let event = CGEvent(
                keyboardEventSource: nil,
                virtualKey: CGKeyCode(stroke.keyCode),
                keyDown: keyDown
            ) else { continue }
            event.flags = []
            event.post(tap: .cghidEventTap)
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
            self?.sendStrokes(strokes, at: index + 1, completion: completion)
        }
    }

    private func verifyHeading() {
        let script = """
        (() => {
          const selection = getSelection();
          const anchor = selection?.anchorNode;
          const block = (anchor?.nodeType === Node.ELEMENT_NODE ? anchor : anchor?.parentElement)?.closest('#write > *');
          return {
            html: document.querySelector('#write').innerHTML,
            markdown: window.Mory.getMarkdown(),
            heading: document.querySelector('#write > h1')?.textContent || '',
            paragraphAfterHeading: Boolean(document.querySelector('#write > h1 + p')),
            activeBlock: block?.tagName || '',
            trace: window.__moryIMETrace
          };
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "Failed to read native IME state: \(error.localizedDescription)")
                return
            }
            guard let result = value as? [String: Any] else {
                finish(failure: "Native IME result has an invalid shape: \(String(describing: value))")
                return
            }
            guard result["heading"] as? String == "\u{4F60}\u{597D}",
                  result["paragraphAfterHeading"] as? Bool == true,
                  result["activeBlock"] as? String == "P" else {
                finish(failure: "Native Simplified Chinese Pinyin heading Enter failed: \(result)")
                return
            }
            print("macOS Simplified Chinese Pinyin regression passed: heading=expected CJK text, followingBlock=P")
            testBoldCompositionBoundary()
        }
    }

    private func testBoldCompositionBoundary() {
        let script = """
        (() => {
          window.Mory.loadMarkdown('');
          window.__moryIMETrace = [];
          const write = document.querySelector('#write');
          const paragraph = write.querySelector('p');
          paragraph.textContent = '****';
          const range = document.createRange();
          range.setStart(paragraph.firstChild, 2);
          range.collapse(true);
          getSelection().removeAllRanges();
          getSelection().addRange(range);
          write.focus();
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] _, error in
            guard let self else { return }
            if let error {
                finish(failure: "Native bold IME initialization failed: \(error.localizedDescription)")
                return
            }
            window.makeKeyAndOrderFront(nil)
            NSApplication.shared.activate(ignoringOtherApps: true)
            window.makeFirstResponder(webView)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) {
                self.sendStrokes(self.boldStrokes) { self.verifyBoldCompositionBoundary() }
            }
        }
    }

    private func verifyBoldCompositionBoundary() {
        let script = """
        (() => ({
          html: document.querySelector('#write').innerHTML,
          markdown: window.Mory.getMarkdown(),
          text: document.querySelector('#write > p')?.textContent || '',
          bold: document.querySelector('#write > p strong')?.textContent || '',
          trace: window.__moryIMETrace
        }))()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            if let error {
                finish(failure: "Failed to read the native bold IME state: \(error.localizedDescription)")
                return
            }
            guard let result = value as? [String: Any],
                  result["text"] as? String == "\u{51B3}\u{7B56}\u{5373}\u{53EF}",
                  result["bold"] as? String == "\u{51B3}\u{7B56}",
                  result["markdown"] as? String == "**\u{51B3}\u{7B56}**\u{5373}\u{53EF}" else {
                finish(failure: "Native Simplified Chinese Pinyin bold boundary failed: \(String(describing: value))")
                return
            }
            print("macOS Simplified Chinese Pinyin bold boundary passed: bold=expected first candidate, following text=plain")
            NSApplication.shared.terminate(nil)
        }
    }

    private func finish(failure: String) {
        fputs("\(failure)\n", stderr)
        Darwin.exit(1)
    }
}

MainActor.assumeIsolated {
    MacIMEInputSmoke.main()
}
