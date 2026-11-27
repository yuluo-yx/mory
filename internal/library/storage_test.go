package library

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNativeStoragePreservesSourceAndMetadata(t *testing.T) {
	for _, tc := range []struct{ format, source string }{
		{"markdown", ""},
		{"markdown", "---\r\ntitle: User front matter\r\n---\r\n# 世界 🌱\r\n\r\n![image](assets/image.png)"},
		{"markdown", "```md\n" + metadataStart + "example\n-->\n```\n"},
		{"markdown", "raw bytes\x00\xff\n"},
		{"xml", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<doc><p>世界 &amp; XML</p></doc>\n"},
		{"xml", "<title>Fragment</title><p>Body</p>"},
	} {
		t.Run(tc.format+"/"+tc.source, func(t *testing.T) {
			l := mustLibrary(t)
			n, err := l.Create("Title -- <>&\"\n世界", tc.source, tc.format)
			if err != nil {
				t.Fatal(err)
			}
			n.Icon, n.Cover = "🌱", "assets/cover--image.png"
			n.CoverX, n.CoverY, n.Width, n.PageWidth = -0.25, 0.75, "Custom", 900
			n.HideInfo, n.ShowCreated, n.HideReferences = true, true, true
			n.Order, n.Folder, n.Parent = -7, "Work\nNotes", strings.Repeat("b", 26)
			n.Pinned, n.Starred = true, true
			n.Opened = time.Now().UTC().Round(0)
			if err := l.Save(n); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(l.notePath(n))
			if err != nil || !bytes.HasPrefix(data, []byte(tc.source+metadataStart)) || json.Valid(data) {
				t.Fatal("source is not a native document", err)
			}
			metadata := data[len(tc.source)+len(metadataStart) : len(data)-len(metadataEnd)]
			if bytes.Contains(metadata, []byte("--")) || bytes.Contains(metadata, []byte("<source>")) {
				t.Fatal("invalid comment or duplicate source")
			}
			opened, err := Open(l.Dir)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := json.Marshal(n)
			got, _ := json.Marshal(opened.Get(n.ID))
			if !bytes.Equal(want, got) {
				t.Fatalf("round trip changed note:\n%s\n%s", want, got)
			}
			if tc.format == "xml" {
				decoder := xml.NewDecoder(bytes.NewReader(data))
				for {
					_, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal("metadata invalidated XML", err)
					}
				}
			}
		})
	}
}

func writeLegacy(t *testing.T, path string, n *Note) []byte {
	t.Helper()
	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, data)
	return data
}

func TestLegacyMigrationIncludesTrashHistoryAndRecovery(t *testing.T) {
	for _, format := range []string{"markdown", "xml"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			now := time.Now().UTC().Round(0)
			n := &Note{ID: strings.Repeat("a", 26), Title: "Legacy", Format: format, Source: "original\r\n", Created: now, Updated: now, Starred: true}
			writeLegacy(t, filepath.Join(dir, "notes", n.ID+".json"), n)
			old := *n
			old.Source = "old version"
			writeLegacy(t, filepath.Join(dir, "history", n.ID, now.Format(historyTimeForm)+".json"), &old)
			recovery := *n
			recovery.Source, recovery.External, recovery.ExternalHash = "unsaved", "/missing/external.md", "original-hash"
			writeLegacy(t, filepath.Join(dir, "recovery", n.ID+".json"), &recovery)
			deleted := *n
			deleted.ID, deleted.Parent, deleted.Deleted, deleted.TrashRoot, deleted.DeletedAt = strings.Repeat("b", 26), n.ID, true, strings.Repeat("b", 26), now
			writeLegacy(t, filepath.Join(dir, "trash", deleted.ID+".json"), &deleted)
			l, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []*Note{n, &deleted} {
				want.FileName = l.Get(want.ID).FileName
				if !reflect.DeepEqual(l.Get(want.ID), want) {
					t.Fatalf("migration changed note: %+v", l.Get(want.ID))
				}
				data, err := os.ReadFile(l.notePath(want))
				if err != nil || !bytes.HasPrefix(data, []byte(want.Source)) {
					t.Fatal("missing native document", err)
				}
			}
			history, err := l.History(n.ID)
			if err != nil || len(history) != 1 || !reflect.DeepEqual(history[0].Note, old) {
				t.Fatal("history changed", history, err)
			}
			recoveries, err := l.Recoveries()
			if err != nil || len(recoveries) != 1 || !reflect.DeepEqual(recoveries[0].Note, recovery) {
				t.Fatal("recovery changed", recoveries, err)
			}
			if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
				if err == nil && (strings.HasSuffix(path, ".json") || strings.HasPrefix(entry.Name(), ".mory-")) {
					t.Errorf("legacy or temporary record remains: %s", path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(dir); err != nil {
				t.Fatal("migration is not repeatable", err)
			}
			l.Get(n.ID).Source = "edited"
			if err := l.Save(l.Get(n.ID)); err != nil {
				t.Fatal("migrated version cannot save", err)
			}
			if _, err := l.RestoreTree(l.Get(deleted.ID)); err != nil {
				t.Fatal("migrated trash cannot restore", err)
			}
		})
	}
}

