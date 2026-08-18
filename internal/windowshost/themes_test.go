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
	source := filepath.Join(t.TempDir(), "\u6D77\u98CE.css")
	asset := filepath.Join(filepath.Dir(source), "font.woff2")
	if err := os.WriteFile(asset, []byte("font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(`@font-face{src:url("font.woff2")}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Store the resource beside the CSS file to model a user-managed theme directory.
	if err := os.WriteFile(filepath.Join(manager.path(), "font.woff2"), []byte("font"), 0o644); err != nil {
		t.Fatal(err)
	}
	themes, err := manager.importFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != 1 || !strings.Contains(themes[0].CSS, "data:font/woff2;base64,") {
		t.Fatalf("theme assets were not inlined: %#v", themes)
	}
	if !strings.HasPrefix(themes[0].ID, "user-") {
		t.Fatalf("theme ID is unstable: %q", themes[0].ID)
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
		t.Fatalf("theme directory was not updated: %#v", result)
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
		t.Fatalf("theme directory was not restored: %q", second.path())
	}
	invalid := filepath.Join(t.TempDir(), "theme.txt")
	if err := os.WriteFile(invalid, []byte("css"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := second.importFile(invalid); err == nil {
		t.Fatal("non-CSS themes should be rejected")
	}
	if _, err := second.setDirectory(""); err == nil {
		t.Fatal("an empty theme directory should be rejected")
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
		t.Fatalf("missing or remote assets should not be rewritten: %q", result)
	}
	large := make([]byte, maxThemeAssetBytes+1)
	if err := os.WriteFile(filepath.Join(directory, "large.woff2"), large, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := inlineThemeAssets(`.a{src:url("large.woff2")}`, directory); err == nil {
		t.Fatal("oversized theme assets should fail")
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
		t.Fatalf("oversized themes should not be listed: %#v", themes)
	}
}
