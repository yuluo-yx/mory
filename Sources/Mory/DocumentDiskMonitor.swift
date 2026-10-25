import Foundation

/// Polls the document shown in the editor and reports its disk content after another program edits it.
/// Polling metadata is used instead of file events because many editors save by writing a temporary file
/// and renaming it, which detaches vnode watchers from the original file. The editor compares the reported
/// content with its last known disk version, so a report for this application's own save is harmless.
final class DocumentDiskMonitor {
    private let interval: TimeInterval
    private let onChange: (_ path: String, _ markdown: String) -> Void
    private var timer: Timer?
    private var lastStamp = ""
    private(set) var url: URL?

    init(interval: TimeInterval = 1, onChange: @escaping (_ path: String, _ markdown: String) -> Void) {
        self.interval = interval
        self.onChange = onChange
    }

    /// Starts tracking a document and reports it immediately, or stops tracking for drafts without a file.
    func track(_ url: URL?) {
        timer?.invalidate()
        timer = nil
        self.url = url
        lastStamp = ""
        guard url != nil else { return }
        let timer = Timer(timeInterval: interval, repeats: true) { [weak self] _ in self?.check() }
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
        check()
    }

    func stop() {
        track(nil)
    }

    /// Reports the tracked document when its size or modification time changed since the last report.
    func check() {
        guard let url else { return }
        // URL resource values are cached per URL instance, so read fresh attributes on every poll.
        guard let attributes = try? FileManager.default.attributesOfItem(atPath: url.path),
              attributes[.type] as? FileAttributeType == .typeRegular else { return }
        let modified = (attributes[.modificationDate] as? Date)?.timeIntervalSinceReferenceDate ?? 0
        let stamp = "\(modified):\((attributes[.size] as? NSNumber)?.int64Value ?? 0)"
        guard stamp != lastStamp, let markdown = try? String(contentsOf: url, encoding: .utf8) else { return }
        lastStamp = stamp
        onChange(url.path, markdown)
    }
}
