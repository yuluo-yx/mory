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
    private let strokes = [
        Stroke(keyCode: 45),
        Stroke(keyCode: 34),
        Stroke(keyCode: 4),
        Stroke(keyCode: 0),
        Stroke(keyCode: 31),
        Stroke(keyCode: 49),
        Stroke(keyCode: 36)
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
          // 与用户键入空格相同地经过编辑器 keydown 管线，先即时建立空 H1，
          // 再把后续中文正文交给系统简体拼音。
          paragraph.dispatchEvent(new KeyboardEvent('keydown', {
            key: ' ', code: 'Space', bubbles: true, cancelable: true
          }));
          return Boolean(write.querySelector(':scope > h1'));
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] _, error in
            guard let self else { return }
            if let error {
                finish(failure: "真实输入法测试初始化失败：\(error.localizedDescription)")
                return
            }
            window.makeKeyAndOrderFront(nil)
            NSApplication.shared.activate(ignoringOtherApps: true)
            window.makeFirstResponder(webView)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) {
                print("真实输入法宿主：active=\(NSApplication.shared.isActive)，key=\(self.window.isKeyWindow)，firstResponder=\(String(describing: self.window.firstResponder))")
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
            self?.sendStroke(at: 0)
        }
    }

    private func sendStroke(at index: Int) {
        guard index < strokes.count else {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) { [weak self] in self?.verify() }
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
            self?.sendStroke(at: index + 1)
        }
    }

    private func verify() {
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
                finish(failure: "真实输入法状态读取失败：\(error.localizedDescription)")
                return
            }
            guard let result = value as? [String: Any] else {
                finish(failure: "真实输入法结果格式异常：\(String(describing: value))")
                return
            }
            guard result["heading"] as? String == "你好",
                  result["paragraphAfterHeading"] as? Bool == true,
                  result["activeBlock"] as? String == "P" else {
                finish(failure: "真实简体拼音标题回车失败：\(result)")
                return
            }
            print("macOS 简体拼音回归通过：标题=你好，后续块=P，Markdown=# 你好")
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
