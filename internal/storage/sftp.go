package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPOptions holds connection settings for an SFTP remote repository.
type SFTPOptions struct {
	Host     string
	Port     int
	User     string
	Password string
	KeyPath  string
	BasePath string
}

// SFTPBackend implements StorageBackend over SSH/SFTP with atomic POSIX file operations.
type SFTPBackend struct {
	opts       SFTPOptions
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// NewSFTPBackend establishes an SSH connection and initializes the SFTP subsystem.
func NewSFTPBackend(opts SFTPOptions) (*SFTPBackend, error) {
	if opts.Port == 0 {
		opts.Port = 22
	}

	var authMethods []ssh.AuthMethod
	if opts.KeyPath != "" {
		keyBytes, err := os.ReadFile(opts.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("sftp: unable to read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("sftp: unable to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if opts.Password != "" {
		authMethods = append(authMethods, ssh.Password(opts.Password))
	}

	sshConfig := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Standard for headless CI/CD runner pools
	}

	addr := net.JoinHostPort(opts.Host, fmt.Sprintf("%d", opts.Port))
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("sftp: ssh dial error: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("sftp: client init error: %w", err)
	}

	return &SFTPBackend{
		opts:       opts,
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}, nil
}

// Close closes the underlying SFTP and SSH clients.
func (s *SFTPBackend) Close() error {
	var errs []string
	if err := s.sftpClient.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := s.sshClient.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("sftp close: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *SFTPBackend) resolve(filePath string) string {
	clean := path.Clean(strings.TrimPrefix(filePath, "/"))
	if s.opts.BasePath == "" {
		return clean
	}
	return path.Join(s.opts.BasePath, clean)
}

func (s *SFTPBackend) Get(ctx context.Context, filePath string) (io.ReadCloser, error) {
	fullPath := s.resolve(filePath)
	file, err := s.sftpClient.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sftp get: %w", err)
	}
	return file, nil
}

func (s *SFTPBackend) Put(ctx context.Context, filePath string, data io.Reader, size int64, contentType string) error {
	fullPath := s.resolve(filePath)
	dir := path.Dir(fullPath)
	if err := s.mkdirAll(dir); err != nil {
		return fmt.Errorf("sftp put mkdir: %w", err)
	}

	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("sftp put rand: %w", err)
	}
	tmpPath := path.Join(dir, fmt.Sprintf(".debpub-tmp-%d-%x", os.Getpid(), randBytes))
	f, err := s.sftpClient.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("sftp put create temp: %w", err)
	}

	if _, err := io.Copy(f, data); err != nil {
		f.Close()
		s.sftpClient.Remove(tmpPath)
		return fmt.Errorf("sftp put copy: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("sftp put close temp: %w", err)
	}

	// Posix rename
	if err := s.sftpClient.PosixRename(tmpPath, fullPath); err != nil {
		// Fallback to Rename if PosixRename extension not supported
		if errFallback := s.sftpClient.Rename(tmpPath, fullPath); errFallback != nil {
			s.sftpClient.Remove(tmpPath)
			return fmt.Errorf("sftp rename: %w", errFallback)
		}
	}

	return nil
}

func (s *SFTPBackend) PutBytes(ctx context.Context, filePath string, data []byte, contentType string) error {
	return s.Put(ctx, filePath, bytes.NewReader(data), int64(len(data)), contentType)
}

func (s *SFTPBackend) Delete(ctx context.Context, filePath string) error {
	fullPath := s.resolve(filePath)
	err := s.sftpClient.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("sftp delete: %w", err)
	}
	return nil
}

func (s *SFTPBackend) Exists(ctx context.Context, filePath string) (bool, error) {
	fullPath := s.resolve(filePath)
	_, err := s.sftpClient.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *SFTPBackend) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := s.resolve(prefix)
	walker := s.sftpClient.Walk(fullPrefix)
	var paths []string

	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}
		if !walker.Stat().IsDir() {
			rel, err := filepath.Rel(s.opts.BasePath, walker.Path())
			if err == nil {
				paths = append(paths, filepath.ToSlash(rel))
			}
		}
	}
	return paths, nil
}

// PutIfNotExist uses SFTP atomic exclusive create (os.O_CREATE | os.O_EXCL).
func (s *SFTPBackend) PutIfNotExist(ctx context.Context, filePath string, data []byte) error {
	fullPath := s.resolve(filePath)
	dir := path.Dir(fullPath)
	if err := s.mkdirAll(dir); err != nil {
		return fmt.Errorf("sftp mkdir: %w", err)
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	f, err := s.sftpClient.OpenFile(fullPath, flags)
	if err != nil {
		if os.IsExist(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("sftp open exclusive: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("sftp write: %w", err)
	}
	return nil
}

func (s *SFTPBackend) mkdirAll(dirPath string) error {
	clean := path.Clean(dirPath)
	if clean == "/" || clean == "." {
		return nil
	}
	parts := strings.Split(clean, "/")
	cur := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		cur = path.Join(cur, p)
		if strings.HasPrefix(clean, "/") {
			cur = "/" + cur
		}
		if err := s.sftpClient.Mkdir(cur); err != nil {
			stat, statErr := s.sftpClient.Stat(cur)
			if statErr != nil || !stat.IsDir() {
				return fmt.Errorf("failed creating directory %s: %w", cur, err)
			}
		}
	}
	return nil
}
