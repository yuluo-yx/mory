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
		t.Fatalf("list documents: %v", err)
	}
	if len(documents) != 3 {
		t.Fatalf("document count = %d, want 3", len(documents))
	}
	for _, document := range documents {
		if document.Name == "2-note.md" && document.Markdown == "# two" {
			return
		}
	}
	t.Fatalf("document content was not loaded: %#v", documents)
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
		t.Fatalf("invalid sort result: %#v", documents)
	}
}

func TestImportAndLoadDocumentImages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	documentPath := filepath.Join(root, "\u793A\u4F8B.md")
	writeAt(t, documentPath, "# \u793A\u4F8B", time.Now())
	pixel := []byte{0x89, 'P', 'N', 'G'}
	result, err := importImage(root, documentPath, "\u793A\u4F8B.md", "\u5C01\u9762.png", "image/png", base64.StdEncoding.EncodeToString(pixel))
	if err != nil {
		t.Fatalf("import image: %v", err)
	}
	if result["relative"] != "\u793A\u4F8B/\u5C01\u9762.png" {
		t.Fatalf("relative path = %v", result["relative"])
	}
	markdown := "![\u5C01\u9762](\u793A\u4F8B/\u5C01\u9762.png)"
	assets := loadDocumentAssets(documentPath, markdown)
	if assets["\u793A\u4F8B/\u5C01\u9762.png"] == "" {
		t.Fatal("document image was not converted to a data URL immediately")
	}
	images, err := listDocumentImages(documentPath)
	if err != nil || len(images) != 1 {
		t.Fatalf("list images: %v, count %d", err, len(images))
	}
}

func TestWorkspacePathsRejectEscapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := safeDescendant(root, filepath.Join("..", "escape.md")); err == nil {
		t.Fatal("relative paths escaping the workspace should be rejected")
	}
	if _, err := createWorkspaceDirectory(root, "../escape"); err == nil {
		t.Fatal("directories escaping the workspace should be rejected")
	}
}

func TestWorkspaceEntryCreateCopyMoveAndCompanionAssets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "\u76EE\u6807")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	document, err := createWorkspaceDocument(root, target, "\u672A\u547D\u540D.md")
	if err != nil || filepath.Dir(document.Path) != target {
		t.Fatalf("create document in directory: %#v, %v", document, err)
	}
	writeAt(t, filepath.Join(target, "\u672A\u547D\u540D", "image.png"), "image", time.Now())

	copied, err := copyWorkspaceEntry(root, document.Path, target)
	if err != nil || filepath.Base(copied.Path) != "\u672A\u547D\u540D \u526F\u672C.md" {
		t.Fatalf("copy document: %#v, %v", copied, err)
	}
	if _, err := os.Stat(filepath.Join(target, "\u672A\u547D\u540D-\u526F\u672C", "image.png")); err != nil {
		t.Fatalf("copy document images: %v", err)
	}

	moved, err := moveWorkspaceEntry(root, copied.Path, root)
	if err != nil || filepath.Dir(moved.Path) != root {
		t.Fatalf("move document: %#v, %v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(root, "\u672A\u547D\u540D-\u526F\u672C", "image.png")); err != nil {
		t.Fatalf("move document images: %v", err)
	}
}

func TestRenameWorkspaceEntryMovesDocumentAssets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "\u539F\u6587.md")
	writeAt(t, source, "# \u539F\u6587\n\n![\u56FE](\u539F\u6587/\u5C01\u9762.png)", time.Now())
	writeAt(t, filepath.Join(root, "\u539F\u6587", "\u5C01\u9762.png"), "image", time.Now())

	renamed, err := renameWorkspaceEntry(root, source, "\u65B0\u6587\u7A3F")
	if err != nil {
		t.Fatalf("rename document: %v", err)
	}
	if renamed.Name != "\u65B0\u6587\u7A3F.md" || filepath.Base(renamed.Path) != "\u65B0\u6587\u7A3F.md" {
		t.Fatalf("rename result: %#v", renamed)
	}
	data, err := os.ReadFile(renamed.Path)
	if err != nil || string(data) != "# \u539F\u6587\n\n![\u56FE](\u65B0\u6587\u7A3F/\u5C01\u9762.png)" {
		t.Fatalf("update image references: %q, %v", data, err)
	}
	if data, err = os.ReadFile(filepath.Join(root, "\u65B0\u6587\u7A3F", "\u5C01\u9762.png")); err != nil || string(data) != "image" {
		t.Fatalf("move image directory: %q, %v", data, err)
	}
	writeAt(t, filepath.Join(root, "\u51B2\u7A81.md"), "existing", time.Now())
	for _, test := range []struct {
		name      string
		requested string
	}{
		{name: "unchanged name", requested: "\u65B0\u6587\u7A3F"},
		{name: "path traversal", requested: "../\u8D8A\u754C.md"},
		{name: "Windows path traversal", requested: `..\outside.md`},
		{name: "name collision", requested: "\u51B2\u7A81.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := renameWorkspaceEntry(root, renamed.Path, test.requested); err == nil {
				t.Fatalf("rename %q should fail", test.requested)
			}
		})
	}
}

