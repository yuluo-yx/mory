package windowshost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThemeManagerInlinesLocalAssets(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	manager := newThemeManager(data)
	if err := manager.initialize(); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "海风.css")
	asset := filepath.Join(filepath.Dir(source), "font.woff2")
	if err := os.WriteFile(asset, []byte("font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(`@font-face{src:url("font.woff2")}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// 资源与 CSS 一起放到主题目录，模拟用户主题文件夹的组织形式。
	if err := os.WriteFile(filepath.Join(manager.path(), "font.woff2"), []byte("font"), 0o644); err != nil {
		t.Fatal(err)
	}
	themes, err := manager.importFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != 1 || !strings.Contains(themes[0].CSS, "data:font/woff2;base64,") {
		t.Fatalf("主题资源未内联：%#v", themes)
	}
	if !strings.HasPrefix(themes[0].ID, "user-") {
		t.Fatalf("主题 ID 不稳定：%q", themes[0].ID)
	}
}

func TestThemeDirectoryCanBeChanged(t *testing.T) {
	t.Parallel()
	manager := newThemeManager(t.TempDir())
	if err := manager.initialize(); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(t.TempDir(), "themes")
	result, err := manager.setDirectory(next)
	if err != nil {
		t.Fatal(err)
	}
	if result["directory"] != next || manager.path() != next {
		t.Fatalf("主题目录未更新：%#v", result)
	}
}

func TestThemeManagerRestoresDirectoryAndRejectsInvalidImport(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	first := newThemeManager(data)
	if err := first.initialize(); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(t.TempDir(), "custom")
	if _, err := first.setDirectory(next); err != nil {
		t.Fatal(err)
	}
	second := newThemeManager(data)
	if err := second.initialize(); err != nil {
		t.Fatal(err)
	}
	if second.path() != next {
		t.Fatalf("主题目录未恢复：%q", second.path())
	}
	invalid := filepath.Join(t.TempDir(), "theme.txt")
	if err := os.WriteFile(invalid, []byte("css"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := second.importFile(invalid); err == nil {
		t.Fatal("非 CSS 主题应被拒绝")
	}
	if _, err := second.setDirectory(""); err == nil {
		t.Fatal("空主题目录应被拒绝")
	}
}

func TestThemeAssetBoundariesAndMissingReferences(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	css := `.a{src:url("missing.woff2")}.b{src:url("https://example.com/font.woff2")}.c{src:url("../escape.woff2")}`
	result, err := inlineThemeAssets(css, directory)
	if err != nil {
		t.Fatal(err)
	}
	if result != css {
		t.Fatalf("缺失或远端资源不应改写：%q", result)
	}
	large := make([]byte, maxThemeAssetBytes+1)
	if err := os.WriteFile(filepath.Join(directory, "large.woff2"), large, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := inlineThemeAssets(`.a{src:url("large.woff2")}`, directory); err == nil {
		t.Fatal("超大主题资源应失败")
	}
}

func TestThemeListSkipsOversizedCSS(t *testing.T) {
	t.Parallel()
	manager := newThemeManager(t.TempDir())
	if err := manager.initialize(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manager.path(), "large.css"), make([]byte, maxThemeBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	themes, err := manager.list()
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != 0 {
		t.Fatalf("超大主题不应显示：%#v", themes)
	}
}
