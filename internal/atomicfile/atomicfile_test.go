package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFailurePreservesOriginal(t *testing.T) {
	for _, phase := range []string{"write", "closed output", "replace"} {
		t.Run(phase, func(t *testing.T) {
			directory := t.TempDir()
			name := filepath.Join(directory, "note.md")
			if phase != "replace" {
				if err := os.WriteFile(name, []byte("original"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			err = Replace(root, "note.md", 0o644, func(file *os.File) error {
				if _, err := file.WriteString("partial"); err != nil {
					return err
				}
				if phase == "closed output" {
					return file.Close()
				}
				if phase == "replace" {
					return os.Mkdir(name, 0o700)
				}
				return errors.New("injected write failure")
			})
			if err == nil {
				t.Fatal("Expected the write or replacement to fail")
			}
			if phase != "replace" {
				data, err := os.ReadFile(name)
				if err != nil || string(data) != "original" {
					t.Fatalf("Original changed: %q, %v", data, err)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != "note.md" {
				t.Fatalf("Temporary file remained: %v, %v", entries, err)
			}
		})
	}
}

func TestWriteFilePreservesModeAndLink(t *testing.T) {
	directory := t.TempDir()
	name := filepath.Join(directory, "note.md")
	if err := WriteFile(name, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "alias.md")
	if err := os.Symlink("note.md", link); err != nil {
		t.Skipf("Symlinks unavailable: %v", err)
	}
	if err := WriteFile(link, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(name)
	if err != nil || string(data) != "second" {
		t.Fatalf("Unexpected content: %q, %v", data, err)
	}
	info, err := os.Stat(name)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("Permissions changed: %v, %v", info, err)
	}
	info, err = os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Link was replaced: %v, %v", info, err)
	}
}
