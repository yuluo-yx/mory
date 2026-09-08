package appcli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

func TestResolveExportRejectsNonRegularDestinations(t *testing.T) {
	for _, kind := range []string{"directory", "symlink", "dangling symlink", "source hard link"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "guide.md")
			destination := filepath.Join(root, "guide.pdf")
			if err := os.WriteFile(source, []byte("# Original"), 0o644); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "directory":
				err = os.Mkdir(destination, 0o755)
			case "symlink":
				err = os.Symlink(source, destination)
			case "dangling symlink":
				err = os.Symlink(filepath.Join(root, "missing"), destination)
			case "source hard link":
				err = os.Link(source, destination)
			}
			if err != nil {
				if runtime.GOOS == "windows" && errors.Is(err, os.ErrPermission) && (kind == "symlink" || kind == "dangling symlink") {
					t.Skip("Creating symbolic links requires Windows Developer Mode or elevated privileges")
				}
				t.Fatal(err)
			}
			for _, force := range []bool{false, true} {
				if _, err := ResolveExport(source, "pdf", root, force); err == nil {
					t.Errorf("accepted %s with force=%v", kind, force)
				}
			}
			data, err := os.ReadFile(source)
			if err != nil || string(data) != "# Original" {
				t.Fatalf("source changed: %q, %v", data, err)
			}
		})
	}
}

func TestResolvePowerPointExport(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "slides.md")
	if err := os.WriteFile(source, []byte("# Slides"), 0o644); err != nil {
		t.Fatal(err)
	}
	request, err := ResolveExport(source, "PPTX", root, false)
	if err != nil {
		t.Fatal(err)
	}
	if request.Format != "pptx" || filepath.Ext(request.Destination) != ".pptx" {
		t.Fatalf("ResolveExport() = %#v", request)
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
