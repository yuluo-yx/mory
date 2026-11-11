package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestWelcomeGuidePreservesUserEdits(t *testing.T) {
	a := testApp(t)
	a.openWelcome()
	first := a.current
	if first == nil || first.Source != welcome {
		t.Fatal("welcome guide not opened")
	}
	image, err := os.ReadFile(filepath.Join(a.lib.Dir, "assets", "mory-welcome.png"))
	if err != nil || !bytes.Equal(image, iconPNG) {
		t.Fatal("offline artwork missing", err)
	}
	a.openWelcome()
	if a.current != first {
		t.Fatal("unchanged guide duplicated")
	}
	first.Source = "My personal notes"
	if err := a.lib.Save(first); err != nil {
		t.Fatal(err)
	}
	a.loadEditors()
	a.openWelcome()
	if a.current == first || first.Source != "My personal notes" {
		t.Fatal("edited guide overwritten")
	}
	guide := a.current
	guide.Deleted = true
	if err := a.lib.Save(guide); err != nil {
		t.Fatal(err)
	}
	a.openWelcome()
	if a.current == guide || !guide.Deleted {
		t.Fatal("trashed guide restored unexpectedly")
	}
}

func TestWelcomeSyntaxAndNativePreview(t *testing.T) {
	a := testApp(t)
	a.openWelcome()
	s := a.blockEditor
	if s == nil || s.doc.Source() != welcome {
		t.Fatal("guide did not load losslessly")
	}
	kinds := map[string]bool{}
	s.doc.Root.Walk(func(n *blocks.Node) { kinds[n.Kind] = true })
	for _, kind := range []string{"h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8", "h9", "p", "ul", "ol", "checkbox", "blockquote", "callout", "pre", "table", "latex", "img", "grid", "countdown", "details", "tabs", "tab-item"} {
		if !kinds[kind] {
			t.Errorf("guide has no %s example", kind)
		}
	}
	tt := ui.NewTester(a.view, 1280, 820)
	for i := range s.visibleBlocks() {
		s.list.ScrollTo(i, ui.Start)
		tt.Frame()
	}
	if a.problem != "" {
		t.Fatal(a.problem)
	}
	for _, mode := range []string{"Source", "Split", "Read", "Write"} {
		a.changeMode(mode)
		tt.Frame()
		if a.current.Source != welcome {
			t.Fatal("mode switch rewrote the guide", mode)
		}
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "welcome.png"))
	}
}
