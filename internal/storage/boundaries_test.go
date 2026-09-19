package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteDownloadsRejectExternalSymlinks(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		root, outside := t.TempDir(), t.TempDir()
		secret := filepath.Join(outside, "secret.md")
		if err := os.WriteFile(secret, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		if err := os.Symlink(secret, filepath.Join(root, "linked.md")); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"escape/secret.md", "escape/new/note.md", "linked.md"} {
			var err error
			if streaming {
				_, err = copyRemoteFile(root, name, strings.NewReader("remote"))
			} else {
				err = writeLocalFile(root, name, []byte("remote"))
			}
			if err == nil {
				t.Errorf("download escaped through %s (streaming=%v)", name, streaming)
			}
		}
		if data, err := os.ReadFile(secret); err != nil || string(data) != "original" {
			t.Errorf("external file changed: %q, %v", data, err)
		}
	}
}

func TestRemoteDownloadsFollowInternalRelativeLinks(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside", filepath.Join(root, "alias")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if err := writeLocalFile(root, "alias/new/note.md", []byte("remote")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "inside/new/note.md")); err != nil || string(data) != "remote" {
		t.Fatalf("internal link failed: %q, %v", data, err)
	}
}
