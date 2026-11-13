package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func xmlApp(t *testing.T, source string) (*app, *ui.Tester) {
	t.Helper()
	a := testApp(t)
	n, err := a.lib.Create("Document", source, "xml")
	if err != nil {
		t.Fatal(err)
	}
	a.openNote(n)
	return a, ui.NewTester(a.view, 1440, 1100)
}

func firstBlock(a *app, kind string) *blocks.Node {
	var found *blocks.Node
	a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
		if found == nil && n.Kind == kind {
			found = n
		}
	})
	return found
}

func TestStructuredTypingNavigationAndSearch(t *testing.T) {
	a, tt := xmlApp(t, `<h1>Heading</h1><p>First text</p><p>Second text</p>`)
	p := firstBlock(a, "p")
	click(t, tt, "Block text "+p.ID)
	tt.Key(ui.Cmd, ui.KeyEnd)
	tt.Type(" plus")
	a.capture()
	if !strings.Contains(a.current.Source, "plus") {
		t.Fatal(a.current.Source)
	}
	field := a.blockField(p, a.documentColor)
	field.doc.SetSelection(5, 5)
	field.doc.InsertText("\nNew ")
	a.captureBlocks()
	tt.Frame()
	if len(blocks.Visible(a.blockEditor.doc.Root.Children)) < 4 {
		t.Fatal("Enter did not split block")
	}
	a.findText = "Second"
	a.findNext()
	anchor, caret := a.doc.Selection()
	if caret-anchor != 6 {
		t.Fatal("cross-block find")
	}
	a.replaceText = "Updated"
	a.replaceAll()
	if strings.Contains(a.current.Source, "Second") {
		t.Fatal("replace all")
	}
	a.blockUndo(false)
	if !strings.Contains(a.current.Source, "Second") {
		t.Fatal("replace undo")
	}
	a.blockUndo(true)
	a.findText = "missing"
	a.findNext()
	if a.status != "No matches" {
		t.Fatal(a.status)
	}
	for _, n := range blocks.Visible(a.blockEditor.doc.Root.Children) {
		if n.Kind == "p" {
			a.focusAdjacent(n.ID, 1)
			a.focusAdjacent(n.ID, -1)
			break
		}
	}
	p = firstBlock(a, "p")
	a.mergeBlock(p.ID, 1)
	tt.Frame()
	a.blockUndo(false)
	a.mergeBlock("missing", -1)
	a.focusAdjacent("missing", 1)
	a.changeMode("Source")
	a.code.Buffer.Set(`<h9>New heading</h9><p>Latest source</p>`)
	a.capture()
	a.changeMode("Read")
	tt.Frame()
	if !strings.Contains(a.blockEditor.doc.Source(), "Latest source") {
		t.Fatal("stale read preview")
	}
	a.changeMode("Split")
	tt.Frame()
	a.code.Buffer.Set("<p>")
	a.capture()
	tt.Frame()
	a.code.Buffer.Set(`<p>Valid again</p>`)
	a.capture()
	tt.Frame()
}

