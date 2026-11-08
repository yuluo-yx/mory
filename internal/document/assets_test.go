package document

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetImportExport(t *testing.T) {
	from, to, export := t.TempDir(), t.TempDir(), t.TempDir()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(from, "hello world.png"), data.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"markdown", "xml"} {
		source := "![Image](<hello world.png>)\n[remote](https://example.com)\n[anchor](#a)"
		if format == "xml" {
			source = `<img href="hello world.png"/>`
		}
		rewritten, err := CopyAssets(source, format, from, to)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(rewritten, "assets/") {
			t.Fatal(rewritten)
		}
		again, err := CopyAssets(rewritten, format, to, export)
		if err != nil || again != rewritten {
			t.Fatal(again, err)
		}
		if _, err = CopyAssets(source, format, from, to); err != nil {
			t.Fatal("duplicate asset", err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(to, "assets"))
	page := `<img src="assets/` + entries[0].Name() + `">`
	embedded, err := EmbedImages(page, to)
	if err != nil || !strings.Contains(embedded, "data:image/png;base64,") {
		t.Fatal(embedded, err)
	}
	if _, err = CopyAssets("![escape](../secret.png)", "markdown", from, to); err == nil {
		t.Fatal("traversal accepted")
	}
	if _, err = CopyAssets("![missing](missing.png)", "markdown", from, to); err == nil {
		t.Fatal("missing accepted")
	}
	if _, err = CopyAssets("", "markdown", filepath.Join(from, "missing"), to); err == nil {
		t.Fatal("missing root accepted")
	}
	if _, err = EmbedImages(`<img src="missing.png">`, to); err == nil {
		t.Fatal("missing image accepted")
	}
	if err = os.WriteFile(filepath.Join(to, "text.txt"), []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = EmbedImages(`<img src="text.txt">`, to); err == nil {
		t.Fatal("non-image accepted")
	}
	if _, err = EmbedImages("", filepath.Join(to, "missing")); err == nil {
		t.Fatal("missing root")
	}
	if out, err := EmbedImages(`<img src="https://example.com/x.png">`, to); err != nil || strings.Contains(out, "base64") {
		t.Fatal("remote fetched")
	}
	if _, err = CopyAssets("[directory](.)", "markdown", from, to); err == nil {
		t.Fatal("directory attachment accepted")
	}
}

func TestTaskXML(t *testing.T) {
	source := ToXML("Tasks", "- [x] Done\n- [ ] Next\n\n```go\nfunc main() {}\n```\n")
	if err := Validate(source, "xml"); err != nil {
		t.Fatal(source, err)
	}
	_, out, err := FromXML(source)
	if err != nil || !strings.Contains(out, "- [x] Done") || !strings.Contains(out, "```go") {
		t.Fatal(out, err)
	}
}
