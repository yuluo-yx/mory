package slidevexport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRunnerExportsWithProjectLocalOfficialCLI(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "slides.md")
	destination := filepath.Join(root, "slides.pptx")
	cli := filepath.Join(root, "node_modules", "@slidev", "cli", "bin", "slidev.mjs")
	browser := filepath.Join(root, "browser")
	if err := os.MkdirAll(filepath.Dir(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cli, []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(browser, []byte("browser"), 0o755); err != nil {
		t.Fatal(err)
	}

	var executed []string
	runner := Runner{
		Getenv: func(name string) string {
			if name == "MORY_SLIDEV_BROWSER" {
				return browser
			}
			return ""
		},
		UserConfigDir: func() (string, error) { return "", errors.New("not found") },
		LookPath: func(name string) (string, error) {
			if name == "node" {
				return "/usr/bin/node", nil
			}
			return "", errors.New("not found")
		},
		Run: func(_ context.Context, directory, executable string, arguments []string) ([]byte, error) {
			executed = append([]string{directory, executable}, arguments...)
			markdown, err := os.ReadFile(arguments[2])
			if err != nil {
				return nil, err
			}
			if string(markdown) != "---\ntheme: seriph\n---\n# Demo" {
				t.Fatalf("temporary Markdown = %q", markdown)
			}
			return nil, os.WriteFile(destination, []byte("pptx"), 0o644)
		},
	}

	err := runner.Export(t.Context(), Request{
		Markdown: "---\ntheme: seriph\n---\n# Demo", SourcePath: source, Destination: destination,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(executed) < 11 || executed[0] != root || executed[1] != "/usr/bin/node" || executed[2] != cli {
		t.Fatalf("executed = %#v", executed)
	}
	if !slices.Contains(executed, "--with-clicks") || !slices.Contains(executed, "pptx") {
		t.Fatalf("Slidev arguments = %#v", executed)
	}
	browserIndex := slices.Index(executed, "--executable-path")
	if browserIndex < 0 || executed[browserIndex+1] != browser {
		t.Fatalf("browser arguments = %#v", executed)
	}
}

func TestRunnerReportsUnavailableOfficialCLI(t *testing.T) {
	runner := Runner{
		Getenv:        func(string) string { return "" },
		LookPath:      func(string) (string, error) { return "", errors.New("not found") },
		UserConfigDir: func() (string, error) { return "", errors.New("not found") },
		Run: func(context.Context, string, string, []string) ([]byte, error) {
			t.Fatal("command must not run")
			return nil, nil
		},
	}
	err := runner.Export(t.Context(), Request{Markdown: "# Demo", Destination: filepath.Join(t.TempDir(), "demo.pptx")})
	if ErrorCode(err) != CodeUnavailable {
		t.Fatalf("error = %v, code = %s", err, ErrorCode(err))
	}
}

func TestRunnerFindsIsolatedMoryRuntime(t *testing.T) {
	runtimeRoot := t.TempDir()
	script := filepath.Join(runtimeRoot, "node_modules", "@slidev", "cli", "bin", "slidev.mjs")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		Getenv: func(name string) string {
			if name == "MORY_SLIDEV_HOME" {
				return runtimeRoot
			}
			return ""
		},
		LookPath:      func(string) (string, error) { return "/usr/bin/node", nil },
		UserConfigDir: func() (string, error) { return "", errors.New("not found") },
	}
	cli, err := runner.findCLI(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cli.executable != "/usr/bin/node" || len(cli.prefix) != 1 || cli.prefix[0] != script {
		t.Fatalf("CLI = %#v", cli)
	}
}

func TestRunnerRejectsNonPowerPointDestination(t *testing.T) {
	err := (Runner{}).Export(t.Context(), Request{Destination: filepath.Join(t.TempDir(), "demo.pdf")})
	if ErrorCode(err) != CodeFailed {
		t.Fatalf("error = %v", err)
	}
}
