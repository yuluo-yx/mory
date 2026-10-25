import Foundation

@main
struct MacDocumentDiskMonitorSmoke {
    static func main() throws {
        let files = FileManager.default
        let root = files.temporaryDirectory.appendingPathComponent("mory-disk-monitor-\(UUID().uuidString)")
        try files.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? files.removeItem(at: root) }
        let note = root.appendingPathComponent("note.md")
        try Data("first".utf8).write(to: note)
        var reports: [String] = []
        let monitor = DocumentDiskMonitor(interval: 0.05) { _, markdown in reports.append(markdown) }
        var failures: [String] = []
        func runLoop(until condition: () -> Bool) {
            let deadline = Date().addingTimeInterval(4)
            while !condition() && Date() < deadline { RunLoop.main.run(until: Date().addingTimeInterval(0.02)) }
        }

        monitor.track(note)
        if reports != ["first"] { failures.append("did not report the selected document immediately: \(reports)") }
        RunLoop.main.run(until: Date().addingTimeInterval(0.2))
        if reports.count != 1 { failures.append("reported an unchanged document repeatedly: \(reports)") }

        // Atomic writes replace the file, which detaches event-based watchers.
        try Data("edited elsewhere".utf8).write(to: note, options: .atomic)
        try files.setAttributes([.modificationDate: Date().addingTimeInterval(5)], ofItemAtPath: note.path)
        runLoop { reports.last == "edited elsewhere" }
        if reports.last != "edited elsewhere" { failures.append("missed an atomic external save: \(reports)") }

        try files.removeItem(at: note)
        RunLoop.main.run(until: Date().addingTimeInterval(0.2))
        let countAfterDelete = reports.count
        monitor.track(nil)
        try Data("recreated".utf8).write(to: note)
        RunLoop.main.run(until: Date().addingTimeInterval(0.2))
        if reports.count != countAfterDelete { failures.append("reported after tracking stopped: \(reports)") }

        guard failures.isEmpty else {
            failures.forEach { FileHandle.standardError.write(Data("FAIL: \($0)\n".utf8)) }
            exit(1)
        }
        print("macOS document disk monitor passed: immediate report, atomic external save, deleted file and stop")
    }
}
