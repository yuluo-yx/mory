// Package library persists local notes as individually atomic, readable JSON files.
package library

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

var ErrConflict = errors.New("file changed on disk; reload or save a copy before continuing")

// Note contains both the exact source and library metadata in one commit unit.
type Note struct {
	Icon           string    `json:"icon,omitempty"`
	Cover          string    `json:"cover,omitempty"`
	CoverX         float32   `json:"coverX,omitempty"`
	CoverY         float32   `json:"coverY,omitempty"`
	Width          string    `json:"width,omitempty"`
	PageWidth      int       `json:"pageWidth,omitempty"` // Preferred logical pixels when Width is Custom.
	HideInfo       bool      `json:"hideInfo,omitempty"`
	ShowCreated    bool      `json:"showCreated,omitempty"`
	HideReferences bool      `json:"hideReferences,omitempty"`
	Order          int64     `json:"order,omitempty"`
	External       string    `json:"external,omitempty"`
	ExternalHash   string    `json:"externalHash,omitempty"`
	TrashRoot      string    `json:"trashRoot,omitempty"`
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Source         string    `json:"source"`
	Format         string    `json:"format"`
	Folder         string    `json:"folder,omitempty"`
	Parent         string    `json:"parent,omitempty"`
	Pinned         bool      `json:"pinned,omitempty"`
	Starred        bool      `json:"starred,omitempty"`
	Deleted        bool      `json:"deleted,omitempty"`
	Created        time.Time `json:"created"`
	Updated        time.Time `json:"updated"`
	Opened         time.Time `json:"opened"`
}

type Settings struct {
	History         HistorySettings   `json:"history,omitzero"`
	Language        string            `json:"language,omitempty"`
	OutlineSide     string            `json:"outlineSide,omitempty"`
	OutlineFontSize float32           `json:"outlineFontSize,omitempty"`
	PagePosition    string            `json:"pagePosition,omitempty"`
	Folders         []string          `json:"folders,omitempty"`
	Keys            map[string]string `json:"keys,omitempty"`
	QuickNote       string            `json:"quickNote,omitempty"`
	Theme           string            `json:"theme"`
	Appearance      string            `json:"appearance"`
	FontSize        float32           `json:"fontSize"`
	Wide            bool              `json:"wide"`
	List            bool              `json:"list"`
	ShowShortcuts   bool              `json:"showShortcuts,omitempty"`
	DisabledPlugins map[string]bool   `json:"disabledPlugins,omitempty"`
}

type Library struct {
	Dir             string
	Notes           []*Note
	Settings        Settings
	versions        map[string][32]byte
	settingsVersion [32]byte
}

func Open(dir string) (*Library, error) {
	l := &Library{Dir: dir, versions: make(map[string][32]byte), Settings: Settings{Theme: "Craft", Appearance: "System", FontSize: 17}}
	for _, child := range []string{"notes", "plugins", "assets", "history", "recovery", "themes"} {
		if err := os.MkdirAll(filepath.Join(dir, child), 0o755); err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "notes"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, "notes", entry.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var n Note
		if err := json.Unmarshal(b, &n); err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if !validID(n.ID) || entry.Name() != n.ID+".json" {
			return nil, fmt.Errorf("invalid note ID in %s", path)
		}
		if n.Format != "markdown" && n.Format != "xml" {
			return nil, fmt.Errorf("unsupported note format in %s", path)
		}
		l.Notes = append(l.Notes, &n)
		l.versions[n.ID] = sha256.Sum256(b)
	}
	b, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err == nil {
		if err = json.Unmarshal(b, &l.Settings); err != nil {
			return nil, fmt.Errorf("read settings: %w", err)
		}
		l.settingsVersion = sha256.Sum256(b)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if l.Settings.FontSize < 12 || l.Settings.FontSize > 30 {
		l.Settings.FontSize = 17
	}
	if l.Settings.DisabledPlugins == nil {
		l.Settings.DisabledPlugins = map[string]bool{}
	}
	if l.Settings.Keys == nil {
		l.Settings.Keys = map[string]string{}
	}
	return l, nil
}

func validID(id string) bool {
	if len(id) != 26 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= '2' && r <= '7') {
			return false
		}
	}
	return true
}

func (l *Library) Get(id string) *Note {
	for _, n := range l.Notes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

func (l *Library) Create(title, source, format string) (*Note, error) {
	now := time.Now()
	n := &Note{ID: strings.ToLower(rand.Text()), Title: title, Source: source, Format: format, Created: now, Updated: now}
	if err := l.Save(n); err != nil {
		return nil, err
	}
	l.Notes = append(l.Notes, n)
	return n, nil
}

// Save detects another writer before atomically replacing this note. UI changes
// remain in memory on error, so failed saves never masquerade as success.
func (l *Library) Save(n *Note) error {
	return l.save(n, false, time.Now())
}

// SaveWithHistory preserves the current saved version even within the ordinary
// checkpoint interval. Use it before replacing a document with an old revision.
func (l *Library) SaveWithHistory(n *Note) error {
	return l.save(n, true, time.Now())
}

func (l *Library) save(n *Note, forceHistory bool, now time.Time) error {
	if !validID(n.ID) {
		return errors.New("invalid note ID")
	}
	if n.Format != "markdown" && n.Format != "xml" {
		return errors.New("unsupported note format")
	}
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(l.Dir, "notes", n.ID+".json")
	if err := checkVersion(path, l.versions[n.ID]); err != nil {
		return err
	}
	if err := l.beforeSave(n, path, forceHistory, now); err != nil {
		return err
	}
	b, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return err
	}
	if err = AtomicWrite(path, b); err != nil {
		return err
	}
	l.versions[n.ID] = sha256.Sum256(b)
	_ = os.Remove(filepath.Join(l.Dir, "recovery", n.ID+".json"))
	_ = os.Remove(l.recoveryPath(n))
	return nil
}

