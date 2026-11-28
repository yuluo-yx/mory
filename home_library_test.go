package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func homeFixture(t *testing.T) *app {
	t.Helper()
	a := testApp(t)
	a.lib.Settings.Folders = []string{"Writing and formatting", "Examples", "Examples/Meetings", "Empty"}
	for _, note := range []struct{ title, folder, source string }{
		{"Welcome to Mory", "", "# A place for your ideas\n\nWrite in Markdown, keep your files locally, and make room for a new thought.\n\n## Getting started\n\n- Create your first note\n- Organize documents into folders\n- Make the theme yours\n\n> Good ideas start with a little space.\n\n```go\nfunc main() {\n    write(\"Hello world\")\n}\n```"},
		{"Tables and columns", "Writing and formatting", "# Tables and columns\n\nStructure information with a table.\n\n| Item | Status |\n| --- | --- |\n| First draft | Ready |\n| Review | In progress |\n\n## Next steps\n\n- Share your notes\n- Collect feedback"},
		{"Weekly meeting", "Examples/Meetings", "# Weekly meeting\n\n**When** Wednesday\n\n## Agenda\n\n- Project updates\n- Open questions\n- Next steps\n\n> Keep decisions in one place."},
		{"Organize and find your notes", "", "# Everything in its place\n\nFolders keep related work together. Search across your library when you need a specific detail.\n\n## Your workspace\n\nUse **favorites** for the documents you return to most.\n\n[Learn more](https://example.com)\n\n## A fresh page\n\nWrite one thought at a time."},
		{"Import, export and back up", "", "# Your files, your choice\n\nImport Markdown and XML files from your Mac.\n\n## Supported formats\n\n| Format | Use |\n| --- | --- |\n| Markdown | Writing |\n| HTML | Sharing |\n| PDF | Printing |\n\nYour library stays local."},
		{"Make it yours", "", "# A style that feels right\n\nThe **yuluo** theme gives every page a clear structure.\n\n## Custom CSS\n\n```css\n.page {\n  font-size: 18px;\n  line-height: 1.6;\n}\n```\n\nEdit or import a stylesheet in settings."},
		{"原生中文写作", "", "# 从一个想法开始\n\n文稿保存在本地，随时打开，继续上一次的思考。\n\n## 清晰的内容层级\n\n- 标题与段落\n- 列表与任务\n- 表格与代码\n\n> 用简单的结构承载完整的想法。\n\n## 下一步\n\n把相关内容整理到文件夹，再开始新的篇章。"},
	} {
		n, err := a.lib.Create(note.title, note.source, "markdown")
		if err != nil {
			t.Fatal(err)
		}
		n.Folder = note.folder
		if err := a.lib.Save(n); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func TestHomeHierarchySearchAndSelection(t *testing.T) {
	a := homeFixture(t)
	rootSource := a.lib.Notes[0].Source
	tt := ui.NewTester(a.view, 1280, 900)
	folders, notes := a.homeContents()
	if len(folders) != 3 || len(notes) != 5 || tt.HasText("Note Weekly meeting") || !tt.HasText("Open folder Examples") {
		t.Fatal("home flattened nested documents", folders, len(notes))
	}
	click(t, tt, "Open folder Examples")
	if a.folder != "Examples" || !tt.HasText("Open folder Examples/Meetings") {
		t.Fatal("folder card did not navigate")
	}
	click(t, tt, "Open folder Examples/Meetings")
	if !tt.HasText("Note Weekly meeting") {
		t.Fatal("nested folder document is missing")
	}
	click(t, tt, "Parent folder")
	if a.folder != "Examples" {
		t.Fatal("parent navigation skipped a level")
	}
	click(t, tt, "Parent folder")
	if a.folder != "" {
		t.Fatal("parent navigation did not return home")
	}
	a.query = "Weekly"
	tt.Frame()
	if !tt.HasText("Note Weekly meeting") || tt.HasText("Open folder Examples") {
		t.Fatal("search did not reach nested documents")
	}
	a.query = ""
	tt.Frame()
	click(t, tt, "Select notes")
	click(t, tt, "Select all")
	if len(a.selectedNotes()) != 5 {
		t.Fatal("batch selection included hidden folder contents")
	}
	click(t, tt, "Open folder Empty")
	if a.selecting || len(a.selectedNotes()) != 0 || !tt.HasText("Create a blank note") {
		t.Fatal("folder navigation kept stale selection")
	}
	if a.lib.Notes[0].Source != rootSource {
		t.Fatal("browsing rewrote source")
	}
}

func TestHomePortraitGeometryAndPreview(t *testing.T) {
	for _, appearance := range []string{"Light", "Dark"} {
		a := homeFixture(t)
		a.lib.Settings.Appearance = appearance
		a.lib.Notes[0].Pinned = true
		tt := ui.NewTester(a.view, 1280, 900)
		for _, width := range []int{1000, 1280, 2560} {
			tt.SetSize(width, 900)
			r, ok := tt.Find("Note Welcome to Mory")
			if !ok || r.W < 180 || r.W > 285 || r.H < r.W || r.H > 380 {
				t.Fatal("paper card lost readable portrait geometry", width, r)
			}
			if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
				saveImage(t, tt, filepath.Join(dir, fmt.Sprintf("home-%s-%d.png", appearance, width)))
			}
		}
		if len(a.homePreviews) == 0 {
			t.Fatal("no document previews were painted")
		}
		click(t, tt, "List view")
		if _, ok := tt.Find("Open folder Examples"); !ok {
			t.Fatal("list view lost folders")
		}
		click(t, tt, "Grid view")
		card, _ := tt.Find("Note Welcome to Mory")
		tt.ClickAt(card.X+card.W/2, card.Y+card.H/2)
		if a.current == nil || a.current.Title != "Welcome to Mory" {
			t.Fatal("preview body did not open document")
		}
	}
}

func TestHomePreviewBoundedAndMalformedSources(t *testing.T) {
	for _, source := range []string{strings.Repeat("中文🌱", 10000), strings.Repeat("line\n", 10000)} {
		preview := previewSource(source)
		if len(preview) > homePreviewBytes || !utf8.ValidString(preview) {
			t.Fatal("preview bound or Unicode broken")
		}
	}
	root := parseHomePreview("<title>Title</title><h1>Heading</h1><p>Keep <b>bold</b></p><table><tr><td>Cell", "xml")
	if !strings.Contains(root.Plain(), "Keep bold") || !strings.Contains(root.Plain(), "Cell") {
		t.Fatal("partial XML lost preceding content")
	}
	root = parseHomePreview("---\nslug: hidden\n---\n\n# Heading\n\nBody", "markdown")
	if len(root.Children) == 0 {
		t.Fatal("Markdown preview missing")
	}
	a := testApp(t)
	n := &library.Note{ID: "sample", Title: "Preview", Source: "<p>Before</p><table><tr><td>One</td><td>Two</td></tr></table><pre><code>code</code></pre><figure><source path='missing.png'/></figure>", Format: "xml"}
	a.lib.Notes = []*library.Note{n}
	tt := ui.NewTester(a.view, 1280, 900)
	before := a.homePreviews[n.ID]
	n.Source = "<p>After</p>"
	tt.Frame()
	if a.homePreviews[n.ID] == before || a.homePreviews[n.ID].root.Plain() != "After" {
		t.Fatal("preview cache retained stale source")
	}
}

func TestHomeLocalImagePreviewAndVisibleWork(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(a.lib.Dir, "assets", "preview.png")
	if err := os.WriteFile(path, iconPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	n := &library.Note{ID: "image", Title: "Local image", Source: "<figure><source path=\"assets/preview.png\"/></figure><p>Caption</p>", Format: "xml"}
	a.lib.Notes = []*library.Note{n}
	image := a.homeImage(n, "assets/preview.png")
	if image == nil || a.homeImage(n, "assets/preview.png") != image {
		t.Fatal("local thumbnail did not load or reuse its cache")
	}
	if a.homeImage(n, "https://example.com/image.png") != nil {
		t.Fatal("home preview unexpectedly loaded a remote image")
	}
	tt := ui.NewTester(a.view, 1280, 900)
	if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
		saveImage(t, tt, filepath.Join(dir, "home-local-image.png"))
	}
	// Large libraries build card controls, but parse only the pages actually
	// painted in the viewport. Repainting reuses the bounded parsed fragments.
	for i := range 250 {
		a.lib.Notes = append(a.lib.Notes, &library.Note{ID: fmt.Sprintf("note-%03d", i), Title: fmt.Sprintf("Document %03d", i), Source: strings.Repeat("## Heading\n\nParagraph text.\n\n", 2000), Format: "markdown"})
	}
	tt.Frame()
	if len(a.homePreviews) > 20 {
		t.Fatal("offscreen pages parsed eagerly", len(a.homePreviews))
	}
	cached := a.homePreviews[n.ID]
	tt.Frame()
	if a.homePreviews[n.ID] != cached {
		t.Fatal("unchanged preview reparsed")
	}
}

func BenchmarkHomeLibraryFrame(b *testing.B) {
	a, err := newApp(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	for i := range 250 {
		a.lib.Notes = append(a.lib.Notes, &library.Note{ID: fmt.Sprintf("note-%03d", i), Title: fmt.Sprintf("Document %03d", i), Source: strings.Repeat("## Heading\n\nParagraph text.\n\n", 2000), Format: "markdown"})
	}
	tt := ui.NewTester(a.view, 1280, 900)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}
