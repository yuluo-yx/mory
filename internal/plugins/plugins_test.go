package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltins(t *testing.T) {
	ps, errs := Load(t.TempDir())
	if len(errs) > 0 || len(ps) != 3 {
		t.Fatal(ps, errs)
	}
	for _, p := range ps {
		out, err := p.Run(context.Background(), Input{Title: "Test", Markdown: "# Heading\n\nText   \n\n## Second\n"})
		if err != nil {
			t.Fatal(p.Name, err)
		}
		if out.Message == "" {
			t.Fatal("missing result")
		}
		if p.Name == "Insert table of contents" {
			if out.Markdown == nil || !strings.Contains(*out.Markdown, "[Heading](#heading)") {
				t.Fatal(out)
			}
			again, err := p.Run(context.Background(), Input{Markdown: *out.Markdown})
			if err != nil || *again.Markdown != *out.Markdown {
				t.Fatal("TOC not idempotent", err)
			}
		}
	}
}

func TestSandboxAndLimits(t *testing.T) {
	for _, source := range []string{`export function transform() { for (;;) {} }`, `for (;;) {} export function transform(){}`, `export function transform() { throw new Error("failed"); }`, `export function transform() { return {}; }`, `export function transform() { return {markdown: process.env.HOME}; }`, `export function transform() { return {markdown: fetch("https://example.com")}; }`} {
		p, err := Compile("test", source)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		_, err = p.Run(ctx, Input{})
		cancel()
		if err == nil {
			t.Fatalf("sandbox accepted %s", source)
		}
	}
	p, err := Compile("valid", `export function transform(x) { return { markdown: x.markdown.toUpperCase() }; }`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Run(context.Background(), Input{Markdown: "hello"})
	if err != nil || *out.Markdown != "HELLO" {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = p.Run(ctx, Input{}); err == nil {
		t.Fatal("cancel ignored")
	}
	for _, source := range []string{"{", `export function wrong(){}`, strings.Repeat("x", (1<<20)+1)} {
		if _, err := Compile("invalid", source); err == nil {
			t.Fatal("invalid plugin accepted")
		}
	}
}

func TestDiscovery(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "custom.js"), []byte(`export function transform() { return { message: "hello" }; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.js"), []byte("bad syntax !!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ps, errs := Load(dir)
	if len(ps) != 4 || len(errs) != 1 {
		t.Fatal(len(ps), errs)
	}
	_, errs = Load(filepath.Join(dir, "missing"))
	if len(errs) != 1 {
		t.Fatal(errs)
	}
}
