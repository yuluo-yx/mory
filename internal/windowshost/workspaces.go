package windowshost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yuluo-yx/mory/internal/storage"
)

// Workspace is the persisted Windows host configuration. Secret fields never return to the frontend.
type Workspace struct {
	storage.Config
	LocalPath  string `json:"localPath,omitempty"`
	IsImplicit bool   `json:"isImplicit,omitempty"`
}

// PublicWorkspace is the redacted workspace configuration exposed to the settings UI.
type PublicWorkspace struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	Provider                  string `json:"provider"`
	LocalPath                 string `json:"localPath"`
	Endpoint                  string `json:"endpoint,omitempty"`
	Region                    string `json:"region,omitempty"`
	Bucket                    string `json:"bucket,omitempty"`
	Prefix                    string `json:"prefix,omitempty"`
	Repository                string `json:"repository,omitempty"`
	Branch                    string `json:"branch,omitempty"`
	Host                      string `json:"host,omitempty"`
	Port                      int    `json:"port,omitempty"`
	Username                  string `json:"username,omitempty"`
	RemotePath                string `json:"remotePath,omitempty"`
	KnownHosts                string `json:"knownHosts,omitempty"`
	IsImplicit                bool   `json:"isImplicit"`
	TokenConfigured           bool   `json:"tokenConfigured"`
	AccessKeySecretConfigured bool   `json:"accessKeySecretConfigured"`
	SessionTokenConfigured    bool   `json:"sessionTokenConfigured"`
	PasswordConfigured        bool   `json:"passwordConfigured"`
	PrivateKeyConfigured      bool   `json:"privateKeyConfigured"`
	AccessKeyIDConfigured     bool   `json:"accessKeyIdConfigured"`
}

// WorkspaceState is the stable data contract consumed by the frontend workspace switcher.
type WorkspaceState struct {
	ActiveID   string            `json:"activeId"`
	Workspaces []PublicWorkspace `json:"workspaces"`
}

type workspaceFile struct {
	Version    int         `json:"version"`
	ActiveID   string      `json:"activeId"`
	Workspaces []Workspace `json:"workspaces"`
}

type workspaceManager struct {
	mu          sync.RWMutex
	configPath  string
	cacheRoot   string
	defaultRoot string
	activeID    string
	workspaces  []Workspace
	newBackend  func(storage.Config) (storage.Backend, error)
}

func newWorkspaceManager(userDataPath, defaultRoot string) *workspaceManager {
	return &workspaceManager{
		configPath:  filepath.Join(userDataPath, "workspaces.json"),
		cacheRoot:   filepath.Join(userDataPath, "workspaces"),
		defaultRoot: defaultRoot,
		newBackend:  storage.New,
	}
}

func (manager *workspaceManager) initialize() error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := os.MkdirAll(manager.cacheRoot, 0o755); err != nil {
		return fmt.Errorf("创建工作区缓存：%w", err)
	}
	data, err := os.ReadFile(manager.configPath)
	if err == nil {
		var stored workspaceFile
		if decodeErr := json.Unmarshal(data, &stored); decodeErr != nil {
			return fmt.Errorf("解析工作区配置：%w", decodeErr)
		}
		manager.workspaces = stored.Workspaces
		manager.activeID = stored.ActiveID
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取工作区配置：%w", err)
	}
	if len(manager.workspaces) == 0 {
		id, idErr := randomID()
		if idErr != nil {
			return idErr
		}
		manager.workspaces = []Workspace{{
			Config:     storage.Config{ID: id, Name: "本地工作区", Provider: storage.ProviderLocal},
			LocalPath:  manager.defaultRoot,
			IsImplicit: true,
		}}
		manager.activeID = id
		if err := manager.persistLocked(); err != nil {
			return err
		}
	}
	if manager.indexLocked(manager.activeID) < 0 {
		manager.activeID = manager.workspaces[0].ID
	}
	return os.MkdirAll(manager.activeRootLocked(), 0o755)
}

