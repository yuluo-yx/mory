import AppKit

enum RecentDocumentKind: Equatable {
    case document
    case workspace
}

struct RecentDocumentEntry: Equatable {
    let url: URL
    let title: String
    let kind: RecentDocumentKind
}

enum RecentDocuments {
    private static let supportedExtensions = Set(["md", "markdown", "mmd", "mdown", "mkd", "txt", "text"])

    static func entries(from urls: [URL], fileManager: FileManager = .default, limit: Int = 10) -> [RecentDocumentEntry] {
        var seen = Set<String>()
        var entries: [RecentDocumentEntry] = []
        for url in urls {
            let standardized = url.standardizedFileURL
            var isDirectory: ObjCBool = false
            guard fileManager.fileExists(atPath: standardized.path, isDirectory: &isDirectory),
                  seen.insert(standardized.path).inserted else { continue }
            let kind: RecentDocumentKind
            let title: String
            if isDirectory.boolValue {
                kind = .workspace
                title = standardized.lastPathComponent
            } else {
                guard supportedExtensions.contains(standardized.pathExtension.lowercased()) else { continue }
                kind = .document
                let parent = standardized.deletingLastPathComponent().lastPathComponent
                title = "\(standardized.lastPathComponent) — \(parent)"
            }
            entries.append(RecentDocumentEntry(url: standardized, title: title, kind: kind))
            if entries.count == limit { break }
        }
        return entries
    }

    @MainActor
    static func menu(
        entries: [RecentDocumentEntry],
        target: AnyObject,
        openAction: Selector,
        clearAction: Selector,
        title: String,
        emptyTitle: String,
        clearTitle: String,
        workspaceSuffix: String
    ) -> NSMenu {
        let menu = NSMenu(title: title)
        guard !entries.isEmpty else {
            let empty = menu.addItem(withTitle: emptyTitle, action: nil, keyEquivalent: "")
            empty.isEnabled = false
            return menu
        }
        for entry in entries {
            let displayTitle = entry.kind == .workspace ? "\(entry.title) — \(workspaceSuffix)" : entry.title
            let item = menu.addItem(withTitle: displayTitle, action: openAction, keyEquivalent: "")
            item.representedObject = entry.url
            item.target = target
            item.image = NSImage(
                systemSymbolName: entry.kind == .workspace ? "folder" : "doc.text",
                accessibilityDescription: nil
            )
        }
        menu.addItem(.separator())
        let clear = menu.addItem(withTitle: clearTitle, action: clearAction, keyEquivalent: "")
        clear.target = target
        return menu
    }
}
