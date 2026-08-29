import AppKit

@MainActor
private final class RecentMenuTarget: NSObject {
    var openedURL: URL?
    var clearCount = 0

    @objc func openRecentItem(_ sender: NSMenuItem) {
        openedURL = sender.representedObject as? URL
    }

    @objc func clearRecentItems() {
        clearCount += 1
    }
}

@MainActor
@main
struct MacRecentDocumentsSmoke {
    static func main() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let first = root.appendingPathComponent("first.md")
        let second = root.appendingPathComponent("second.txt")
        let unsupported = root.appendingPathComponent("image.png")
        let workspace = root.appendingPathComponent("workspace", isDirectory: true)
        try Data("first".utf8).write(to: first)
        try Data("second".utf8).write(to: second)
        try Data("image".utf8).write(to: unsupported)
        try FileManager.default.createDirectory(at: workspace, withIntermediateDirectories: true)

        let entries = RecentDocuments.entries(from: [first, unsupported, workspace, first, second])
        guard entries.map(\.url) == [first.standardizedFileURL, workspace.standardizedFileURL, second.standardizedFileURL],
              entries.map(\.kind) == [.document, .workspace, .document],
              entries[0].title.contains("first.md") else {
            throw NSError(domain: "MacRecentDocumentsSmoke", code: 1)
        }

        let target = RecentMenuTarget()
        let menu = RecentDocuments.menu(
            entries: entries,
            target: target,
            openAction: #selector(RecentMenuTarget.openRecentItem(_:)),
            clearAction: #selector(RecentMenuTarget.clearRecentItems),
            title: "Open Recent",
            emptyTitle: "No Recent Items",
            clearTitle: "Clear Menu",
            workspaceSuffix: "Workspace"
        )
        guard menu.items[0].target === target,
              menu.items[1].title == "workspace — Workspace",
              menu.items[1].image != nil,
              NSApplication.shared.sendAction(menu.items[1].action!, to: menu.items[1].target, from: menu.items[1]),
              target.openedURL == workspace.standardizedFileURL,
              menu.items.last?.target === target,
              NSApplication.shared.sendAction(menu.items.last!.action!, to: menu.items.last!.target, from: menu.items.last!),
              target.clearCount == 1 else {
            throw NSError(domain: "MacRecentDocumentsSmoke", code: 2)
        }
        print("macOS recent documents smoke test passed")
    }
}
