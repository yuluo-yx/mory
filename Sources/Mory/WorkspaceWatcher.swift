import CoreServices
import Foundation

private func workspaceEventCallback(
    _ stream: ConstFSEventStreamRef,
    _ context: UnsafeMutableRawPointer?,
    _ eventCount: Int,
    _ eventPaths: UnsafeMutableRawPointer,
    _ eventFlags: UnsafePointer<FSEventStreamEventFlags>,
    _ eventIds: UnsafePointer<FSEventStreamEventId>
) {
    guard let context else { return }
    let watcher = Unmanaged<WorkspaceWatcher>.fromOpaque(context).takeUnretainedValue()
    // Without kFSEventStreamCreateFlagUseCFTypes the paths arrive as a C string array.
    let paths = eventPaths.assumingMemoryBound(to: UnsafePointer<CChar>.self)
    let changed = (0..<eventCount).map { String(cString: paths[$0]) }
    if changed.contains(where: { WorkspaceWatcher.isRelevant(path: $0, roots: watcher.roots) }) {
        watcher.scheduleRefresh()
    }
}

/// Recursively monitors the workspace using the native macOS FSEvents service.
final class WorkspaceWatcher: @unchecked Sendable {
    private var stream: FSEventStreamRef?
    private var watchedPath = ""
    private var resolvedWatchedPath = ""
    private var refreshWorkItem: DispatchWorkItem?
    private let latency: TimeInterval
    private let onChange: @Sendable () -> Void

    init(latency: TimeInterval = 0.18, onChange: @escaping @Sendable () -> Void) {
        self.latency = latency
        self.onChange = onChange
    }

    deinit {
        stop()
    }

    /// The watched root as configured and as resolved by FSEvents, which reports symlink-free paths.
    fileprivate var roots: [String] { [watchedPath, resolvedWatchedPath] }

    /// Hidden entries such as `.git` or `.DS_Store` never appear in workspace snapshots, so their churn is ignored.
    /// Paths outside the root (including the root itself) stay relevant so root renames and deletions still refresh.
    static func isRelevant(path: String, roots: [String]) -> Bool {
        for root in roots {
            let normalizedRoot = root.hasSuffix("/") ? String(root.dropLast()) : root
            guard !normalizedRoot.isEmpty, path.hasPrefix(normalizedRoot + "/") else { continue }
            let relative = path.dropFirst(normalizedRoot.count + 1)
            return !relative.split(separator: "/").contains { $0.hasPrefix(".") }
        }
        return true
    }

    func start(at root: URL) {
        let path = root.standardizedFileURL.path
        guard stream == nil || watchedPath != path else { return }
        stop()

        var context = FSEventStreamContext(
            version: 0,
            info: Unmanaged.passUnretained(self).toOpaque(),
            retain: nil,
            release: nil,
            copyDescription: nil
        )
        let flags = FSEventStreamCreateFlags(
            kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagWatchRoot
        )
        guard let nextStream = FSEventStreamCreate(
            nil,
            workspaceEventCallback,
            &context,
            [path] as CFArray,
            FSEventStreamEventId(kFSEventStreamEventIdSinceNow),
            latency,
            flags
        ) else { return }

        watchedPath = path
        // Foundation symlink resolution keeps `/var` instead of `/private/var`; realpath matches FSEvents paths.
        resolvedWatchedPath = realpath(path, nil).map { pointer in
            defer { free(pointer) }
            return String(cString: pointer)
        } ?? path
        stream = nextStream
        FSEventStreamSetDispatchQueue(nextStream, DispatchQueue.main)
        if !FSEventStreamStart(nextStream) { stop() }
    }

    func stop() {
        refreshWorkItem?.cancel()
        refreshWorkItem = nil
        guard let stream else {
            watchedPath = ""
            resolvedWatchedPath = ""
            return
        }
        FSEventStreamStop(stream)
        FSEventStreamInvalidate(stream)
        FSEventStreamRelease(stream)
        self.stream = nil
        watchedPath = ""
        resolvedWatchedPath = ""
    }

    fileprivate func scheduleRefresh() {
        refreshWorkItem?.cancel()
        let workItem = DispatchWorkItem { [weak self] in self?.onChange() }
        refreshWorkItem = workItem
        DispatchQueue.main.asyncAfter(deadline: .now() + latency, execute: workItem)
    }
}
