package library

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const historyTimeForm = "20060102T150405.000000000"

// Zero-valued legacy settings keep automatic history enabled with bounded defaults.
type HistorySettings struct {
	Disabled        bool `json:"disabled,omitempty"`
	IntervalMinutes int  `json:"intervalMinutes,omitempty"`
	RetentionDays   int  `json:"retentionDays,omitempty"`
	MaxVersions     int  `json:"maxVersions,omitempty"`
	MaxMiB          int  `json:"maxMiB,omitempty"`
}

func DefaultHistorySettings() HistorySettings {
	return HistorySettings{IntervalMinutes: 5, RetentionDays: 30, MaxVersions: 100, MaxMiB: 64}
}

// HistoryPolicy validates persisted settings without rewriting the user's file.
func (l *Library) HistoryPolicy() HistorySettings {
	p, defaults := l.Settings.History, DefaultHistorySettings()
	if p.IntervalMinutes < 1 || p.IntervalMinutes > 60 {
		p.IntervalMinutes = defaults.IntervalMinutes
	}
	if p.RetentionDays < 1 || p.RetentionDays > 365 {
		p.RetentionDays = defaults.RetentionDays
	}
	if p.MaxVersions < 1 || p.MaxVersions > 1000 {
		p.MaxVersions = defaults.MaxVersions
	}
	if p.MaxMiB < 1 || p.MaxMiB > 1024 {
		p.MaxMiB = defaults.MaxMiB
	}
	return p
}

type historyFile struct {
	name    string
	at      time.Time
	size    int64
	restore bool
}

// historyFiles also recognizes legacy timestamp names. Unknown files are never
// removed by automatic retention. The caller holds the library write lock.
func (l *Library) historyFiles(id string) ([]historyFile, error) {
	dir := filepath.Join(l.Dir, "history", id)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []historyFile
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || !e.Type().IsRegular() {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		restore := strings.HasSuffix(name, "-restore")
		at, err := time.Parse(historyTimeForm, strings.TrimSuffix(name, "-restore"))
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, historyFile{e.Name(), at, info.Size(), restore})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name > files[j].name })
	return files, nil
}

// checkpoint retains the previous saved content at most once per interval.
// Restore checkpoints bypass coalescing so restoring an old version is reversible.
func (l *Library) checkpoint(id string, previous []byte, force bool, now time.Time) error {
	policy := l.HistoryPolicy()
	if policy.Disabled && !force {
		return nil
	}
	files, err := l.historyFiles(id)
	if err != nil {
		return err
	}
	create := true
	for _, f := range files {
		if !f.restore {
			create = now.Sub(f.at) >= time.Duration(policy.IntervalMinutes)*time.Minute
			break
		}
	}
	if force || create {
		dir := filepath.Join(l.Dir, "history", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		// Keep explicit restore checkpoints newest even after a clock rollback,
		// and never overwrite a checkpoint with the same timestamp.
		stamp := now.UTC()
		if len(files) > 0 && !stamp.After(files[0].at) {
			stamp = files[0].at.Add(time.Nanosecond)
		}
		name := stamp.Format(historyTimeForm)
		if force {
			name += "-restore"
		}
		if err := AtomicWrite(filepath.Join(dir, name+".json"), previous); err != nil {
			return err
		}
		files = append([]historyFile{{name: name + ".json", at: stamp, size: int64(len(previous)), restore: force}}, files...)
	}
	// Reuse the locked directory snapshot instead of scanning every file twice.
	return l.pruneHistoryFiles(id, files, policy, now)
}

func (l *Library) pruneHistory(id string, now time.Time) error {
	policy := l.HistoryPolicy()
	if policy.Disabled {
		return nil
	}
	files, err := l.historyFiles(id)
	if err != nil {
		return err
	}
	return l.pruneHistoryFiles(id, files, policy, now)
}

func (l *Library) pruneHistoryFiles(id string, files []historyFile, policy HistorySettings, now time.Time) error {
	if policy.Disabled {
		return nil
	}
	var count int
	var size int64
	var lastNormal time.Time
	for _, f := range files {
		keep := count == 0 || (count < policy.MaxVersions && !f.at.Before(now.Add(-time.Duration(policy.RetentionDays)*24*time.Hour)) &&
			size+f.size <= int64(policy.MaxMiB)<<20 && (f.restore || lastNormal.IsZero() || lastNormal.Sub(f.at) >= time.Duration(policy.IntervalMinutes)*time.Minute))
		if keep {
			// Even an oversized or old document keeps its latest recovery point.
			count++
			size += f.size
			if !f.restore {
				lastNormal = f.at
			}
			continue
		}
		if err := os.Remove(filepath.Join(l.Dir, "history", id, f.name)); err != nil {
			return err
		}
	}
	return nil
}

// removeNoteCopies only runs for explicit permanent deletion. Shared assets and
// external originals are not owned by the note and must remain untouched.
func (l *Library) removeNoteCopies(id string) error {
	dir := filepath.Join(l.Dir, "history", id)
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	if err := os.Remove(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	entries, err = os.ReadDir(filepath.Join(l.Dir, "recovery"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type().IsRegular() && (e.Name() == id+".json" || strings.HasPrefix(e.Name(), id+"-") && strings.HasSuffix(e.Name(), ".json")) {
			if err := os.Remove(filepath.Join(l.Dir, "recovery", e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
