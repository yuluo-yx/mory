package windowshost

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePlatform struct {
	mu              sync.Mutex
	chosenDirectory string
	chosenFile      string
	savePath        string
	confirmed       bool
	scripts         []string
	titles          []string
	locales         []string
	exports         []ExportRequest
	maximised       int
	revealed        []string
	opened          []string
}

func (platform *fakePlatform) ChooseDirectory(string) (string, error) {
	return platform.chosenDirectory, nil
}
func (platform *fakePlatform) ChooseFile(string, []string) (string, error) {
	return platform.chosenFile, nil
}
func (platform *fakePlatform) ChooseSavePath(string, []string) (string, error) {
	return platform.savePath, nil
}
func (platform *fakePlatform) Confirm(string, string, string) (bool, error) {
	return platform.confirmed, nil
}
func (platform *fakePlatform) Trash(path string) error { return os.RemoveAll(path) }
func (platform *fakePlatform) Reveal(path string) error {
	platform.revealed = append(platform.revealed, path)
	return nil
}
func (platform *fakePlatform) OpenDirectory(path string) error {
	platform.opened = append(platform.opened, path)
	return nil
}
func (platform *fakePlatform) Evaluate(script string) {
	platform.mu.Lock()
	platform.scripts = append(platform.scripts, script)
	platform.mu.Unlock()
}
func (platform *fakePlatform) SetTitle(title string) {
	platform.mu.Lock()
	platform.titles = append(platform.titles, title)
	platform.mu.Unlock()
}
func (platform *fakePlatform) SetLocale(locale string) {
	platform.mu.Lock()
	platform.locales = append(platform.locales, locale)
	platform.mu.Unlock()
}
func (platform *fakePlatform) ToggleMaximise() { platform.maximised++ }
func (platform *fakePlatform) Export(request ExportRequest) error {
	platform.exports = append(platform.exports, request)
	return nil
}

func TestHostBridgesReadyOpenChangeLocaleAndExport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "01.md"), "# 你好", time.Now())
	platform := &fakePlatform{}
	host := New(platform, t.TempDir(), root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := host.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()

	if err := host.Send(map[string]any{"type": "ready"}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "openFile", "path": filepath.Join(root, "01.md")}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "changed", "markdown": "# 已修改", "name": "01.md"}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "localeChanged", "locale": "en"}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "windowTitlebarDoubleClick"}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "export", "options": map[string]any{"format": "html", "html": "<h1>ok</h1>"}}); err != nil {
		t.Fatal(err)
	}

	platform.mu.Lock()
	defer platform.mu.Unlock()
	joined := strings.Join(platform.scripts, "\n")
	if !strings.Contains(joined, "window.Mory.setWorkspaceSnapshot") || !strings.Contains(joined, "window.Mory.openDocument") {
		t.Fatalf("前端调用不完整：%s", joined)
	}
	if len(platform.locales) != 1 || platform.locales[0] != "en" {
		t.Fatalf("菜单语言未更新：%v", platform.locales)
	}
	if platform.maximised != 1 {
		t.Fatalf("最大化次数 = %d", platform.maximised)
	}
	if len(platform.exports) != 1 || platform.exports[0].HTML == "" {
		t.Fatalf("导出任务错误：%#v", platform.exports)
	}
}

