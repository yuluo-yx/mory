package storage

import (
	"context"
	"crypto/sha1" // #nosec G505 -- Git 对象标识规范固定使用 SHA-1，不用于密码学安全。
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
)

const gitHubRequestTimeout = 60 * time.Second

type gitHubBackend struct {
	config Config
	client *github.Client
	owner  string
	repo   string
}

func newGitHubBackend(config Config, clients ...*http.Client) (Backend, error) {
	options := []github.ClientOptionsFunc{
		github.WithAuthToken(config.Token),
		github.WithTimeout(gitHubRequestTimeout),
		github.WithUserAgent("Mory/0.1"),
	}
	if config.Endpoint != "" {
		endpoint := strings.TrimRight(config.Endpoint, "/") + "/"
		options = append(options, github.WithURLs(&endpoint, nil))
	}
	if len(clients) > 0 && clients[0] != nil {
		options = append(options, github.WithHTTPClient(clients[0]))
	}
	client, err := github.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create github client: %w", err)
	}
	owner, repo, _ := strings.Cut(config.Repository, "/")
	return &gitHubBackend{config: config, client: client, owner: owner, repo: repo}, nil
}

func (backend *gitHubBackend) Pull(ctx context.Context, root string) (Summary, error) {
	branch := backend.branch()
	tree, _, err := backend.client.Git.GetTree(ctx, backend.owner, backend.repo, branch, true)
	if err != nil {
		return Summary{}, gitHubAPIError("list github tree", err)
	}
	if tree.GetTruncated() {
		return Summary{}, errors.New("github tree is truncated; narrow the workspace prefix before syncing")
	}

	prefix := strings.Trim(backend.config.Prefix, "/")
	var summary Summary
	for _, item := range tree.Entries {
		if item.GetType() != "blob" {
			continue
		}
		relative, ok := objectRelative(prefix, item.GetPath())
		if !ok {
			continue
		}
		destination, err := safeLocalPath(root, relative)
		if err != nil {
			return summary, err
		}
		if localGitBlobSHA(destination) == item.GetSHA() {
			continue
		}
		data, _, err := backend.client.Git.GetBlobRaw(ctx, backend.owner, backend.repo, item.GetSHA())
		if err != nil {
			return summary, gitHubAPIError(fmt.Sprintf("download github file %q", item.GetPath()), err)
		}
		if err := writeLocalFile(root, relative, data); err != nil {
			return summary, err
		}
		summary.Files++
		summary.Bytes += int64(len(data))
	}
	return summary, nil
}

func (backend *gitHubBackend) Push(ctx context.Context, root string) (Summary, error) {
	files, err := localFiles(root)
	if err != nil {
		return Summary{}, err
	}
	branch := backend.branch()
	reference, _, err := backend.client.Git.GetRef(ctx, backend.owner, backend.repo, "heads/"+branch)
	if err != nil {
		return Summary{}, gitHubAPIError("get github branch", err)
	}
	commitSHA := reference.GetObject().GetSHA()
	baseTree, _, err := backend.client.Git.GetTree(ctx, backend.owner, backend.repo, commitSHA, true)
	if err != nil {
		return Summary{}, gitHubAPIError("get github base tree", err)
	}
	if baseTree.GetTruncated() {
		return Summary{}, errors.New("github tree is truncated; narrow the workspace prefix before syncing")
	}
	remote := make(map[string]string, len(baseTree.Entries))
	for _, item := range baseTree.Entries {
		if item.GetType() == "blob" {
			remote[item.GetPath()] = item.GetSHA()
		}
	}

	entries := make([]*github.TreeEntry, 0, len(files))
	var summary Summary
	for _, file := range files {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return summary, fmt.Errorf("read local file %q: %w", file.Relative, err)
		}
		remotePath := objectKey(backend.config.Prefix, file.Relative)
		if gitBlobSHA(data) == remote[remotePath] {
			continue
		}
		content := base64.StdEncoding.EncodeToString(data)
		encoding := "base64"
		blob, _, err := backend.client.Git.CreateBlob(ctx, backend.owner, backend.repo, github.Blob{
			Content:  &content,
			Encoding: &encoding,
		})
		if err != nil {
			return summary, gitHubAPIError(fmt.Sprintf("upload github blob %q", file.Relative), err)
		}
		mode, kind := "100644", "blob"
		entries = append(entries, &github.TreeEntry{Path: &remotePath, Mode: &mode, Type: &kind, SHA: blob.SHA})
		summary.Files++
		summary.Bytes += int64(len(data))
	}
	if len(entries) == 0 {
		return summary, nil
	}

	newTree, _, err := backend.client.Git.CreateTree(ctx, backend.owner, backend.repo, baseTree.GetSHA(), entries)
	if err != nil {
		return summary, gitHubAPIError("create github tree", err)
	}
	message := fmt.Sprintf("Mory：同步 %d 个文件", summary.Files)
	commit, _, err := backend.client.Git.CreateCommit(ctx, backend.owner, backend.repo, github.Commit{
		Message: &message,
		Tree:    newTree,
		Parents: []*github.Commit{{SHA: &commitSHA}},
	}, nil)
	if err != nil {
		return summary, gitHubAPIError("create github commit", err)
	}
	force := false
	if _, _, err := backend.client.Git.UpdateRef(ctx, backend.owner, backend.repo, "heads/"+branch, github.UpdateRef{
		SHA:   commit.GetSHA(),
		Force: &force,
	}); err != nil {
		return summary, gitHubAPIError("update github branch", err)
	}
	return summary, nil
}

func (backend *gitHubBackend) branch() string {
	if backend.config.Branch == "" {
		return "main"
	}
	return backend.config.Branch
}

func gitBlobSHA(data []byte) string {
	hash := sha1.New() // #nosec G401 -- Git blob ID 明确定义为 SHA-1。
	_, _ = fmt.Fprintf(hash, "blob %d\x00", len(data))
	_, _ = hash.Write(data)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func localGitBlobSHA(name string) string {
	data, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	return gitBlobSHA(data)
}

func gitHubAPIError(action string, err error) error {
	var primary *github.RateLimitError
	if errors.As(err, &primary) {
		return fmt.Errorf("%s: github rate limit reached, reset at %s: %w", action, primary.Rate.Reset.Time.Format(time.RFC3339), err)
	}
	var secondary *github.AbuseRateLimitError
	if errors.As(err, &secondary) {
		return fmt.Errorf("%s: github secondary rate limit reached: %w", action, err)
	}
	return fmt.Errorf("%s: %w", action, err)
}
