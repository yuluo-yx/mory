package windowshost

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuluo-yx/mory/internal/storage"
)

type fakeStorageBackend struct {
	pulls  int
	pushes int
}

func (backend *fakeStorageBackend) Pull(context.Context, string) (storage.Summary, error) {
	backend.pulls++
	return storage.Summary{Files: 1, Bytes: 2}, nil
}
func (backend *fakeStorageBackend) Push(context.Context, string) (storage.Summary, error) {
	backend.pushes++
	return storage.Summary{Files: 3, Bytes: 4}, nil
}

func TestWorkspaceManagerPersistsAndHidesSecrets(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	root := filepath.Join(t.TempDir(), "notes")
	manager := newWorkspaceManager(data, root)
	if err := manager.initialize(); err != nil {
		t.Fatalf("初始化工作区：%v", err)
	}
	initial := manager.state()
	if len(initial.Workspaces) != 1 || !initial.Workspaces[0].IsImplicit {
		t.Fatalf("默认工作区错误：%#v", initial)
	}

	remote := Workspace{Config: storage.Config{
		Name: "文库", Provider: storage.ProviderGitHub, Repository: "owner/repo", Token: "secret",
	}}
	state, err := manager.save(remote)
	if err != nil {
		t.Fatalf("保存远端工作区：%v", err)
	}
	public := state.Workspaces[len(state.Workspaces)-1]
	if !public.TokenConfigured {
		t.Fatal("前端应只看到令牌已配置标记")
	}
	raw, err := os.ReadFile(filepath.Join(data, "workspaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" {
		t.Fatal("工作区配置未持久化")
	}

	remote.ID = state.ActiveID
	remote.Token = ""
	remote.Repository = "owner/repo"
	if _, err := manager.save(remote); err != nil {
		t.Fatalf("空令牌更新应保留已有凭证：%v", err)
	}
	if manager.active().Token != "secret" {
		t.Fatal("更新工作区时丢失已有令牌")
	}
}

func TestWorkspaceManagerActivationAndRemoval(t *testing.T) {
	t.Parallel()
	manager := newWorkspaceManager(t.TempDir(), filepath.Join(t.TempDir(), "default"))
	if err := manager.initialize(); err != nil {
		t.Fatal(err)
	}
	first := manager.state().ActiveID
	secondRoot := filepath.Join(t.TempDir(), "second")
	state, err := manager.save(Workspace{Config: storage.Config{Name: "Second", Provider: storage.ProviderLocal}, LocalPath: secondRoot})
	if err != nil {
		t.Fatal(err)
	}
	second := state.ActiveID
	if _, err := manager.activate(first); err != nil {
		t.Fatal(err)
	}
	state, err = manager.remove(first)
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveID != second || len(state.Workspaces) != 1 {
		t.Fatalf("删除后的状态错误：%#v", state)
	}
	if _, err := manager.remove(second); err == nil {
		t.Fatal("不应允许删除最后一个工作区")
	}
}

func TestLocalizedStorageErrors(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{storage.ProviderGitHub, storage.ProviderS3, storage.ProviderS4, storage.ProviderOSS, storage.ProviderSFTP, "unknown"} {
		if localizedStorageError(provider, os.ErrInvalid) == nil {
			t.Fatalf("%s 应返回错误", provider)
		}
	}
}

func TestWorkspaceManagerReloadsPersistedConfiguration(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	root := filepath.Join(t.TempDir(), "root")
	first := newWorkspaceManager(data, root)
	if err := first.initialize(); err != nil {
		t.Fatal(err)
	}
	second := newWorkspaceManager(data, root)
	if err := second.initialize(); err != nil {
		t.Fatal(err)
	}
	if second.state().ActiveID != first.state().ActiveID {
		t.Fatal("重启后活动工作区未恢复")
	}

	if err := os.WriteFile(filepath.Join(data, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken := newWorkspaceManager(data, root)
	broken.configPath = filepath.Join(data, "broken.json")
	if err := broken.initialize(); err == nil {
		t.Fatal("损坏配置应返回错误")
	}
}

func TestRemoteWorkspaceSyncDispatchesPullAndPush(t *testing.T) {
	t.Parallel()
	manager := newWorkspaceManager(t.TempDir(), filepath.Join(t.TempDir(), "default"))
	if err := manager.initialize(); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.save(Workspace{Config: storage.Config{
		Name: "Remote", Provider: storage.ProviderGitHub, Repository: "owner/repo", Token: "token",
	}}); err != nil {
		t.Fatal(err)
	}
	backend := &fakeStorageBackend{}
	manager.newBackend = func(storage.Config) (storage.Backend, error) { return backend, nil }
	if summary, err := manager.syncWorkspace(context.Background(), "pull"); err != nil || summary.Files != 1 {
		t.Fatalf("拉取：%#v，%v", summary, err)
	}
	if summary, err := manager.syncWorkspace(context.Background(), "push"); err != nil || summary.Files != 3 {
		t.Fatalf("推送：%#v，%v", summary, err)
	}
	if backend.pulls != 1 || backend.pushes != 1 {
		t.Fatalf("调用次数：%#v", backend)
	}
}

func TestWorkspaceValidationErrors(t *testing.T) {
	t.Parallel()
	manager := newWorkspaceManager(t.TempDir(), filepath.Join(t.TempDir(), "default"))
	if err := manager.initialize(); err != nil {
		t.Fatal(err)
	}
	cases := []Workspace{
		{Config: storage.Config{Name: "bad local", Provider: storage.ProviderLocal}},
		{Config: storage.Config{Name: "bad github", Provider: storage.ProviderGitHub}},
		{Config: storage.Config{Name: "bad s3", Provider: storage.ProviderS3}},
		{Config: storage.Config{Name: "bad sftp", Provider: storage.ProviderSFTP}},
	}
	for _, workspace := range cases {
		if _, err := manager.save(workspace); err == nil {
			t.Fatalf("无效工作区应失败：%#v", workspace)
		}
	}
}
