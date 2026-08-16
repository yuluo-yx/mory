package windowshost

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListDocumentsReadsContentAndSkipsHiddenDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAt(t, filepath.Join(root, "10-note.md"), "# ten", time.Now())
	writeAt(t, filepath.Join(root, "2-note.md"), "# two", time.Now())
	writeAt(t, filepath.Join(root, "later.md"), "# later", time.Now())
	writeAt(t, filepath.Join(root, ".git", "ignored.md"), "# ignored", time.Now())

	documents, err := listDocuments(root, true)
	if err != nil {
		t.Fatalf("列出文稿：%v", err)
	}
	if len(documents) != 3 {
		t.Fatalf("文稿数量 = %d，期望 3", len(documents))
	}
	for _, document := range documents {
		if document.Name == "2-note.md" && document.Markdown == "# two" {
			return
		}
	}
	t.Fatalf("文稿内容未读取：%#v", documents)
}

func TestSortDocumentsByCreationAndNaturalName(t *testing.T) {
	t.Parallel()
	documents := []Document{
		{Name: "later.md", CreatedAt: 2},
		{Name: "10-note.md", CreatedAt: 1},
		{Name: "2-note.md", CreatedAt: 1},
	}
	sortDocuments(documents)
	if documents[0].Name != "2-note.md" || documents[1].Name != "10-note.md" || documents[2].Name != "later.md" {
		t.Fatalf("排序结果错误：%#v", documents)
	}
}

func TestImportAndLoadDocumentImages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	documentPath := filepath.Join(root, "示例.md")
	writeAt(t, documentPath, "# 示例", time.Now())
	pixel := []byte{0x89, 'P', 'N', 'G'}
	result, err := importImage(root, documentPath, "示例.md", "封面.png", "image/png", base64.StdEncoding.EncodeToString(pixel))
	if err != nil {
		t.Fatalf("导入图片：%v", err)
	}
	if result["relative"] != "示例/封面.png" {
		t.Fatalf("相对路径 = %v", result["relative"])
	}
	markdown := "![封面](示例/封面.png)"
	assets := loadDocumentAssets(documentPath, markdown)
	if assets["示例/封面.png"] == "" {
		t.Fatal("文稿图片没有即时转换为 data URL")
	}
	images, err := listDocumentImages(documentPath)
	if err != nil || len(images) != 1 {
		t.Fatalf("列出图片：%v，数量 %d", err, len(images))
	}
}

func TestWorkspacePathsRejectEscapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := safeDescendant(root, filepath.Join("..", "escape.md")); err == nil {
		t.Fatal("应拒绝逃逸工作区的相对路径")
	}
	if _, err := createWorkspaceDirectory(root, "../escape"); err == nil {
		t.Fatal("应拒绝逃逸工作区的目录")
	}
}

func TestWorkspaceEntryCreateCopyMoveAndCompanionAssets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "目标")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	document, err := createWorkspaceDocument(root, target, "未命名.md")
	if err != nil || filepath.Dir(document.Path) != target {
		t.Fatalf("在目录中新建文稿：%#v，%v", document, err)
	}
	writeAt(t, filepath.Join(target, "未命名", "image.png"), "image", time.Now())

	copied, err := copyWorkspaceEntry(root, document.Path, target)
	if err != nil || filepath.Base(copied.Path) != "未命名 副本.md" {
		t.Fatalf("复制文稿：%#v，%v", copied, err)
	}
	if _, err := os.Stat(filepath.Join(target, "未命名-副本", "image.png")); err != nil {
		t.Fatalf("复制文稿图片：%v", err)
	}

	moved, err := moveWorkspaceEntry(root, copied.Path, root)
	if err != nil || filepath.Dir(moved.Path) != root {
		t.Fatalf("移动文稿：%#v，%v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(root, "未命名-副本", "image.png")); err != nil {
		t.Fatalf("移动文稿图片：%v", err)
	}
}

