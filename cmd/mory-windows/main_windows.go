package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/yuluo-yx/mory"
	"github.com/yuluo-yx/mory/internal/windowshost"
)

type WindowsHost struct {
	core     *windowshost.Host
	platform *windowsPlatform
}

func (host *WindowsHost) startup(ctx context.Context) {
	host.platform.setContext(ctx)
	if err := host.core.Start(ctx); err != nil {
		host.platform.showError("Mory 启动失败", err)
	}
}

func (host *WindowsHost) shutdown(context.Context) { host.core.Stop() }

// Send mirrors the fire-and-forget entry point exposed by the Electron preload script.
func (host *WindowsHost) Send(payload map[string]any) error { return host.core.Send(payload) }

// Request mirrors the request-response entry point exposed by the Electron preload script.
func (host *WindowsHost) Request(method string, args map[string]any) (any, error) {
	return host.core.Request(method, args)
}

func main() {
	userData, err := os.UserConfigDir()
	if err != nil {
		panic(fmt.Errorf("读取用户配置目录：%w", err))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("读取用户目录：%w", err))
	}
	platform := &windowsPlatform{}
	host := &WindowsHost{
		platform: platform,
		core:     windowshost.New(platform, filepath.Join(userData, "Mory"), filepath.Join(home, "Documents", "Mory")),
	}
	platform.host = host.core

	err = wails.Run(&options.App{
		Title:            "未命名 — Mory",
		Width:            1180,
		Height:           790,
		MinWidth:         760,
		MinHeight:        520,
		BackgroundColour: &options.RGBA{R: 251, G: 251, B: 250, A: 255},
		AssetServer:      &assetserver.Options{Assets: mory.WebAssets()},
		Menu:             buildMenu(host.core, false),
		OnStartup:        host.startup,
		OnShutdown:       host.shutdown,
		Bind:             []any{host},
		Windows: &windows.Options{
			Theme:                windows.SystemDefault,
			IsZoomControlEnabled: true,
			DisablePinchZoom:     false,
			ResizeDebounceMS:     8,
			Messages: &windows.Messages{
				InstallationRequired: "Mory 需要 Microsoft Edge WebView2 Runtime。按“确定”后将下载并安装。",
				UpdateRequired:       "Mory 需要更新 Microsoft Edge WebView2 Runtime。",
				MissingRequirements:  "缺少运行环境",
				Webview2NotInstalled: "尚未安装 WebView2 Runtime",
				Error:                "错误",
				FailedToInstall:      "WebView2 Runtime 安装失败，请稍后重试。",
				DownloadPage:         "Mory 需要 WebView2 Runtime，按“确定”打开官方下载页。最低版本：",
				PressOKToInstall:     "按“确定”安装。",
				ContactAdmin:         "请联系管理员安装 WebView2 Runtime。",
				InvalidFixedWebview2: "指定的 WebView2 Runtime 无效。",
				WebView2ProcessCrash: "WebView2 进程意外退出，Mory 需要重新启动。",
			},
		},
	})
	if err != nil {
		panic(err)
	}
}

