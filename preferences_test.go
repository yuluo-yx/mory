package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/library"
)

func TestLanguagePageAndDocumentPreservation(t *testing.T) {
	a := testApp(t)
	a.createNote("# Original\n\nSettings", "Settings")
	note := a.current
	original := note.Source
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Key(ui.Cmd, ui.KeyComma)
	if tt.HasText("Document editor") {
		t.Fatal("settings overlays the document instead of replacing the workspace")
	}
	click(t, tt, "Application language")
	click(t, tt, "简体中文")
	if !tt.HasText("应用语言") || a.lib.Settings.Language != "zh-CN" {
		t.Fatal("language did not switch")
	}
	click(t, tt, "主题")
	click(t, tt, "外观")
	click(t, tt, "深色")
	if a.lib.Settings.Appearance != "Dark" {
		t.Fatal("translated option changed its persisted value")
	}
	click(t, tt, "编辑器")
	click(t, tt, "大纲显示位置")
	click(t, tt, "左侧")
	if a.outlineSide() != "Left" {
		t.Fatal("outline preference not applied")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "settings-zh.png"))
	}
	click(t, tt, "返回文稿")
	if a.current != note || note.Source != original || note.Title != "Settings" {
		t.Fatal("language/navigation changed document data")
	}
	if !tt.HasText("所有修改已保存") {
		t.Fatal("editor status was not translated")
	}
	reopened, err := newApp(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.lib.Settings.Language != "zh-CN" || reopened.outlineSide() != "Left" {
		t.Fatal("preferences did not persist")
	}
	a.setLanguage("en")
	tt.Frame()
	if !tt.HasText("All changes saved") {
		t.Fatal("English did not return")
	}
}

func TestSettingsCategoriesAndOutlineSize(t *testing.T) {
	a := testApp(t)
	a.createNote("# Heading\n\nBody", "Outline settings")
	note, original := a.current, a.current.Source
	tt := ui.NewTester(a.view, 1280, 820)
	before, _ := tt.Find("Heading Heading")
	click(t, tt, "Settings")
	for _, category := range []struct {
		tab, present, absent string
	}{
		{"General", "Application language", "Writing style"},
		{"Themes", "Writing style", "Outline text size"},
		{"Keyboard shortcuts", "Global quick note shortcut", "Application language"},
		{"Editor", "Outline text size", "Writing style"},
	} {
		click(t, tt, category.tab)
		if !tt.HasText(category.present) || tt.HasText(category.absent) {
			t.Fatalf("settings category %s contains the wrong controls", category.tab)
		}
	}
	previewBefore, _ := tt.Find("Outline font preview")
	click(t, tt, "Outline text size")
	click(t, tt, "22")
	previewAfter, _ := tt.Find("Outline font preview")
	if a.outlineFontSize() != 22 || previewAfter.H <= previewBefore.H {
		t.Fatal("outline size or live preview did not change")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "settings-editor.png"))
	}
	click(t, tt, "Back from settings")
	after, _ := tt.Find("Heading Heading")
	if after.H <= before.H || a.current != note || note.Source != original || a.lib.Settings.FontSize != 17 {
		t.Fatal("outline size did not apply independently of document text")
	}
	reopened, err := newApp(a.lib.Dir)
	if err != nil || reopened.outlineFontSize() != 22 {
		t.Fatalf("outline size did not persist: %v", err)
	}
	click(t, tt, "Settings")
	click(t, tt, "Use default size")
	if a.outlineFontSize() != defaultOutlineFontSize {
		t.Fatal("outline size did not reset")
	}
}

func TestOutlineSizeAcrossFormatsAndWindows(t *testing.T) {
	for _, format := range []string{"markdown", "xml"} {
		t.Run(format, func(t *testing.T) {
			a := testApp(t)
			source := "# Heading\n\n## Child\n\nBody"
			if format == "xml" {
				source = "<h1>Heading</h1><h2>Child</h2><p>Body</p>"
			}
			n, err := a.lib.Create("Outline", source, format)
			if err != nil {
				t.Fatal(err)
			}
			a.openNote(n)
			tt := ui.NewTester(a.view, 1280, 820)
			for _, size := range []float32{12, 14, 22} {
				a.setOutlineFontSize(size)
				tt.Frame()
				parent, ok := tt.Find("Heading Heading")
				child, childOK := tt.Find("Heading Child")
				if !ok || !childOK || parent.H < size || child.Y < parent.Y+parent.H || child.X+child.W > 1280 {
					t.Fatalf("outline entries overlap or overflow at size %v: parent=%+v child=%+v", size, parent, child)
				}
			}
			b, err := newApp(a.lib.Dir)
			if err != nil {
				t.Fatal(err)
			}
			previous := appWindows
			appWindows = []*app{a, b}
			t.Cleanup(func() { appWindows = previous })
			a.setOutlineFontSize(18)
			if b.outlineFontSize() != 18 {
				t.Fatal("sibling window did not receive outline size")
			}
		})
	}
}