func (manager *workspaceManager) state() WorkspaceState {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	result := WorkspaceState{ActiveID: manager.activeID, Workspaces: make([]PublicWorkspace, 0, len(manager.workspaces))}
	for _, workspace := range manager.workspaces {
		result.Workspaces = append(result.Workspaces, manager.publicLocked(workspace))
	}
	return result
}

func (manager *workspaceManager) active() Workspace {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.workspaces[manager.activeIndexLocked()]
}

func (manager *workspaceManager) activeRoot() string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.activeRootLocked()
}

func (manager *workspaceManager) save(input Workspace) (WorkspaceState, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	index := manager.indexLocked(input.ID)
	var existing Workspace
	if index >= 0 {
		existing = manager.workspaces[index]
	} else {
		id, err := randomID()
		if err != nil {
			return WorkspaceState{}, err
		}
		input.ID = id
	}
	preserveSecrets(&input, existing)
	input.IsImplicit = false
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	if input.Provider == "" {
		input.Provider = storage.ProviderLocal
	}
	if strings.TrimSpace(input.Name) == "" {
		if input.Provider == storage.ProviderLocal {
			input.Name = "本地工作区"
		} else {
			input.Name = strings.ToUpper(input.Provider)
		}
	}
	if input.Provider == storage.ProviderLocal {
		if strings.TrimSpace(input.LocalPath) == "" {
			return WorkspaceState{}, errors.New("请选择本地工作目录")
		}
		absolute, err := filepath.Abs(input.LocalPath)
		if err != nil {
			return WorkspaceState{}, fmt.Errorf("解析本地工作目录：%w", err)
		}
		input.LocalPath = absolute
	}
	if err := input.Config.Validate(); err != nil {
		return WorkspaceState{}, localizedStorageError(input.Provider, err)
	}
	if index >= 0 {
		manager.workspaces[index] = input
	} else {
		manager.workspaces = append(manager.workspaces, input)
	}
	manager.activeID = input.ID
	if err := os.MkdirAll(manager.rootLocked(input), 0o755); err != nil {
		return WorkspaceState{}, fmt.Errorf("创建工作目录：%w", err)
	}
	if err := manager.persistLocked(); err != nil {
		return WorkspaceState{}, err
	}
	return manager.stateLocked(), nil
}

func (manager *workspaceManager) activate(id string) (WorkspaceState, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	index := manager.indexLocked(id)
	if index < 0 {
		return WorkspaceState{}, errors.New("工作区不存在")
	}
	manager.activeID = id
	if err := os.MkdirAll(manager.rootLocked(manager.workspaces[index]), 0o755); err != nil {
		return WorkspaceState{}, fmt.Errorf("创建工作目录：%w", err)
	}
	if err := manager.persistLocked(); err != nil {
		return WorkspaceState{}, err
	}
	return manager.stateLocked(), nil
}

func (manager *workspaceManager) remove(id string) (WorkspaceState, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.workspaces) <= 1 {
		return WorkspaceState{}, errors.New("至少保留一个工作区")
	}
	index := manager.indexLocked(id)
	if index < 0 {
		return WorkspaceState{}, errors.New("工作区不存在")
	}
	manager.workspaces = append(manager.workspaces[:index], manager.workspaces[index+1:]...)
	if manager.activeID == id {
		manager.activeID = manager.workspaces[0].ID
	}
	if err := manager.persistLocked(); err != nil {
		return WorkspaceState{}, err
	}
	return manager.stateLocked(), nil
}

