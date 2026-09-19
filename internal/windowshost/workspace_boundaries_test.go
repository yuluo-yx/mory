package windowshost

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceSymlinkBoundaries(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		filepath.Join(outside, "secret.png"): "outside",
		filepath.Join(inside, "safe.png"):    "inside",
		filepath.Join(root, "note.md"):       "original",
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range map[string]string{"escape": outside, "alias": inside, "note": outside, "dangling": filepath.Join(outside, "missing"), "cycle": filepath.Join(root, "cycle")} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
	}
	t.Run("external operations", func(t *testing.T) {
		if _, err := createWorkspaceDirectory(root, "escape/new/nested"); err == nil {
			t.Error("created a directory outside workspace")
		}
		if _, err := createWorkspaceDocument(root, filepath.Join(root, "escape"), "new.md"); err == nil {
			t.Error("created a document outside workspace")
		}
		if _, err := readDocumentImage(root, filepath.Join(root, "escape", "secret.png")); err == nil {
			t.Error("read an image outside workspace")
		}
		if _, err := importImage(root, filepath.Join(root, "note.md"), "note.md", "new.png", "image/png", base64.StdEncoding.EncodeToString([]byte("new"))); err == nil {
			t.Error("imported an image outside workspace")
		}
	})
	t.Run("embedded resources", func(t *testing.T) {
		if assets := loadDocumentAssets(root, filepath.Join(root, "note.md"), "![secret](escape/secret.png)"); len(assets) != 0 {
			t.Error("embedded an external image")
		}
		css := "p { background: url(escape/secret.png) }"
		if actual, err := inlineThemeAssets(css, root); err != nil || actual != css {
			t.Errorf("embedded an external theme resource: %v", err)
		}
	})
	t.Run("internal links", func(t *testing.T) {
		if _, err := readDocumentImage(root, filepath.Join(root, "alias", "safe.png")); err != nil {
			t.Fatal(err)
		}
		if _, err := createWorkspaceDirectory(root, "alias/new/nested"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unsafe companion assets", func(t *testing.T) {
		destination := filepath.Join(root, "destination")
		if err := os.Mkdir(destination, 0o755); err != nil {
			t.Fatal(err)
		}
		note := filepath.Join(root, "note.md")
		if _, err := copyWorkspaceEntry(root, note, destination); err == nil {
			t.Error("copied unsafe companion assets")
		}
		if _, err := moveWorkspaceEntry(root, note, destination); err == nil {
			t.Error("moved unsafe companion assets")
		}
		if _, err := relocateDocumentAssets(root, "![x](note/secret.png)", note, "note.md", filepath.Join(destination, "copy.md")); err == nil {
			t.Error("saved unsafe companion assets")
		}
		if data, err := os.ReadFile(note); err != nil || string(data) != "original" {
			t.Errorf("changed original note: %v", err)
		}
	})
	t.Run("dangling and cyclic links", func(t *testing.T) {
		for _, name := range []string{"dangling", "cycle"} {
			if _, err := createWorkspaceDirectory(root, name+"/child"); err == nil {
				t.Errorf("accepted %s link", name)
			}
		}
	})
}

func TestWorkspaceScanSkipsExternalDocumentSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("external contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "linked.md")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	documents, err := listDocuments(root, true)
	if err != nil || len(documents) != 0 {
		t.Fatalf("external document leaked through workspace scan: %#v, %v", documents, err)
	}
}
