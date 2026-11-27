package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDocumentPathMenus(t *testing.T) {
	for _, kind := range []string{"markdown", "xml", "external"} {
		for _, more := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/context", true: "/more"}[more], func(t *testing.T) {
				a := testApp(t)
				a.createNote("Saved body", "Path target")
				n := a.current
				if kind == "xml" {
					n.Format, n.Source = "xml", "<p>Saved body</p>"
					if err := a.lib.Save(n); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "external" {
					path := filepath.Join(t.TempDir(), "External notes 世界.md")
					if err := os.WriteFile(path, []byte("Original body"), 0o644); err != nil {
						t.Fatal(err)
					}
					var err error
					n, err = a.lib.OpenExternal(path)
					if err != nil {
						t.Fatal(err)
					}
				}
				want, err := a.lib.DocumentPath(n.ID)
				if err != nil {
					t.Fatal(err)
				}
				saved, err := os.ReadFile(want)
				if err != nil {
					t.Fatal(err)
				}
				// Acting on another row must not navigate or save the current draft.
				a.createNote("Other body", "Current draft")
				current := a.current
				a.changeMode("Source")
				tt := ui.NewTester(a.view, 1280, 820)
				click(t, tt, "Source editor")
				tt.Key(ui.Cmd, ui.KeyEnd)
				tt.Type(" pending")
				var shown string
				original := showItemInFolder
				showItemInFolder = func(path string) { shown = path }
				t.Cleanup(func() { showItemInFolder = original })
				for _, action := range []string{"Copy absolute path", "Show in Finder"} {
					if more {
						row, ok := tt.Find("Open " + n.Title)
						if !ok {
							t.Fatal("missing target row")
						}
						tt.Move(row.X+50, row.Y+row.H/2)
						click(t, tt, "Actions "+n.Title)
					} else if err := tt.RightClick("Open " + n.Title); err != nil {
						t.Fatal(err)
					}
					if err := tt.ChooseMenuItem(action); err != nil {
						t.Fatal(err)
					}
				}
				if tt.Clipboard() != want || shown != want || a.problem != "" {
					t.Fatalf("clipboard=%q Finder=%q want=%q problem=%q", tt.Clipboard(), shown, want, a.problem)
				}
				if a.current != current || a.code.Buffer.String() != "Other body pending" {
					t.Fatal("path actions changed the current document or draft", a.code.Buffer.String())
				}
				if got, err := os.ReadFile(want); err != nil || string(got) != string(saved) {
					t.Fatal("path actions changed the file", err)
				}
			})
		}
	}
}

func TestFinderMissingDocumentAndInvalidTarget(t *testing.T) {
	a := testApp(t)
	a.createNote("Body", "Missing target")
	n := a.current
	want, err := a.lib.DocumentPath(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}
	var calls int
	original := showItemInFolder
	showItemInFolder = func(string) { calls++ }
	t.Cleanup(func() { showItemInFolder = original })
	tt := ui.NewTester(a.view, 1280, 820)
	a.copyDocumentPath(n)
	if tt.Clipboard() != want {
		t.Fatal("missing path cannot be copied", tt.Clipboard())
	}
	a.showDocumentInFinder(n)
	if a.problem == "" || calls != 0 {
		t.Fatal("missing document opened Finder")
	}
	a.problem = ""
	if err := os.Mkdir(want, 0o755); err != nil {
		t.Fatal(err)
	}
	a.showDocumentInFinder(n)
	if a.problem == "" || calls != 0 {
		t.Fatal("directory accepted as a document")
	}
	a.lib.Notes = nil
	a.problem = ""
	a.copyDocumentPath(n)
	if a.problem == "" || tt.Clipboard() != want {
		t.Fatal("stale menu target changed clipboard")
	}
}

func TestFolderFinderMenuAndErrors(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Folders = []string{"Empty", "Nested/Group"}
	var opened []string
	original := openFolderPath
	openFolderPath = func(path string) error { opened = append(opened, path); return nil }
	t.Cleanup(func() { openFolderPath = original })
	tt := ui.NewTester(a.view, 1280, 820)
	for _, folder := range a.lib.Settings.Folders {
		if err := tt.RightClick("Folder " + folder); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem("Open storage folder in Finder"); err != nil {
			t.Fatal(err)
		}
	}
	want, _ := filepath.Abs(filepath.Join(a.lib.Dir, "notes"))
	if len(opened) != 2 || opened[0] != want || opened[1] != want || a.problem != "" {
		t.Fatal("logical folders did not open real storage", opened, a.problem)
	}
	openFolderPath = func(string) error { return errors.New("Finder unavailable") }
	a.openStorageInFinder()
	if a.problem != "Finder unavailable" {
		t.Fatal("shell error was lost", a.problem)
	}
	if err := checkFinderPath(filepath.Join(a.lib.Dir, "missing"), true); err == nil {
		t.Fatal("missing directory accepted")
	}
	file := filepath.Join(t.TempDir(), "file.md")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkFinderPath(file, true); err == nil {
		t.Fatal("regular file accepted as a storage directory")
	}
}
