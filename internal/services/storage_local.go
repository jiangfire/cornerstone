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

// ResolveSecureStoragePath resolves storageURL to an absolute path,
// validates it is within fileUploadDir, and returns the safe absolute path or an error.
func ResolveSecureStoragePath(storageURL string) (string, error) {
	if strings.TrimSpace(storageURL) == "" {
		return "", errors.New("file path is empty")
	}
	rootAbs, err := filepath.Abs(fileUploadDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve upload directory: %w", err)
	}
	targetAbs, err := filepath.Abs(storageURL)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("illegal file path")
	}
	return targetAbs, nil
}

// LocalStorageProvider stores files on the local filesystem.
type LocalStorageProvider struct {
	dir string
}

// NewLocalStorageProvider creates a LocalStorageProvider rooted at dir.
func NewLocalStorageProvider(dir string) *LocalStorageProvider {
	return &LocalStorageProvider{dir: dir}
}

func (p *LocalStorageProvider) Upload(_ context.Context, key string, reader io.Reader, size int64) (string, error) {
	if err := os.MkdirAll(p.dir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	filename := filepath.Base(key)
	if filename == "." || filename == "" {
		return "", errors.New("invalid storage key")
	}

	targetPath := filepath.Join(p.dir, filename)

	dirAbs, _ := filepath.Abs(p.dir)
	targetAbs, _ := filepath.Abs(targetPath)
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
	targetPath := filepath.Join(p.dir, filepath.Base(key))
	file, err := os.Open(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("file not found")
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

func (p *LocalStorageProvider) Delete(_ context.Context, key string) error {
	targetPath := filepath.Join(p.dir, filepath.Base(key))
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (p *LocalStorageProvider) SupportsPresignedDownload() bool {
	return false
}

func (p *LocalStorageProvider) PresignedDownloadURL(_ context.Context, _ string) (string, error) {
	return "", errors.New("presigned download not supported for local storage")
}
