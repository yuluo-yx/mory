import AppKit
import Carbon
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
    private var previousInputSource: TISInputSource?
    private var inputMonitor: Any?
    private let eventMarker: Int64 = 0x4D4F5259
    private let sentencePrefix = "\u{6362}\u{800C}\u{8A00}\u{4E4B}\u{53EA}\u{9700}\u{8981}"
    private let pinyinID = "com.apple.inputmethod.SCIM.ITABC"
    private var finished = false
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
        DispatchQueue.main.asyncAfter(deadline: .now() + 30) {
            delegate.finish(failure: "Native IME test exceeded its 30-second deadline")
        }
        application.run()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        previousInputSource = TISCopyCurrentKeyboardInputSource().takeRetainedValue()
        inputMonitor = NSEvent.addLocalMonitorForEvents(matching: [.keyDown, .keyUp]) { [weak self] event in
            guard let self, !finished else { return event }
            guard event.cgEvent?.getIntegerValueField(.eventSourceUserData) == eventMarker else {
                finishInconclusive("Unrelated keyboard input reached the test window (key code \(event.keyCode))")
                return nil
            }
            return event
        }
        let configuration = WKWebViewConfiguration()
        webView = WKWebView(frame: NSRect(x: 0, y: 0, width: 900, height: 620), configuration: configuration)
        webView.navigationDelegate = self
        window = IMEKeyWindow(
            contentRect: webView.frame,
            styleMask: [.titled],
            backing: .buffered,
            defer: false
        )
        window.title = "Mory native Pinyin regression"
        window.center()
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
          window.__moryKeyCount = 0;
          window.__moryComposing = false;
          const write = document.querySelector('#write');
          const record = event => {
            if (event.type === 'keydown') window.__moryKeyCount += 1;
            if (event.type === 'compositionstart') window.__moryComposing = true;
            if (event.type === 'compositionend') window.__moryComposing = false;
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
            prepareInput(deadline: Date().addingTimeInterval(5))
        }
    }

    private func inputSourceID(_ source: TISInputSource) -> String {
        guard let value = TISGetInputSourceProperty(source, kTISPropertyInputSourceID) else { return "" }
        return Unmanaged<CFString>.fromOpaque(value).takeUnretainedValue() as String
    }

    private func prepareInput(deadline: Date) {
        window.makeKeyAndOrderFront(nil)
        NSApplication.shared.activate(ignoringOtherApps: true)
        window.makeFirstResponder(webView)
        guard NSApplication.shared.isActive, window.isKeyWindow else {
            guard Date() < deadline else { finish(failure: "Native IME window did not acquire focus; no keys were sent"); return }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) { self.prepareInput(deadline: deadline) }
            return
        }
        let filter = [kTISPropertyInputSourceID as String: pinyinID] as CFDictionary
        let sources = TISCreateInputSourceList(filter, false).takeRetainedValue() as! [TISInputSource]
        guard let pinyin = sources.first, TISSelectInputSource(pinyin) == noErr else {
            fputs("SKIP: Simplified Pinyin is not installed or selectable.\n", stderr)
            cleanup()
            Darwin.exit(77)
        }
        guard inputSourceID(TISCopyCurrentKeyboardInputSource().takeRetainedValue()) == pinyinID else {
            finish(failure: "The selected input source is not Simplified Pinyin")
            return
        }
        waitForScript("document.activeElement === document.querySelector('#write')", deadline: deadline) {
            // Input-source restoration by the window system may lag application activation.
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) {
                guard Date() < deadline else { self.finishInconclusive("Native IME input source did not settle"); return }
                guard NSApplication.shared.isActive, self.window.isKeyWindow,
                      self.inputSourceID(TISCopyCurrentKeyboardInputSource().takeRetainedValue()) == self.pinyinID else {
                    self.prepareInput(deadline: deadline)
                    return
                }
                print("Native IME focus and Simplified Pinyin verified")
                if ProcessInfo.processInfo.arguments.contains("--inject-unrelated-key") {
                    // Exercise the inconclusive result with a real, untagged native event.
                    for keyDown in [true, false] {
                        CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: keyDown)?.postToPid(getpid())
                    }
                    return
                }
                self.clearPendingComposition()
            }
        }
    }

    private func waitForScript(_ predicate: String, deadline: Date, completion: @escaping () -> Void) {
        webView.evaluateJavaScript(predicate) { [weak self] value, error in
            guard let self, !finished else { return }
            if let error { finish(failure: "Native IME readiness check failed: \(error)"); return }
            if value as? Bool == true { completion(); return }
            guard Date() < deadline else { finish(failure: "Native IME event acknowledgment timed out: \(predicate)"); return }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.05) {
                self.waitForScript(predicate, deadline: deadline, completion: completion)
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
            event.setIntegerValueField(.eventSourceUserData, value: eventMarker)
            event.postToPid(getpid())
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
        guard NSApplication.shared.isActive, window.isKeyWindow,
              inputSourceID(TISCopyCurrentKeyboardInputSource().takeRetainedValue()) == pinyinID else {
            finishInconclusive("Native IME focus or input source changed before a key was sent")
            return
        }
        webView.evaluateJavaScript("window.__moryKeyCount") { [weak self] value, error in
            guard let self, let count = value as? Int, error == nil else {
                self?.finish(failure: "Unable to read the native IME key acknowledgment counter")
                return
            }
            for keyDown in [true, false] {
                guard let event = CGEvent(keyboardEventSource: nil, virtualKey: stroke.keyCode, keyDown: keyDown) else {
                    finish(failure: "Unable to create a native keyboard event")
                    return
                }
                event.flags = []
                event.setIntegerValueField(.eventSourceUserData, value: self.eventMarker)
                event.postToPid(getpid())
            }
            let committed = stroke.keyCode == 49 ? " && !window.__moryComposing" : ""
            waitForScript("window.__moryKeyCount > \(count)\(committed)", deadline: Date().addingTimeInterval(3)) {
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                    self.sendStrokes(strokes, at: index + 1, completion: completion)
                }
            }
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
          paragraph.textContent = '\(sentencePrefix)****';
          const range = document.createRange();
          range.setStart(paragraph.firstChild, \(sentencePrefix.utf16.count + 2));
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
                  result["text"] as? String == sentencePrefix + "\u{51B3}\u{7B56}\u{5373}\u{53EF}",
                  result["bold"] as? String == "\u{51B3}\u{7B56}",
                  result["markdown"] as? String == sentencePrefix + "**\u{51B3}\u{7B56}**\u{5373}\u{53EF}" else {
                finish(failure: "Native Simplified Chinese Pinyin bold boundary failed: \(String(describing: value))")
                return
            }
            print("macOS Simplified Chinese Pinyin bold boundary passed: sentence prefix preserved, bold=expected first candidate, following text=plain")
            cleanup()
            NSApplication.shared.terminate(nil)
        }
    }

    private func finish(failure: String) {
        guard !finished else { return }
        fputs("\(failure)\n", stderr)
        cleanup()
        Darwin.exit(1)
    }

    private func finishInconclusive(_ reason: String) {
        guard !finished else { return }
        fputs("SKIP: \(reason); no product conclusion can be drawn from this run.\n", stderr)
        cleanup()
        Darwin.exit(77)
    }

    private func cleanup() {
        finished = true
        if let inputMonitor { NSEvent.removeMonitor(inputMonitor); self.inputMonitor = nil }
        if let previousInputSource {
            guard TISSelectInputSource(previousInputSource) == noErr,
                  inputSourceID(TISCopyCurrentKeyboardInputSource().takeRetainedValue()) == inputSourceID(previousInputSource) else {
                fputs("Failed to restore the previous input source.\n", stderr)
                Darwin.exit(1)
            }
            print("Native IME input source restored")
        }
        window?.orderOut(nil)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        finish(failure: "Native IME page load failed: \(error)")
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        finish(failure: "Native IME navigation failed: \(error)")
    }
}

MainActor.assumeIsolated {
    MacIMEInputSmoke.main()
}
