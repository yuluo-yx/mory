package library

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const metadataStart = "\n\n<!-- mory:metadata:v1\n"
const metadataEnd = "\n-->\n"

func noteExtension(n *Note) string {
	if n.Format == "xml" {
		return ".xml"
	}
	return ".md"
}

func recordFile(name string) bool {
	switch filepath.Ext(name) {
	case ".md", ".xml", ".json":
		return true
	}
	return false
}

// Format changes retain the previous file's version check until the new
// extension has been committed. Missing files still produce a save conflict.
func (l *Library) savedNotePath(n *Note) string {
	path := l.notePath(n)
	if l.versions[n.ID] != ([32]byte{}) {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			other := *n
			other.Format = "xml"
			if n.Format == "xml" {
				other.Format = "markdown"
			}
			return l.notePath(&other)
		}
	}
	return path
}

// The source stays byte-for-byte readable, including its own front matter and
// XML declaration. A trailing comment keeps metadata in the same atomic write
// without displaying it in Markdown or changing an XML document's root.
func marshalNote(n *Note) ([]byte, error) {
	if !validID(n.ID) || (n.Format != "markdown" && n.Format != "xml") {
		return nil, fmt.Errorf("invalid note identity or format")
	}
	metadata, err := xml.MarshalIndent(n, "", "  ")
	if err != nil {
		return nil, err
	}
	// encoding/xml replaces characters that XML cannot represent. Reject those
	// metadata values instead of silently changing an old library on migration.
	var decoded Note
	if err := xml.Unmarshal(metadata, &decoded); err != nil {
		return nil, err
	}
	original, restored := reflect.ValueOf(n).Elem(), reflect.ValueOf(decoded)
	for i := range original.NumField() {
		field := original.Type().Field(i)
		if field.Tag.Get("xml") != "-" && original.Field(i).Kind() == reflect.String && original.Field(i).String() != restored.Field(i).String() {
			return nil, fmt.Errorf("note metadata %s contains characters XML cannot preserve", field.Name)
		}
	}
	// XML comments cannot contain double hyphens. Character references preserve
	// arbitrary metadata values when the embedded XML is decoded separately.
	metadata = bytes.ReplaceAll(metadata, []byte("-"), []byte("&#45;"))
	out := make([]byte, 0, len(n.Source)+len(metadata)+len(metadataStart)+len(metadataEnd))
	out = append(out, n.Source...)
	out = append(out, metadataStart...)
	out = append(out, metadata...)
	out = append(out, metadataEnd...)
	return out, nil
}

func unmarshalNote(data []byte, n *Note) error {
	startMarker, endMarker := metadataStart, metadataEnd
	start := bytes.LastIndex(data, []byte(startMarker))
	// Editors may normalize the entire document to CRLF or trim the final LF.
	// Accept those changes without normalizing the authored source ourselves.
	crlfStart := strings.ReplaceAll(metadataStart, "\n", "\r\n")
	if crlf := bytes.LastIndex(data, []byte(crlfStart)); crlf > start {
		start, startMarker, endMarker = crlf, crlfStart, strings.ReplaceAll(metadataEnd, "\n", "\r\n")
	}
	if start < 0 {
		// JSON is accepted only for migration of existing libraries and snapshots.
		if err := json.Unmarshal(data, n); err != nil {
			return fmt.Errorf("read legacy note or missing Mory metadata: %w", err)
		}
	} else {
		trimmed := bytes.TrimRight(data, "\r\n\t ")
		endMarker = strings.TrimRight(endMarker, "\r\n")
		if !bytes.HasSuffix(trimmed, []byte(endMarker)) {
			return fmt.Errorf("incomplete Mory metadata")
		}
		if err := xml.Unmarshal(trimmed[start+len(startMarker):len(trimmed)-len(endMarker)], n); err != nil {
			return fmt.Errorf("read Mory metadata: %w", err)
		}
		n.Source = string(data[:start])
	}
	if !validID(n.ID) || (n.Format != "markdown" && n.Format != "xml") {
		return fmt.Errorf("invalid note identity or format")
	}
	return nil
}

// migrateRecord installs a complete native document before removing the legacy
// record. Link/unlink makes interruption retryable and never overwrites a target.
// The caller holds the library lock.
func migrateRecord(path string, data []byte, n *Note) (string, []byte, error) {
	if filepath.Ext(path) != ".json" {
		return path, data, nil
	}
	target := strings.TrimSuffix(path, ".json") + noteExtension(n)
	native, err := marshalNote(n)
	if err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mory-*")
	if err != nil {
		return "", nil, err
	}
	defer os.Remove(f.Name())
	mode := fs.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(native)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return "", nil, errors.Join(err, closeErr)
	}
	if err = os.Link(f.Name(), target); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return "", nil, err
		}
		existing, readErr := os.ReadFile(target)
		var other Note
		if readErr != nil || unmarshalNote(existing, &other) != nil {
			return "", nil, fmt.Errorf("migration destination already exists: %s: %w", target, ErrConflict)
		}
		canonical, encodeErr := marshalNote(&other)
		if encodeErr != nil || !bytes.Equal(canonical, native) {
			return "", nil, fmt.Errorf("conflicting migration destination: %s: %w", target, ErrConflict)
		}
		native = existing
	}
	if err := os.Remove(path); err != nil {
		return "", nil, err
	}
	return target, native, nil
}

// migrateSnapshots converts old history and recovery records without changing
// timestamps, source, or retention. Non-record files are left alone.
func (l *Library) migrateSnapshots() error {
	for _, dir := range []string{"history", "recovery"} {
		err := filepath.WalkDir(filepath.Join(l.Dir, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var n Note
			if err := unmarshalNote(data, &n); err != nil {
				return fmt.Errorf("migrate %s: %w", path, err)
			}
			if dir == "history" && filepath.Base(filepath.Dir(path)) != n.ID {
				return fmt.Errorf("invalid history note ID in %s", path)
			}
			if dir == "recovery" && entry.Name() != n.ID+".json" && !strings.HasPrefix(entry.Name(), n.ID+"-") {
				return fmt.Errorf("invalid recovery note ID in %s", path)
			}
			_, _, err = migrateRecord(path, data, &n)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