func buildMenu(host *windowshost.Host, english bool) *menu.Menu {
	label := func(chinese, translated string) string {
		if english {
			return translated
		}
		return chinese
	}
	application := menu.NewMenu()
	file := application.AddSubmenu(label("文件", "File"))
	file.AddText(label("新建", "New"), keys.CmdOrCtrl("n"), func(*menu.CallbackData) { host.NewDocument() })
	file.AddText(label("新建目录", "New Folder"), keys.Combo("n", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { host.Evaluate("window.Mory.newFolder()") })
	file.AddText(label("打开…", "Open…"), keys.CmdOrCtrl("o"), func(*menu.CallbackData) { _ = host.OpenDocument() })
	file.AddText(label("打开文件夹…", "Open Folder…"), keys.Combo("o", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { _ = host.OpenFolder() })
	file.AddSeparator()
	file.AddText(label("保存", "Save"), keys.CmdOrCtrl("s"), func(*menu.CallbackData) { _ = host.Save() })
	file.AddText(label("另存为…", "Save As…"), keys.Combo("s", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { _ = host.SaveAs() })
	file.AddSeparator()
	file.AddText(label("导出…", "Export…"), nil, func(*menu.CallbackData) { host.Evaluate("window.Mory.toggleExport()") })
	file.AddSeparator()
	file.AddText(label("退出", "Quit"), nil, func(*menu.CallbackData) { host.Evaluate("window.runtime.Quit()") })

	edit := application.AddSubmenu(label("编辑", "Edit"))
	edit.AddText(label("撤销", "Undo"), keys.CmdOrCtrl("z"), func(*menu.CallbackData) { host.Evaluate("document.execCommand('undo')") })
	edit.AddText(label("重做", "Redo"), keys.Combo("z", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { host.Evaluate("document.execCommand('redo')") })
	edit.AddSeparator()
	edit.AddText(label("剪切", "Cut"), keys.CmdOrCtrl("x"), func(*menu.CallbackData) { host.Evaluate("document.execCommand('cut')") })
	edit.AddText(label("复制", "Copy"), keys.CmdOrCtrl("c"), func(*menu.CallbackData) { host.Evaluate("document.execCommand('copy')") })
	edit.AddText(label("粘贴", "Paste"), keys.CmdOrCtrl("v"), func(*menu.CallbackData) { host.Evaluate("document.execCommand('paste')") })
	edit.AddText(label("全选", "Select All"), keys.CmdOrCtrl("a"), func(*menu.CallbackData) { host.Evaluate("document.execCommand('selectAll')") })
	edit.AddSeparator()
	edit.AddText(label("查找和替换", "Find and Replace"), keys.CmdOrCtrl("f"), func(*menu.CallbackData) { host.Evaluate("window.Mory.showFind()") })

	format := application.AddSubmenu(label("格式", "Format"))
	format.AddText(label("加粗", "Bold"), keys.CmdOrCtrl("b"), func(*menu.CallbackData) { host.Evaluate("window.Mory.command('bold')") })
	format.AddText(label("斜体", "Italic"), keys.CmdOrCtrl("i"), func(*menu.CallbackData) { host.Evaluate("window.Mory.command('italic')") })
	format.AddText(label("删除线", "Strikethrough"), nil, func(*menu.CallbackData) { host.Evaluate("window.Mory.command('strike')") })
	format.AddText(label("行内代码", "Inline Code"), nil, func(*menu.CallbackData) { host.Evaluate("window.Mory.command('code')") })
	format.AddSeparator()
	for level := 1; level <= 6; level++ {
		value := level
		format.AddText(label(fmt.Sprintf("%d 级标题", level), fmt.Sprintf("Heading %d", level)), nil, func(*menu.CallbackData) { host.Evaluate(fmt.Sprintf("window.Mory.heading(%d)", value)) })
	}

	view := application.AddSubmenu(label("显示", "View"))
	view.AddText(label("显示／隐藏侧边栏", "Show/Hide Sidebar"), keys.Combo("l", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { host.Evaluate("window.Mory.toggleSidebar()") })
	view.AddText(label("源代码模式", "Source Mode"), keys.CmdOrCtrl("/"), func(*menu.CallbackData) { host.Evaluate("window.Mory.toggleSource()") })
	view.AddText(label("专注模式", "Focus Mode"), nil, func(*menu.CallbackData) { host.Evaluate("window.Mory.toggleFocus()") })
	view.AddText(label("打字机模式", "Typewriter Mode"), nil, func(*menu.CallbackData) { host.Evaluate("window.Mory.toggleTypewriter()") })
	view.AddSeparator()
	view.AddText(label("实际大小", "Actual Size"), keys.CmdOrCtrl("0"), func(*menu.CallbackData) { host.Evaluate("window.Mory.zoom(0)") })
	view.AddText(label("放大", "Zoom In"), keys.CmdOrCtrl("+"), func(*menu.CallbackData) { host.Evaluate("window.Mory.zoom(1)") })
	view.AddText(label("缩小", "Zoom Out"), keys.CmdOrCtrl("-"), func(*menu.CallbackData) { host.Evaluate("window.Mory.zoom(-1)") })

	help := application.AddSubmenu(label("帮助", "Help"))
	help.AddText(label("偏好设置", "Preferences"), keys.CmdOrCtrl(","), func(*menu.CallbackData) { host.Evaluate("window.Mory.togglePreferences()") })
	return application
}
