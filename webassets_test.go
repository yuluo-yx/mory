package mory

import (
	"io/fs"
	"testing"
)

func TestWebAssetsContainsEditorEntry(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(WebAssets(), "index.html")
	if err != nil {
		t.Fatalf("读取嵌入页面：%v", err)
	}
	if len(data) == 0 {
		t.Fatal("嵌入页面不应为空")
	}
}

func TestWebAssetsContainsLapisCVTheme(t *testing.T) {
	t.Parallel()

	assets := WebAssets()
	for _, name := range []string{"themes/lapis-cv.css", "themes/lapis-cv.LICENSE"} {
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Fatalf("read embedded Lapis CV asset %q: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("embedded Lapis CV asset %q must not be empty", name)
		}
	}
}
