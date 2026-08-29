package appcli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExport(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	request, err := ResolveExport(source, "PDF", root, false)
	if err != nil {
		t.Fatal(err)
	}
	if request.Source != source || request.Destination != filepath.Join(root, "guide.pdf") || request.Format != "pdf" {
		t.Fatalf("ResolveExport() = %#v", request)
	}

	if err := os.WriteFile(request.Destination, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveExport(source, "pdf", root, false); err == nil {
		t.Fatal("existing output should require overwrite permission")
	}
	if _, err := ResolveExport(source, "pdf", root, true); err != nil {
		t.Fatalf("force export should accept an existing output: %v", err)
	}
}

func TestResolveDocumentRejectsUnsupportedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.rtf")
	if err := os.WriteFile(path, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDocument(path); err == nil {
		t.Fatal("unsupported document should fail")
	}
}

func TestResolveExportSanitizesCrossPlatformReservedNames(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "CON.md")
	if err := os.WriteFile(source, []byte("# Reserved"), 0o644); err != nil {
		t.Fatal(err)
	}
	request, err := ResolveExport(source, "html", root, false)
	if err != nil {
		t.Fatal(err)
	}
	if request.Destination != filepath.Join(root, "CON_.html") {
		t.Fatalf("sanitized destination = %q", request.Destination)
	}
}

func TestParseDesktopRequest(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "note.md")
	if err := os.WriteFile(source, []byte("# Note"), 0o644); err != nil {
		t.Fatal(err)
	}

	request, err := ParseDesktopRequest([]string{
		"--mory-cli-export", "--format", "html", "--output", filepath.Join(root, "note.html"), source,
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Document != source || request.Export == nil || request.Export.Destination != filepath.Join(root, "note.html") {
		t.Fatalf("ParseDesktopRequest() = %#v", request)
	}
	if _, err := ParseDesktopRequest([]string{
		"--mory-cli-export", "--format", "pdf", "--output", filepath.Join(root, "note.png"), source,
	}); err == nil {
		t.Fatal("mismatched output extension should fail")
	}
}
