package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/yuluo-yx/mory/internal/blocks"
	"github.com/yuluo-yx/mory/internal/library"
)

func firstCodeBlock(t *testing.T, a *app) *blocks.Node {
	t.Helper()
	if a.blockEditor == nil {
		t.Fatal("code did not activate the block editor")
	}
	var code *blocks.Node
	a.blockEditor.doc.Root.Walk(func(n *blocks.Node) {
		if code == nil && n.Kind == "pre" {
			code = n
		}
	})
	if code == nil {
		t.Fatalf("code block is missing: source=%q tree=%s", a.current.Source, a.blockEditor.doc.Root.XML())
	}
	return code
}

func TestTypedCodeFenceKeepsLanguageAndFocus(t *testing.T) {
	for _, structured := range []bool{false, true} {
		a := testApp(t)
		a.createNote("Draft", "Typed code")
		label := "Document editor"
		if structured {
			if !a.enableBlocks() {
				t.Fatal(a.problem)
			}
			label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
		}
		tt := ui.NewTester(a.view, 1280, 820)
		click(t, tt, label)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("```go")
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		code := firstCodeBlock(t, a)
		if code.Attr("lang") != "go" || !tt.HasText("Code language") || !tt.Focused("Code editor") {
			t.Fatalf("fence language or editor focus lost (structured=%t): %+v", structured, code)
		}
		tt.Type("package main")
		tt.Key(0, ui.KeyEnter)
		tt.Type("// Next line")
		a.capture()
		if !strings.Contains(a.current.Source, "```go") || !strings.Contains(a.current.Source, "package main") || strings.Contains(a.current.Source, "```go\n```go") {
			t.Fatal(a.current.Source)
		}
		if !strings.Contains(a.current.Source, "package main\n// Next line") {
			t.Fatal("code editor did not receive Enter", a.current.Source)
		}
		if !a.save() {
			t.Fatal(a.problem)
		}
		stored, err := library.Open(a.lib.Dir)
		if err != nil || stored.Get(a.current.ID).Source != a.current.Source {
			t.Fatal("typed code did not persist", err)
		}
	}
}

func TestMarkdownCodeLanguageHighlightAndRoundTrip(t *testing.T) {
	const source = "# Before\n\n```go\npackage main\n\nfunc main() {\n\tfmt.Println(\"hello world!\")\n}\n```\n\nAfter paragraph.\n"
	a := testApp(t)
	a.createNote(source, "Code languages")
	tt := ui.NewTester(a.view, 1440, 1000)
	code := firstCodeBlock(t, a)
	for _, mode := range []string{"Write", "Read", "Split", "Source", "Write"} {
		a.changeMode(mode)
		tt.Frame()
		if a.current.Source != source {
			t.Fatalf("viewing code in %s rewrote the source", mode)
		}
	}
	for _, appearance := range []string{"Light", "Dark"} {
		a.lib.Settings.Appearance = appearance
		tt.Frame()
		f := a.blockEditor.fields[code.ID]
		if f.code.Buffer.Language() != codeLanguage("go") || f.code.Theme.Tokens[editor.TokenKeyword].Color == f.code.Theme.Text || f.code.Theme.Tokens[editor.TokenString].Color == f.code.Theme.Text {
			t.Fatal("Go syntax is not highlighted in " + appearance)
		}
		if dir := os.Getenv("MORY_TEST_IMAGES"); dir != "" {
			a.lib.Settings.Language = "zh-CN"
			tt.Frame()
			saveImage(t, tt, filepath.Join(dir, "code-block-"+strings.ToLower(appearance)+".png"))
			a.lib.Settings.Language = "en"
			tt.Frame()
		}
	}
	click(t, tt, "Code language")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("python")
	if a.blockEditor.doc.Find(code.ID).Attr("lang") != "python" || a.blockEditor.fields[code.ID].code.Buffer.Language() != codeLanguage("python") {
		t.Fatalf("language input did not update the code tokenizer: %s", a.current.Source)
	}
	a.capture()
	if !strings.Contains(a.current.Source, "```python") || !strings.Contains(a.current.Source, "fmt.Println") || !strings.Contains(a.current.Source, "After paragraph.") {
		t.Fatal(a.current.Source)
	}
}

func TestCodeBlockToolbarAndInsertion(t *testing.T) {
	a := testApp(t)
	a.createNote("package main", "Toolbar code")
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "Code block")
	tt.Frame()
	code := firstCodeBlock(t, a)
	if strings.TrimSuffix(code.Plain(), "\n") != "package main" || !tt.HasText("Code language") {
		t.Fatalf("toolbar code lost its content or language control: %q", code.Plain())
	}
	click(t, tt, "Code block")
	if strings.Contains(a.current.Source, "```") || !strings.Contains(a.current.Source, "package main") {
		t.Fatal("toggling code off lost content", a.current.Source)
	}
	a.createNote("", "Insert code")
	a.insert("\n```go\nfunc main() {}\n```\n")
	tt.Frame()
	if firstCodeBlock(t, a).Attr("lang") != "go" || !tt.HasText("Code language") {
		t.Fatal("insert menu code lost its language")
	}
}

func TestInlineCodeTyping(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, chunked := range []bool{false, true} {
			t.Run(fmt.Sprintf("structured=%t/chunked=%t", structured, chunked), func(t *testing.T) {
				a := testApp(t)
				a.createNote("Draft", "Inline code")
				label := "Document editor"
				if structured {
					if !a.enableBlocks() {
						t.Fatal(a.problem)
					}
					label = "Block text " + a.blockEditor.doc.Root.Children[0].ID
				}
				tt := ui.NewTester(a.view, 1280, 820)
				click(t, tt, label)
				tt.Key(ui.Cmd, ui.KeyA)
				const typed = "这是测试 `func test()` 代码块内容。"
				if chunked {
					tt.Type(typed)
				} else {
					for _, r := range typed {
						tt.Type(string(r))
					}
				}
				text := a.doc.Text()
				if text.String() != "这是测试 func test() 代码块内容。" || !text.AttributesAt(5).Code || text.AttributesAt(text.Len()-1).Code {
					t.Fatalf("inline code did not format independently of surrounding text: %q", text.String())
				}
				a.capture()
				if !strings.Contains(a.current.Source, "`func test()`") {
					t.Fatal("inline code was not serialized", a.current.Source)
				}
			})
		}
	}
}

func TestEmptyBlockDoesNotShowEnglishPlaceholder(t *testing.T) {
	a := testApp(t)
	a.createNote("# Heading\n\nParagraph", "Blank block")
	if !a.enableBlocks() {
		t.Fatal(a.problem)
	}
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	if tt.HasText("Write something…") {
		t.Fatal("empty body block exposed an editor placeholder")
	}
}
