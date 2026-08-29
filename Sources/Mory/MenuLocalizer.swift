import AppKit

@MainActor
enum MenuLocalizer {
    private static let englishTitles = [
        "关于 Mory": "About Mory", "偏好设置…": "Preferences…", "退出 Mory": "Quit Mory", "文件": "File", "新建": "New", "新建目录": "New Folder",
        "打开…": "Open…", "打开文件夹…": "Open Folder…", "最近打开": "Open Recent", "无最近文稿": "No Recent Documents", "清除菜单": "Clear Menu", "保存": "Save", "另存为…": "Save As…", "导出…": "Export…",
        "编辑": "Edit", "撤销": "Undo", "重做": "Redo", "剪切": "Cut", "复制": "Copy", "粘贴": "Paste", "全选": "Select All",
        "查找和替换": "Find and Replace", "格式": "Format", "加粗": "Bold", "斜体": "Italic", "删除线": "Strikethrough", "行内代码": "Inline Code",
        "显示": "View", "显示／隐藏侧边栏": "Show/Hide Sidebar", "源代码模式": "Source Mode", "专注模式": "Focus Mode",
        "打字机模式": "Typewriter Mode", "实际大小": "Actual Size", "放大": "Zoom In", "缩小": "Zoom Out"
    ]

    static func localize(_ menu: NSMenu, locale: String) {
        guard locale == "en" else { return }
        menu.title = englishTitle(for: menu.title)

        for item in menu.items {
            if let submenu = item.submenu {
                localize(submenu, locale: locale)
                if !submenu.title.isEmpty {
                    item.title = submenu.title
                }
            } else {
                item.title = englishTitle(for: item.title)
            }
        }
    }

    private static func englishTitle(for title: String) -> String {
        if let translated = englishTitles[title] {
            return translated
        }
        if title.hasSuffix(" 级标题"), let level = title.split(separator: " ").first {
            return "Heading \(level)"
        }
        return title
    }
}
