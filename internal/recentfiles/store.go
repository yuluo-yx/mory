// Package recentfiles persists recent documents and workspaces used by the Windows host.
package recentfiles

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const maximumEntries = 10

var supportedExtensions = map[string]bool{
	".md": true, ".markdown": true, ".mmd": true, ".mdown": true, ".mkd": true,
	".txt": true, ".text": true,
}

// Store owns a bounded, most-recent-first list of document and workspace paths.
type Store struct {
	mu   sync.Mutex
	path string
}

// New creates a recent-document store at path.
func New(path string) *Store { return &Store{path: path} }

// List returns existing supported documents and workspaces in most-recent-first order.
func (store *Store) List() ([]string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	entries, err := store.read()
	if err != nil {
		return nil, err
	}
	filtered := filter(entries)
	if len(filtered) != len(entries) {
		if err := store.write(filtered); err != nil {
			return nil, err
		}
	}
	return append([]string(nil), filtered...), nil
}

// Add moves a document or workspace to the beginning of the recent list.
func (store *Store) Add(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !isSupportedEntry(absolute) {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	entries, err := store.read()
	if err != nil {
		return err
	}
	next := []string{absolute}
	for _, entry := range entries {
		if samePath(entry, absolute) || !isSupportedEntry(entry) {
			continue
		}
		next = append(next, entry)
		if len(next) == maximumEntries {
			break
		}
	}
	return store.write(next)
}

// Clear removes all recent-document entries.
func (store *Store) Clear() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.write(nil)
}

func (store *Store) read() ([]string, error) {
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (store *Store) write(entries []string) error {
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(store.path, data, 0o600)
}

func filter(entries []string) []string {
	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !isSupportedEntry(entry) {
			continue
		}
		duplicate := false
		for _, current := range filtered {
			if samePath(current, entry) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			filtered = append(filtered, entry)
		}
		if len(filtered) == maximumEntries {
			break
		}
	}
	return filtered
}

func isSupportedEntry(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir() || (info.Mode().IsRegular() && supportedExtensions[strings.ToLower(filepath.Ext(path))])
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
