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
	_ = a.croppedImage("missing.png", ".1 .1 .9 .9", image)
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
