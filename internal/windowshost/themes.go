package windowshost

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	maxThemeBytes      = 1024 * 1024
	maxThemeAssetBytes = 5 * 1024 * 1024
)

var cssURL = regexp.MustCompile(`(?i)url\(\s*(["']?)([^"')]+)["']?\s*\)`)

// Theme is a user-provided CSS theme available to the settings UI.
type Theme struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Filename string `json:"filename"`
	CSS      string `json:"css"`
}

type themeManager struct {
	mu           sync.RWMutex
	directory    string
	settingsPath string
}

func newThemeManager(userDataPath string) *themeManager {
	return &themeManager{directory: filepath.Join(userDataPath, "themes"), settingsPath: filepath.Join(userDataPath, "theme-settings.json")}
}

func (manager *themeManager) initialize() error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	data, err := os.ReadFile(manager.settingsPath)
	if err == nil {
		var settings struct {
			Directory string `json:"directory"`
		}
		if json.Unmarshal(data, &settings) == nil && filepath.IsAbs(settings.Directory) {
			manager.directory = filepath.Clean(settings.Directory)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取主题设置：%w", err)
	}
	if err := os.MkdirAll(manager.directory, 0o755); err != nil {
		return fmt.Errorf("创建主题目录：%w", err)
	}
	return nil
}

func (manager *themeManager) path() string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.directory
}

func (manager *themeManager) list() ([]Theme, error) {
	manager.mu.RLock()
	directory := manager.directory
	manager.mu.RUnlock()
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("读取主题目录：%w", err)
	}
	sort.SliceStable(entries, func(i, j int) bool { return naturalLess(entries[i].Name(), entries[j].Name()) })
	themes := make([]Theme, 0)
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".css" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, fmt.Errorf("读取主题信息：%w", infoErr)
		}
		if info.Size() > maxThemeBytes {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("读取主题：%w", readErr)
		}
		css, inlineErr := inlineThemeAssets(string(data), directory)
		if inlineErr != nil {
			return nil, inlineErr
		}
		themes = append(themes, Theme{ID: themeID(entry.Name()), Name: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), Filename: entry.Name(), CSS: css})
	}
	return themes, nil
}

func (manager *themeManager) importFile(source string) ([]Theme, error) {
	if strings.ToLower(filepath.Ext(source)) != ".css" {
		return nil, errors.New("请选择 CSS 主题文件")
	}
	info, err := os.Stat(source)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxThemeBytes {
		return nil, errors.New("主题文件无效或超过 1 MB")
	}
	destination := filepath.Join(manager.path(), filepath.Base(source))
	if filepath.Clean(source) != filepath.Clean(destination) {
		data, readErr := os.ReadFile(source)
		if readErr != nil {
			return nil, fmt.Errorf("读取主题文件：%w", readErr)
		}
		if writeErr := os.WriteFile(destination, data, 0o644); writeErr != nil {
			return nil, fmt.Errorf("导入主题文件：%w", writeErr)
		}
	}
	return manager.list()
}

func (manager *themeManager) setDirectory(directory string) (map[string]any, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("请选择有效的主题目录")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("解析主题目录：%w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return nil, fmt.Errorf("创建主题目录：%w", err)
	}
	manager.mu.Lock()
	manager.directory = absolute
	manager.mu.Unlock()
	data, _ := json.MarshalIndent(map[string]string{"directory": absolute}, "", "  ")
	data = append(data, '\n')
	if err := os.WriteFile(manager.settingsPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("保存主题目录：%w", err)
	}
	themes, err := manager.list()
	if err != nil {
		return nil, err
	}
	return map[string]any{"directory": absolute, "themes": themes}, nil
}

func inlineThemeAssets(css, directory string) (string, error) {
	total := 0
	var inlineErr error
	result := cssURL.ReplaceAllStringFunc(css, func(value string) string {
		if inlineErr != nil {
			return value
		}
		match := cssURL.FindStringSubmatch(value)
		if len(match) < 3 {
			return value
		}
		reference := strings.TrimSpace(match[2])
		if reference == "" || hasRemoteScheme(reference) || strings.HasPrefix(reference, "#") {
			return value
		}
		resolved, err := safeDescendant(directory, strings.SplitN(strings.SplitN(filepath.FromSlash(reference), "?", 2)[0], "#", 2)[0])
		if err != nil {
			return value
		}
		data, err := os.ReadFile(resolved)
		if errors.Is(err, os.ErrNotExist) {
			return value
		}
		if err != nil {
			inlineErr = fmt.Errorf("读取主题资源：%w", err)
			return value
		}
		total += len(data)
		if total > maxThemeAssetBytes {
			inlineErr = errors.New("主题资源总大小不能超过 5 MB")
			return value
		}
		mimeType := mimeTypeForPath(resolved)
		return `url("data:` + mimeType + `;base64,` + base64.StdEncoding.EncodeToString(data) + `")`
	})
	return result, inlineErr
}

func themeID(filename string) string {
	base := sanitizeSegment(strings.ToLower(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))))
	digest := sha256.Sum256([]byte(filename))
	return "user-" + base + "-" + hex.EncodeToString(digest[:4])
}
