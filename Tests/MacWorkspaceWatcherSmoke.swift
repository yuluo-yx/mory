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
        let nested = root.appendingPathComponent("\u{5D4C}\u{5957}", isDirectory: true)
        try fileManager.createDirectory(at: nested, withIntermediateDirectories: true)
        first = nested.appendingPathComponent("\u{7B2C}\u{4E00}\u{7BC7}.md")
        renamed = nested.appendingPathComponent("\u{5DF2}\u{91CD}\u{547D}\u{540D}.md")
    }

    func run() {
        watcher = WorkspaceWatcher(latency: 0.06) { [weak self] in
            Task { @MainActor in self?.handleChange() }
        }
        watcher.start(at: root)
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) { [weak self] in
            guard let self else { return }
            try? "# \u{7B2C}\u{4E00}\u{7BC7}".write(to: first, atomically: true, encoding: .utf8)
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 8) { [weak self] in
            self?.finish(failure: "macOS FSEvents workspace watcher did not finish within 8 seconds")
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
            print("macOS FSEvents workspace create, rename, and delete watch passed")
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
