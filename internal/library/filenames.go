package library

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// FileName is a basename, never a path. Imported Unicode, spaces, punctuation,
// and extension casing remain intact; only generated names need sanitizing.
func validDocumentName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 255 || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\\x00") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".mdx", ".xml":
		return true
	}
	return false
}

func storageFilename(n *Note) string {
	if n.FileName == "" || !validDocumentName(n.FileName) {
		return n.ID + noteExtension(n)
	}
	ext := filepath.Ext(n.FileName)
	if (strings.EqualFold(ext, ".xml")) == (n.Format == "xml") {
		return n.FileName
	}
	return strings.TrimSuffix(n.FileName, ext) + noteExtension(n)
}

func trimFilenameBytes(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	for limit > 0 && !utf8.RuneStart(name[limit]) {
		limit--
	}
	return name[:limit]
}

func readableFilename(title, extension string) string {
	stem := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune("/:\\", r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(title))
	stem = strings.TrimLeft(stem, ".")
	if stem == "" {
		stem = "Untitled"
	}
	return trimFilenameBytes(stem, 255-len(extension)) + extension
}

// APFS commonly treats case and Unicode normalization variants as one name.
// Renaming that same directory entry is safe; linking then unlinking it would
// remove the only entry. Other existing destinations still remain conflicts.
func sameFilenameAlias(from, to string) bool {
	if from == to || filepath.Dir(from) != filepath.Dir(to) || !strings.EqualFold(norm.NFC.String(filepath.Base(from)), norm.NFC.String(filepath.Base(to))) {
		return false
	}
	a, err := os.Lstat(from)
	if err != nil {
		return false
	}
	b, err := os.Lstat(to)
	return err == nil && a.Mode().IsRegular() && b.Mode().IsRegular() && os.SameFile(a, b)
}

// Reserve names across live notes and Trash, including case/Unicode aliases,
// so a later restore never collides with a document created after deletion.
// The caller holds the library lock; unrelated disk entries also reserve names.
func (l *Library) availableFilename(preferred, id string) (string, error) {
	used := map[string]bool{}
	key := func(name string) string { return strings.ToLower(norm.NFC.String(name)) }
	for _, dir := range []string{"notes", "trash"} {
		entries, err := os.ReadDir(filepath.Join(l.Dir, dir))
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			path := filepath.Join(l.Dir, dir, entry.Name())
			if path == l.paths[id] {
				continue
			}
			used[key(entry.Name())] = true
		}
	}
	ext := filepath.Ext(preferred)
	stem := strings.TrimSuffix(preferred, ext)
	for i := 1; i <= len(used)+1; i++ {
		suffix := ""
		if i > 1 {
			suffix = fmt.Sprintf(" (%d)", i)
		}
		candidate := trimFilenameBytes(stem, 255-len(ext)-len(suffix)) + suffix + ext
		if !used[key(candidate)] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot allocate document filename")
}

// Initial creation uses a fully written temporary inode and a no-replace link.
// This preserves the existing writer's lock and conflict semantics without an
// overwrite window between checking the chosen basename and installing it.
func writeNewRecord(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".mory-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0o644); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err := os.Link(f.Name(), path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("destination already exists: %s: %w", path, ErrConflict)
		}
		return err
	}
	return nil
}