func TestInterruptedMigrationAndConflicts(t *testing.T) {
	for _, location := range []string{"notes", "history", "recovery"} {
		for _, conflict := range []bool{false, true} {
			t.Run(location+"/"+map[bool]string{false: "resume", true: "conflict"}[conflict], func(t *testing.T) {
				l := mustLibrary(t)
				n := mustNote(t, l, "Migration")
				dir, name := filepath.Join(l.Dir, location), n.ID
				if location == "history" {
					dir, name = filepath.Join(dir, n.ID), time.Now().UTC().Format(historyTimeForm)
				}
				legacy := filepath.Join(dir, name+".json")
				original := writeLegacy(t, legacy, n)
				other := *n
				if conflict {
					other.Source = "competing native content"
				}
				native, _ := marshalNote(&other)
				target := filepath.Join(dir, name+".md")
				mustWrite(t, target, native)
				_, err := Open(l.Dir)
				if conflict {
					if !errors.Is(err, ErrConflict) {
						t.Fatal("conflict was not detected", err)
					}
					for path, want := range map[string][]byte{legacy: original, target: native} {
						got, err := os.ReadFile(path)
						if err != nil || !bytes.Equal(want, got) {
							t.Fatal("conflict overwrote data", path, err)
						}
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if _, err := os.Stat(legacy); !os.IsNotExist(err) {
						t.Fatal("interrupted migration did not finish", err)
					}
				}
			})
		}
	}
}

func TestNativeExternalEditAndOpen(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Native")
	path := l.notePath(n)
	data, _ := os.ReadFile(path)
	changed := bytes.Replace(data, []byte(n.Source), []byte("Edited by another editor\n"), 1)
	mustWrite(t, path, changed)
	if err := l.Save(n); !errors.Is(err, ErrConflict) {
		t.Fatal("external edit overwritten", err)
	}
	if err := l.Reload(n.ID); err != nil || n.Source != "Edited by another editor\n" {
		t.Fatal("external edit did not reload", err)
	}
	opened, err := l.OpenExternal(path)
	if err != nil || opened != n || n.External != "" || len(l.Notes) != 1 {
		t.Fatal("native document was duplicated as an external file", err)
	}
}

func TestNativeEditorLineEndings(t *testing.T) {
	n := &Note{ID: strings.Repeat("a", 26), Format: "markdown", Title: "Title", Source: "Line one\nLine two"}
	data, _ := marshalNote(n)
	for _, crlf := range []bool{false, true} {
		for _, finalNewline := range []bool{false, true} {
			edited, want := data, n.Source
			if crlf {
				edited = bytes.ReplaceAll(edited, []byte("\n"), []byte("\r\n"))
				want = strings.ReplaceAll(want, "\n", "\r\n")
			}
			if !finalNewline {
				edited = bytes.TrimRight(edited, "\r\n")
			}
			var read Note
			if err := unmarshalNote(edited, &read); err != nil || read.Source != want || read.Title != n.Title {
				t.Fatal("editor line-ending change lost data", err)
			}
		}
	}
}

func TestMigrationFailurePreservesLegacy(t *testing.T) {
	for _, obstacle := range []string{"directory", "corrupt", "wrong identity"} {
		t.Run(obstacle, func(t *testing.T) {
			l := mustLibrary(t)
			n := mustNote(t, l, "Legacy snapshot")
			legacy := filepath.Join(l.Dir, "recovery", n.ID+".json")
			original := writeLegacy(t, legacy, n)
			target := strings.TrimSuffix(legacy, ".json") + ".md"
			switch obstacle {
			case "directory":
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				mustWrite(t, target, []byte("Keep this file"))
			case "wrong identity":
				wrong := *n
				wrong.ID = strings.Repeat("b", 26)
				writeLegacy(t, legacy, &wrong)
				original, _ = os.ReadFile(legacy)
			}
			if _, err := Open(l.Dir); err == nil {
				t.Fatal("migration failure ignored")
			}
			got, err := os.ReadFile(legacy)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatal("failed migration lost legacy record", err)
			}
		})
	}
}

func TestUnrepresentableMetadataRetainsLegacy(t *testing.T) {
	for _, title := range []string{"Title\x00", "Title\x01", "Title\ufffe"} {
		dir := t.TempDir()
		n := &Note{ID: strings.Repeat("a", 26), Format: "markdown", Title: title, Source: "Keep this source"}
		path := filepath.Join(dir, "notes", n.ID+".json")
		original := writeLegacy(t, path, n)
		if _, err := Open(dir); err == nil {
			t.Fatal("lossy migration was accepted")
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, original) {
			t.Fatal("lossy migration removed the original", err)
		}
	}
}

