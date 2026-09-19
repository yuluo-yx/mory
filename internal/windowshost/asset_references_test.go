package windowshost

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameAndSaveAsKeepResolvedImages(t *testing.T) {
	for _, operation := range []string{"rename", "save as"} {
		for _, markdown := range []string{"![x](Old/image.png)", "![x](./Old/image.png)", `<img src="Old/image.png">`} {
			root := t.TempDir()
			source, target := filepath.Join(root, "Old.md"), filepath.Join(root, "New.md")
			if operation == "save as" {
				target = filepath.Join(root, "destination", "New.md")
			}
			if err := os.Mkdir(filepath.Join(root, "Old"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, []byte(markdown), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "Old/image.png"), []byte("image"), 0o600); err != nil {
				t.Fatal(err)
			}
			if len(loadDocumentAssets(root, source, markdown)) != 1 {
				t.Fatal("Invalid image fixture")
			}
			if operation == "rename" {
				if _, err := renameWorkspaceEntry(root, source, "New.md"); err != nil {
					t.Fatal(err)
				}
			} else {
				rewritten, err := relocateDocumentAssets(root, markdown, source, "Old.md", target)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(rewritten), 0o600); err != nil {
					t.Fatal(err)
				}
				original, err := os.ReadFile(source)
				if err != nil || string(original) != markdown {
					t.Fatalf("Save As changed source: %q, %v", original, err)
				}
			}
			saved, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if len(loadDocumentAssets(root, target, string(saved))) != 1 {
				t.Fatalf("%s lost image: %q", operation, saved)
			}
		}
	}
}
