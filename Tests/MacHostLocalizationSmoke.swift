import Foundation

@main
struct MacHostLocalizationSmoke {
    static func main() throws {
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
}
