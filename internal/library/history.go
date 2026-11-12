package library

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func sourceHash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

// beforeSave checkpoints the previous source and checks external-file ownership.
// A recovery copy is durable before replacing an external document, since two
// separate filesystem paths cannot be committed with a single rename.
func (l *Library) beforeSave(n *Note, path string, forceHistory bool, now time.Time) error {
	// Check ownership before creating history, so rejected external writes leave
	// neither duplicate snapshots nor a changed external document.
	var current []byte
	if n.External != "" {
		var err error
		current, err = os.ReadFile(n.External)
		if err != nil {
			return err
		}
		if sourceHash(current) != n.ExternalHash {
			return ErrConflict
		}
	}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var old Note
	if len(b) != 0 {
		if err := json.Unmarshal(b, &old); err != nil {
			return err
		}
		if old.Source != n.Source {
			if err := l.checkpoint(n.ID, b, forceHistory, now); err != nil {
				return err
			}
		}
	}
	if n.External == "" {
		return nil
	}
	if string(current) == n.Source {
		return nil
	}
	if err := l.Recover(n); err != nil {
		return err
	}
	if err := AtomicWrite(n.External, []byte(n.Source)); err != nil {
		return err
	}
	n.ExternalHash = sourceHash([]byte(n.Source))
	return nil
}

// OpenExternal tracks the original file without copying or deleting it.
func (l *Library) OpenExternal(path string) (*Note, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	for _, n := range l.Notes {
		if n.External == path {
			return n, l.Reload(n.ID)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	format := "markdown"
	if strings.EqualFold(filepath.Ext(path), ".xml") {
		format = "xml"
	}
	n, err := l.Create(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), string(b), format)
	if err != nil {
		return nil, err
	}
	n.External, n.ExternalHash = path, sourceHash(b)
	return n, l.Save(n)
}

func (l *Library) Reload(id string) error {
	n := l.Get(id)
	if n == nil {
		return fmt.Errorf("note no longer exists")
	}
	b, err := os.ReadFile(filepath.Join(l.Dir, "notes", id+".json"))
	if err != nil {
		return err
	}
	var fresh Note
	if err := json.Unmarshal(b, &fresh); err != nil {
		return err
	}
	if fresh.External != "" {
		data, err := os.ReadFile(fresh.External)
		if err != nil {
			return err
		}
		fresh.Source, fresh.ExternalHash = string(data), sourceHash(data)
	}
	*n = fresh
	l.versions[id] = sha256.Sum256(b)
	return nil
}

func (l *Library) Changed(n *Note) bool {
	if checkVersion(filepath.Join(l.Dir, "notes", n.ID+".json"), l.versions[n.ID]) != nil {
		return true
	}
	if n.External != "" {
		b, err := os.ReadFile(n.External)
		return err != nil || sourceHash(b) != n.ExternalHash
	}
	return false
}

// Recover retains unsaved edits independently of the optimistic write lock.
func (l *Library) Recover(n *Note) error {
	if !validID(n.ID) {
		return fmt.Errorf("invalid note ID")
	}
	b, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(l.recoveryPath(n), b)
}

func (l *Library) recoveryPath(n *Note) string {
	return filepath.Join(l.Dir, "recovery", n.ID+"-"+sourceHash([]byte(n.Source))[:16]+".json")
}

type Revision struct {
	Name string
	Note Note
}

func (l *Library) History(id string) ([]Revision, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid note ID")
	}
	unlock, err := l.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := l.pruneHistory(id, time.Now()); err != nil {
		return nil, err
	}
	dir := filepath.Join(l.Dir, "history", id)
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var out []Revision
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var n Note
		if err := json.Unmarshal(b, &n); err != nil {
			return nil, err
		}
		out = append(out, Revision{Name: e.Name(), Note: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

func (l *Library) Recoveries() ([]Revision, error) {
	entries, err := os.ReadDir(filepath.Join(l.Dir, "recovery"))
	if err != nil {
		return nil, err
	}
	var out []Revision
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(l.Dir, "recovery", e.Name()))
		if err != nil {
			return nil, err
		}
		var n Note
		if err := json.Unmarshal(b, &n); err != nil {
			return nil, err
		}
		out = append(out, Revision{Name: e.Name(), Note: n})
	}
	return out, nil
}
