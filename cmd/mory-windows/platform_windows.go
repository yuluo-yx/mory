package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"

	"github.com/yuluo-yx/mory/internal/recentfiles"
	"github.com/yuluo-yx/mory/internal/slidevexport"
	"github.com/yuluo-yx/mory/internal/windowshost"
)

type windowsPlatform struct {
	mu     sync.RWMutex
	ctx    context.Context
	host   *windowshost.Host
	recent *recentfiles.Store
	locale string
}

func (platform *windowsPlatform) setContext(ctx context.Context) {
	platform.mu.Lock()
	platform.ctx = ctx
	platform.mu.Unlock()
}

func (platform *windowsPlatform) context() context.Context {
	platform.mu.RLock()
	defer platform.mu.RUnlock()
	return platform.ctx
}

func (platform *windowsPlatform) ChooseDirectory(defaultDirectory string) (string, error) {
	return runtime.OpenDirectoryDialog(platform.context(), runtime.OpenDialogOptions{
		DefaultDirectory:     defaultDirectory,
		Title:                "选择工作目录",
		CanCreateDirectories: true,
	})
}

func (platform *windowsPlatform) ChooseFile(defaultDirectory string, extensions []string) (string, error) {
	patterns := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		patterns = append(patterns, "*."+strings.TrimPrefix(extension, "."))
	}
	return runtime.OpenFileDialog(platform.context(), runtime.OpenDialogOptions{
		DefaultDirectory: defaultDirectory,
		Title:            "打开文件",
		Filters:          []runtime.FileFilter{{DisplayName: "支持的文件", Pattern: strings.Join(patterns, ";")}},
	})
}

func (platform *windowsPlatform) ChooseSavePath(defaultPath string, extensions []string) (string, error) {
	patterns := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		patterns = append(patterns, "*."+strings.TrimPrefix(extension, "."))
	}
	return runtime.SaveFileDialog(platform.context(), runtime.SaveDialogOptions{
		DefaultDirectory:     filepath.Dir(defaultPath),
		DefaultFilename:      filepath.Base(defaultPath),
		Title:                "保存文件",
		CanCreateDirectories: true,
		Filters:              []runtime.FileFilter{{DisplayName: "支持的文件", Pattern: strings.Join(patterns, ";")}},
	})
}

func (platform *windowsPlatform) Confirm(title, message, detail string) (bool, error) {
	result, err := runtime.MessageDialog(platform.context(), runtime.MessageDialogOptions{
		Type:          runtime.WarningDialog,
		Title:         title,
		Message:       message + "\n\n" + detail,
		Buttons:       []string{"确定", "取消"},
		DefaultButton: "取消",
		CancelButton:  "取消",
	})
	return result == "确定", err
}

func (platform *windowsPlatform) Trash(path string) error { return moveToRecycleBin(path) }

func (platform *windowsPlatform) Reveal(path string) error {
	return exec.Command("explorer.exe", "/select,"+path).Start()
}

func (platform *windowsPlatform) OpenDirectory(path string) error {
	return exec.Command("explorer.exe", path).Start()
}

func (platform *windowsPlatform) OpenURL(url string) error {
	runtime.BrowserOpenURL(platform.context(), url)
	return nil
}

func (platform *windowsPlatform) Evaluate(script string) {
	runtime.WindowExecJS(platform.context(), script)
}

func (platform *windowsPlatform) SetTitle(title string) {
	runtime.WindowSetTitle(platform.context(), title)
}

func (platform *windowsPlatform) SetLocale(locale string) {
	platform.mu.Lock()
	platform.locale = locale
	platform.mu.Unlock()
	platform.rebuildMenu()
}

func (platform *windowsPlatform) NoteRecentDocument(path string) {
	if platform.recent == nil || platform.recent.Add(path) != nil {
		return
	}
	platform.rebuildMenu()
}

func (platform *windowsPlatform) ToggleMaximise() {
	runtime.WindowToggleMaximise(platform.context())
}

func (platform *windowsPlatform) ShowAbout(locale string) {
	english := locale == "en"
	title := "关于 Mory"
	detail := "一个原生、专注的 Markdown 编辑器。\n\n版本 " + appVersion
	button := "确定"
	if english {
		title = "About Mory"
		detail = "A native, focused Markdown editor.\n\nVersion " + appVersion
		button = "OK"
	}
	_, _ = runtime.MessageDialog(platform.context(), runtime.MessageDialogOptions{
		Type: runtime.InfoDialog, Title: title, Message: "Mory\n\n" + detail,
		Buttons: []string{button}, DefaultButton: button,
	})
}

