package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuluo-yx/mory/internal/blocks"
)

func benchmarkFixture(b *testing.B, name string) string {
	b.Helper()
	data, err := os.ReadFile(filepath.Join("fixtures", name))
	if err != nil {
		b.Fatal(err)
	}
	return string(data)
}

func BenchmarkParseStressFixtures(b *testing.B) {
	for _, name := range []string{
		"table-heavy-1m.md",
		"table-long-cells-1m.md",
		"target-shape-12x1m.md",
		"memory-heavy-1m.md",
		"structured-1m.md",
		"long-line-1m.md",
	} {
		b.Run(name, func(b *testing.B) {
			source := benchmarkFixture(b, name)
			b.SetBytes(int64(len(source)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := blocks.Parse(source, "markdown"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
