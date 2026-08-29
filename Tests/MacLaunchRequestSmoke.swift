import Foundation

@main
struct MacLaunchRequestSmoke {
    static func main() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let source = root.appendingPathComponent("guide.md")
        try "# Guide".write(to: source, atomically: true, encoding: .utf8)

        let opened = try LaunchRequest.parse(arguments: [source.path], currentDirectory: root)
        guard opened.document == source.standardizedFileURL, opened.export == nil else {
            throw NSError(domain: "MacLaunchRequestSmoke", code: 1)
        }

        let destination = root.appendingPathComponent("guide.pdf")
        let exported = try LaunchRequest.parse(arguments: [
            "--mory-cli-export", "--format=pdf", "--output", destination.path, source.path
        ], currentDirectory: root)
        guard exported.export == LaunchExportRequest(source: source.standardizedFileURL, destination: destination.standardizedFileURL, format: "pdf") else {
            throw NSError(domain: "MacLaunchRequestSmoke", code: 2)
        }

        let slides = root.appendingPathComponent("guide.pptx")
        let slideExport = try LaunchRequest.parse(arguments: [
            "--mory-cli-export", "--format=pptx", "--output", slides.path, source.path
        ], currentDirectory: root)
        guard slideExport.export == LaunchExportRequest(source: source.standardizedFileURL, destination: slides.standardizedFileURL, format: "pptx") else {
            throw NSError(domain: "MacLaunchRequestSmoke", code: 5)
        }

        let workspace = WorkspaceConfig(dictionary: [
            "id": "remote", "name": "Repository", "provider": "github", "repository": "owner/site",
            "token": "secret", "localPath": root.path, "isImplicit": false
        ]).storageDictionary()
        guard workspace["repository"] as? String == "owner/site", workspace["token"] as? String == "secret",
              workspace["localPath"] == nil, workspace["isImplicit"] == nil else {
            throw NSError(domain: "MacLaunchRequestSmoke", code: 4)
        }

        do {
            _ = try LaunchRequest.parse(arguments: [
                "--mory-cli-export", "--format", "pdf", "--output", root.appendingPathComponent("guide.png").path, source.path
            ], currentDirectory: root)
            throw NSError(domain: "MacLaunchRequestSmoke", code: 3)
        } catch let error as NSError where error.domain == "Mory.Launch" {
            print("macOS launch request smoke test passed")
        }
    }
}