func TestBlockActionsAndSelection(t *testing.T) {
	for _, action := range []string{"Insert paragraph below", "Duplicate block", "Fold content", "Properties…", "Delete block"} {
		t.Run(action, func(t *testing.T) {
			a, tt := xmlApp(t, `<p>Alpha</p><p>Beta</p>`)
			if err := tt.RightClick("Block actions p"); err != nil {
				t.Fatal(err)
			}
			if err := tt.ChooseMenuItem(action); err != nil {
				t.Fatal(err)
			}
			if action == "Properties…" {
				click(t, tt, "Cancel")
			} else if a.blockEditor.doc.Version <= 1 {
				t.Fatal("block action did nothing")
			}
		})
	}
	a, tt := xmlApp(t, `<p>Alpha</p><p>Beta</p><p>Gamma</p>`)
	if err := tt.RightClick("Block actions p"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Background", "light-blue"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.current.Source, "light-blue") {
		t.Fatal("background")
	}
	for _, n := range a.blockEditor.doc.Root.Elements()[:2] {
		a.blockEditor.chosen.Add(n.ID)
	}
	tt.Frame()
	click(t, tt, "Copy blocks")
	data := a.services.ReadClipboardData()
	raw, err := data.Read(transfer.Format("application/vnd.mory.docxml"))
	if err != nil || !strings.Contains(string(raw), "Beta") {
		t.Fatal("copy selection", err)
	}
	click(t, tt, "Cut blocks")
	if strings.Contains(a.current.Source, "Beta") {
		t.Fatal("cut did not remove blocks")
	}
	a.pasteStructured()
	tt.Frame()
	if !strings.Contains(a.current.Source, "Beta") {
		t.Fatal("structured paste lost blocks")
	}
	a.blockUndo(false)
	if a.blockEditor.list.Reorder != nil {
		a.blockEditor.list.Reorder([]int{0}, len(a.blockEditor.doc.Root.Elements()))
		tt.Frame()
	}
}