func TestOutlineSizeDefaultsAndFailedSave(t *testing.T) {
	a := testApp(t)
	for _, value := range []float32{0, -1, 11, 23} {
		a.lib.Settings.OutlineFontSize = value
		if a.outlineFontSize() != defaultOutlineFontSize {
			t.Fatalf("invalid outline size %v did not use the default", value)
		}
	}
	a.setOutlineFontSize(100)
	if a.outlineFontSize() != 22 {
		t.Fatal("upper limit not enforced")
	}
	a.setOutlineFontSize(1)
	if a.outlineFontSize() != 12 {
		t.Fatal("lower limit not enforced")
	}
	other, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Settings.OutlineFontSize = 20
	if err := other.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	a.setOutlineFontSize(16)
	if a.problem == "" || a.outlineFontSize() != 12 {
		t.Fatal("failed settings write did not preserve the previous size")
	}
}

func TestOutlineMovesAndSettingsFit(t *testing.T) {
	a := testApp(t)
	a.createNote("# Heading one\n\nBody", "Outline")
	tt := ui.NewTester(a.view, 1280, 820)
	before, _ := tt.Find("ON THIS PAGE")
	click(t, tt, "Outline on Left")
	after, _ := tt.Find("ON THIS PAGE")
	if after.X >= before.X {
		t.Fatal("outline did not move to the left")
	}
	click(t, tt, "Outline on Right")
	again, _ := tt.Find("ON THIS PAGE")
	if again.X <= after.X {
		t.Fatal("outline did not return to the right")
	}
	// Hiding either dock releases its space and the toolbar restores that dock.
	a.current.Width = "Full"
	for _, side := range []string{"Left", "Right"} {
		click(t, tt, "Outline on "+side)
		shown, _ := tt.Find("Document title")
		source := a.current.Source
		click(t, tt, "Hide outline")
		tt.Frame()
		hidden, _ := tt.Find("Document title")
		if _, visible := tt.Find("ON THIS PAGE"); visible || a.outline || hidden.W <= shown.W {
			t.Fatal("hiding the outline did not release the sidebar space")
		}
		click(t, tt, "Toggle outline")
		if _, visible := tt.Find("ON THIS PAGE"); !visible || !a.outline || a.outlineSide() != side || a.current.Source != source {
			t.Fatal("restoring the outline changed its position or document contents")
		}
	}
	for _, size := range [][2]int{{1000, 620}, {1280, 820}, {1600, 1000}} {
		tt.SetSize(size[0], size[1])
		a.settingsOpen = true
		a.settingsTab = "Keyboard shortcuts"
		tt.Frame()
		for _, label := range []string{"Save shortcuts", "Global quick note shortcut", "Back from settings"} {
			r, ok := tt.Find(label)
			if !ok || r.Y < 0 || r.Y+r.H > float32(size[1]) {
				t.Fatalf("%s outside %v: %+v", label, size, r)
			}
		}
	}
}

