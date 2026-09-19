import Foundation

struct HostLocalizer {
    let messages: [String: String]

    init(url: URL?) {
        messages = url.flatMap { try? Data(contentsOf: $0) }
            .flatMap { try? JSONDecoder().decode([String: String].self, from: $0) } ?? [:]
    }

    func text(_ message: String, locale: String) -> String {
        if locale == "en", let translated = messages[message] { return translated }
        var key = message
        if key.hasSuffix("。") || key.hasSuffix(".") { key.removeLast() }
        if locale != "en" {
            return key.lowercased() == "path must remain inside the selected directory" ? "路径必须位于所选目录内。" : message
        }
        if let translated = messages[key] { return translated }
        if let separator = key.firstIndex(of: "："), let translated = messages[String(key[..<separator])] {
            return translated + ": " + text(String(key[key.index(after: separator)...]), locale: locale)
        }
        return message
    }
}
