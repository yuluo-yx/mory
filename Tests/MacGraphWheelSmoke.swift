import AppKit
import WebKit

final class GraphWheelWindow: NSWindow {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }
}

@MainActor
final class MacGraphWheelSmoke: NSObject, NSApplicationDelegate, WKNavigationDelegate {
    private var window: NSWindow!
    private var webView: WKWebView!

    static func main() {
        let application = NSApplication.shared
        let delegate = MacGraphWheelSmoke()
        application.delegate = delegate
        application.setActivationPolicy(.regular)
        DispatchQueue.main.asyncAfter(deadline: .now() + 15) {
            delegate.finish(failure: "macOS 知识图谱滚轮测试 15 秒内未完成")
        }
        application.run()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        webView = WKWebView(frame: NSRect(x: 0, y: 0, width: 1180, height: 790))
        webView.navigationDelegate = self
        window = GraphWheelWindow(
            contentRect: webView.frame,
            styleMask: [.borderless],
            backing: .buffered,
            defer: false
        )
        window.alphaValue = 0.02
        window.contentView = webView
        window.center()
        window.makeKeyAndOrderFront(nil)
        NSApplication.shared.activate(ignoringOtherApps: true)

        let defaultPath = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
            .appendingPathComponent("Sources/Mory/Web/index.html").path
        let source = URL(fileURLWithPath: ProcessInfo.processInfo.environment["MORY_WEB_INDEX"] ?? defaultPath)
        webView.loadFileURL(source, allowingReadAccessTo: source.deletingLastPathComponent())
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        let script = """
        (() => {
          window.Mory.setWorkspaceDocuments([
            { name: '入口.md', path: '/virtual/入口.md', markdown: '# 入口\\n[[设计]]' },
            { name: '设计.md', path: '/virtual/设计.md', markdown: '# 设计\\n[[入口]]' },
            { name: '孤立.md', path: '/virtual/孤立.md', markdown: '# 孤立' }
          ]);
          document.querySelector('#graph-button').click();
          const editor = document.querySelector('#editor-scroll');
          editor.dataset.scrollBeforeNativeWheel = String(editor.scrollTop);
          window.__moryNativeWheelTrace = [];
          document.addEventListener('wheel', event => {
            window.__moryNativeWheelTrace.push({
              target: event.target?.id || event.target?.className?.baseVal || event.target?.className || event.target?.tagName || '',
              deltaY: event.deltaY,
              deltaMode: event.deltaMode,
              clientX: event.clientX,
              clientY: event.clientY
            });
          }, true);
          return true;
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] _, error in
            guard let self else { return }
            if let error {
                finish(failure: "macOS 图谱初始化失败：\(error.localizedDescription)")
                return
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) { [weak self] in
                self?.postNativeWheel()
            }
        }
    }

    private func postNativeWheel() {
        let script = """
        (() => {
          const svg = document.querySelector('#graph-svg');
          const rect = svg.getBoundingClientRect();
          return {
            ready: document.querySelectorAll('#graph-svg .graph-node').length === 3,
            x: rect.left + rect.width / 2,
            y: rect.top + rect.height / 2
          };
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            guard error == nil,
                  let result = value as? [String: Any],
                  result["ready"] as? Bool == true,
                  let x = (result["x"] as? NSNumber)?.doubleValue,
                  let y = (result["y"] as? NSNumber)?.doubleValue else {
                finish(failure: "macOS 图谱画布尚未就绪：\(String(describing: value))")
                return
            }

            let wheelScript = """
            document.querySelector('#graph-canvas').dispatchEvent(new WheelEvent('wheel', {
              deltaY: -3,
              deltaMode: WheelEvent.DOM_DELTA_LINE,
              clientX: \(x),
              clientY: \(y),
              bubbles: true,
              cancelable: true
            }));
            """
            webView.evaluateJavaScript(wheelScript) { [weak self] _, wheelError in
                guard let self else { return }
                if let wheelError {
                    finish(failure: "WKWebView line-unit 滚轮执行失败：\(wheelError.localizedDescription)")
                    return
                }
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) { [weak self] in
                    self?.verifyZoom()
                }
            }
        }
    }

    private func verifyZoom() {
        let script = """
        (() => ({
          transform: document.querySelector('#graph-stage')?.getAttribute('transform') || '',
          zoom: document.querySelector('#graph-zoom')?.value || '',
          editorStayed: document.querySelector('#editor-scroll').scrollTop
            === Number(document.querySelector('#editor-scroll').dataset.scrollBeforeNativeWheel),
          trace: window.__moryNativeWheelTrace,
          hit: (() => {
            const rect = document.querySelector('#graph-svg').getBoundingClientRect();
            const node = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
            return node?.id || node?.className?.baseVal || node?.className || node?.tagName || '';
          })()
        }))()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self else { return }
            guard error == nil,
                  let result = value as? [String: Any],
                  let transform = result["transform"] as? String,
                  transform.contains("scale("),
                  let zoom = result["zoom"] as? String,
                  zoom != "100%",
                  result["editorStayed"] as? Bool == true else {
                finish(failure: "WKWebView line-unit 滚轮未缩放知识图谱：\(String(describing: value))")
                return
            }
            print("macOS WKWebView line-unit 滚轮缩放通过：\(zoom)，\(transform)")
            NSApplication.shared.terminate(nil)
        }
    }

    private func finish(failure: String) {
        fputs("\(failure)\n", stderr)
        Darwin.exit(1)
    }
}

MainActor.assumeIsolated {
    MacGraphWheelSmoke.main()
}
