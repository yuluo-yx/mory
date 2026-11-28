package document

import (
	"bytes"
	"errors"
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
	// Exercise the former 32 MiB limit through import, deduplication, export
	// and embedding in the same resource round trip.
	data.Write(bytes.Repeat([]byte("x"), 33<<20))
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

func TestAssetFailureReasons(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large.png")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxAssetBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	for _, test := range []struct{ path, reason string }{{large, "too-large"}, {dir, "not-file"}, {filepath.Join(dir, "missing.png"), "read"}} {
		_, err := ReadAsset(test.path)
		var detail *AssetError
		if !errors.As(err, &detail) || detail.Reason != test.reason || detail.Error() == "" {
			t.Fatal("missing actionable error", err)
		}
		if test.reason == "too-large" && detail.Size != MaxAssetBytes+1 {
			t.Fatal("incorrect reported size")
		}
	}
	_, err = CopyAssets("![Large](large.png)", "markdown", dir, t.TempDir())
	var detail *AssetError
	if !errors.As(err, &detail) || detail.Reason != "too-large" {
		t.Fatal("import did not share the size limit", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = StoreAsset([]byte("image"), "image.png", dir)
	if !errors.As(err, &detail) || detail.Reason != "write" || errors.Unwrap(detail) == nil {
		t.Fatal("storage error lost its cause", err)
	}
	to := t.TempDir()
	path, err := StoreAsset([]byte("original"), "image.png", to)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(to, path), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StoreAsset([]byte("original"), "image.png", to); !errors.As(err, &detail) || detail.Reason != "write" {
		t.Fatal("asset collision was overwritten", err)
	}
}
