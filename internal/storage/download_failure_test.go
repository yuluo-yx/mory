package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingDownload struct {
	io.Reader
	readErr, closeErr error
	closes            int
}

func (source *failingDownload) Read(data []byte) (int, error) {
	n, err := source.Reader.Read(data)
	if err == io.EOF && source.readErr != nil {
		return n, source.readErr
	}
	return n, err
}

func (source *failingDownload) Close() error { source.closes++; return source.closeErr }

func TestFailedDownloadPreservesPreviousFile(t *testing.T) {
	for _, phase := range []string{"before data", "after data", "source close"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			name := filepath.Join(root, "note.md")
			if err := os.WriteFile(name, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("injected download failure")
			source := &failingDownload{Reader: strings.NewReader("partial"), readErr: failure}
			if phase == "before data" {
				source.Reader = strings.NewReader("")
			}
			if phase == "source close" {
				source.readErr = nil
				source.closeErr = failure
			}
			if _, err := copyRemoteFile(root, "note.md", source); !errors.Is(err, failure) {
				t.Fatalf("Unexpected error: %v", err)
			}
			data, err := os.ReadFile(name)
			if err != nil || string(data) != "original" {
				t.Fatalf("Original changed: %q, %v", data, err)
			}
			if source.closes != 1 {
				t.Fatalf("Source closed %d times", source.closes)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatalf("Temporary files remained: %v, %v", entries, err)
			}
		})
	}
}

func TestDownloadClosesSourceOnInvalidDestination(t *testing.T) {
	source := &failingDownload{Reader: strings.NewReader("data")}
	if _, err := copyRemoteFile(t.TempDir(), "../outside", source); err == nil {
		t.Fatal("Accepted an escaping destination")
	}
	if source.closes != 1 {
		t.Fatalf("Source closed %d times", source.closes)
	}
}
