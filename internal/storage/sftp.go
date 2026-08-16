package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type sftpBackend struct{ config Config }

func newSFTPBackend(config Config) Backend { return &sftpBackend{config: config} }

func (backend *sftpBackend) Pull(ctx context.Context, root string) (Summary, error) {
	client, closeClient, err := backend.connect(ctx)
	if err != nil {
		return Summary{}, err
	}
	defer closeClient()

	remoteRoot := path.Clean(backend.config.RemotePath)
	walker := client.Walk(remoteRoot)
	var summary Summary
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return summary, fmt.Errorf("walk sftp workspace: %w", err)
		}
		info := walker.Stat()
		if info == nil || !info.Mode().IsRegular() {
			continue
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(walker.Path(), remoteRoot), "/")
		destination, err := safeLocalPath(root, relative)
		if err != nil {
			return summary, err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return summary, fmt.Errorf("create local directory: %w", err)
		}
		remoteFile, err := client.Open(walker.Path())
		if err != nil {
			return summary, fmt.Errorf("open sftp file %q: %w", relative, err)
		}
		localFile, err := os.Create(destination)
		if err != nil {
			remoteFile.Close()
			return summary, fmt.Errorf("create local file %q: %w", relative, err)
		}
		written, copyErr := io.Copy(localFile, remoteFile)
		closeErr := errors.Join(localFile.Close(), remoteFile.Close())
		if copyErr != nil || closeErr != nil {
			return summary, fmt.Errorf("download sftp file %q: %w", relative, errors.Join(copyErr, closeErr))
		}
		summary.Files++
		summary.Bytes += written
	}
	return summary, nil
}

func (backend *sftpBackend) Push(ctx context.Context, root string) (Summary, error) {
	client, closeClient, err := backend.connect(ctx)
	if err != nil {
		return Summary{}, err
	}
	defer closeClient()
	if err := client.MkdirAll(path.Clean(backend.config.RemotePath)); err != nil {
		return Summary{}, fmt.Errorf("create sftp workspace: %w", err)
	}
	files, err := localFiles(root)
	if err != nil {
		return Summary{}, err
	}
	var summary Summary
	for _, file := range files {
		remoteName := path.Join(backend.config.RemotePath, filepath.ToSlash(file.Relative))
		if err := client.MkdirAll(path.Dir(remoteName)); err != nil {
			return summary, fmt.Errorf("create sftp directory: %w", err)
		}
		localFile, err := os.Open(file.Path)
		if err != nil {
			return summary, fmt.Errorf("open local file %q: %w", file.Relative, err)
		}
		remoteFile, err := client.Create(remoteName)
		if err != nil {
			localFile.Close()
			return summary, fmt.Errorf("create sftp file %q: %w", file.Relative, err)
		}
		written, copyErr := io.Copy(remoteFile, localFile)
		closeErr := errors.Join(remoteFile.Close(), localFile.Close())
		if copyErr != nil || closeErr != nil {
			return summary, fmt.Errorf("upload sftp file %q: %w", file.Relative, errors.Join(copyErr, closeErr))
		}
		summary.Files++
		summary.Bytes += written
	}
	return summary, nil
}

func (backend *sftpBackend) connect(ctx context.Context) (*sftp.Client, func(), error) {
	auth, err := backend.authMethod()
	if err != nil {
		return nil, nil, err
	}
	hostKey, err := backend.hostKeyCallback()
	if err != nil {
		return nil, nil, err
	}
	port := backend.config.Port
	if port == 0 {
		port = 22
	}
	address := net.JoinHostPort(backend.config.Host, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: 20 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, fmt.Errorf("connect sftp server: %w", err)
	}
	sshConfig := &ssh.ClientConfig{
		User:            backend.config.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: hostKey,
		Timeout:         20 * time.Second,
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, address, sshConfig)
	if err != nil {
		connection.Close()
		return nil, nil, fmt.Errorf("authenticate sftp server: %w", err)
	}
	sshClient := ssh.NewClient(clientConnection, channels, requests)
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, nil, fmt.Errorf("start sftp session: %w", err)
	}
	return client, func() { client.Close(); sshClient.Close() }, nil
}

func (backend *sftpBackend) authMethod() (ssh.AuthMethod, error) {
	if backend.config.PrivateKey == "" {
		return ssh.Password(backend.config.Password), nil
	}
	keyData := []byte(backend.config.PrivateKey)
	if !strings.Contains(backend.config.PrivateKey, "BEGIN") {
		var err error
		keyData, err = os.ReadFile(backend.config.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("read sftp private key: %w", err)
		}
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse sftp private key: %w", err)
	}
	return ssh.PublicKeys(signer), nil
}

func (backend *sftpBackend) hostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHosts := backend.config.KnownHosts
	if knownHosts == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("locate known_hosts: %w", err)
		}
		knownHosts = filepath.Join(userHome, ".ssh", "known_hosts")
	}
	callback, err := knownhosts.New(knownHosts)
	if err != nil {
		return nil, fmt.Errorf("load sftp known_hosts: %w", err)
	}
	return callback, nil
}