func TestWorkspaceDirectoryCopyMoveGuardsDescendants(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "资料")
	child := filepath.Join(source, "子目录")
	writeAt(t, filepath.Join(child, "note.md"), "# note", time.Now())
	if _, err := copyWorkspaceEntry(root, source, child); err == nil {
		t.Fatal("目录不应复制到自身子目录")
	}
	if _, err := moveWorkspaceEntry(root, source, child); err == nil {
		t.Fatal("目录不应移动到自身子目录")
	}
	copied, err := copyWorkspaceEntry(root, source, root)
	if err != nil || !copied.IsDirectory || filepath.Base(copied.Path) != "资料 副本" {
		t.Fatalf("复制目录：%#v，%v", copied, err)
	}
	if _, err := os.Stat(filepath.Join(copied.Path, "子目录", "note.md")); err != nil {
		t.Fatalf("目录内容未复制：%v", err)
	}
}

func TestReadImageAndCopyDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	imagePath := filepath.Join(root, "images", "x.png")
	writeAt(t, imagePath, string([]byte{0x89, 'P', 'N', 'G'}), time.Now())
	image, err := readDocumentImage(root, imagePath)
	if err != nil || !strings.HasPrefix(image["dataURL"].(string), "data:image/png;base64,") {
		t.Fatalf("读取图片：%#v，%v", image, err)
	}
	destination := filepath.Join(t.TempDir(), "copy")
	if err := copyDirectory(filepath.Join(root, "images"), destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "x.png")); err != nil {
		t.Fatalf("复制目录失败：%v", err)
	}
}

func TestImageValidationAndDuplicateNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	documentPath := filepath.Join(root, "note.md")
	writeAt(t, documentPath, "", time.Now())
	encoded := base64.StdEncoding.EncodeToString([]byte("image"))
	first, err := importImage(root, documentPath, "note.md", "same.png", "image/png", encoded)
	if err != nil {
		t.Fatal(err)
	}
	second, err := importImage(root, documentPath, "note.md", "same.png", "image/png", encoded)
	if err != nil {
		t.Fatal(err)
	}
	if first["relative"] == second["relative"] || second["relative"] != "note/same-2.png" {
		t.Fatalf("重复图片命名错误：%#v %#v", first, second)
	}
	if _, err := importImage(root, documentPath, "note.md", "x", "bad/type", encoded); err == nil {
		t.Fatal("未知 MIME 应失败")
	}
	if _, err := importImage(root, documentPath, "note.md", "x", "image/png", "not-base64"); err == nil {
		t.Fatal("无效 base64 应失败")
	}
	nonImage := filepath.Join(root, "x.txt")
	writeAt(t, nonImage, "x", time.Now())
	if _, err := readDocumentImage(root, nonImage); err == nil {
		t.Fatal("非图片应失败")
	}
}

func TestDocumentAssetsIgnoreRemoteMissingAndEscapingPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	documentPath := filepath.Join(root, "note.md")
	writeAt(t, documentPath, "", time.Now())
	markdown := "![remote](https://example.com/a.png)\n![missing](missing.png)\n![escape](../outside.png)"
	if assets := loadDocumentAssets(documentPath, markdown); len(assets) != 0 {
		t.Fatalf("不应加载这些资源：%#v", assets)
	}
	if _, err := loadDocument(filepath.Join(root, "missing.md")); err == nil {
		t.Fatal("缺失文稿应失败")
	}
}

func TestSuggestedAndAvailableDocumentName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := suggestedDocumentName("# 中文 标题\n", "未命名.md")
	if name != "中文-标题.md" {
		t.Fatalf("建议文件名 = %q", name)
	}
	writeAt(t, filepath.Join(root, name), "", time.Now())
	if got := filepath.Base(availableDocumentPath(root, name)); got != "中文-标题 2.md" {
		t.Fatalf("不冲突文件名 = %q", got)
	}
}

func TestRelocateDocumentAssetsOnSaveAs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	oldPath := filepath.Join(root, "旧名.md")
	newPath := filepath.Join(root, "新名.md")
	writeAt(t, oldPath, "", time.Now())
	writeAt(t, filepath.Join(root, "旧名", "image.png"), "image", time.Now())
	markdown, err := relocateDocumentAssets(root, "![图](旧名/image.png)", oldPath, "旧名.md", newPath)
	if err != nil {
		t.Fatal(err)
	}
	if markdown != "![图](新名/image.png)" {
		t.Fatalf("图片引用未更新：%q", markdown)
	}
	if _, err := os.Stat(filepath.Join(root, "新名", "image.png")); err != nil {
		t.Fatalf("图片未迁移：%v", err)
	}
}

func writeAt(t *testing.T, path, content string, timestamp time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
}
