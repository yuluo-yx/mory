package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsReloadPreservesNoteVersions(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, err := l.Create("Draft", "Original", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Settings.PagePosition = "Right"
	other.Settings.OutlineFontSize = 19
	if err := other.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err != nil {
		t.Fatal(err)
	}
	if l.Settings.PagePosition != "Right" || l.Settings.OutlineFontSize != 19 {
		t.Fatal("stale preferences")
	}
	n.Source = "Edited after reload"
	if err := l.Save(n); err != nil {
		t.Fatal("reload lost note version", err)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"Docusaurus"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err != nil || l.Settings.Keys == nil || l.Settings.DisabledPlugins == nil {
		t.Fatal("legacy defaults", err)
	}
	if err := os.WriteFile(path, []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err == nil || l.Settings.Theme != "Docusaurus" {
		t.Fatal("invalid settings replaced live preferences")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := l.ReloadSettings(); err == nil {
		t.Fatal("missing settings accepted")
	}
}
