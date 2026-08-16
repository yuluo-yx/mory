package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{name: "local", config: Config{Provider: ProviderLocal}},
		{name: "github", config: Config{Provider: ProviderGitHub, Repository: "owner/repo", Token: "token"}},
		{name: "invalid github", config: Config{Provider: ProviderGitHub}, wantErr: true},
		{name: "s3", config: objectConfig(ProviderS3)},
		{name: "s4", config: objectConfig(ProviderS4)},
		{name: "oss", config: objectConfig(ProviderOSS)},
		{name: "invalid object", config: Config{Provider: ProviderS3}, wantErr: true},
		{name: "sftp", config: Config{Provider: ProviderSFTP, Host: "server", Username: "user", Password: "pass", RemotePath: "/docs"}},
		{name: "invalid sftp", config: Config{Provider: ProviderSFTP}, wantErr: true},
		{name: "unknown", config: Config{Provider: "unknown"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := test.config.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestLocalFilesAndSafePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "专题", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "专题", "文章.md"), []byte("# 文章"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "专题", ".git", "ignored"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := localFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Relative != "专题/文章.md" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if _, err := safeLocalPath(root, "../escape"); err == nil {
		t.Fatal("expected traversal error")
	}
	if err := writeLocalFile(root, "文章/image.png", []byte("image")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "文章", "image.png")); err != nil || string(data) != "image" {
		t.Fatalf("unexpected file: %q, %v", data, err)
	}
}

func TestObjectPaths(t *testing.T) {
	t.Parallel()
	if got := objectKey("docs/", "专题\\文章.md"); got != "docs/专题/文章.md" {
		t.Fatalf("objectKey = %q", got)
	}
	if got, ok := objectRelative("docs", "docs/专题/文章.md"); !ok || got != "专题/文章.md" {
		t.Fatalf("objectRelative = %q, %v", got, ok)
	}
	if _, ok := objectRelative("docs", "other/file"); ok {
		t.Fatal("unexpected prefix match")
	}
}

func TestGitHubPullAndPush(t *testing.T) {
	var createdTree map[string]any
	requests := make(map[string]int)
	remoteContent := []byte("# 远端")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing authorization")
		}
		requests[request.Method+" "+request.URL.Path]++
		status := http.StatusOK
		body := ""
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/trees/main"):
			body = `{"sha":"tree-pull","tree":[{"path":"docs/文章.md","type":"blob","sha":"remote-blob"}]}`
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/blobs/remote-blob"):
			body = string(remoteContent)
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/ref/heads/main"):
			body = `{"ref":"refs/heads/main","object":{"type":"commit","sha":"commit-old"}}`
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/trees/commit-old"):
			body = `{"sha":"tree-old","tree":[{"path":"docs/文章.md","type":"blob","sha":"remote-blob"}]}`
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/git/blobs"):
			status = http.StatusCreated
			body = `{"sha":"blob-new"}`
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/git/trees"):
			encoded, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(encoded, &createdTree); err != nil {
				return nil, err
			}
			status = http.StatusCreated
			body = `{"sha":"tree-new"}`
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/git/commits"):
			status = http.StatusCreated
			body = `{"sha":"commit-new"}`
		case request.Method == http.MethodPatch && strings.HasSuffix(request.URL.Path, "/git/refs/heads/main"):
			body = `{"ref":"refs/heads/main","object":{"type":"commit","sha":"commit-new"}}`
		default:
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}

	backend, err := newGitHubBackend(Config{Provider: ProviderGitHub, Repository: "owner/repo", Token: "token", Branch: "main", Prefix: "docs", Endpoint: "https://api.example.test"}, client)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	summary, err := backend.Pull(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Files != 1 {
		t.Fatalf("pull summary = %#v", summary)
	}
	if data, err := os.ReadFile(filepath.Join(root, "文章.md")); err != nil || string(data) != "# 远端" {
		t.Fatalf("pull file = %q, %v", data, err)
	}
	if err := os.Remove(filepath.Join(root, "文章.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.md"), []byte("# 本地"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err = backend.Push(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Files != 1 || createdTree["base_tree"] != "tree-old" {
		t.Fatalf("push summary = %#v, tree = %#v", summary, createdTree)
	}
	if requests["POST /repos/owner/repo/git/commits"] != 1 || requests["PATCH /repos/owner/repo/git/refs/heads/main"] != 1 {
		t.Fatalf("github push did not create one commit: %#v", requests)
	}
}

func TestGitHubPushSkipsUnchangedFiles(t *testing.T) {
	root := t.TempDir()
	data := []byte("# 未变化")
	if err := os.WriteFile(filepath.Join(root, "same.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	mutatingRequests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		switch {
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/ref/heads/main"):
			body = `{"object":{"type":"commit","sha":"commit-old"}}`
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/trees/commit-old"):
			body = fmt.Sprintf(`{"sha":"tree-old","tree":[{"path":"same.md","type":"blob","sha":%q}]}`, gitBlobSHA(data))
		default:
			mutatingRequests++
			return &http.Response{StatusCode: http.StatusInternalServerError, Status: "unexpected", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"message":"unexpected"}`)), Request: request}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}
	backend, err := newGitHubBackend(Config{Provider: ProviderGitHub, Repository: "owner/repo", Token: "token", Endpoint: "https://api.example.test"}, client)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := backend.Push(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Files != 0 || mutatingRequests != 0 {
		t.Fatalf("unchanged push = %#v, mutating requests = %d", summary, mutatingRequests)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func objectConfig(provider string) Config {
	config := Config{Provider: provider, Region: "region", Bucket: "bucket", AccessKeyID: "id", AccessKeySecret: "secret"}
	if provider == ProviderS4 {
		config.Endpoint = "https://s4.example"
	}
	return config
}
