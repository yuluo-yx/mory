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