func checkVersion(path string, want [32]byte) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if want != ([32]byte{}) {
			return ErrConflict
		}
		return nil
	}
	if err != nil {
		return err
	}
	if sha256.Sum256(b) != want {
		return ErrConflict
	}
	return nil
}

// ReloadSettings refreshes preferences without disturbing open note versions.
func (l *Library) ReloadSettings() error {
	b, err := os.ReadFile(filepath.Join(l.Dir, "settings.json"))
	if err != nil {
		return err
	}
	var settings Settings
	if err := json.Unmarshal(b, &settings); err != nil {
		return err
	}
	if settings.Keys == nil {
		settings.Keys = map[string]string{}
	}
	if settings.DisabledPlugins == nil {
		settings.DisabledPlugins = map[string]bool{}
	}
	l.Settings, l.settingsVersion = settings, sha256.Sum256(b)
	return nil
}

func (l *Library) SaveSettings() error {
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(l.Dir, "settings.json")
	if err := checkVersion(path, l.settingsVersion); err != nil {
		return err
	}
	b, err := json.MarshalIndent(l.Settings, "", "  ")
	if err != nil {
		return err
	}
	if err = AtomicWrite(path, b); err != nil {
		return err
	}
	l.settingsVersion = sha256.Sum256(b)
	return nil
}

func (l *Library) Duplicate(n *Note) (*Note, error) {
	title := n.Title
	if strings.TrimSpace(title) != "" {
		title += " copy"
	}
	copy, err := l.Create(title, n.Source, n.Format)
	if err != nil {
		return nil, err
	}
	copy.Folder = n.Folder
	copy.Parent = n.Parent
	copy.Icon, copy.Cover, copy.CoverX, copy.CoverY = n.Icon, n.Cover, n.CoverX, n.CoverY
	copy.Width, copy.HideInfo, copy.ShowCreated = n.Width, n.HideInfo, n.ShowCreated
	copy.PageWidth = n.PageWidth
	return copy, l.Save(copy)
}

// SetDeleted keeps every byte of a note; restoring does not depend on a second index.
func (l *Library) SetDeleted(n *Note, deleted bool) error {
	old := n.Deleted
	n.Deleted = deleted
	if err := l.Save(n); err != nil {
		n.Deleted = old
		return err
	}
	return nil
}

func (l *Library) Purge(n *Note) error {
	if !validID(n.ID) {
		return errors.New("invalid note ID")
	}
	if !n.Deleted {
		return errors.New("only trashed notes can be permanently deleted")
	}
	unlock, err := l.lock()
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(l.Dir, "notes", n.ID+".json")
	if err := checkVersion(path, l.versions[n.ID]); err != nil {
		return err
	}
	if err := l.removeNoteCopies(n.ID); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	for i, item := range l.Notes {
		if item.ID == n.ID {
			l.Notes = append(l.Notes[:i], l.Notes[i+1:]...)
			break
		}
	}
	delete(l.versions, n.ID)
	return nil
}

// A process lock closes the check-then-rename race between two app instances.
// macOS releases it automatically if a writer crashes.
func (l *Library) lock() (func(), error) {
	f, err := os.OpenFile(filepath.Join(l.Dir, ".write.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}

func (l *Library) Query(section, query, folder, order string) []*Note {
	result := []*Note{}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, n := range l.Notes {
		if n.Deleted != (section == "Trash") {
			continue
		}
		if section == "Favorites" && !n.Starred {
			continue
		}
		if section == "Recent" && n.Opened.IsZero() {
			continue
		}
		if folder != "" && n.Folder != folder {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(n.Title+"\n"+n.Source), query) {
			continue
		}
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		if order == "Manual" {
			if a.Order != b.Order {
				return a.Order < b.Order
			}
		} else if order == "Title" {
			if a.Title != b.Title {
				return strings.ToLower(a.Title) < strings.ToLower(b.Title)
			}
		} else if order == "Created" {
			if !a.Created.Equal(b.Created) {
				return a.Created.After(b.Created)
			}
		} else if section == "Recent" {
			if !a.Opened.Equal(b.Opened) {
				return a.Opened.After(b.Opened)
			}
		} else if !a.Updated.Equal(b.Updated) {
			return a.Updated.After(b.Updated)
		}
		return a.ID < b.ID
	})
	return result
}

func (l *Library) Folders() []string {
	set := map[string]bool{}
	for _, folder := range l.Settings.Folders {
		set[folder] = true
	}
	for _, n := range l.Notes {
		if !n.Deleted && n.Folder != "" {
			set[n.Folder] = true
		}
	}
	result := []string{}
	for f := range set {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// AtomicWrite syncs a sibling temporary file before rename. The old file remains
// intact until the replacement succeeds, and temporary files are always removed.
func AtomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".mory-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	mode := fs.FileMode(0o644)
	if info, e := os.Stat(path); e == nil {
		mode = info.Mode().Perm()
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
