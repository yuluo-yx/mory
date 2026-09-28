import Foundation

@main
struct MacHostLocalizationSmoke {
    static func main() throws {
        try verifyPackagedResourceLookupDoesNotEvaluateFallback()
        let url = URL(fileURLWithPath: FileManager.default.currentDirectoryPath).appendingPathComponent("Sources/Mory/Web/host-messages.json")
        let localizer = HostLocalizer(url: url)
        guard localizer.messages.count > 100 else { throw workspaceError("Host message catalog is missing") }
        for (chinese, english) in localizer.messages {
            guard localizer.text(chinese, locale: "en") == english,
                  localizer.text(chinese, locale: "zh-CN") == chinese else {
                throw workspaceError("Host message translation changed")
            }
        }
        guard localizer.text("\u{5BFC}\u{51FA}\u{5931}\u{8D25}\u{FF1A}ENOENT /notes/file.md", locale: "en") == "Export failed: ENOENT /notes/file.md" else {
            throw workspaceError("Host error details changed")
        }
        print("macOS host localization passed: shared catalog, bilingual messages and error details")
    }

    private static func verifyPackagedResourceLookupDoesNotEvaluateFallback() throws {
        let temporaryRoot = FileManager.default.temporaryDirectory
            .appendingPathComponent("mory-resource-locator-\(UUID().uuidString)", isDirectory: true)
        defer { try? FileManager.default.removeItem(at: temporaryRoot) }
        let packagedURL = temporaryRoot.appendingPathComponent("Web/host-messages.json")
        try FileManager.default.createDirectory(
            at: packagedURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        try Data("{}".utf8).write(to: packagedURL)

        var fallbackWasEvaluated = false
        let locatedURL = AppResourceLocator.existingURL(
            from: temporaryRoot,
            relativePath: "Web/host-messages.json"
        ) {
            fallbackWasEvaluated = true
            return nil
        }
        guard locatedURL == packagedURL, !fallbackWasEvaluated else {
            throw workspaceError("Packaged resource lookup evaluated the SwiftPM fallback")
        }

        let fallbackURL = temporaryRoot.appendingPathComponent("Mory_Mory.bundle/Web/index.html")
        try FileManager.default.createDirectory(
            at: fallbackURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        try Data().write(to: fallbackURL)
        let fallbackResult = AppResourceLocator.existingURL(
            from: nil,
            relativePath: "Web/index.html"
        ) {
            fallbackWasEvaluated = true
            return fallbackURL
        }
        guard fallbackResult == fallbackURL, fallbackWasEvaluated else {
            throw workspaceError("SwiftPM resource fallback was not evaluated")
        }
    }
}
