import Foundation

@MainActor
private final class WorkspaceWatcherSmoke {
    private let fileManager = FileManager.default
    private let root: URL
    private let first: URL
    private let renamed: URL
    private var watcher: WorkspaceWatcher!
    private var stage = 0

    init() throws {
        root = fileManager.temporaryDirectory
            .appendingPathComponent("mory-fsevents-\(ProcessInfo.processInfo.processIdentifier)-\(UUID().uuidString)", isDirectory: true)
        let nested = root.appendingPathComponent("嵌套", isDirectory: true)
        try fileManager.createDirectory(at: nested, withIntermediateDirectories: true)
        first = nested.appendingPathComponent("第一篇.md")
        renamed = nested.appendingPathComponent("已重命名.md")
    }

    func run() {
        watcher = WorkspaceWatcher(latency: 0.06) { [weak self] in
            Task { @MainActor in self?.handleChange() }
        }
        watcher.start(at: root)
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) { [weak self] in
            guard let self else { return }
            try? "# 第一篇".write(to: first, atomically: true, encoding: .utf8)
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 8) { [weak self] in
            self?.finish(failure: "macOS FSEvents 工作区监听 8 秒内未完成")
        }
    }

    private func handleChange() {
        switch stage {
        case 0 where fileManager.fileExists(atPath: first.path):
            stage = 1
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
                guard let self else { return }
                try? fileManager.moveItem(at: first, to: renamed)
            }
        case 1 where fileManager.fileExists(atPath: renamed.path):
            stage = 2
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
                guard let self else { return }
                try? fileManager.removeItem(at: renamed)
            }
        case 2 where !fileManager.fileExists(atPath: renamed.path):
            watcher.stop()
            try? fileManager.removeItem(at: root)
            print("macOS FSEvents 工作区新增、重命名、删除监听通过")
            Foundation.exit(0)
        default:
            break
        }
    }

    private func finish(failure: String) {
        watcher?.stop()
        try? fileManager.removeItem(at: root)
        fputs("\(failure)\n", stderr)
        Foundation.exit(1)
    }
}

@main
private struct MacWorkspaceWatcherSmokeRunner {
    @MainActor
    static func main() throws {
        let smoke = try WorkspaceWatcherSmoke()
        smoke.run()
        RunLoop.main.run()
    }
}