func TestWorkspaceDirectoryCopyMoveGuardsDescendants(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "\u8D44\u6599")
	child := filepath.Join(source, "\u5B50\u76EE\u5F55")
	writeAt(t, filepath.Join(child, "note.md"), "# note", time.Now())
	if _, err := copyWorkspaceEntry(root, source, child); err == nil {
		t.Fatal("directory should not copy into its descendant")
	}
	if _, err := moveWorkspaceEntry(root, source, child); err == nil {
		t.Fatal("directory should not move into its descendant")
	}
	copied, err := copyWorkspaceEntry(root, source, root)
	if err != nil || !copied.IsDirectory || filepath.Base(copied.Path) != "\u8D44\u6599 \u526F\u672C" {
		t.Fatalf("copy directory: %#v, %v", copied, err)
	}
	if _, err := os.Stat(filepath.Join(copied.Path, "\u5B50\u76EE\u5F55", "note.md")); err != nil {
		t.Fatalf("directory content was not copied: %v", err)
	}
}

func TestReadImageAndCopyDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	imagePath := filepath.Join(root, "images", "x.png")
	writeAt(t, imagePath, string([]byte{0x89, 'P', 'N', 'G'}), time.Now())
	image, err := readDocumentImage(root, imagePath)
	if err != nil || !strings.HasPrefix(image["dataURL"].(string), "data:image/png;base64,") {
		t.Fatalf("read image: %#v, %v", image, err)
	}
	destination := filepath.Join(t.TempDir(), "copy")
	if err := copyDirectory(filepath.Join(root, "images"), destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "x.png")); err != nil {
		t.Fatalf("copy directory failed: %v", err)
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
		t.Fatalf("invalid duplicate image naming: %#v %#v", first, second)
	}
	if _, err := importImage(root, documentPath, "note.md", "x", "bad/type", encoded); err == nil {
		t.Fatal("unknown MIME type should fail")
	}
	if _, err := importImage(root, documentPath, "note.md", "x", "image/png", "not-base64"); err == nil {
		t.Fatal("invalid base64 should fail")
	}
	nonImage := filepath.Join(root, "x.txt")
	writeAt(t, nonImage, "x", time.Now())
	if _, err := readDocumentImage(root, nonImage); err == nil {
		t.Fatal("non-image input should fail")
	}
}

func TestDocumentAssetsIgnoreRemoteMissingAndEscapingPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	documentPath := filepath.Join(root, "note.md")
	writeAt(t, documentPath, "", time.Now())
	markdown := "![remote](https://example.com/a.png)\n![missing](missing.png)\n![escape](../outside.png)"
	if assets := loadDocumentAssets(documentPath, markdown); len(assets) != 0 {
		t.Fatalf("unexpected assets were loaded: %#v", assets)
	}
	if _, err := loadDocument(filepath.Join(root, "missing.md")); err == nil {
		t.Fatal("missing document should fail")
	}
}

func TestSuggestedAndAvailableDocumentName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := suggestedDocumentName("# \u4E2D\u6587 \u6807\u9898\n", "\u672A\u547D\u540D.md")
	if name != "\u4E2D\u6587-\u6807\u9898.md" {
		t.Fatalf("suggested filename = %q", name)
	}
	writeAt(t, filepath.Join(root, name), "", time.Now())
	if got := filepath.Base(availableDocumentPath(root, name)); got != "\u4E2D\u6587-\u6807\u9898 2.md" {
		t.Fatalf("available filename = %q", got)
	}
}

func TestRelocateDocumentAssetsOnSaveAs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	oldPath := filepath.Join(root, "\u65E7\u540D.md")
	newPath := filepath.Join(root, "\u65B0\u540D.md")
	writeAt(t, oldPath, "", time.Now())
	writeAt(t, filepath.Join(root, "\u65E7\u540D", "image.png"), "image", time.Now())
	markdown, err := relocateDocumentAssets(root, "![\u56FE](\u65E7\u540D/image.png)", oldPath, "\u65E7\u540D.md", newPath)
	if err != nil {
		t.Fatal(err)
	}
	if markdown != "![\u56FE](\u65B0\u540D/image.png)" {
		t.Fatalf("image reference was not updated: %q", markdown)
	}
	if _, err := os.Stat(filepath.Join(root, "\u65B0\u540D", "image.png")); err != nil {
		t.Fatalf("image was not migrated: %v", err)
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
