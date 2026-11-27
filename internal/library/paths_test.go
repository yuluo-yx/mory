package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentPathTracksPhysicalFile(t *testing.T) {
	for _, format := range []string{"markdown", "xml"} {
		t.Run(format, func(t *testing.T) {
			l := mustLibrary(t)
			n, err := l.Create("Display title", "Body", format)
			if err != nil {
				t.Fatal(err)
			}
			n.Folder = "Logical/Nested group"
			want, _ := filepath.Abs(l.notePath(n))
			assertPath := func(want string) {
				t.Helper()
				if got, err := l.DocumentPath(n.ID); err != nil || got != want {
					t.Fatalf("DocumentPath = %q, %v; want %q", got, err, want)
				}
			}
			assertPath(want)
			originalFormat := n.Format
			n.Format = "xml"
			if originalFormat == "xml" {
				n.Format = "markdown"
			}
			assertPath(want) // A pending conversion must still locate the saved file.
			if err := l.Save(n); err != nil {
				t.Fatal(err)
			}
			want, _ = filepath.Abs(l.notePath(n))
			assertPath(want)
			if err := l.TrashTree(n, true); err != nil {
				t.Fatal(err)
			}
			want, _ = filepath.Abs(l.notePath(n))
			assertPath(want)
			if err := os.Remove(want); err != nil {
				t.Fatal(err)
			}
			assertPath(want) // Missing files keep their actual extension.
		})
	}
}

func TestDocumentPathExternalAndInvalid(t *testing.T) {
	l := mustLibrary(t)
	path := filepath.Join(t.TempDir(), "External notes 世界.md")
	mustWrite(t, path, []byte("Original"))
	n, err := l.OpenExternal(path)
	if err != nil {
		t.Fatal(err)
	}
	// OpenExternal canonicalizes symlinks (including macOS's temporary root).
	want := n.External
	for _, missing := range []bool{false, true} {
		if missing {
			if err := os.Remove(want); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := l.DocumentPath(n.ID); err != nil || got != want {
			t.Fatalf("External path = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := l.DocumentPath("missing"); err == nil {
		t.Fatal("unknown document accepted")
	}
	n.FolderTrash = true
	if _, err := l.DocumentPath(n.ID); err == nil {
		t.Fatal("folder trash record exposed as a document")
	}
}
