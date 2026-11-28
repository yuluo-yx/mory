package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/yuluo-yx/mory/internal/doctheme"
)

func TestControlHoverOpensDocumentLinkWithoutEditing(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, mode := range []string{"Write", "Read", "Split"} {
			t.Run(fmt.Sprintf("structured=%t/%s", structured, mode), func(t *testing.T) {
				a := testApp(t)
				target, err := a.lib.Create("Destination", "Destination body", "markdown")
				if err != nil {
					t.Fatal(err)
				}
				source := "[Open destination](mory:" + target.ID + ") and keep this text"
				if structured {
					source = "<p><a href=\"mory:" + target.ID + "\">Open destination</a> and keep this text</p>"
				}
				a.createNote(source, "Link source")
				original := a.current
				a.changeMode(mode)
				tt := ui.NewTester(a.view, 1280, 900)
				label := "Document editor"
				if a.blockEditor != nil {
					label = "Block text " + firstBlock(a, "p").ID
				}
				r, ok := tt.Find(label)
				if !ok {
					t.Fatal("missing link editor", label)
				}
				if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" && !structured && mode == "Write" {
					saveImage(t, tt, filepath.Join(dir, "hyperlink-color.png"))
				}
				x := r.X + 10
				y := r.Y + 10
				if !structured && mode == "Write" {
					x += documentScrollGutter
					// The native editor includes the scrolling title space above
					// its first paragraph; hover the actual link's line box.
					y = r.Y + a.doc.VisibleParagraphs()[0].Top + 10
				}
				tt.Move(x, y)
				tt.Frame()
				if a.current != original {
					t.Fatal("ordinary hover opened a link")
				}
				tt.HoldModifiers(ui.Ctrl)
				tt.Frame()
				tt.Frame()
				if a.current != target {
					t.Fatal("Ctrl hover did not navigate", mode, a.linkHover)
				}
				if original.Source != source {
					t.Fatal("hover changed the stored document", original.Source)
				}
			})
		}
	}
}

func TestLinkHoverGeometryAndOneShot(t *testing.T) {
	for _, style := range []richtext.ParagraphStyle{{}, {Heading: 2}, {List: richtext.Bullets, Indent: 1}, {Quote: true}, {Align: ui.Center}, {Align: ui.End}} {
		t.Run(fmt.Sprint(style), func(t *testing.T) {
			a := testApp(t)
			a.createNote("Body", "Geometry")
			doc := richtext.NewDocument(richtext.TextOf(richtext.Paragraph{Style: style, Runs: []richtext.Run{
				{Text: "Before 🌱 "},
				{Text: "链接世界 link words wrap across several lines", Attributes: richtext.Attributes{Link: "#target", Weight: 700}},
				{Text: " after"},
			}}))
			font := ui.Font{Size: 18, LineHeight: 1.6}
			theme := doctheme.Parse(doctheme.Builtin("Notion")).Rich(ui.LightTheme(), font)
			var ed ui.Element
			tt := ui.NewTester(func(c *ui.Context) {
				a.beginLinkHover(c)
				defer a.endLinkHover()
				ed = richtext.EditorBase(c, doc, richtext.EditorOptions{Font: font, Theme: theme}).Width(240).Height(400).Label("Links")
				a.hoverDocumentLink(c, ed, doc, font, theme)
			}, 400, 500)
			r, _ := tt.Find("Links")
			// Use the native editor's caret geometry as the independent oracle,
			// including wrapped lines and styled/indented/aligned paragraphs.
			for _, at := range []int{12, 18, 30, 45} {
				doc.SetSelection(at, at)
				tt.Frame()
				caretRect, ok := doc.CaretRect()
				if !ok || richLinkAt(doc, font, theme, r.W, caretRect.X+1, caretRect.Y+caretRect.H/2) != "#target" {
					t.Fatalf("link hit region differs from native text at rune %d: %+v", at, caretRect)
				}
			}
			beforeText := doc.Text()
			anchor, caret := doc.Selection()
			var hits []ui.Point
			for y := float32(2); y < 220; y += 4 {
				for x := float32(2); x < 238; x += 4 {
					if richLinkAt(doc, font, theme, r.W, x, y) == "#target" {
						hits = append(hits, ui.Point{X: x, Y: y})
					}
				}
			}
			if len(hits) == 0 {
				t.Fatal("wrapped link had no hit regions")
			}
			point := hits[len(hits)/2]
			tt.Move(r.X+point.X, r.Y+point.Y)
			tt.HoldModifiers(ui.Ctrl)
			tt.Frame()
			tt.Frame()
			version := a.preview.AnchorVersion
			if a.preview.Anchor != "#target" || version != 1 {
				t.Fatal("hover did not open exactly once", version)
			}
			for range 4 {
				tt.Frame()
			}
			if a.preview.AnchorVersion != version {
				t.Fatal("stationary hover repeatedly opened the link")
			}
			tt.Move(r.X+239, r.Y+390)
			tt.Frame()
			if a.preview.AnchorVersion != version {
				t.Fatal("blank area opened nearest link")
			}
			tt.Move(r.X+point.X, r.Y+point.Y)
			tt.HoldModifiers(ui.Ctrl)
			tt.Frame()
			if a.preview.AnchorVersion != version+1 {
				t.Fatal("leaving and returning did not rearm hover")
			}
			gotAnchor, gotCaret := doc.Selection()
			if !doc.Text().Equal(beforeText) || gotAnchor != anchor || gotCaret != caret || doc.CanUndo() {
				t.Fatal("hover changed text, selection, or undo history")
			}
		})
	}
}

func TestControlHoverDoesNotFollowWhileDragging(t *testing.T) {
	a := testApp(t)
	target, err := a.lib.Create("Destination", "Body", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	a.createNote("[Linked text](mory:"+target.ID+") and body", "Source")
	original := a.current
	tt := ui.NewTester(a.view, 1280, 900)
	r, _ := tt.Find("Document editor")
	tt.Press(r.X+10, r.Y+10)
	tt.HoldModifiers(ui.Ctrl)
	tt.Frame()
	if a.current != original {
		t.Fatal("holding Control during a pointer press navigated")
	}
	tt.Move(r.X+r.W-10, r.Y+10)
	tt.Release(r.X+r.W-10, r.Y+10)
	tt.Frame()
	if a.current != original {
		t.Fatal("dragging linked text navigated")
	}
}

func TestLinkColorAcrossWritingThemes(t *testing.T) {
	for _, name := range []string{"Craft", "Feishu", "Notion", "Obsidian", "Docusaurus"} {
		for _, base := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
			a := testApp(t)
			a.docTheme = doctheme.Parse(doctheme.Builtin(name))
			page := a.pageTheme(base)
			rich := a.docTheme.Rich(page, ui.Font{Size: 17})
			if rich.Link == rich.Text || rich.Link != page.Accent || rich.Link.A != 255 {
				t.Fatal("links are indistinguishable or inconsistent", name, base.Dark, rich.Link, rich.Text)
			}
		}
	}
	theme := doctheme.Parse(".page{color:#37352f}a{color:#37352f}")
	if theme.LinkColor(ui.LightTheme()) != ui.Hex("#0969da") {
		t.Fatal("existing neutral theme failed to gain link color")
	}
	custom := strings.ReplaceAll(theme.Source, "a{color:#37352f}", "a{color:#7c3aed}")
	if doctheme.Parse(custom).LinkColor(ui.LightTheme()) != ui.Hex("#7c3aed") {
		t.Fatal("custom chromatic link color was lost")
	}
}
