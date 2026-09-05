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
	mu               sync.Mutex
	chosenDirectory  string
	chosenFile       string
	savePath         string
	draftDestination string
	draftPrompts     int
	confirmed        bool
	scripts          []string
	titles           []string
	locales          []string
	exports          []ExportRequest
	maximised        int
	aboutLocales     []string
	revealed         []string
	opened           []string
	urls             []string
	copiedTexts      []string
	recent           []string
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
func (platform *fakePlatform) ChooseDraftSaveDestination(string) (string, error) {
	platform.draftPrompts++
	return platform.draftDestination, nil
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
func (platform *fakePlatform) OpenURL(url string) error {
	platform.urls = append(platform.urls, url)
	return nil
}
func (platform *fakePlatform) CopyText(text string) error {
	platform.copiedTexts = append(platform.copiedTexts, text)
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
func (platform *fakePlatform) NoteRecentDocument(path string) {
	platform.mu.Lock()
	platform.recent = append(platform.recent, path)
	platform.mu.Unlock()
}
func (platform *fakePlatform) ToggleMaximise() { platform.maximised++ }
func (platform *fakePlatform) ShowAbout(locale string) {
	platform.aboutLocales = append(platform.aboutLocales, locale)
}
func (platform *fakePlatform) Export(request ExportRequest) error {
	platform.exports = append(platform.exports, request)
	return nil
}

func TestHostBridgesReadyOpenChangeLocaleAndExport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "01.md"), "# \u4F60\u597D", time.Now())
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
	if err := host.Send(map[string]any{"type": "changed", "markdown": "# \u5DF2\u4FEE\u6539", "name": "01.md"}); err != nil {
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
		t.Fatalf("incomplete renderer calls: %s", joined)
	}
	if len(platform.locales) != 1 || platform.locales[0] != "en" {
		t.Fatalf("menu locale was not updated: %v", platform.locales)
	}
	if platform.maximised != 1 {
		t.Fatalf("maximize count = %d", platform.maximised)
	}
	host.ShowAbout()
	if len(platform.aboutLocales) != 1 || platform.aboutLocales[0] != "en" {
		t.Fatalf("about locale = %v", platform.aboutLocales)
	}
	if len(platform.exports) != 1 || platform.exports[0].HTML == "" {
		t.Fatalf("invalid export tasks: %#v", platform.exports)
	}
	if len(platform.recent) != 1 || platform.recent[0] != filepath.Join(root, "01.md") {
		t.Fatalf("recent documents = %#v", platform.recent)
	}
}

func TestHostPassesCurrentMarkdownAndSourcePathToPowerPointExport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	document := filepath.Join(root, "slides.md")
	writeAt(t, document, "# Original", time.Now())
	platform := &fakePlatform{}
	host := New(platform, t.TempDir(), root)
	if err := host.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()
	if err := host.OpenExternalFile(document); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "changed", "markdown": "# Updated", "name": "slides.md"}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "export", "options": map[string]any{"format": "pptx"}}); err != nil {
		t.Fatal(err)
	}
	if len(platform.exports) != 1 || platform.exports[0].Markdown != "# Updated" || platform.exports[0].SourcePath != document {
		t.Fatalf("PowerPoint export = %#v", platform.exports)
	}
}

