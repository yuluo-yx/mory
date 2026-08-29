package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuluo-yx/mory/appcli"
)

type fakeDesktopClient struct {
	opened   string
	exported appcli.ExportRequest
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
	command.SetArgs([]string{"export", "--format=png", "--path", root, document})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if client.exported.Format != "png" || client.exported.Destination != filepath.Join(root, "guide.png") {
		t.Fatalf("export request = %#v", client.exported)
	}
	if output.String() != filepath.Join(root, "guide.png")+"\n" {
		t.Fatalf("output = %q", output.String())
	}
}
