package windowshost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ExportRequest is the rendered document payload handed to WebView2 for export.
type ExportRequest struct {
	Format     string `json:"format"`
	Theme      string `json:"theme"`
	Paper      string `json:"paper"`
	Width      int    `json:"width"`
	Background bool   `json:"background"`
	HTML       string `json:"html"`
	Name       string `json:"name"`
}

// Platform isolates Wails runtime services so host behavior can be tested on other platforms.
type Platform interface {
	ChooseDirectory(defaultDirectory string) (string, error)
	ChooseFile(defaultDirectory string, extensions []string) (string, error)
	ChooseSavePath(defaultPath string, extensions []string) (string, error)
	Confirm(title, message, detail string) (bool, error)
	Trash(path string) error
	Reveal(path string) error
	OpenDirectory(path string) error
	Evaluate(script string)
	SetTitle(title string)
	SetLocale(locale string)
	ToggleMaximise()
	Export(ExportRequest) error
}

// Host implements the stable protocol between the frontend and the Windows WebView2 host.
type Host struct {
	mu              sync.RWMutex
	platform        Platform
	workspaces      *workspaceManager
	themes          *themeManager
	ctx             context.Context
	cancel          context.CancelFunc
	currentPath     string
	currentMarkdown string
	currentName     string
	locale          string
	exporting       bool
	watchRoot       string
	watchSignature  string
}

// New creates the Windows host core. Call Start before processing frontend requests.
func New(platform Platform, userDataPath, defaultWorkspace string) *Host {
	return &Host{
		platform:    platform,
		workspaces:  newWorkspaceManager(userDataPath, defaultWorkspace),
		themes:      newThemeManager(userDataPath),
		currentName: "未命名.md",
		locale:      "zh-CN",
	}
}

// Start initializes persisted state and begins polling for workspace changes.
func (host *Host) Start(parent context.Context) error {
	if err := host.workspaces.initialize(); err != nil {
		return err
	}
	if err := host.themes.initialize(); err != nil {
		return err
	}
	host.ctx, host.cancel = context.WithCancel(parent)
	go host.watchWorkspace()
	return nil
}

// Stop terminates background workspace monitoring.
func (host *Host) Stop() {
	if host.cancel != nil {
		host.cancel()
	}
}

// Send handles editor events that do not require a return value.
func (host *Host) Send(payload map[string]any) error {
	typeName := stringValue(payload, "type")
	switch typeName {
	case "ready":
		return host.refreshWorkspace()
	case "changed":
		host.mu.Lock()
		if markdown, ok := payload["markdown"].(string); ok {
			host.currentMarkdown = markdown
		}
		if name, ok := payload["name"].(string); ok && name != "" {
			host.currentName = name
		}
		title := documentTitle(host.currentPath, host.currentName)
		host.mu.Unlock()
		host.platform.SetTitle("● " + title + " — Mory")
		return nil
	case "documentSelected":
		host.selectDocument(payload)
		return nil
	case "openFile":
		return host.OpenFile(stringValue(payload, "path"))
	case "title":
		host.mu.RLock()
		path := host.currentPath
		name := host.currentName
		host.mu.RUnlock()
		if path == "" {
			value := stringValue(payload, "value")
			if value == "" {
				value = strings.TrimSuffix(name, filepath.Ext(name))
			}
			host.platform.SetTitle(value + " — Mory")
		}
		return nil
	case "export":
		var request ExportRequest
		if err := decodeValue(payload["options"], &request); err != nil {
			return fmt.Errorf("解析导出参数：%w", err)
		}
		host.mu.Lock()
		if host.exporting {
			host.mu.Unlock()
			host.platform.Evaluate("window.Mory.exportBusy()")
			return nil
		}
		host.exporting = true
		host.mu.Unlock()
		defer func() {
			host.mu.Lock()
			host.exporting = false
			host.mu.Unlock()
		}()
		if request.Name == "" {
			host.mu.RLock()
			request.Name = strings.TrimSuffix(host.currentName, filepath.Ext(host.currentName))
			host.mu.RUnlock()
		}
		host.evaluate("window.Mory.exportStarted", request.Format)
		if err := host.platform.Export(request); err != nil {
			return err
		}
		host.evaluate("window.Mory.didExport", request.Format)
		return nil
	case "localeChanged":
		locale := "zh-CN"
		if stringValue(payload, "locale") == "en" {
			locale = "en"
		}
		host.mu.Lock()
		host.locale = locale
		host.mu.Unlock()
		host.platform.SetLocale(locale)
		return nil
	case "windowTitlebarDoubleClick":
		host.platform.ToggleMaximise()
		return nil
	case "windowDragStart", "windowDragMove", "windowDragEnd":
		// Windows uses Wails CSS drag regions and does not need manual screen-coordinate handling.
		return nil
	default:
		return fmt.Errorf("未知宿主消息：%s", typeName)
	}
}

