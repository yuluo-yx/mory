import Foundation

@main
struct MacWorkspaceBoundarySmoke {
    static func main() throws {
        let files = FileManager.default
        let base = files.temporaryDirectory.appendingPathComponent("mory-boundary-\(UUID().uuidString)")
        defer { try? files.removeItem(at: base) }
        let root = base.appendingPathComponent("root")
        let outside = base.appendingPathComponent("outside")
        let support = base.appendingPathComponent("support")
        for directory in [root, outside, support, root.appendingPathComponent("inside")] {
            try files.createDirectory(at: directory, withIntermediateDirectories: true)
        }
        let config = ["id": "audit", "name": "Audit", "provider": "local", "localPath": root.path]
        try JSONSerialization.data(withJSONObject: ["version": 1, "activeId": "audit", "workspaces": [config]])
            .write(to: support.appendingPathComponent("workspaces.json"))
        let workspace = try WorkspaceManager(supportRoot: support)
        _ = try workspace.activate("audit")
        try Data("outside".utf8).write(to: outside.appendingPathComponent("secret.png"))
        try Data("original".utf8).write(to: root.appendingPathComponent("note.md"))
        try Data("inside".utf8).write(to: root.appendingPathComponent("inside/safe.png"))
        for (name, target) in ["escape": outside, "note": outside, "alias": root.appendingPathComponent("inside"), "dangling": outside.appendingPathComponent("missing"), "cycle": root.appendingPathComponent("cycle")] {
            try files.createSymbolicLink(at: root.appendingPathComponent(name), withDestinationURL: target)
        }
        var failures: [String] = []
        func rejects(_ label: String, _ action: () throws -> Void) {
            do { try action(); failures.append(label) } catch { }
        }
        rejects("created an external directory") { _ = try workspace.createDirectory(relativePath: "escape/new/nested") }
        rejects("read an external image") { _ = try workspace.image(at: root.appendingPathComponent("escape/secret.png").path) }
        rejects("imported an external image") {
            _ = try workspace.importImage(arguments: ["documentPath": root.appendingPathComponent("note.md").path, "documentName": "note.md", "name": "new.png", "mime": "image/png", "data": Data("new".utf8).base64EncodedString()])
        }
        rejects("created through a dangling link") { _ = try workspace.createDirectory(relativePath: "dangling/child") }
        rejects("created through a cyclic link") { _ = try workspace.createDirectory(relativePath: "cycle/child") }
        let assets = workspace.assets(for: root.appendingPathComponent("note.md"), markdown: "![secret](escape/secret.png)")
        if !assets.isEmpty { failures.append("embedded an external image") }
        let css = "p { background: url(escape/secret.png) }"
        if try ThemeManager.inlineAssets(in: css, base: root) != css { failures.append("embedded an external theme resource") }
        let destination = root.appendingPathComponent("destination")
        try files.createDirectory(at: destination, withIntermediateDirectories: true)
        rejects("copied unsafe companion assets") { _ = try workspace.copyEntry(path: root.appendingPathComponent("note.md").path, destinationPath: destination.path) }
        rejects("moved unsafe companion assets") { _ = try workspace.moveEntry(path: root.appendingPathComponent("note.md").path, destinationPath: destination.path) }
        if try String(contentsOf: root.appendingPathComponent("note.md"), encoding: .utf8) != "original" { failures.append("changed original note") }
        let image = try workspace.image(at: root.appendingPathComponent("alias/safe.png").path)
        if image["dataURL"]?.hasSuffix(Data("inside".utf8).base64EncodedString()) != true { failures.append("rejected an internal image") }
        _ = try workspace.createDirectory(relativePath: "alias/new/nested")
        _ = try workspace.createDocument(directoryPath: root.appendingPathComponent("alias/new").path, name: "created.md")
        if !files.fileExists(atPath: root.appendingPathComponent("inside/new/created.md").path) { failures.append("rejected an internal destination") }
        let rootAlias = base.appendingPathComponent("root-alias")
        try files.createSymbolicLink(at: rootAlias, withDestinationURL: root)
        _ = try workspace.save(["provider": "local", "name": "Alias", "localPath": rootAlias.path])
        _ = try workspace.image(at: rootAlias.appendingPathComponent("alias/safe.png").path)
        let contractData = try Data(contentsOf: URL(fileURLWithPath: "Tests/fixtures/asset-paths.json"))
        let contracts = try JSONSerialization.jsonObject(with: contractData) as! [[String: String]]
        for item in contracts {
            if CompanionAssetPaths.rewrite(item["source"]!, from: item["from"]!, to: item["to"]!) != item["expected"]! {
                failures.append("asset URL contract: " + item["name"]!)
            }
        }
        for operation in ["rename", "save-as"] {
            for (index, markdown) in ["![x](Old/image.png)", "![x](./Old/image.png)", "<img src=\"Old/image.png\">"].enumerated() {
                let directory = root.appendingPathComponent("\(operation)-\(index)")
                try files.createDirectory(at: directory.appendingPathComponent("Old"), withIntermediateDirectories: true)
                _ = try workspace.save(["provider": "local", "name": "Image references", "localPath": directory.path])
                let source = directory.appendingPathComponent("Old.md")
                try Data(markdown.utf8).write(to: source)
                try Data("image".utf8).write(to: directory.appendingPathComponent("Old/image.png"))
                let parent = operation == "rename" ? directory : directory.appendingPathComponent("destination")
                try files.createDirectory(at: parent, withIntermediateDirectories: true)
                let target = parent.appendingPathComponent("New.md")
                if operation == "rename" { _ = try workspace.renameEntry(path: source.path, name: "New.md") }
                else {
                    let rewritten = try workspace.relocateAssets(markdown: markdown, oldURL: source, oldName: "Old.md", newURL: target)
                    try Data(rewritten.utf8).write(to: target)
                    if try String(contentsOf: source, encoding: .utf8) != markdown { failures.append("Save As changed the source") }
                }
                let saved = try String(contentsOf: target, encoding: .utf8)
                if workspace.assets(for: target, markdown: saved).count != 1 { failures.append("\(operation) lost image \(index)") }
            }
        }
        guard failures.isEmpty else {
            fputs("Workspace boundary failures: \(failures.joined(separator: "; "))\n", stderr)
            exit(1)
        }
        print("macOS asset URL contracts passed: \(contracts.count) fixtures and six rename/Save As scenarios")
        print("macOS workspace boundaries passed: links, safe copy/move, image import, themes, embedding and root alias")
    }
}
