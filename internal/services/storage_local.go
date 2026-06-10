package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// fileUploadDir is the root directory for file storage (relative to process working directory).
const fileUploadDir = "./uploads"

// ResolveSecureStoragePath validates that storageURL is within the configured upload directory.
//
// Deprecated: Use LocalStorageProvider.Download() instead for new code.
func ResolveSecureStoragePath(storageURL string) (string, error) {
	p := NewLocalStorageProvider(fileUploadDir)
	return p.resolveSecurePath(storageURL)
}

// LocalStorageProvider stores files on the local filesystem.
type LocalStorageProvider struct {
	dir string
}

// NewLocalStorageProvider creates a LocalStorageProvider rooted at dir.
func NewLocalStorageProvider(dir string) *LocalStorageProvider {
	return &LocalStorageProvider{dir: dir}
}

func (p *LocalStorageProvider) Upload(_ context.Context, key string, reader io.Reader, size int64, _ string) (string, error) {
	if err := os.MkdirAll(p.dir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	filename := filepath.Base(key)
	if filename == "." || filename == "" {
		return "", errors.New("invalid storage key")
	}

	targetPath := filepath.Join(p.dir, filename)

	dirAbs, err := filepath.Abs(p.dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve storage directory: %w", err)
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve target path: %w", err)
	}
	if !strings.HasPrefix(targetAbs, dirAbs) {
		return "", errors.New("illegal file path")
	}

	dst, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, reader); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return targetPath, nil
}

func (p *LocalStorageProvider) Download(_ context.Context, key string) (io.ReadCloser, error) {
	safePath, err := p.resolveStorageKey(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("file not found")
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

func (p *LocalStorageProvider) Delete(_ context.Context, key string) error {
	safePath, err := p.resolveStorageKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(safePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (p *LocalStorageProvider) resolveSecurePath(storageURL string) (string, error) {
	if strings.TrimSpace(storageURL) == "" {
		return "", errors.New("file path is empty")
	}

	dirAbs, err := filepath.Abs(p.dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve storage directory: %w", err)
	}

	targetAbs, err := filepath.Abs(storageURL)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	rel, err := filepath.Rel(dirAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("illegal file path")
	}
	return targetAbs, nil
}

func (p *LocalStorageProvider) resolveStorageKey(key string) (string, error) {
	if filepath.IsAbs(key) {
		return p.resolveSecurePath(key)
	}
	targetPath := filepath.Join(p.dir, filepath.Base(key))
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return abs, nil
}

func (p *LocalStorageProvider) SupportsPresignedDownload() bool {
	return false
}

func (p *LocalStorageProvider) PresignedDownloadURL(_ context.Context, _ string) (string, error) {
	return "", errors.New("presigned download not supported for local storage")
}