func TestHostProcessesFileAssociationAndCLIExportAfterReady(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	document := filepath.Join(t.TempDir(), "outside.md")
	writeAt(t, document, "# Outside", time.Now())
	platform := &fakePlatform{}
	host := New(platform, t.TempDir(), root)
	completed := make(chan error, 1)
	host.ConfigureStartup(document, &StartupExport{Format: "html", Destination: filepath.Join(root, "outside.html")}, func(err error) {
		completed <- err
	})
	if err := host.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()

	if err := host.Send(map[string]any{"type": "ready"}); err != nil {
		t.Fatal(err)
	}
	platform.mu.Lock()
	scripts := strings.Join(platform.scripts, "\n")
	platform.mu.Unlock()
	if !strings.Contains(scripts, "window.Mory.openDocument") || !strings.Contains(scripts, "window.Mory.exportToHost") {
		t.Fatalf("startup scripts = %s", scripts)
	}
	if err := host.Send(map[string]any{"type": "export", "options": map[string]any{
		"format": "html", "html": "<h1>Outside</h1>", "destination": filepath.Join(root, "outside.html"),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if len(platform.exports) != 1 || platform.exports[0].Destination == "" {
		t.Fatalf("CLI exports = %#v", platform.exports)
	}
}

func TestHostAsksBeforeSavingUntitledDocumentInsideExplicitWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	platform := &fakePlatform{draftDestination: "workspace"}
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
	if err := host.Send(map[string]any{"type": "documentSelected", "name": "\u672A\u547D\u540D.md", "markdown": "# \u4E2D\u6587\u6807\u9898", "dirty": true}); err != nil {
		t.Fatal(err)
	}
	if err := host.Save(); err != nil {
		t.Fatal(err)
	}
	if platform.draftPrompts != 1 {
		t.Fatalf("draft save prompts = %d, want 1", platform.draftPrompts)
	}
	if data, err := os.ReadFile(filepath.Join(root, "\u4E2D\u6587\u6807\u9898.md")); err != nil || string(data) != "# \u4E2D\u6587\u6807\u9898" {
		t.Fatalf("draft was not written to the explicit workspace: %q, %v", data, err)
	}

	host.NewDocument()
	if err := host.Send(map[string]any{"type": "changed", "markdown": "elsewhere", "name": "draft.md"}); err != nil {
		t.Fatal(err)
	}
	platform.draftDestination = "elsewhere"
	platform.savePath = filepath.Join(t.TempDir(), "elsewhere.md")
	if err := host.Save(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(platform.savePath); err != nil || string(data) != "elsewhere" {
		t.Fatalf("draft was not written to the chosen location: %q, %v", data, err)
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
		t.Fatalf("delete result: %#v", result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("document still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "delete")); !os.IsNotExist(err) {
		t.Fatalf("document image directory still exists: %v", err)
	}

	directory := filepath.Join(root, "directory")
	writeAt(t, filepath.Join(directory, "nested.md"), "nested", time.Now())
	result, err = host.Request("deleteWorkspaceEntry", map[string]any{"path": directory, "name": "directory"})
	if err != nil || result.(map[string]bool)["deleted"] != true {
		t.Fatalf("delete directory: %#v, %v", result, err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("directory still exists: %v", err)
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
		t.Fatalf("create document in selected directory: %#v, %v", created, err)
	}
	copied, err := host.Request("copyWorkspaceEntry", map[string]any{
		"path": created.(Document).Path, "destinationPath": root,
	})
	if err != nil || copied.(WorkspaceMutation).Path == "" {
		t.Fatalf("copy document: %#v, %v", copied, err)
	}
	moved, err := host.Request("moveWorkspaceEntry", map[string]any{
		"path": copied.(WorkspaceMutation).Path, "destinationPath": filepath.Join(root, "nested"),
	})
	if err != nil || filepath.Dir(moved.(WorkspaceMutation).Path) != filepath.Join(root, "nested") {
		t.Fatalf("move document: %#v, %v", moved, err)
	}
	renamed, err := host.Request("renameWorkspaceEntry", map[string]any{
		"path": moved.(WorkspaceMutation).Path, "name": "renamed.md",
	})
	if err != nil || filepath.Base(renamed.(WorkspaceMutation).Path) != "renamed.md" {
		t.Fatalf("rename document: %#v, %v", renamed, err)
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
	if _, err := host.Request("openExternal", map[string]any{"url": "https://example.com/docs"}); err != nil || len(platform.urls) != 1 {
		t.Fatalf("open external URL: %v, %#v", err, platform.urls)
	}
	if _, err := host.Request("openExternal", map[string]any{"url": "file:///outside"}); err == nil {
		t.Fatal("unsafe external URL should fail")
	}
	if _, err := host.Request("copyText", map[string]any{"text": "nested/note.md"}); err != nil || len(platform.copiedTexts) != 1 || platform.copiedTexts[0] != "nested/note.md" {
		t.Fatalf("copy text: %v, %#v", err, platform.copiedTexts)
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
		t.Fatalf("workspace documents: %#v, %v", documents, err)
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
	if err := os.Mkdir(platform.chosenDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := host.OpenFolder(); err != nil {
		t.Fatal(err)
	}
	host.NewDocument()
	if err := host.Send(map[string]any{"type": "changed", "markdown": "# saved", "name": "\u672A\u547D\u540D.md"}); err != nil {
		t.Fatal(err)
	}
	platform.savePath = filepath.Join(platform.chosenDirectory, "saved.md")
	if err := host.SaveAs(); err != nil {
		t.Fatal(err)
	}
	host.Evaluate("window.Mory.showFind()")
	if _, err := host.Request("unknown", nil); err == nil {
		t.Fatal("unknown request should return an error")
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

	if err := host.Send(map[string]any{"type": "documentSelected", "name": "\u8349\u7A3F.md", "markdown": "\u6B63\u6587"}); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(map[string]any{"type": "title", "value": "\u65B0\u6807\u9898"}); err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"windowDragStart", "windowDragMove", "windowDragEnd"} {
		if err := host.Send(map[string]any{"type": message}); err != nil {
			t.Fatal(err)
		}
	}
	if err := host.Send(map[string]any{"type": "unknown"}); err == nil {
		t.Fatal("unknown message should fail")
	}
	if err := host.Send(map[string]any{"type": "export", "options": "bad"}); err == nil {
		t.Fatal("invalid export arguments should fail")
	}

	if result, err := host.Request("chooseLocalWorkspace", nil); err != nil || result.(map[string]bool)["canceled"] != true {
		t.Fatalf("cancel directory selection: %#v, %v", result, err)
	}
	if result, err := host.Request("importTheme", nil); err != nil || result.(map[string]bool)["canceled"] != true {
		t.Fatalf("cancel theme selection: %#v, %v", result, err)
	}
	if result, err := host.Request("chooseThemeFolder", nil); err != nil || result.(map[string]bool)["canceled"] != true {
		t.Fatalf("cancel theme-folder selection: %#v, %v", result, err)
	}
	if assets, err := host.Request("documentAssets", map[string]any{"markdown": ""}); err != nil || len(assets.(map[string]string)) != 0 {
		t.Fatalf("draft assets: %#v, %v", assets, err)
	}
	if _, err := host.Request("saveWorkspace", map[string]any{"workspace": "bad"}); err == nil {
		t.Fatal("invalid workspace payload should fail")
	}
	if _, err := host.Request("activateWorkspace", map[string]any{"id": "missing"}); err == nil {
		t.Fatal("missing workspace should not activate")
	}
	if _, err := host.Request("removeWorkspace", map[string]any{"id": host.workspaces.state().ActiveID}); err == nil {
		t.Fatal("the only workspace should not be deleted")
	}
	if _, err := host.Request("readDocument", map[string]any{"path": filepath.Join(t.TempDir(), "outside.md")}); err == nil {
		t.Fatal("documents outside the workspace should not be read")
	}
	if _, err := host.Request("documentImage", map[string]any{"path": filepath.Join(root, "bad.txt")}); err == nil {
		t.Fatal("non-image files should not be read as images")
	}
	if _, err := host.Request("importImage", map[string]any{"mime": "image/unknown", "data": "bad"}); err == nil {
		t.Fatal("unknown image types should not be imported")
	}

	platform.chosenFile = filepath.Join(t.TempDir(), "outside.md")
	writeAt(t, platform.chosenFile, "outside", time.Now())
	if err := host.OpenDocument(); err != nil {
		t.Fatalf("system open should allow files outside the workspace: %v", err)
	}
	platform.chosenFile = ""
	if err := host.OpenDocument(); err != nil {
		t.Fatalf("cancelling open should not return an error: %v", err)
	}
	platform.chosenDirectory = ""
	if err := host.OpenFolder(); err != nil {
		t.Fatalf("cancelling folder open should not return an error: %v", err)
	}
	platform.savePath = ""
	if err := host.SaveAs(); err != nil {
		t.Fatalf("cancelling Save As should not return an error: %v", err)
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
	t.Fatal("external file changes did not trigger a workspace snapshot")
}
