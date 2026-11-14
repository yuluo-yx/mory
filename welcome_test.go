package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestOpenWelcomeSelectsGuide(t *testing.T) {
	a := testApp(t)
	a.openWelcome()
	if a.current == nil || a.current.Title != "Welcome to Mory" {
		t.Fatalf("welcome guide was not opened: current=%+v problem=%s", a.current, a.problem)
	}
}

func TestFirstLibraryGetsBothWelcomeGuides(t *testing.T) {
	a := testApp(t)
	if err := a.ensureWelcomeNotes(); err != nil {
		t.Fatal(err)
	}
	if len(a.lib.Notes) != 2 {
		t.Fatalf("expected two welcome guides, got %d", len(a.lib.Notes))
	}
	var english, chinese bool
	for _, n := range a.lib.Notes {
		switch n.Title {
		case "Welcome to Mory":
			english = n.Source == welcome
		case "欢迎使用 Mory":
			chinese = n.Source == welcomeChinese
		}
	}
	if !english || !chinese {
		t.Fatalf("welcome guides were not seeded correctly: english=%v chinese=%v", english, chinese)
	}
	if _, err := blocks.Parse(welcomeChinese, "markdown"); err != nil {
		t.Fatalf("Chinese welcome guide is not valid Markdown: %v", err)
	}
}

func TestWelcomeAssetIsRestoredForExistingLibrary(t *testing.T) {
	a := testApp(t)
	if _, err := a.lib.Create("Existing", "Text", "markdown"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.lib.Dir, "assets", "mory-welcome.png")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := a.ensureWelcomeNotes(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("welcome asset was not restored: %v", err)
	}
	if len(a.lib.Notes) != 1 {
		t.Fatalf("existing library was seeded with welcome notes: %d", len(a.lib.Notes))
	}
}