func TestFormatChangeMovesFileAndPreservesHistory(t *testing.T) {
	l := mustLibrary(t)
	n := mustNote(t, l, "Formats")
	for _, format := range []string{"xml", "markdown"} {
		oldPath, oldSource := l.notePath(n), n.Source
		n.Format, n.Source = format, "<p>Updated</p>"
		if err := l.SaveWithHistory(n); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Fatal("old extension remains", err)
		}
		history, err := l.History(n.ID)
		if err != nil || history[0].Note.Source != oldSource {
			t.Fatal("format change lost history", err)
		}
		if _, err := Open(l.Dir); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInterruptedFormatChange(t *testing.T) {
	for _, both := range []bool{false, true} {
		for _, format := range []string{"markdown", "xml"} {
			l := mustLibrary(t)
			n, err := l.Create("Format", "body", format)
			if err != nil {
				t.Fatal(err)
			}
			from := l.notePath(n)
			n.Format = "xml"
			if format == "xml" {
				n.Format = "markdown"
			}
			data, _ := marshalNote(n)
			mustWrite(t, from, data)
			if both {
				mustWrite(t, l.notePath(n), data)
			}
			opened, err := Open(l.Dir)
			if err != nil || opened.Get(n.ID).Format != n.Format {
				t.Fatal("format move did not resume", err)
			}
			if _, err := os.Stat(from); !os.IsNotExist(err) {
				t.Fatal("old extension remains", err)
			}
		}
	}
}

func TestInvalidNativeMetadata(t *testing.T) {
	n := &Note{ID: strings.Repeat("a", 26), Format: "markdown", Source: "text"}
	valid, _ := marshalNote(n)
	for _, data := range [][]byte{
		[]byte("plain text without metadata"),
		bytes.TrimSuffix(valid, []byte(metadataEnd)),
		bytes.Replace(valid, []byte("<id>"), []byte("<broken>"), 1),
		bytes.Replace(valid, []byte(n.ID), []byte("../escape"), 1),
		bytes.Replace(valid, []byte("<format>markdown</format>"), []byte("<format>unknown</format>"), 1),
	} {
		var decoded Note
		if err := unmarshalNote(data, &decoded); err == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
	if _, err := marshalNote(&Note{ID: "invalid"}); err == nil {
		t.Fatal("invalid identity encoded")
	}
}

func TestNativeHistoryRecoveryAndPurge(t *testing.T) {
	for _, format := range []string{"markdown", "xml"} {
		t.Run(format, func(t *testing.T) {
			l := mustLibrary(t)
			n, err := l.Create("Lifecycle", "original", format)
			if err != nil {
				t.Fatal(err)
			}
			n.Source = "saved edit"
			if err := l.Save(n); err != nil {
				t.Fatal(err)
			}
			history, err := l.History(n.ID)
			if err != nil || len(history) != 1 || history[0].Note.Source != "original" || filepath.Ext(history[0].Name) != noteExtension(n) {
				t.Fatal("history did not use native format", history, err)
			}
			n.Source = "unsaved edit"
			if err := l.Recover(n); err != nil {
				t.Fatal(err)
			}
			recovery, err := l.Recoveries()
			if err != nil || len(recovery) != 1 || recovery[0].Note.Source != n.Source || filepath.Ext(recovery[0].Name) != noteExtension(n) {
				t.Fatal("recovery did not use native format", recovery, err)
			}
			if err := l.Save(n); err != nil {
				t.Fatal(err)
			}
			if recovery, err := l.Recoveries(); err != nil || len(recovery) != 0 {
				t.Fatal("successful save left its recovery", err)
			}
			if err := l.TrashTree(n, true); err != nil {
				t.Fatal(err)
			}
			l, err = Open(l.Dir)
			if err != nil {
				t.Fatal(err)
			}
			n = l.Get(n.ID)
			if _, err := l.RestoreTree(n); err != nil || n.Source != "unsaved edit" {
				t.Fatal("restore changed source", err)
			}
			if err := l.TrashTree(n, true); err != nil {
				t.Fatal(err)
			}
			if err := l.Purge(n); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{l.notePath(n), filepath.Join(l.Dir, "history", n.ID)} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("purge retained native document", path, err)
				}
			}
		})
	}
}

func TestExternalFormatsRemainPlainSource(t *testing.T) {
	for _, ext := range []string{".md", ".markdown", ".mdx", ".xml"} {
		t.Run(ext, func(t *testing.T) {
			l := mustLibrary(t)
			path := filepath.Join(t.TempDir(), "original"+ext)
			original := "---\ntitle: Front matter\n---\n<Component />\n"
			format := "markdown"
			if ext == ".xml" {
				original, format = "<?xml version=\"1.0\"?>\n<doc/>\n", "xml"
			}
			mustWrite(t, path, []byte(original))
			n, err := l.OpenExternal(path)
			if err != nil || n.Format != format || n.Source != original {
				t.Fatal("external format changed", err)
			}
			n.Source += "\n"
			if err := l.Save(n); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != original+"\n" || bytes.Contains(data, []byte(metadataStart)) {
				t.Fatal("external original acquired library metadata", err)
			}
			if err := l.Reload(n.ID); err != nil || n.Source != string(data) {
				t.Fatal("external source did not reload", err)
			}
		})
	}
}
