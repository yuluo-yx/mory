package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuluo-yx/mory/internal/library"
)

func TestImportKeepsOriginalBasenames(t *testing.T) {
	for _, fixture := range []struct{ name, source string }{
		{"Original name 世界.MDX", "# A different heading\n\nBody"},
		{"Source export.xml", "<title>Different XML title</title><p>Body</p>"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			a := testApp(t)
			path := filepath.Join(t.TempDir(), fixture.name)
			if err := os.WriteFile(path, []byte(fixture.source), 0o644); err != nil {
				t.Fatal(err)
			}
			a.importFile(path)
			if a.problem != "" || a.current == nil || a.current.FileName != fixture.name {
				t.Fatal("import renamed original basename", a.current, a.problem)
			}
			first := a.current
			a.importFile(path)
			if a.problem != "" || a.current.ID == first.ID || a.current.FileName == first.FileName {
				t.Fatal("duplicate import replaced the first document", a.problem)
			}
			reopened, err := library.Open(a.lib.Dir)
			if err != nil || reopened.Get(first.ID).FileName != fixture.name || reopened.Get(first.ID).Source != fixture.source {
				t.Fatal("import did not survive reopen", err)
			}
			original, err := os.ReadFile(path)
			if err != nil || string(original) != fixture.source {
				t.Fatal("import changed the original", err)
			}
		})
	}
}