// Request handles workspace operations that return a result.
func (host *Host) Request(method string, args map[string]any) (any, error) {
	switch method {
	case "workspaceState":
		return host.workspaces.state(), nil
	case "chooseLocalWorkspace":
		chosen, err := host.platform.ChooseDirectory(host.workspaces.activeRoot())
		if err != nil || chosen == "" {
			return map[string]bool{"canceled": true}, err
		}
		workspace := Workspace{}
		workspace.ID = stringValue(args, "id")
		workspace.Name = stringValue(args, "name")
		if workspace.Name == "" {
			workspace.Name = filepath.Base(chosen)
		}
		workspace.Provider = "local"
		workspace.LocalPath = chosen
		state, err := host.workspaces.save(workspace)
		if err == nil {
			err = host.refreshWorkspace()
		}
		return state, err
	case "saveWorkspace":
		var workspace Workspace
		if err := decodeValue(args["workspace"], &workspace); err != nil {
			return nil, fmt.Errorf("解析工作区设置：%w", err)
		}
		state, err := host.workspaces.save(workspace)
		if err == nil {
			err = host.refreshWorkspace()
		}
		return state, err
	case "activateWorkspace":
		state, err := host.workspaces.activate(stringValue(args, "id"))
		if err == nil {
			err = host.refreshWorkspace()
		}
		return state, err
	case "removeWorkspace":
		state, err := host.workspaces.remove(stringValue(args, "id"))
		if err == nil {
			err = host.refreshWorkspace()
		}
		return state, err
	case "deleteDocument":
		return host.deleteDocument(stringValue(args, "path"), stringValue(args, "name"))
	case "deleteWorkspaceEntry":
		return host.deleteWorkspaceEntry(stringValue(args, "path"), stringValue(args, "name"))
	case "createDirectory":
		directory, err := createWorkspaceDirectory(host.workspaces.activeRoot(), stringValue(args, "relativePath"))
		if err == nil {
			err = host.refreshWorkspace()
		}
		return directory, err
	case "createDocument":
		document, err := createWorkspaceDocument(host.workspaces.activeRoot(), stringValue(args, "directoryPath"), stringValue(args, "name"))
		if err == nil {
			err = host.refreshWorkspace()
		}
		return document, err
	case "copyWorkspaceEntry":
		result, err := copyWorkspaceEntry(host.workspaces.activeRoot(), stringValue(args, "path"), stringValue(args, "destinationPath"))
		if err == nil {
			err = host.refreshWorkspace()
		}
		return result, err
	case "moveWorkspaceEntry":
		result, err := moveWorkspaceEntry(host.workspaces.activeRoot(), stringValue(args, "path"), stringValue(args, "destinationPath"))
		if err == nil {
			err = host.refreshWorkspace()
		}
		return result, err
	case "renameWorkspaceEntry":
		result, err := renameWorkspaceEntry(host.workspaces.activeRoot(), stringValue(args, "path"), stringValue(args, "name"))
		if err == nil {
			err = host.refreshWorkspace()
		}
		return result, err
	case "syncWorkspace":
		action := "pull"
		if stringValue(args, "action") == "push" {
			action = "push"
		}
		summary, err := host.workspaces.syncWorkspace(host.ctx, action)
		if err == nil {
			err = host.refreshWorkspace()
		}
		return summary, err
	case "importImage":
		result, err := importImage(host.workspaces.activeRoot(), stringValue(args, "documentPath"), stringValue(args, "documentName"), stringValue(args, "name"), stringValue(args, "mime"), stringValue(args, "data"))
		if err == nil {
			_ = host.refreshWorkspace()
		}
		return result, err
	case "documentAssets":
		host.mu.RLock()
		path := host.currentPath
		host.mu.RUnlock()
		if path == "" {
			return map[string]string{}, nil
		}
		return loadDocumentAssets(path, stringValue(args, "markdown")), nil
	case "documentImage":
		return readDocumentImage(host.workspaces.activeRoot(), stringValue(args, "path"))
	case "revealFile":
		path, err := safeExistingPath(host.workspaces.activeRoot(), stringValue(args, "path"))
		if err != nil {
			return nil, err
		}
		return map[string]bool{"revealed": true}, host.platform.Reveal(path)
	case "readDocument":
		path, err := safeExistingPath(host.workspaces.activeRoot(), stringValue(args, "path"))
		if err != nil {
			return nil, err
		}
		return loadDocument(path)
	case "workspaceDocuments":
		return listDocuments(host.workspaces.activeRoot(), true)
	case "listThemes":
		return host.themes.list()
	case "importTheme":
		path, err := host.platform.ChooseFile(host.themes.path(), []string{"css"})
		if err != nil || path == "" {
			return map[string]bool{"canceled": true}, err
		}
		themes, err := host.themes.importFile(path)
		return map[string]any{"themes": themes}, err
	case "openThemeFolder":
		return map[string]bool{"opened": true}, host.platform.OpenDirectory(host.themes.path())
	case "chooseThemeFolder":
		path, err := host.platform.ChooseDirectory(host.themes.path())
		if err != nil || path == "" {
			return map[string]bool{"canceled": true}, err
		}
		return host.themes.setDirectory(path)
	default:
		return nil, fmt.Errorf("未知宿主请求：%s", method)
	}
}