func TestHostSavesUntitledDocumentInsideExplicitWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	platform := &fakePlatform{}
	host := New(platform, t.TempDir(), filepath.Join(t.TempDir(), "implicit"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := host.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()

	_, err := host.Request("saveWorkspace", map[string]any{"workspace": map[string]any{
		"name": "Notes", "provider": "local", "localPath": root,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "documentSelected", "name": "未命名.md", "markdown": "# 中文标题", "dirty": true}); err != nil {
		t.Fatal(err)
	}
	if err := host.Save(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "中文标题.md")); err != nil || string(data) != "# 中文标题" {
		t.Fatalf("草稿没有写入显式工作区：%q，%v", data, err)
	}
}

func TestHostDeleteDocumentUsesConfirmation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "delete.md")
	writeAt(t, path, "delete", time.Now())
	writeAt(t, filepath.Join(root, "delete", "image.png"), "image", time.Now())
	platform := &fakePlatform{confirmed: true}
	host := New(platform, t.TempDir(), root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := host.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()
	result, err := host.Request("deleteDocument", map[string]any{"path": path, "name": "delete.md"})
	if err != nil {
		t.Fatal(err)
	}
	if result.(map[string]bool)["deleted"] != true {
		t.Fatalf("删除结果：%#v", result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("文稿仍存在：%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "delete")); !os.IsNotExist(err) {
		t.Fatalf("文稿图片目录仍存在：%v", err)
	}

	directory := filepath.Join(root, "directory")
	writeAt(t, filepath.Join(directory, "nested.md"), "nested", time.Now())
	result, err = host.Request("deleteWorkspaceEntry", map[string]any{"path": directory, "name": "directory"})
	if err != nil || result.(map[string]bool)["deleted"] != true {
		t.Fatalf("删除目录：%#v，%v", result, err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("目录仍存在：%v", err)
	}
}

func TestHostWorkspaceRequestMatrixAndMenuActions(t *testing.T) {
	root := t.TempDir()
	documentPath := filepath.Join(root, "note.md")
	writeAt(t, documentPath, "# note\n![x](note/p.png)", time.Now())
	writeAt(t, filepath.Join(root, "note", "p.png"), string([]byte{0x89, 'P', 'N', 'G'}), time.Now())
	themeSource := filepath.Join(t.TempDir(), "custom.css")
	writeAt(t, themeSource, ".write{color:red}", time.Now())
	platform := &fakePlatform{chosenDirectory: root, chosenFile: documentPath, savePath: filepath.Join(root, "saved.md")}
	host := New(platform, t.TempDir(), filepath.Join(t.TempDir(), "implicit"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := host.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()

	if _, err := host.Request("workspaceState", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Request("chooseLocalWorkspace", map[string]any{"name": "Root"}); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Request("createDirectory", map[string]any{"relativePath": "nested/folder"}); err != nil {
		t.Fatal(err)
	}
	created, err := host.Request("createDocument", map[string]any{
		"directoryPath": filepath.Join(root, "nested", "folder"), "name": "created.md",
	})
	if err != nil || filepath.Dir(created.(Document).Path) != filepath.Join(root, "nested", "folder") {
		t.Fatalf("在所选目录创建文稿：%#v，%v", created, err)
	}
	copied, err := host.Request("copyWorkspaceEntry", map[string]any{
		"path": created.(Document).Path, "destinationPath": root,
	})
	if err != nil || copied.(WorkspaceMutation).Path == "" {
		t.Fatalf("复制文稿：%#v，%v", copied, err)
	}
	moved, err := host.Request("moveWorkspaceEntry", map[string]any{
		"path": copied.(WorkspaceMutation).Path, "destinationPath": filepath.Join(root, "nested"),
	})
	if err != nil || filepath.Dir(moved.(WorkspaceMutation).Path) != filepath.Join(root, "nested") {
		t.Fatalf("移动文稿：%#v，%v", moved, err)
	}
	if _, err := host.Request("syncWorkspace", map[string]any{"action": "pull"}); err != nil {
		t.Fatal(err)
	}
	if err := host.OpenFile(documentPath); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Request("documentAssets", map[string]any{"markdown": "![x](note/p.png)"}); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Request("documentImage", map[string]any{"path": filepath.Join(root, "note", "p.png")}); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Request("revealFile", map[string]any{"path": documentPath}); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Request("readDocument", map[string]any{"path": documentPath}); err != nil {
		t.Fatal(err)
	}
	if documents, err := host.Request("workspaceDocuments", nil); err != nil || len(documents.([]Document)) != 3 {
		t.Fatalf("工作区文稿：%#v，%v", documents, err)
	}
	if _, err := host.Request("listThemes", nil); err != nil {
		t.Fatal(err)
	}

	platform.chosenFile = themeSource
	if _, err := host.Request("importTheme", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Request("openThemeFolder", nil); err != nil {
		t.Fatal(err)
	}
	newThemeDirectory := filepath.Join(t.TempDir(), "theme-dir")
	platform.chosenDirectory = newThemeDirectory
	if _, err := host.Request("chooseThemeFolder", nil); err != nil {
		t.Fatal(err)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G'})
	if _, err := host.Request("importImage", map[string]any{
		"documentPath": documentPath, "documentName": "note.md", "name": "new.png", "mime": "image/png", "data": encoded,
	}); err != nil {
		t.Fatal(err)
	}

	platform.chosenFile = documentPath
	if err := host.OpenDocument(); err != nil {
		t.Fatal(err)
	}
	platform.chosenDirectory = filepath.Join(t.TempDir(), "opened-workspace")
	if err := host.OpenFolder(); err != nil {
		t.Fatal(err)
	}
	host.NewDocument()
	if err := host.Send(map[string]any{"type": "changed", "markdown": "# saved", "name": "未命名.md"}); err != nil {
		t.Fatal(err)
	}
	platform.savePath = filepath.Join(platform.chosenDirectory, "saved.md")
	if err := host.SaveAs(); err != nil {
		t.Fatal(err)
	}
	host.Evaluate("window.Mory.showFind()")
	if _, err := host.Request("unknown", nil); err == nil {
		t.Fatal("未知请求应返回错误")
	}
}

func TestHostCancellationValidationAndRemainingMessages(t *testing.T) {
	root := t.TempDir()
	platform := &fakePlatform{}
	host := New(platform, t.TempDir(), root)
	ctx, cancel := context.WithCancel(context.Background())
	if err := host.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()

	if err := host.Send(map[string]any{"type": "documentSelected", "name": "草稿.md", "markdown": "正文"}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "title", "value": "新标题"}); err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"windowDragStart", "windowDragMove", "windowDragEnd"} {
		if err := host.Send(map[string]any{"type": message}); err != nil {
			t.Fatal(err)
		}
	}
	if err := host.Send(map[string]any{"type": "unknown"}); err == nil {
		t.Fatal("未知消息应失败")
	}
	if err := host.Send(map[string]any{"type": "export", "options": "bad"}); err == nil {
		t.Fatal("无效导出参数应失败")
	}

	if result, err := host.Request("chooseLocalWorkspace", nil); err != nil || result.(map[string]bool)["canceled"] != true {
		t.Fatalf("取消目录：%#v，%v", result, err)
	}
	if result, err := host.Request("importTheme", nil); err != nil || result.(map[string]bool)["canceled"] != true {
		t.Fatalf("取消主题：%#v，%v", result, err)
	}
	if result, err := host.Request("chooseThemeFolder", nil); err != nil || result.(map[string]bool)["canceled"] != true {
		t.Fatalf("取消主题目录：%#v，%v", result, err)
	}
	if assets, err := host.Request("documentAssets", map[string]any{"markdown": ""}); err != nil || len(assets.(map[string]string)) != 0 {
		t.Fatalf("草稿资源：%#v，%v", assets, err)
	}
	if _, err := host.Request("saveWorkspace", map[string]any{"workspace": "bad"}); err == nil {
		t.Fatal("无效工作区载荷应失败")
	}
	if _, err := host.Request("activateWorkspace", map[string]any{"id": "missing"}); err == nil {
		t.Fatal("不存在工作区不应激活")
	}
	if _, err := host.Request("removeWorkspace", map[string]any{"id": host.workspaces.state().ActiveID}); err == nil {
		t.Fatal("不应删除唯一工作区")
	}
	if _, err := host.Request("readDocument", map[string]any{"path": filepath.Join(t.TempDir(), "outside.md")}); err == nil {
		t.Fatal("不应读取工作区外文稿")
	}
	if _, err := host.Request("documentImage", map[string]any{"path": filepath.Join(root, "bad.txt")}); err == nil {
		t.Fatal("不应读取非图片")
	}
	if _, err := host.Request("importImage", map[string]any{"mime": "image/unknown", "data": "bad"}); err == nil {
		t.Fatal("不应导入未知图片")
	}

	platform.chosenFile = filepath.Join(t.TempDir(), "outside.md")
	writeAt(t, platform.chosenFile, "outside", time.Now())
	if err := host.OpenDocument(); err != nil {
		t.Fatalf("系统打开应允许工作区外文件：%v", err)
	}
	platform.chosenFile = ""
	if err := host.OpenDocument(); err != nil {
		t.Fatalf("取消打开不应报错：%v", err)
	}
	platform.chosenDirectory = ""
	if err := host.OpenFolder(); err != nil {
		t.Fatalf("取消打开目录不应报错：%v", err)
	}
	platform.savePath = ""
	if err := host.SaveAs(); err != nil {
		t.Fatalf("取消另存为不应报错：%v", err)
	}

	cancel()
}

func TestHostWatcherRefreshesAfterExternalChange(t *testing.T) {
	root := t.TempDir()
	platform := &fakePlatform{}
	host := New(platform, t.TempDir(), root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := host.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()
	if err := host.Send(map[string]any{"type": "ready"}); err != nil {
		t.Fatal(err)
	}
	platform.mu.Lock()
	before := len(platform.scripts)
	platform.mu.Unlock()
	writeAt(t, filepath.Join(root, "external.md"), "# external", time.Now())
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		platform.mu.Lock()
		refreshed := len(platform.scripts) > before
		platform.mu.Unlock()
		if refreshed {
			return
		}
	}
	t.Fatal("外部文件变化没有触发工作区快照")
}
