package main

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

func TestCalloutEndArrowsCreateBodyParagraph(t *testing.T) {
	for _, key := range []ui.Key{ui.KeyDown, ui.KeyRight} {
		a := testApp(t)
		const source = "> [!NOTE]\n> Inside 世界"
		a.createNote(source, "Callout exit")
		tt := ui.NewTester(a.view, 1280, 900)
		n := firstBlock(a, "callout")
		text := n.Children[0]
		click(t, tt, "Block text "+text.ID)
		field := a.blockEditor.fields[text.ID]
		end := field.doc.Text().Len()
		field.doc.SetSelection(end, end)
		tt.Key(0, key)
		tt.Type("Outside")
		tt.Type(" body")
		a.capture()
		if got := firstBlock(a, "callout").Plain(); got != "Inside 世界" {
			t.Fatalf("boundary arrow kept typing inside callout: %q", got)
		}
		if !strings.Contains(a.current.Source, "Outside body") {
			t.Fatal("body did not receive typing", a.current.Source)
		}
		a.loadEditors()
		tt.Frame()
		if firstBlock(a, "callout").Plain() != "Inside 世界" {
			t.Fatal("reopening absorbed the body into the callout", a.current.Source)
		}
	}
}

func TestCalloutOffersTypePicker(t *testing.T) {
	a, tt := xmlApp(t, `<callout alert="NOTE"><p>Inside</p></callout>`)
	if _, ok := tt.Access("Callout type"); !ok {
		t.Fatal("callout has no visible type picker")
	}
	click(t, tt, "Callout type")
	click(t, tt, "warning")
	if !strings.Contains(strings.ToLower(a.current.Source), "warning") {
		t.Fatal("type picker did not update the document", a.current.Source)
	}
	a.blockUndo(false)
	if blocks.CalloutType(firstBlock(a, "callout")) != "note" {
		t.Fatal("undo did not restore the previous type")
	}
	a.blockUndo(true)
	if blocks.CalloutType(firstBlock(a, "callout")) != "warning" {
		t.Fatal("redo did not restore the chosen type")
	}
}

func TestNestedCalloutExitStaysInParent(t *testing.T) {
	a, tt := xmlApp(t, `<callout><p>Outer</p><callout><p>Inner</p></callout></callout>`)
	outer := a.blockEditor.doc.Root.Children[0]
	inner := outer.Children[1]
	click(t, tt, "Block text "+inner.Children[0].ID)
	a.doc.SetSelection(5, 5)
	tt.Key(0, ui.KeyRight)
	tt.Type("Within outer")
	a.capture()
	outer = a.blockEditor.doc.Find(outer.ID)
	if len(a.blockEditor.doc.Root.Children) != 1 || len(outer.Children) != 3 || outer.Children[2].Plain() != "Within outer" || outer.Children[1].Plain() != "Inner" {
		t.Fatal("inner callout exit escaped or changed its parent", a.current.Source)
	}
}