func (platform *windowsPlatform) Export(request windowshost.ExportRequest) error {
	if request.Format == "" {
		request.Format = "html"
	}
	extension := request.Format
	if extension == "jpeg" {
		extension = "jpg"
	} else if extension == "mindmap" {
		extension = "html"
	}
	defaultName := request.Name
	if defaultName == "" {
		defaultName = "未命名"
	}
	path := request.Destination
	if path == "" {
		var err error
		path, err = platform.ChooseSavePath(defaultName+"."+extension, []string{extension})
		if err != nil || path == "" {
			return err
		}
	} else {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve export destination: %w", err)
		}
		if !strings.EqualFold(filepath.Ext(absolute), "."+extension) {
			return fmt.Errorf("export destination extension must be .%s", extension)
		}
		path = absolute
	}
	if request.HTML == "" {
		if request.Format == "pptx" {
			return platform.exportPowerPoint(request, path)
		}
		return errors.New("前端没有返回已渲染的导出页面")
	}
	if request.Format == "pptx" {
		return platform.exportPowerPoint(request, path)
	}
	if request.Format == "html" || request.Format == "mindmap" {
		return writeExportFile(path, []byte(request.HTML))
	}
	return exportWithEdge(platform.context(), request, path)
}

func (platform *windowsPlatform) exportPowerPoint(request windowshost.ExportRequest, destination string) error {
	err := slidevexport.Export(platform.context(), slidevexport.Request{
		Markdown: request.Markdown, SourcePath: request.SourcePath, Destination: destination,
	})
	if err == nil {
		return nil
	}
	platform.mu.RLock()
	english := platform.locale == "en"
	platform.mu.RUnlock()
	if slidevexport.ErrorCode(err) == slidevexport.CodeUnavailable {
		if english {
			return errors.New("Slidev export is unavailable. Install Node.js, then run: npm install -g @slidev/cli playwright-chromium")
		}
		return errors.New("未找到 Slidev 导出环境。请安装 Node.js，然后运行：npm install -g @slidev/cli playwright-chromium")
	}
	if english {
		return fmt.Errorf("Slidev export failed: %w", err)
	}
	return fmt.Errorf("Slidev 导出失败：%w", err)
}

func (platform *windowsPlatform) rebuildMenu() {
	platform.mu.RLock()
	ctx := platform.ctx
	english := platform.locale == "en"
	platform.mu.RUnlock()
	if ctx != nil {
		runtime.MenuSetApplicationMenu(ctx, buildMenu(platform, english))
	}
}

func (platform *windowsPlatform) openRecentItem(path string) {
	info, statErr := os.Stat(path)
	var err error
	if statErr == nil && info.IsDir() {
		err = platform.host.OpenExternalFolder(path)
	} else if statErr != nil {
		err = statErr
	} else {
		err = platform.host.OpenExternalFile(path)
	}
	if err != nil {
		platform.showError("Mory", err)
		platform.rebuildMenu()
	}
}

func (platform *windowsPlatform) clearRecentDocuments() {
	if err := platform.recent.Clear(); err != nil {
		platform.showError("Mory", err)
	}
	platform.rebuildMenu()
}

func (platform *windowsPlatform) showError(title string, cause error) {
	_, _ = runtime.MessageDialog(platform.context(), runtime.MessageDialogOptions{
		Type:          runtime.ErrorDialog,
		Title:         title,
		Message:       cause.Error(),
		Buttons:       []string{"确定"},
		DefaultButton: "确定",
	})
}

type shFileOperation struct {
	Window               windows.Handle
	Function             uint32
	From                 *uint16
	To                   *uint16
	Flags                uint16
	AnyOperationsAborted int32
	NameMappings         uintptr
	ProgressTitle        *uint16
}

var shFileOperationW = windows.NewLazySystemDLL("shell32.dll").NewProc("SHFileOperationW")

func moveToRecycleBin(path string) error {
	// SHFileOperationW requires the source list to end with two NUL characters.
	from, err := windows.UTF16PtrFromString(path + "\x00")
	if err != nil {
		return fmt.Errorf("编码回收站路径：%w", err)
	}
	operation := shFileOperation{
		Function: 3, // FO_DELETE
		From:     from,
		Flags:    0x0040 | 0x0010 | 0x0400, // FOF_ALLOWUNDO | FOF_NOCONFIRMATION | FOF_NOERRORUI
	}
	result, _, callErr := shFileOperationW.Call(uintptr(unsafe.Pointer(&operation)))
	if result != 0 {
		return fmt.Errorf("移动到回收站失败，系统错误码 %d：%w", result, callErr)
	}
	if operation.AnyOperationsAborted != 0 {
		return errors.New("移动到回收站的操作已取消")
	}
	return nil
}
