package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	ProviderLocal  = "local"
	ProviderGitHub = "github"
	ProviderS3     = "s3"
	ProviderS4     = "s4"
	ProviderOSS    = "oss"
	ProviderSFTP   = "sftp"
)

// Config 是宿主与存储侧车之间稳定的插件配置契约。
type Config struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint,omitempty"`
	Region          string `json:"region,omitempty"`
	Bucket          string `json:"bucket,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	AccessKeySecret string `json:"accessKeySecret,omitempty"`
	SessionToken    string `json:"sessionToken,omitempty"`
	Repository      string `json:"repository,omitempty"`
	Branch          string `json:"branch,omitempty"`
	Token           string `json:"token,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	PrivateKey      string `json:"privateKey,omitempty"`
	KnownHosts      string `json:"knownHosts,omitempty"`
	RemotePath      string `json:"remotePath,omitempty"`
}

type Summary struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

type Backend interface {
	Pull(context.Context, string) (Summary, error)
	Push(context.Context, string) (Summary, error)
}

func New(config Config) (Backend, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	switch config.Provider {
	case ProviderLocal:
		return localBackend{}, nil
	case ProviderGitHub:
		return newGitHubBackend(config)
	case ProviderS3, ProviderS4:
		return newS3Backend(config), nil
	case ProviderOSS:
		return newOSSBackend(config), nil
	case ProviderSFTP:
		return newSFTPBackend(config), nil
	default:
		return nil, fmt.Errorf("unsupported workspace provider %q", config.Provider)
	}
}

func (config Config) Validate() error {
	switch config.Provider {
	case ProviderLocal:
		return nil
	case ProviderGitHub:
		if len(strings.Split(config.Repository, "/")) != 2 || config.Token == "" {
			return errors.New("github repository and token are required")
		}
	case ProviderS3, ProviderS4, ProviderOSS:
		if config.Bucket == "" || config.Region == "" || config.AccessKeyID == "" || config.AccessKeySecret == "" {
			return errors.New("object storage bucket, region and credentials are required")
		}
		if config.Provider == ProviderS4 && config.Endpoint == "" {
			return errors.New("s4 compatible endpoint is required")
		}
	case ProviderSFTP:
		if config.Host == "" || config.Username == "" || config.RemotePath == "" || (config.Password == "" && config.PrivateKey == "") {
			return errors.New("sftp host, username, remote path and authentication are required")
		}
	default:
		return fmt.Errorf("unsupported workspace provider %q", config.Provider)
	}
	return nil
}

type localBackend struct{}

func (localBackend) Pull(context.Context, string) (Summary, error) { return Summary{}, nil }
func (localBackend) Push(context.Context, string) (Summary, error) { return Summary{}, nil }

type localFile struct {
	Path     string
	Relative string
	Size     int64
}

func localFiles(root string) ([]localFile, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local root: %w", err)
	}
	var files []localFile
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == ".git" || entry.Name() == ".mory") {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, localFile{Path: path, Relative: filepath.ToSlash(relative), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk local workspace: %w", err)
	}
	return files, nil
}

func safeLocalPath(root, remoteName string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(remoteName, "/")))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe remote path %q", remoteName)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve local root: %w", err)
	}
	destination := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("remote path escapes workspace %q", remoteName)
	}
	return destination, nil
}

func writeLocalFile(root, remoteName string, data []byte) error {
	destination, err := safeLocalPath(root, remoteName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create local directory: %w", err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		return fmt.Errorf("write local file: %w", err)
	}
	return nil
}

func objectKey(prefix, relative string) string {
	prefix = strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/")
	relative = strings.TrimLeft(strings.ReplaceAll(filepath.ToSlash(relative), "\\", "/"), "/")
	if prefix == "" {
		return relative
	}
	return prefix + "/" + relative
}

func objectRelative(prefix, key string) (string, bool) {
	prefix = strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/")
	key = strings.TrimLeft(key, "/")
	if prefix == "" {
		return key, key != ""
	}
	needle := prefix + "/"
	if !strings.HasPrefix(key, needle) {
		return "", false
	}
	return strings.TrimPrefix(key, needle), true
}
