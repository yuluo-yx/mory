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
    Unmanaged<WorkspaceWatcher>.fromOpaque(context).takeUnretainedValue().scheduleRefresh()
}

/// Recursively monitors the workspace using the native macOS FSEvents service.
final class WorkspaceWatcher: @unchecked Sendable {
    private var stream: FSEventStreamRef?
    private var watchedPath = ""
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
        stream = nextStream
        FSEventStreamSetDispatchQueue(nextStream, DispatchQueue.main)
        if !FSEventStreamStart(nextStream) { stop() }
    }

    func stop() {
        refreshWorkItem?.cancel()
        refreshWorkItem = nil
        guard let stream else {
            watchedPath = ""
            return
        }
        FSEventStreamStop(stream)
        FSEventStreamInvalidate(stream)
        FSEventStreamRelease(stream)
        self.stream = nil
        watchedPath = ""
    }

    fileprivate func scheduleRefresh() {
        refreshWorkItem?.cancel()
        let workItem = DispatchWorkItem { [weak self] in self?.onChange() }
        refreshWorkItem = workItem
        DispatchQueue.main.asyncAfter(deadline: .now() + latency, execute: workItem)
    }
}
