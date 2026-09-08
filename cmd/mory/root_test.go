package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo-yx/mory/appcli"
)

type fakeDesktopClient struct {
	opened   string
	exported appcli.ExportRequest
}

func TestExportCommandScenarios(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		format    string
		extension string
		flags     []string
		existing  string
		wantError string
	}{
		{name: "default PDF", extension: ".pdf"},
		{name: "HTML", format: "html", extension: ".html"},
		{name: "PNG", format: "png", extension: ".png"},
		{name: "JPEG", format: "jpeg", extension: ".jpg"},
		{name: "JPG alias", format: "JPG", extension: ".jpg"},
		{name: "trimmed format", format: " PDF ", extension: ".pdf"},
		{name: "slides", format: "pptx", extension: ".pptx"},
		{name: "existing output", extension: ".pdf", existing: "file", wantError: "use --force"},
		{name: "explicit replacement", extension: ".pdf", existing: "file", flags: []string{"--force"}},
		{name: "short replacement flag", extension: ".pdf", existing: "file", flags: []string{"-f"}},
		{name: "directory collision", extension: ".pdf", existing: "directory", flags: []string{"--force"}, wantError: "not a regular file"},
		{name: "invalid format", format: "docx", wantError: "unsupported export format"},
		{name: "unknown option", flags: []string{"--overwrite"}, wantError: "unknown flag"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "\u6587\u6863 with spaces.MD")
			if err := os.WriteFile(source, []byte("# Original"), 0o644); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(root, "\u6587\u6863 with spaces"+scenario.extension)
			if scenario.existing == "file" {
				if err := os.WriteFile(destination, []byte("existing"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if scenario.existing == "directory" {
				if err := os.Mkdir(destination, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			client := &fakeDesktopClient{}
			var selectedApp string
			command := newRootCommandWithClient(func(app string) desktopClient { selectedApp = app; return client })
			output := new(bytes.Buffer)
			command.SetOut(output)
			args := []string{"export", "--app", "custom app", "--path", root}
			if scenario.format != "" {
				args = append(args, "--format", scenario.format)
			}
			args = append(args, scenario.flags...)
			command.SetArgs(append(args, "--", source))
			err := command.ExecuteContext(t.Context())
			if scenario.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), scenario.wantError) {
					t.Fatalf("error = %v; want %q", err, scenario.wantError)
				}
				if client.exported.Source != "" || output.Len() != 0 {
					t.Fatal("rejected export must not launch or report success")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if selectedApp != "custom app" || client.exported.Source != source || client.exported.Destination != destination {
					t.Fatalf("export = %#v, app = %q", client.exported, selectedApp)
				}
				if scenario.extension == ".jpg" && client.exported.Format != "jpeg" {
					t.Fatalf("host format = %q", client.exported.Format)
				}
				if output.String() != destination+"\n" {
					t.Fatalf("output = %q", output.String())
				}
			}
			if scenario.existing == "file" {
				data, err := os.ReadFile(destination)
				if err != nil || string(data) != "existing" {
					t.Fatalf("validation changed existing output: %q, %v", data, err)
				}
			}
		})
	}
}

func TestOpenCommandHandlesRelativeAndFlagLikeNames(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, name := range []string{"guide.md", "-draft.md", "\u6587\u6863 with spaces.MD"} {
		if err := os.WriteFile(name, []byte("# Guide"), 0o644); err != nil {
			t.Fatal(err)
		}
		client := &fakeDesktopClient{}
		command := newRootCommandWithClient(func(string) desktopClient { return client })
		command.SetArgs([]string{"--", name})
		if err := command.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}
		if client.opened != filepath.Join(root, name) {
			t.Fatalf("opened = %q", client.opened)
		}
	}
}

func (client *fakeDesktopClient) Open(_ context.Context, path string) error {
	client.opened = path
	return nil
}

func (client *fakeDesktopClient) Export(_ context.Context, request appcli.ExportRequest) error {
	client.exported = request
	return nil
}

func TestCommandsRouteOpenAndExportRequests(t *testing.T) {
	root := t.TempDir()
	document := filepath.Join(root, "guide.md")
	if err := os.WriteFile(document, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := &fakeDesktopClient{}
	command := newRootCommandWithClient(func(string) desktopClient { return client })
	command.SetArgs([]string{document})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if client.opened != document {
		t.Fatalf("opened path = %q", client.opened)
	}

	output := new(bytes.Buffer)
	command = newRootCommandWithClient(func(string) desktopClient { return client })
	command.SetOut(output)
	command.SetArgs([]string{"export", "--format=png", "--path=" + root, document})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if client.exported.Format != "png" || client.exported.Destination != filepath.Join(root, "guide.png") {
		t.Fatalf("export request = %#v", client.exported)
	}
	if output.String() != filepath.Join(root, "guide.png")+"\n" {
		t.Fatalf("output = %q", output.String())
	}

	help := new(bytes.Buffer)
	command = newRootCommandWithClient(func(string) desktopClient { return client })
	command.SetOut(help)
	command.SetArgs([]string{"export", "--help"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(help.Bytes(), []byte("mory export --format=pdf --path=./ guide.md")) {
		t.Fatalf("export help = %q", help.String())
	}
}
