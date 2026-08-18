import AppKit

@main
@MainActor
enum MacMenuLocalizationSmoke {
    static func main() {
        let fileTitle = "\u{6587}\u{4EF6}"
        let newTitle = "\u{65B0}\u{5EFA}"
        let mainMenu = NSMenu()
        let fileItem = NSMenuItem()
        let fileMenu = NSMenu(title: fileTitle)
        fileMenu.addItem(withTitle: newTitle, action: nil, keyEquivalent: "n")
        fileItem.submenu = fileMenu
        mainMenu.addItem(fileItem)

        MenuLocalizer.localize(mainMenu, locale: "en")

        guard fileMenu.title == "File" else {
            fputs("Menu localization did not translate the submenu title.\n", stderr)
            Darwin.exit(1)
        }
        guard fileItem.title == "File" else {
            fputs("Menu localization did not update the top-level menu item.\n", stderr)
            Darwin.exit(1)
        }
        guard fileMenu.items.first?.title == "New" else {
            fputs("Menu localization did not translate the submenu command.\n", stderr)
            Darwin.exit(1)
        }

        print("macOS menu localization smoke test passed.")
    }
}