func TestBlockPropertiesCodeMathAndColumns(t *testing.T) {
	for _, source := range []string{`<callout emoji="💡"><p>Note</p></callout>`, `<countdown duration="60"/>`, `<pre lang="go" caption="Example"><code>package main</code></pre>`, `<grid><column><p>Left</p></column><column><p>Right</p></column></grid>`, `<p><time date="2026-11-09"/><button text="Go"/><cite title="Other"/></p>`} {
		t.Run(source[:min(20, len(source))], func(t *testing.T) {
			a, tt := xmlApp(t, source)
			n := a.blockEditor.doc.Root.Elements()[0]
			a.editBlockProperties(n)
			tt.Frame()
			click(t, tt, "Apply properties")
			tt.Frame()
			if a.problem != "" {
				t.Fatal(a.problem)
			}
			if n.Kind == "countdown" {
				click(t, tt, "Start / reset")
				click(t, tt, "Stop")
			}
			if n.Kind == "pre" {
				click(t, tt, "Code language")
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("python")
				click(t, tt, "Wrap")
				click(t, tt, "Copy code")
				f := a.blockEditor.fields[n.ID]
				f.code.Buffer.Set("print(1)")
				a.captureBlocks()
				if !strings.Contains(a.current.Source, "print(1)") {
					t.Fatal("code save")
				}
			}
			if n.Kind == "grid" {
				click(t, tt, "+ Column")
				click(t, tt, "Remove last column")
				click(t, tt, "+ Block")
				tt.Frame()
			}
		})
	}
	a, tt := xmlApp(t, `<latex display="true">\frac{1}{2}</latex>`)
	click(t, tt, "Edit equation")
	click(t, tt, "LaTeX source")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(`\sqrt{x}`)
	click(t, tt, "Apply equation")
	if !strings.Contains(a.current.Source, `\sqrt{x}`) {
		t.Fatal(a.current.Source)
	}
	click(t, tt, "Edit equation")
	a.blockEditor.formulaText = `\frac{`
	tt.Frame()
	click(t, tt, "Cancel")
	a.blockEditor.doc.Root.Children[0].Children = []*blocks.Node{blocks.Text(`\frac{`)}
	tt.Frame()
	a, tt = xmlApp(t, `<p>Inline <latex>x^2</latex> <time date="2026-11-09" time="12:00"/> <button text="Action"/> <doc-link title="Other"/></p>`)
	for _, kind := range []string{"latex", "time", "button", "doc-link"} {
		n := firstBlock(a, kind)
		a.openLink("mory-inline:" + n.ID)
		tt.Frame()
		if kind == "latex" {
			click(t, tt, "Cancel")
		} else {
			click(t, tt, "Apply properties")
		}
	}
	for _, kind := range []string{"latex", "time", "cite", "button", "unknown"} {
		n := blocks.New(kind)
		if kind == "latex" {
			n.Children = []*blocks.Node{blocks.Text(`\frac{`)}
		}
		if a.inlineObject(n, ui.Hex("#333333")) == nil {
			t.Fatal(kind)
		}
	}
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestImagesCoverInlineAndExport(t *testing.T) {
	a, tt := xmlApp(t, `<title>Picture</title><p>Before image</p>`)
	data := pngBytes(t)
	a.insertAsset(data, "Picture.png")
	tt.Frame()
	n := firstBlock(a, "img")
	if n == nil {
		t.Fatal("image insertion")
	}
	path := n.Attr("href")
	a.current.Cover = path
	a.current.Icon = "🌱"
	a.current.ShowCreated = true
	a.syncMetadata()
	tt.Frame()
	if !strings.Contains(a.current.Source, `cover="`) {
		t.Fatal("cover metadata")
	}
	image := a.blockImage(path)
	if image == nil {
		t.Fatal("image decode")
	}
	cropped := a.croppedImage(path, ".1 .1 .9 .9", image)
	if w, _ := cropped.Size(); w >= 40 {
		t.Fatal("crop did not apply")
	}
	if a.croppedImage(path, ".1 .1 .9 .9", image) != cropped {
		t.Fatal("crop cache")
	}
	if a.croppedImage(path, "invalid", image) != image {
		t.Fatal("bad crop")
	}
	n.Set("image.crop", ".1 .1 .9 .9")
	n.Set("align", "center")
	tt.Frame()
	n.Set("align", "right")
	tt.Frame()
	click(t, tt, "Image size, crop & caption")
	a.blockEditor.props["width"] = "-1"
	click(t, tt, "Apply properties")
	if a.problem == "" {
		t.Fatal("invalid width accepted")
	}
	a.problem = ""
	a.blockEditor.props["width"] = "240"
	a.blockEditor.props["height"] = "180"
	tt.Frame()
	click(t, tt, "Apply properties")
	page, err := renderExport(a.current, a.lib.Dir)
	if err != nil || !strings.Contains(page, "data:image/png") {
		t.Fatal(err)
	}
	for _, ext := range []string{"pdf", "mp4"} {
		a.insertAsset([]byte("attachment"), "File."+ext)
		tt.Frame()
	}
	if !strings.Contains(a.current.Source, "video") {
		t.Fatal("video insertion")
	}
	a.insertAsset(make([]byte, 33<<20), "large.bin")
	if a.problem == "" {
		t.Fatal("oversize asset")
	}
	a.problem = ""
	for _, n := range []*blocks.Node{blocks.New("bookmark"), blocks.New("figure", blocks.New("source")), blocks.New("unknown", blocks.Text("Preserved"))} {
		a.insertNodes(n)
		tt.Frame()
	}
	a.openLink("#anchor")
	a.openLink("mory:" + a.current.ID)
	if a.loadAssetBitmap("missing") == nil {
	} else {
		t.Fatal("missing bitmap")
	}
	pathFile := filepath.Join(a.lib.Dir, "assets", "plain.txt")
	if err := os.WriteFile(pathFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.attachFile(pathFile)
	a.attachFile(filepath.Join(a.lib.Dir, "absent"))
}

func TestInsertionPositionAndReminders(t *testing.T) {
	a := testApp(t)
	a.createNote("Alpha beta gamma", "Position")
	a.doc.SetSelection(6, 10)
	a.insertNodes(blocks.NewTable(2, 2))
	if !strings.Contains(a.current.Source, "Alpha") || !strings.Contains(a.current.Source, "gamma") {
		t.Fatal("insert lost surrounding text")
	}
	tt := ui.NewTester(a.view, 1280, 900)
	a.blockEditor.active = firstBlock(a, "p").ID
	a.setHeading(9)
	tt.Frame()
	if !strings.Contains(a.current.Source, "h9") {
		t.Fatal("level nine heading")
	}
	a.setHeading(0)
	now := time.Now()
	source := fmt.Sprintf(`<countdown start-time="%d" duration="1"/><time date="%s" time="%s"/><time time="%d"/><countdown notify="false"/><countdown start-time="bad"/><p>Text</p>`, now.Add(-2*time.Second).UnixMilli(), now.Format("2006-01-02"), now.Add(-time.Minute).Format("15:04"), now.Add(-time.Second).UnixMilli())
	a.current.Source = source
	a.current.Format = "xml"
	a.checkReminders(now)
	if len(a.notified) != 3 {
		t.Fatal(a.notified)
	}
	a.checkReminders(now)
	if len(a.notified) != 3 {
		t.Fatal("duplicate reminder")
	}
	if len(documentReminders("<bad>", "xml")) != 0 {
		t.Fatal("malformed reminders")
	}
}

func TestFormattingCommandRegistry(t *testing.T) {
	for _, name := range []string{"Bold", "Italic", "Underline", "Strikethrough", "Inline code", "Link", "Bullets", "Numbers", "Tasks", "Quote", "Code block", "Indent", "Outdent", "Align left", "Align center", "Align right", "Heading 0", "Heading 1", "Heading 6", "Heading 7", "Heading 8", "Heading 9", "Undo", "Redo", "Find", "Replace", "Write mode", "Read mode", "Source mode", "Split mode", "Outline", "Export", "Focus mode", "Settings", "Shortcuts", "Insert block", "Save", "Library"} {
		t.Run(name, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Editable text", "Commands")
			a.doc.SetSelection(0, a.doc.Text().Len())
			a.doc.InsertText("Changed text")
			a.doc.SetSelection(0, a.doc.Text().Len())
			a.capture()
			found := false
			for _, action := range a.actions() {
				if action.Name == name {
					found = true
					action.Run()
					break
				}
			}
			if !found {
				t.Fatal("missing command")
			}
			if a.problem != "" {
				t.Fatal(a.problem)
			}
			if isTextAction(name) && name != "Undo" && name != "Redo" && name != "Link" && a.current != nil && a.current.Source == "" {
				t.Fatal("formatting erased text")
			}
		})
	}
}

func TestInlineInsertionAndToolbarMenus(t *testing.T) {
	for _, kind := range []string{"latex", "time", "button", "doc-link"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			a.createNote("Inline content", "Inline")
			a.doc.SetSelection(7, 7)
			a.insertInline(kind)
			tt := ui.NewTester(a.view, 1440, 1100)
			if kind == "latex" {
				click(t, tt, "Apply equation")
			} else {
				click(t, tt, "Apply properties")
			}
			marker := "<" + kind
			if kind == "latex" {
				marker = "$x^2$"
			}
			if !strings.Contains(a.current.Source, marker) {
				t.Fatal("inline object absent", a.current.Source)
			}
		})
	}
	a := testApp(t)
	a.createNote("Formatted", "Toolbar")
	tt := ui.NewTester(a.view, 1440, 1000)
	click(t, tt, "Document editor")
	tt.Key(ui.Cmd, ui.KeyA)
	for _, menu := range []struct{ button, item string }{{"Color", "red"}, {"Highlight", "light-yellow"}, {"Align", "Center"}, {"Color", "Default"}, {"Highlight", "None"}, {"Align", "Right"}} {
		click(t, tt, menu.button)
		if err := tt.ChooseMenuItem(menu.item); err != nil {
			t.Fatal(err)
		}
	}
	if a.doc.Formatting().Paragraph.Align != ui.End {
		t.Fatal("alignment not applied")
	}
	a.findText = "Formatted"
	a.replaceText = "Replaced"
	a.replaceAll()
	if !strings.Contains(a.current.Source, "Replaced") {
		t.Fatal("rich replace all")
	}
}

