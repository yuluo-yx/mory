package main

import (
	"os"
	"strings"
	"testing"

	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestLargeDocumentsOpenInWriteMode(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"prose", strings.Repeat("A responsive paragraph.\n\n", 22000)},
		{"table", "| A | B |\n| --- | --- |\n" + strings.Repeat("| value | value |\n", 32000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			n, err := a.lib.Create(tc.name, tc.source, "markdown")
			if err != nil {
				t.Fatal(err)
			}
			a.openNote(n)
			if a.mode != "Write" || a.blockEditor == nil || a.doc != nil {
				t.Fatal("large document did not open directly in the block editor")
			}
			if a.current.Source != tc.source || a.blockEditor.sourceRevision != a.sourceRevision {
				t.Fatal("opening the document changed or invalidated its source")
			}
		})
	}
}

func TestVeryLongLineUsesSourceMode(t *testing.T) {
	a := testApp(t)
	n, err := a.lib.Create("Long line", strings.Repeat("x", maxWriteLineBytes+1), "markdown")
	if err != nil {
		t.Fatal(err)
	}
	a.openNote(n)
	if a.mode != "Source" {
		t.Fatalf("very long line opened in %s mode", a.mode)
	}
	if a.blockEditor != nil || a.doc != nil {
		t.Fatal("very long line initialized a rich or block editor")
	}
	a.changeMode("Write")
	if a.mode != "Source" || !strings.Contains(a.status, "Source mode") {
		t.Fatalf("Write mode was not guarded: mode=%s status=%q", a.mode, a.status)
	}
	a.changeMode("Read")
	if a.mode != "Source" {
		t.Fatalf("Read mode was not guarded: mode=%s", a.mode)
	}
}

// Measure lossless serialization separately from rendering and capture.
func BenchmarkStructuredSource(b *testing.B) {
	for _, name := range []string{"structured-1m", "memory-heavy-1m"} {
		b.Run(name, func(b *testing.B) {
			source, err := os.ReadFile("benchmark/fixtures/" + name + ".md")
			if err != nil {
				b.Fatal(err)
			}
			doc, err := blocks.Parse(string(source), "markdown")
			if err != nil {
				b.Fatal(err)
			}
			if err := doc.Splice(doc.Root.ID, 0, 0, blocks.Paragraph("Changed")); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = doc.Source()
			}
		})
	}
}