func TestShortcutDraftValidationAndPersistence(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	a.helpOpen = true
	tt.Frame()
	if _, ok := tt.Access("Enable global quick note"); !ok {
		t.Fatal("global quick note does not expose an explicit enable toggle")
	}
	if field, ok := tt.Access("Global quick note shortcut"); !ok || !field.Disabled {
		t.Fatalf("global quick note field should be clearly disabled until enabled: %+v", field)
	}
	click(t, tt, "Enable global quick note")
	if a.quickDraft != "Ctrl+Alt+Space" {
		t.Fatalf("enabling global quick note did not provide a starter shortcut: %q", a.quickDraft)
	}
	click(t, tt, "Shortcut New note")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Opt+Shift+J")
	if a.keyFor("New note", "Cmd+N") != "Cmd+N" {
		t.Fatal("draft applied before save")
	}
	a.quickDraft = "Alt+Shift+J"
	click(t, tt, "Save shortcuts")
	if !strings.Contains(a.problem, "conflict") {
		t.Fatal("local/global collision accepted")
	}
	a.problem = ""
	a.quickDraft = ""
	tt.Frame()
	click(t, tt, "Save shortcuts")
	if a.lib.Settings.Keys["New note"] != "Alt+Shift+J" {
		t.Fatal("modifier aliases not normalized")
	}
	before := len(a.lib.Notes)
	tt.Key(ui.Cmd, ui.KeyN)
	if len(a.lib.Notes) != before {
		t.Fatal("old shortcut remained active")
	}
	tt.Key(ui.Alt|ui.Shift, ui.KeyJ)
	if len(a.lib.Notes) != before+1 {
		t.Fatal("new shortcut did not activate")
	}
	a.helpOpen = true
	tt.Frame()
	a.shortcutDraft["New note"] = ""
	click(t, tt, "Save shortcuts")
	tt.Key(ui.Alt|ui.Shift, ui.KeyJ)
	if len(a.lib.Notes) != before+1 {
		t.Fatal("disabled shortcut still active")
	}
	reopened, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := reopened.Settings.Keys["New note"]; !ok || value != "" || reopened.Settings.QuickNote != "" {
		t.Fatal("disabled bindings not saved")
	}
	for _, key := range []string{"Bad+K", "Cmd+NoSuchKey", "J", "Shift+A"} {
		if _, _, err := a.validateShortcuts(map[string]string{"New note": key}, ""); err == nil {
			t.Fatalf("accepted invalid key %s", key)
		}
	}
	for _, key := range []string{"Cmd+F12", "Ctrl+PageDown", "Opt+Space", "Command+Return", "Cmd+-", "Cmd+Esc"} {
		if _, err := canonicalKey(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGlobalHotkeyReplacementAndDisable(t *testing.T) {
	a := testApp(t)
	oldRegister, oldUnregister, oldKey := registerGlobal, unregisterGlobal, quickKey
	t.Cleanup(func() { registerGlobal, unregisterGlobal, quickKey = oldRegister, oldUnregister, oldKey })
	quickKey = "Alt+Space"
	var events []string
	registerGlobal = func(key string, fn func()) error {
		events = append(events, "register "+key)
		if key == "Cmd+Space" {
			return errors.New("occupied")
		}
		return nil
	}
	unregisterGlobal = func(key string) { events = append(events, "unregister "+key) }
	if err := a.applyQuickKey("Cmd+Space"); err == nil || quickKey != "Alt+Space" {
		t.Fatal("failed replacement released old hotkey")
	}
	if err := a.applyQuickKey("Ctrl+Opt+Space"); err != nil {
		t.Fatal(err)
	}
	if quickKey != "Ctrl+Alt+Space" || events[len(events)-1] != "unregister Alt+Space" {
		t.Fatal(events)
	}
	if err := a.applyQuickKey(""); err != nil || quickKey != "" {
		t.Fatal("empty did not unregister")
	}
	count := len(events)
	a.registerQuickNote()
	if len(events) != count {
		t.Fatal("empty preference fell back to Alt+Space")
	}
	if err := a.applyQuickKey("Bogus+K"); err == nil {
		t.Fatal("invalid global binding")
	}
}

func TestMenuMappingsAndWindowPreferences(t *testing.T) {
	a := testApp(t)
	b, err := newApp(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	previous := appWindows
	appWindows = []*app{a, b}
	t.Cleanup(func() { appWindows = previous })
	a.setLanguage("zh-CN")
	if b.lib.Settings.Language != "zh-CN" {
		t.Fatal("other window did not switch")
	}
	b.setOutlineSide("Left")
	if a.outlineSide() != "Left" || b.problem != "" {
		t.Fatal("sibling settings version is stale")
	}
	a.lib.Settings.Keys["Formatting toolbar"] = "Ctrl+Alt+T"
	a.lib.Settings.Keys["Outline"] = "Ctrl+Alt+O"
	a.lib.Settings.Keys["Cut"] = ""
	a.lib.Settings.Keys["Copy"] = "Cmd+Shift+C"
	menu := a.buildMenu(&mygo.MenuItem{Label: "Updates"})
	found := map[string]*mygo.MenuItem{}
	var walk func([]*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, item := range items {
			found[item.Label] = item
			if item.Role == mygo.RoleCut && item.Accelerator != "" {
				t.Fatal("disabled role default was restored")
			}
			if item.Role == mygo.RoleCopy && item.Accelerator != "Cmd+Shift+C" {
				t.Fatal("copy remapping ignored")
			}
			walk(item.Submenu)
		}
	}
	walk(menu.Items())
	for name, key := range map[string]string{"Formatting toolbar": "Ctrl+Alt+T", "Outline": "Ctrl+Alt+O"} {
		item := found[a.tr(name)]
		if item == nil || item.Accelerator != key {
			t.Fatalf("menu/action mismatch %s", name)
		}
	}
	if found["文件"] == nil {
		t.Fatal("native menus not translated")
	}
	if a.shortcutHint("Missing") != "" || !strings.Contains(a.shortcutHints("Copy"), "Cmd+Shift+C") {
		t.Fatal("shortcut hints stale")
	}
}

func TestFormattingRebindAndSettingsConflict(t *testing.T) {
	a := testApp(t)
	a.createNote("Text", "Formatting")
	tt := ui.NewTester(a.view, 1280, 820)
	a.lib.Settings.Keys["Bold"] = "Cmd+Alt+B"
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyB)
	if a.doc.Formatting().Bold == richtext.FormatOn {
		t.Fatal("old formatting binding was not disabled")
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyB)
	if a.doc.Formatting().Bold != richtext.FormatOn {
		t.Fatal("custom formatting binding was not applied")
	}
	a.lib.Settings.Keys["Bold"] = ""
	tt.Frame()
	tt.Key(ui.Cmd|ui.Alt, ui.KeyB)
	if a.doc.Formatting().Bold != richtext.FormatOn {
		t.Fatal("disabled formatting shortcut still active")
	}
	other, err := library.Open(a.lib.Dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Settings.Language = "zh-CN"
	if err := other.SaveSettings(); err != nil {
		t.Fatal(err)
	}
	a.setLanguage("en")
	if a.problem == "" || a.lib.Settings.Language != "" {
		t.Fatal("failed preference write did not roll back")
	}
	a.setOutlineSide("Left")
	if a.outlineSide() != "Right" {
		t.Fatal("failed outline write did not roll back")
	}
	a.shortcutDraft = map[string]string{"New note": "Cmd+Alt+J"}
	if a.saveShortcuts() || a.lib.Settings.Keys["New note"] != "" {
		t.Fatal("failed shortcut write was applied")
	}
	if err := a.lib.ReloadSettings(); err != nil {
		t.Fatal(err)
	}
	if a.lib.Settings.Language != "zh-CN" {
		t.Fatal("settings reload did not resolve conflict")
	}
}

func TestLocalizedChromeAndMessages(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Language = "zh-CN"
	for _, action := range a.actions() {
		if a.tr(action.Name) == action.Name {
			t.Errorf("missing action translation: %s", action.Name)
		}
	}
	for _, message := range []string{"Exported document.md", "Editing original file · /notes/test.md", "Reminder: Meeting", "Imported 3 documents", "Imported 3 documents and their metadata", "All changes saved"} {
		if a.statusText(message) == message {
			t.Errorf("untranslated status: %s", message)
		}
	}
	for _, message := range []string{"invalid shortcut: Foo", "Shortcut conflict: Copy and Cut", "unknown modifier: Foo", "global shortcut Alt+Space unavailable", "Enter a positive number for width", "Give your note a name.", "disk write failed"} {
		if a.localizedProblem(message) == message {
			t.Errorf("untranslated problem: %s", message)
		}
	}
	when := time.Date(2026, 11, 9, 12, 34, 0, 0, time.UTC)
	if a.dateText(when, true) != "2026-11-09 12:34" {
		t.Fatal("Chinese date")
	}
	a.lib.Settings.Language = "en"
	if a.localizedProblem("invalid shortcut: Foo") != "invalid shortcut: Foo" || a.dateText(when, false) != "Nov 9, 2026" {
		t.Fatal("English fallback")
	}
	if a.tr("An external plugin message") != "An external plugin message" {
		t.Fatal("unknown content changed")
	}
}

func TestHistorySettingsControlsAndRollback(t *testing.T) {
	a := testApp(t)
	a.createNote("Unsaved work", "History settings")
	tt := ui.NewTester(func(c *ui.Context) { ui.Column(c).Fill().Children(func() { a.historySettings(c) }) }, 720, 640)
	for _, control := range []struct{ label, value string }{
		{"History snapshot interval", "10"},
		{"History retention period", "7"},
		{"History version limit", "20"},
		{"History storage limit", "16"},
	} {
		click(t, tt, control.label)
		click(t, tt, control.value)
	}
	want := library.HistorySettings{IntervalMinutes: 10, RetentionDays: 7, MaxVersions: 20, MaxMiB: 16}
	if a.lib.HistoryPolicy() != want {
		t.Fatal("history controls did not update the policy")
	}
	click(t, tt, "Keep version history")
	want.Disabled = true
	b, err := newApp(a.lib.Dir)
	if err != nil || b.lib.HistoryPolicy() != want || a.current.Source != "Unsaved work" {
		t.Fatal("preferences were lost or modified the document", err)
	}
	previousWindows := appWindows
	appWindows = []*app{a, b}
	t.Cleanup(func() { appWindows = previousWindows })
	click(t, tt, "Reset history settings")
	if b.lib.HistoryPolicy() != library.DefaultHistorySettings() {
		t.Fatal("default history settings did not propagate to the other window")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		a.lib.Settings.Language = "zh-CN"
		tt.Frame()
		saveImage(t, tt, filepath.Join(dir, "history-settings.png"))
	}
	// A conflicting settings file must leave the previous policy active.
	if err := os.WriteFile(filepath.Join(a.lib.Dir, "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a.setHistorySettings(want)
	if a.lib.HistoryPolicy() != library.DefaultHistorySettings() || a.problem == "" {
		t.Fatal("failed settings write did not roll back")
	}
}

func TestStylesheetEditorDraftAndUndo(t *testing.T) {
	a := testApp(t)
	a.createNote("# Original document\n\nKeep this text.", "Stylesheet editing")
	original := a.current.Source
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Settings")
	click(t, tt, "Themes")
	click(t, tt, "Edit CSS")
	initial := a.cssText
	if a.cssEditor == nil || a.cssEditor.state.Buffer.Language().Name != "CSS" {
		t.Fatal("stylesheet editor has no CSS highlighter")
	}
	const draft = "/* Syntax highlighting */\n:root { --accent: #007aff; }\n.page { color: var(--accent); font-size: 18px; }\nh1 { font-size: 2em; }\n"
	click(t, tt, "Theme CSS")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(draft)
	tt.Frame()
	if a.cssText != draft {
		t.Fatal("stylesheet validation draft did not follow editor input")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if a.cssText != initial || a.current.Source != original {
		t.Fatal("stylesheet undo changed the document or failed to restore the CSS")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	tt.Frame()
	if a.cssText != draft {
		t.Fatal("stylesheet redo did not restore the draft")
	}
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		for _, appearance := range []string{"Light", "Dark"} {
			a.lib.Settings.Appearance = appearance
			tt.Frame()
			saveImage(t, tt, filepath.Join(dir, "stylesheet-"+appearance+".png"))
		}
	}
	a.cssName = "Syntax"
	click(t, tt, "Apply stylesheet")
	saved, err := os.ReadFile(filepath.Join(a.lib.Dir, "themes", "syntax.css"))
	if err != nil || string(saved) != draft || a.lib.Settings.Theme != "Syntax" || a.cssEditor != nil {
		t.Fatalf("applied stylesheet does not match the editor: %v", err)
	}
	click(t, tt, "Settings")
	click(t, tt, "Duplicate theme")
	if a.cssText != draft || a.cssEditor == nil {
		t.Fatal("new stylesheet session did not load the current theme")
	}
	click(t, tt, "Theme CSS")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(".page { color: red; }")
	click(t, tt, "Cancel")
	if a.cssEditor != nil || a.current.Source != original {
		t.Fatal("cancel retained the editor or changed the document")
	}
	if _, err := os.Stat(filepath.Join(a.lib.Dir, "themes", "syntax-copy.css")); !os.IsNotExist(err) {
		t.Fatal("cancel saved the draft")
	}
}