func TestNativeTableContextOperations(t *testing.T) {
	for _, action := range []string{"Move row up", "Move row down", "Move column left", "Move column right", "Merge selected cells", "Split cell", "Cell background"} {
		t.Run(action, func(t *testing.T) {
			table := blocks.NewTable(3, 3)
			model := blocks.ReadTable(table)
			model.At(1, 1).Node.Children = []*blocks.Node{blocks.Paragraph("Selected cell")}
			model.Write(table)
			a, tt := xmlApp(t, table.XML())
			click(t, tt, "Select cell 2 2")
			if err := tt.RightClick("Select cell 2 2"); err != nil {
				t.Fatal(err)
			}
			path := []string{action}
			if action == "Cell background" {
				path = append(path, "light-green")
			}
			if err := tt.ChooseMenuItem(path...); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a.problem != "" {
				t.Fatal(a.problem)
			}
			if !strings.Contains(a.current.Source, "Selected cell") {
				t.Fatal("table action lost content")
			}
		})
	}
	a, tt := xmlApp(t, blocks.NewTable(3, 3).XML())
	click(t, tt, "Select column B")
	if a.blockEditor.cells.Bottom != 2 {
		t.Fatal("column selection")
	}
	click(t, tt, "Table options")
	click(t, tt, "Cancel")
	table := firstBlock(a, "table")
	cell := blocks.ReadTable(table).At(0, 0).Node
	a.editBlockProperties(cell)
	tt.Frame()
	click(t, tt, "Apply properties")
}