// OpenFile loads a note from disk and sends it to the frontend.
func (host *Host) OpenFile(path string) error {
	return host.openFile(path, true)
}

func (host *Host) openFile(path string, requireWorkspace bool) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("文稿路径为空")
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析文稿路径：%w", err)
	}
	if requireWorkspace {
		if _, err := safeDescendant(host.workspaces.activeRoot(), resolved); err != nil {
			return err
		}
	}
	document, err := loadDocument(resolved)
	if err != nil {
		return err
	}
	host.mu.Lock()
	host.currentPath = resolved
	host.currentMarkdown = document.Markdown
	host.currentName = filepath.Base(resolved)
	host.mu.Unlock()
	host.platform.SetTitle(filepath.Base(resolved) + " — Mory")
	host.evaluate("window.Mory.openDocument", document)
	return nil
}

// OpenDocument displays the system file picker.
func (host *Host) OpenDocument() error {
	path, err := host.platform.ChooseFile(host.workspaces.activeRoot(), []string{"md", "markdown", "mmd", "mdown", "mkd", "txt", "text"})
	if err != nil || path == "" {
		return err
	}
	return host.openFile(path, false)
}

// OpenFolder persists the selected directory as the active local workspace.
func (host *Host) OpenFolder() error {
	path, err := host.platform.ChooseDirectory(host.workspaces.activeRoot())
	if err != nil || path == "" {
		return err
	}
	workspace := Workspace{LocalPath: path}
	workspace.Provider = "local"
	workspace.Name = filepath.Base(path)
	if _, err := host.workspaces.save(workspace); err != nil {
		return err
	}
	return host.refreshWorkspace()
}

// NewDocument clears the active host path and asks the frontend to create a standalone draft.
func (host *Host) NewDocument() {
	host.mu.Lock()
	host.currentPath = ""
	host.currentMarkdown = ""
	host.currentName = "未命名.md"
	host.mu.Unlock()
	host.platform.SetTitle("未命名 — Mory")
	host.platform.Evaluate("window.Mory.newDocument()")
}

// Save writes the active note and assigns a collision-free name to drafts in explicit workspaces.
func (host *Host) Save() error {
	host.mu.RLock()
	path, markdown, name := host.currentPath, host.currentMarkdown, host.currentName
	host.mu.RUnlock()
	if path == "" {
		if host.workspaces.active().IsImplicit {
			return host.SaveAs()
		}
		path = availableDocumentPath(host.workspaces.activeRoot(), suggestedDocumentName(markdown, name))
	}
	return host.writeDocument(path, markdown)
}

// SaveAs displays the system Save As dialog.
func (host *Host) SaveAs() error {
	host.mu.RLock()
	path, markdown, name := host.currentPath, host.currentMarkdown, host.currentName
	host.mu.RUnlock()
	if path == "" {
		path = filepath.Join(host.workspaces.activeRoot(), suggestedDocumentName(markdown, name))
	}
	chosen, err := host.platform.ChooseSavePath(path, []string{"md"})
	if err != nil || chosen == "" {
		return err
	}
	return host.writeDocument(chosen, markdown)
}

// Evaluate invokes a public frontend command for menu actions and keyboard shortcuts.
func (host *Host) Evaluate(script string) { host.platform.Evaluate(script) }

func (host *Host) writeDocument(path, markdown string) error {
	host.mu.RLock()
	oldPath, oldName := host.currentPath, host.currentName
	host.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建文稿目录：%w", err)
	}
	var err error
	markdown, err = relocateDocumentAssets(host.workspaces.activeRoot(), markdown, oldPath, oldName, path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("保存文稿：%w", err)
	}
	document, err := loadDocument(path)
	if err != nil {
		return err
	}
	host.mu.Lock()
	host.currentPath = path
	host.currentName = filepath.Base(path)
	host.currentMarkdown = markdown
	host.mu.Unlock()
	host.platform.SetTitle(filepath.Base(path) + " — Mory")
	host.evaluate("window.Mory.didSave", document)
	return host.refreshWorkspace()
}