func (manager *workspaceManager) syncWorkspace(ctx context.Context, action string) (storage.Summary, error) {
	workspace := manager.active()
	if workspace.Provider == storage.ProviderLocal {
		return storage.Summary{}, nil
	}
	backend, err := manager.newBackend(workspace.Config)
	if err != nil {
		return storage.Summary{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if action == "push" {
		return backend.Push(ctx, manager.activeRoot())
	}
	return backend.Pull(ctx, manager.activeRoot())
}

func (manager *workspaceManager) activeIndexLocked() int {
	index := manager.indexLocked(manager.activeID)
	if index < 0 {
		return 0
	}
	return index
}

func (manager *workspaceManager) indexLocked(id string) int {
	for index := range manager.workspaces {
		if manager.workspaces[index].ID == id {
			return index
		}
	}
	return -1
}

func (manager *workspaceManager) activeRootLocked() string {
	return manager.rootLocked(manager.workspaces[manager.activeIndexLocked()])
}

func (manager *workspaceManager) rootLocked(workspace Workspace) string {
	if workspace.Provider == storage.ProviderLocal {
		return workspace.LocalPath
	}
	return filepath.Join(manager.cacheRoot, workspace.ID)
}

func (manager *workspaceManager) publicLocked(workspace Workspace) PublicWorkspace {
	return PublicWorkspace{
		ID: workspace.ID, Name: workspace.Name, Provider: workspace.Provider, LocalPath: manager.rootLocked(workspace),
		Endpoint: workspace.Endpoint, Region: workspace.Region, Bucket: workspace.Bucket, Prefix: workspace.Prefix,
		Repository: workspace.Repository, Branch: workspace.Branch, Host: workspace.Host, Port: workspace.Port,
		Username: workspace.Username, RemotePath: workspace.RemotePath, KnownHosts: workspace.KnownHosts, IsImplicit: workspace.IsImplicit,
		TokenConfigured: workspace.Token != "", AccessKeyIDConfigured: workspace.AccessKeyID != "",
		AccessKeySecretConfigured: workspace.AccessKeySecret != "", SessionTokenConfigured: workspace.SessionToken != "",
		PasswordConfigured: workspace.Password != "", PrivateKeyConfigured: workspace.PrivateKey != "",
	}
}

func (manager *workspaceManager) stateLocked() WorkspaceState {
	state := WorkspaceState{ActiveID: manager.activeID, Workspaces: make([]PublicWorkspace, 0, len(manager.workspaces))}
	for _, workspace := range manager.workspaces {
		state.Workspaces = append(state.Workspaces, manager.publicLocked(workspace))
	}
	return state
}

func (manager *workspaceManager) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(manager.configPath), 0o755); err != nil {
		return fmt.Errorf("创建配置目录：%w", err)
	}
	data, err := json.MarshalIndent(workspaceFile{Version: 1, ActiveID: manager.activeID, Workspaces: manager.workspaces}, "", "  ")
	if err != nil {
		return fmt.Errorf("编码工作区配置：%w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(manager.configPath, data, 0o600); err != nil {
		return fmt.Errorf("保存工作区配置：%w", err)
	}
	return nil
}

func preserveSecrets(input *Workspace, existing Workspace) {
	if input.Token == "" {
		input.Token = existing.Token
	}
	if input.AccessKeyID == "" {
		input.AccessKeyID = existing.AccessKeyID
	}
	if input.AccessKeySecret == "" {
		input.AccessKeySecret = existing.AccessKeySecret
	}
	if input.SessionToken == "" {
		input.SessionToken = existing.SessionToken
	}
	if input.Password == "" {
		input.Password = existing.Password
	}
	if input.PrivateKey == "" {
		input.PrivateKey = existing.PrivateKey
	}
}

func randomID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("生成工作区标识：%w", err)
	}
	data[6] = data[6]&0x0f | 0x40
	data[8] = data[8]&0x3f | 0x80
	encoded := hex.EncodeToString(data)
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func localizedStorageError(provider string, err error) error {
	switch provider {
	case storage.ProviderGitHub:
		return errors.New("GitHub 工作区需要 owner/repo 格式的仓库和 Access Token")
	case storage.ProviderS3, storage.ProviderS4, storage.ProviderOSS:
		return errors.New("对象存储需要区域、Bucket、Access Key 和 Secret Key；S4 还需要 Endpoint")
	case storage.ProviderSFTP:
		return errors.New("SFTP 需要服务器、用户名、远端目录，以及密码或私钥")
	default:
		return err
	}
}
