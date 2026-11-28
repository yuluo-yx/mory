package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/doctheme"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestLegacyThemesResolveWithoutRewritingFiles(t *testing.T) {
	for _, name := range []string{"Craft", "Feishu", "Notion", "Obsidian", "Docusaurus", "Missing", ""} {
		for _, custom := range []bool{false, true} {
			if name == "" && custom {
				continue
			}
			t.Run(name+"/custom="+map[bool]string{true: "yes", false: "no"}[custom], func(t *testing.T) {
				a := testApp(t)
				a.createNote("Exact source\n\nKeep it.", "Original")
				a.lib.Settings.Theme = name
				if err := a.lib.SaveSettings(); err != nil {
					t.Fatal(err)
				}
				settingsPath := filepath.Join(a.lib.Dir, "settings.json")
				before, _ := os.ReadFile(settingsPath)
				cssPath := filepath.Join(a.lib.Dir, "themes", name+".css")
				const css = ".page{color:#123456;font-size:19px}"
				if custom {
					// Theme filenames are normalized by the existing CSS editor.
					cssPath = filepath.Join(a.lib.Dir, "themes", strings.ToLower(name)+".css")
					if err := os.WriteFile(cssPath, []byte(css), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				reopened, err := newApp(a.lib.Dir)
				if err != nil {
					t.Fatal(err)
				}
				reopened.reloadTheme()
				wantName, wantCSS := doctheme.DefaultName, doctheme.Builtin(doctheme.DefaultName)
				if custom {
					wantName, wantCSS = name, css
				}
				if reopened.lib.Settings.Theme != wantName || reopened.docTheme.Source != wantCSS || reopened.lib.Get(a.current.ID).Source != a.current.Source {
					t.Fatal("migration lost the selection, custom CSS or document")
				}
				after, _ := os.ReadFile(settingsPath)
				if string(after) != string(before) {
					t.Fatal("opening the library rewrote preferences")
				}
				if !reopened.savePreferences() {
					t.Fatal(reopened.problem)
				}
				stored, err := library.Open(a.lib.Dir)
				if err != nil || stored.Settings.Theme != wantName {
					t.Fatal("resolved selection did not persist", err)
				}
			})
		}
	}
}

func TestSingleThemeAndCustomCSSFlow(t *testing.T) {
	a := testApp(t)
	a.createNote("# Original\n\nUnchanged", "Theme test")
	original := a.current.Source
	a.settingsOpen, a.settingsTab = true, "Themes"
	tt := ui.NewTester(a.view, 1280, 820)
	if info, ok := tt.Access("Writing style"); !ok || info.Value != "yuluo" || info.Role != ui.RoleText {
		t.Fatal("single default should not show an empty choice", info)
	}
	if got := doctheme.Names(filepath.Join(a.lib.Dir, "themes")); !reflect.DeepEqual(got, []string{"yuluo"}) {
		t.Fatal(got)
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		a.lib.Settings.Language = "zh-CN"
		tt.Frame()
		saveImage(t, tt, filepath.Join(dir, "theme-default-settings.png"))
		a.lib.Settings.Language = "en"
		tt.Frame()
	}
	click(t, tt, "Edit CSS")
	if a.cssName != "yuluo" || a.cssText != doctheme.Builtin("yuluo") {
		t.Fatal("CSS editor did not open the default style")
	}
	a.cssName, a.cssText = "Personal", ".page{color:#123456;font-size:19px}"
	tt.Frame()
	click(t, tt, "Apply stylesheet")
	click(t, tt, "Settings")
	click(t, tt, "Writing style")
	click(t, tt, "yuluo")
	if a.docTheme.Source != doctheme.Builtin("yuluo") {
		t.Fatal("built-in style did not return")
	}
	click(t, tt, "Writing style")
	click(t, tt, "personal")
	if a.docTheme.Value(".page", "color") != "#123456" {
		t.Fatal("custom style was not selectable")
	}
	click(t, tt, "Reset theme")
	if a.lib.Settings.Theme != "yuluo" || a.current.Source != original {
		t.Fatal("reset changed the document or failed to select yuluo")
	}
	if css, err := os.ReadFile(filepath.Join(a.lib.Dir, "themes", "personal.css")); err != nil || string(css) != ".page{color:#123456;font-size:19px}" {
		t.Fatal("reset removed custom CSS", err)
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		a.lib.Settings.Language = "zh-CN"
		tt.Frame()
		saveImage(t, tt, filepath.Join(dir, "theme-custom-settings.png"))
	}
}

func TestResetDefaultOverrideKeepsCopiesAndRollsBackOnConflict(t *testing.T) {
	a := testApp(t)
	dir := filepath.Join(a.lib.Dir, "themes")
	path := filepath.Join(dir, "yuluo.css")
	const custom = ".page{font-size:20px}"
	for name, contents := range map[string]string{"yuluo.css": custom, "yuluo-custom-1.css": "/* Previous copy */"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a.resetTheme()
	copy, err := os.ReadFile(filepath.Join(dir, "yuluo-custom-2.css"))
	if err != nil || string(copy) != custom || a.docTheme.Source != doctheme.Builtin("yuluo") {
		t.Fatal("reset did not preserve the default override", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("override still hides the built-in theme")
	}
	if err := os.WriteFile(path, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.lib.Dir, "settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.resetTheme()
	copy, err = os.ReadFile(path)
	if err != nil || string(copy) != custom || a.problem == "" {
		t.Fatal("failed settings save lost the CSS override", err)
	}
}