func (host *Host) deleteDocument(path, name string) (any, error) {
	return host.deleteWorkspaceEntry(path, name)
}

func (host *Host) deleteWorkspaceEntry(path, name string) (any, error) {
	resolved, err := safeExistingPath(host.workspaces.activeRoot(), path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = filepath.Base(resolved)
	}
	host.mu.RLock()
	english := host.locale == "en"
	host.mu.RUnlock()
	kind := "文稿"
	if info.IsDir() {
		kind = "目录"
	}
	title, message, detail := "删除"+kind, "要将“"+name+"”移到回收站吗？", "可以从系统回收站中恢复该"+kind+"。"
	if english {
		title, message, detail = "Delete entry", "Move “"+name+"” to the Recycle Bin?", "The entry can be restored from the system Recycle Bin."
	}
	confirmed, err := host.platform.Confirm(title, message, detail)
	if err != nil || !confirmed {
		return map[string]bool{"canceled": true}, err
	}
	if err := host.platform.Trash(resolved); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if !info.IsDir() {
		assets := companionAssets(resolved)
		if assetInfo, statErr := os.Stat(assets); statErr == nil && assetInfo.IsDir() {
			if trashErr := host.platform.Trash(assets); trashErr != nil && !errors.Is(trashErr, os.ErrNotExist) {
				return nil, trashErr
			}
		}
	}
	return map[string]bool{"deleted": true}, host.refreshWorkspace()
}

func (host *Host) selectDocument(payload map[string]any) {
	host.mu.Lock()
	host.currentPath = stringValue(payload, "path")
	host.currentMarkdown = stringValue(payload, "markdown")
	host.currentName = stringValue(payload, "name")
	if host.currentName == "" {
		host.currentName = "未命名.md"
	}
	title := documentTitle(host.currentPath, host.currentName)
	dirty, _ := payload["dirty"].(bool)
	host.mu.Unlock()
	if dirty {
		title = "● " + title
	}
	host.platform.SetTitle(title + " — Mory")
}

func (host *Host) refreshWorkspace() error {
	root := host.workspaces.activeRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("创建工作目录：%w", err)
	}
	files, err := listDocuments(root, false)
	if err != nil {
		return err
	}
	directories, err := listDirectories(root)
	if err != nil {
		return err
	}
	snapshot := map[string]any{"state": host.workspaces.state(), "files": files, "directories": directories}
	host.evaluate("window.Mory.setWorkspaceSnapshot", snapshot)
	host.mu.Lock()
	host.watchRoot = root
	host.watchSignature = workspaceSignature(root)
	host.mu.Unlock()
	return nil
}

func (host *Host) watchWorkspace() {
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-host.ctx.Done():
			return
		case <-ticker.C:
			host.mu.RLock()
			root, previous := host.watchRoot, host.watchSignature
			host.mu.RUnlock()
			if root == "" {
				continue
			}
			next := workspaceSignature(root)
			if next != previous {
				_ = host.refreshWorkspace()
			}
		}
	}
}

func workspaceSignature(root string) string {
	var builder strings.Builder
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path != root && entry.IsDir() && hiddenWorkspaceEntry(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr == nil {
			builder.WriteString(path)
			builder.WriteString(fmt.Sprintf(":%d:%d\n", info.Size(), info.ModTime().UnixNano()))
		}
		return nil
	})
	return builder.String()
}

func (host *Host) evaluate(functionName string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	host.platform.Evaluate(functionName + "(" + string(data) + ")")
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func decodeValue(value any, target any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func documentTitle(path, name string) string {
	if path != "" {
		return filepath.Base(path)
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

var headingName = regexp.MustCompile(`(?m)^#\s+(.+?)\s*#*\s*$`)

func suggestedDocumentName(markdown, fallback string) string {
	name := strings.TrimSuffix(fallback, filepath.Ext(fallback))
	if match := headingName.FindStringSubmatch(markdown); len(match) > 1 {
		name = strings.TrimSpace(strings.NewReplacer("*", "", "_", "", "`", "", "~", "").Replace(match[1]))
	}
	return sanitizeSegment(name) + ".md"
}

func availableDocumentPath(root, filename string) string {
	extension := filepath.Ext(filename)
	if extension == "" {
		extension = ".md"
	}
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	for serial := 1; ; serial++ {
		name := base + extension
		if serial > 1 {
			name = fmt.Sprintf("%s %d%s", base, serial, extension)
		}
		candidate := filepath.Join(root, name)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}
