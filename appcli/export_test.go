package appcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestExportPreservesDestinationWhenRenderingFails(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new output", true: "replacement"}[existing], func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "note.md")
			output := filepath.Join(root, "note.pdf")
			if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
				t.Fatal(err)
			}
			if existing {
				if err := os.WriteFile(output, []byte("original"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			request, err := ResolveExport(source, "pdf", root, existing)
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("renderer stopped after partial output")
			err = renderExport(t.Context(), request, func(_ context.Context, staged ExportRequest) error {
				if err := os.WriteFile(staged.Destination, []byte("partial"), 0o600); err != nil {
					t.Fatal(err)
				}
				return failure
			})
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			data, err := os.ReadFile(output)
			if existing && (err != nil || string(data) != "original") {
				t.Fatalf("existing output changed: %q, %v", data, err)
			}
			if !existing && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed export left output: %q, %v", data, err)
			}
			assertNoExportStaging(t, root)
		})
	}
}

func TestConcurrentExportsNeverOverwriteWithoutForce(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "note.md")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	request, err := ResolveExport(source, "pdf", root, false)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, value := range []string{"first complete output", "second complete output"} {
		workers.Go(func() {
			results <- renderExport(t.Context(), request, func(_ context.Context, staged ExportRequest) error {
				err := os.WriteFile(staged.Destination, []byte(value), 0o600)
				ready <- struct{}{}
				<-release
				return err
			})
		})
	}
	<-ready
	<-ready
	close(release)
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, os.ErrExist) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful exports = %d; want exactly one", successes)
	}
	data, err := os.ReadFile(request.Destination)
	if err != nil || (string(data) != "first complete output" && string(data) != "second complete output") {
		t.Fatalf("output = %q, %v", data, err)
	}
	assertNoExportStaging(t, root)
}

func TestExportRejectsLateCollisionsAndIncompleteResults(t *testing.T) {
	for _, scenario := range []string{"late file", "late directory", "no output", "empty output", "symlink output", "cancelled", "replace"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "note.md")
			if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
				t.Fatal(err)
			}
			request, err := ResolveExport(source, "pdf", root, scenario == "replace")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			err = renderExport(ctx, request, func(_ context.Context, staged ExportRequest) error {
				if scenario == "late file" || scenario == "replace" {
					if err := os.WriteFile(request.Destination, []byte("concurrent"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "late directory" {
					if err := os.Mkdir(request.Destination, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "no output" {
					return nil
				}
				if scenario == "symlink output" {
					if err := os.Symlink(source, staged.Destination); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
					return nil
				}
				data := []byte("complete")
				if scenario == "empty output" {
					data = nil
				}
				if scenario == "cancelled" {
					cancel()
				}
				return os.WriteFile(staged.Destination, data, 0o600)
			})
			if scenario == "replace" {
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(request.Destination)
				if err != nil || string(data) != "complete" {
					t.Fatalf("replacement = %q, %v", data, err)
				}
			} else if err == nil {
				t.Fatal("unsafe or incomplete export succeeded")
			}
			if scenario == "late file" {
				data, err := os.ReadFile(request.Destination)
				if err != nil || string(data) != "concurrent" {
					t.Fatalf("concurrent output changed: %q, %v", data, err)
				}
			}
			assertNoExportStaging(t, root)
		})
	}
}

func TestCancelledExportDoesNotStartRenderer(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := renderExport(ctx, ExportRequest{}, func(context.Context, ExportRequest) error {
		t.Fatal("renderer started for a cancelled export")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v; want context.Canceled", err)
	}
}

func assertNoExportStaging(t *testing.T, root string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, ".mory-export-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staging files left behind: %v, %v", matches, err)
	}
}