func TestCalloutPointerExitAndUndo(t *testing.T) {
	for _, route := range []string{"below", "menu"} {
		t.Run(route, func(t *testing.T) {
			a := testApp(t)
			const source = "> [!WARNING]\n> Keep\n\n## Following\n"
			a.createNote(source, "Pointer exit")
			tt := ui.NewTester(a.view, 1280, 900)
			exit := func() {
				if route == "below" {
					click(t, tt, "Write below callout")
				} else {
					if err := tt.RightClick("Block actions callout"); err != nil {
						t.Fatal(err)
					}
					if err := tt.ChooseMenuItem("Insert paragraph below"); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
			}
			exit()
			count := len(a.blockEditor.doc.Root.Children)
			exit()
			if len(a.blockEditor.doc.Root.Children) != count {
				t.Fatal("repeated exit inserted duplicate empty paragraphs")
			}
			tt.Type("Body")
			tt.Type(" text")
			a.capture()
			if firstBlock(a, "callout").Plain() != "Keep" || firstBlock(a, "h2").Plain() != "Following" || !strings.Contains(a.current.Source, "Body text") {
				t.Fatal("pointer exit replaced or absorbed existing content", a.current.Source)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Key(ui.Cmd, ui.KeyZ)
			if a.current.Source != source {
				t.Fatal("undo did not restore exact source", a.current.Source)
			}
		})
	}
}

func TestCalloutNavigationStaysLocalUntilBoundary(t *testing.T) {
	a, tt := xmlApp(t, `<callout><p>First</p><p>Last</p></callout><h2>After</h2>`)
	n := firstBlock(a, "callout")
	first, last := n.Children[0], n.Children[1]
	click(t, tt, "Block text "+first.ID)
	a.doc.SetSelection(5, 5)
	tt.Key(0, ui.KeyRight)
	if !tt.Focused("Block text " + last.ID) {
		t.Fatal("arrow skipped a paragraph inside the callout")
	}
	a.doc.SetSelection(0, 4)
	tt.Key(0, ui.KeyRight)
	if !tt.Focused("Block text " + last.ID) {
		t.Fatal("collapsing a selection exited the callout")
	}
	a.doc.SetSelection(4, 4)
	tt.Key(ui.Shift, ui.KeyRight)
	if !tt.Focused("Block text " + last.ID) {
		t.Fatal("extending a selection exited the callout")
	}
	a.doc.SetSelection(4, 4)
	tt.Key(0, ui.KeyEnter)
	tt.Type("New inner paragraph")
	a.capture()
	if !strings.Contains(firstBlock(a, "callout").Plain(), "New inner paragraph") {
		t.Fatal("Enter exited the callout")
	}
	tt.Key(0, ui.KeyDown)
	tt.Type("Next ")
	a.capture()
	if firstBlock(a, "h2").Plain() != "Next After" {
		t.Fatal("boundary arrow failed to enter existing following text", a.current.Source)
	}
}

func TestCalloutTypePropertiesReopenAndReadOnly(t *testing.T) {
	a := testApp(t)
	a.lib.Settings.Language = "zh-CN"
	a.createNote("> [!WARNING]\n> Content\n", "Type persistence")
	tt := ui.NewTester(a.view, 1280, 900)
	n := firstBlock(a, "callout")
	a.editBlockProperties(n)
	tt.Frame()
	click(t, tt, a.tr("Property callout type"))
	click(t, tt, "danger")
	click(t, tt, a.tr("Apply properties"))
	if blocks.CalloutType(firstBlock(a, "callout")) != "danger" {
		t.Fatal("properties did not apply the type")
	}
	if !a.save() {
		t.Fatal(a.problem)
	}
	a.loadEditors()
	tt.Frame()
	if blocks.CalloutType(firstBlock(a, "callout")) != "danger" || firstBlock(a, "callout").Plain() != "Content" {
		t.Fatal("reopening lost the type or content")
	}
	a.changeMode("Read")
	tt.Frame()
	if _, ok := tt.Access(a.tr("Callout type")); ok {
		t.Fatal("read mode exposes an editable type picker")
	}
	if _, ok := tt.Access(a.tr("Write below callout")); ok {
		t.Fatal("read mode exposes a writable exit")
	}
	before := a.current.Source
	a.changeCalloutType(n.ID, "note")
	if a.leaveBlock(firstBlock(a, "callout").ID, 1, true) || a.current.Source != before {
		t.Fatal("read-only callout was changed")
	}
}

func TestCalloutSemanticColors(t *testing.T) {
	for _, appearance := range []string{"Light", "Dark"} {
		a, tt := xmlApp(t, `<callout alert="NOTE"><p>Note content</p></callout><callout directive="info"><p>Information</p></callout><callout alert="WARNING"><p>Warning content</p></callout><callout directive="danger"><p>Danger content</p></callout>`)
		a.lib.Settings.Appearance = appearance
		tt.Frame()
		seen := map[color.RGBA]bool{}
		for _, n := range a.blockEditor.doc.Root.Children {
			r, ok := tt.Find("Block text " + n.Children[0].ID)
			if !ok {
				t.Fatal("missing callout text")
			}
			pixel := tt.Image().RGBAAt(int(r.X+r.W-10), int(r.Y+8))
			if seen[pixel] {
				t.Fatalf("semantic backgrounds are indistinguishable in %s: %v", appearance, pixel)
			}
			seen[pixel] = true
		}
		if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
			saveImage(t, tt, filepath.Join(dir, "callouts-"+appearance+".png"))
		}
	}
}
