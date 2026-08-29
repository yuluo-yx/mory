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
		{name: "github empty owner", config: Config{Provider: ProviderGitHub, Repository: "/repo", Token: "token"}, wantErr: true},
		{name: "github empty repository", config: Config{Provider: ProviderGitHub, Repository: "owner/", Token: "token"}, wantErr: true},
		{name: "github nested repository", config: Config{Provider: ProviderGitHub, Repository: "owner/repo/path", Token: "token"}, wantErr: true},
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
	if err := os.MkdirAll(filepath.Join(root, "\u4E13\u9898", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "\u4E13\u9898", "article.md"), []byte("# \u6587\u7AE0"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "\u4E13\u9898", ".git", "ignored"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := localFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Relative != "\u4E13\u9898/article.md" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if _, err := safeLocalPath(root, "../escape"); err == nil {
		t.Fatal("expected traversal error")
	}
	if err := writeLocalFile(root, "\u6587\u7AE0/image.png", []byte("image")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "\u6587\u7AE0", "image.png")); err != nil || string(data) != "image" {
		t.Fatalf("unexpected file: %q, %v", data, err)
	}
}

func TestObjectPaths(t *testing.T) {
	t.Parallel()
	if got := objectKey("docs/", "\u4E13\u9898\\article.md"); got != "docs/\u4E13\u9898/article.md" {
		t.Fatalf("objectKey = %q", got)
	}
	if got, ok := objectRelative("docs", "docs/\u4E13\u9898/article.md"); !ok || got != "\u4E13\u9898/article.md" {
		t.Fatalf("objectRelative = %q, %v", got, ok)
	}
	if _, ok := objectRelative("docs", "other/file"); ok {
		t.Fatal("unexpected prefix match")
	}
}

func TestGitHubPullAndPush(t *testing.T) {
	var createdTree map[string]any
	requests := make(map[string]int)
	remoteContent := []byte("# \u8FDC\u7AEF")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing authorization")
		}
		requests[request.Method+" "+request.URL.Path]++
		status := http.StatusOK
		body := ""
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/repos/owner/repo":
			body = `{"full_name":"owner/repo","default_branch":"main"}`
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/trees/commit-old") && request.URL.Query().Get("recursive") == "1":
			body = `{"sha":"tree-old","tree":[{"path":"docs/article.md","type":"blob","sha":"remote-blob"}]}`
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/blobs/remote-blob"):
			body = string(remoteContent)
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/ref/heads/main"):
			body = `{"ref":"refs/heads/main","object":{"type":"commit","sha":"commit-old"}}`
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
	if data, err := os.ReadFile(filepath.Join(root, "article.md")); err != nil || string(data) != "# \u8FDC\u7AEF" {
		t.Fatalf("pull file = %q, %v", data, err)
	}
	if err := os.Remove(filepath.Join(root, "article.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.md"), []byte("# \u672C\u5730"), 0o644); err != nil {
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
	data := []byte("# \u672A\u53D8\u5316")
	if err := os.WriteFile(filepath.Join(root, "same.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	mutatingRequests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/repos/owner/repo":
			body = `{"full_name":"owner/repo","default_branch":"main"}`
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

func TestGitHubPullUsesRepositoryDefaultBranch(t *testing.T) {
	requests := make(map[string]int)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests[request.Method+" "+request.URL.Path]++
		body := ""
		status := http.StatusOK
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/repos/owner/repo":
			body = `{"full_name":"owner/repo","default_branch":"trunk"}`
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/ref/heads/trunk"):
			body = `{"ref":"refs/heads/trunk","object":{"type":"commit","sha":"trunk-commit"}}`
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/git/trees/trunk-commit"):
			body = `{"sha":"tree","tree":[]}`
		default:
			status = http.StatusNotFound
			body = `{"message":"Not Found"}`
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}
	backend, err := newGitHubBackend(Config{Provider: ProviderGitHub, Repository: "owner/repo", Token: "token", Endpoint: "https://api.example.test"}, client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Pull(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if requests["GET /repos/owner/repo/git/ref/heads/trunk"] != 1 || requests["GET /repos/owner/repo/git/trees/trunk-commit"] != 1 {
		t.Fatalf("default branch was not resolved: %#v", requests)
	}
}

func TestGitHubPullReportsInaccessibleRepository(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Status: "Not Found", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"message":"Not Found"}`)), Request: request}, nil
	})}
	backend, err := newGitHubBackend(Config{Provider: ProviderGitHub, Repository: "owner/missing", Token: "secret-token", Endpoint: "https://api.example.test"}, client)
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.Pull(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `github repository "owner/missing" was not found or the token cannot access it`) {
		t.Fatalf("unexpected repository error: %v", err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("repository error leaked the token: %v", err)
	}
}

func TestGitHubPullReportsMissingConfiguredBranch(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		status := http.StatusOK
		body := `{"full_name":"owner/repo","default_branch":"main"}`
		if strings.HasSuffix(request.URL.Path, "/git/ref/heads/missing") {
			status = http.StatusNotFound
			body = `{"message":"Not Found"}`
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}
	backend, err := newGitHubBackend(Config{Provider: ProviderGitHub, Repository: "owner/repo", Branch: "missing", Token: "token", Endpoint: "https://api.example.test"}, client)
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.Pull(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `github branch "missing" was not found in repository "owner/repo"`) {
		t.Fatalf("unexpected branch error: %v", err)
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