func TestCropDraggingAndQuickLookThumbnail(t *testing.T) {
	a, tt := xmlApp(t, `<p>Picture</p>`)
	a.insertAsset(pngBytes(t), "pic.png")
	tt.Frame()
	click(t, tt, "Image size, crop & caption")
	for _, name := range []string{"Crop corner 1", "Crop corner 4"} {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatal(name)
		}
		x, y := r.X+r.W/2, r.Y+r.H/2
		tt.Press(x, y)
		tt.Move(x+12, y+8)
		tt.Release(x+12, y+8)
	}
	if a.blockEditor.props["image.crop"] == "" {
		t.Fatal("dragging did not set crop")
	}
	click(t, tt, "Apply properties")
	n := firstBlock(a, "img")
	if n.Attr("image.crop") == "" {
		t.Fatal("crop not saved")
	}
	path := n.Attr("href")
	thumbnail := quickLookThumbnail(a.lib.Dir, path)
	if len(thumbnail) == 0 {
		t.Fatal("macOS did not generate an image thumbnail")
	}
	if quickLookThumbnail(a.lib.Dir, "missing") != nil {
		t.Fatal("missing preview")
	}
	if quickLookThumbnail(filepath.Join(a.lib.Dir, "missing"), "file") != nil {
		t.Fatal("invalid preview root")
	}
	entries, _ := os.ReadDir(a.lib.Dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".preview-") {
			t.Fatal("thumbnail temporary files leaked")
		}
	}
}

func TestNestedAndUnknownNativeBlocks(t *testing.T) {
	source := `<title>All blocks</title><h7>Deep heading</h7><p>Parent<p>Child</p></p><ul><li>Bullet<ul><li>Nested</li></ul></li></ul><ol><li seq="3">Third</li><li>Fourth</li></ol><checkbox done="true">Done</checkbox><blockquote><p>Quoted</p></blockquote><hr/><sheet token="local"/><whiteboard type="blank"/><sub-page-list/><figure><source path="assets/a.pdf" name="PDF"/></figure><p><img path="missing"/><time time="12:00"/><cite/><button/><widget>Unknown</widget></p>`
	a, tt := xmlApp(t, source)
	tt.SetSize(1440, 2200)
	tt.Frame()
	if !tt.HasText("All blocks") {
		t.Fatal("document missing")
	}
	if a.current.Source != source {
		t.Fatal("rendering rewrote source")
	}
	a.changeMode("Read")
	tt.Frame()
	a.changeMode("Write")
	tt.Frame()
	p := firstBlock(a, "p")
	f := a.blockField(p, a.documentColor)
	f.doc.SetSelection(0, 0)
	a.blockEditor.active = p.ID
	a.insertInline("button")
	tt.Frame()
	click(t, tt, "Apply properties")
}
