package services

import (
	"context"
	"io"
	"sync"
)

// StorageProvider abstracts file storage operations.
// Implementations include local filesystem and S3-compatible object storage.
type StorageProvider interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64) (string, error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	SupportsPresignedDownload() bool
	PresignedDownloadURL(ctx context.Context, key string) (string, error)
}

var (
	defaultStorage StorageProvider
	storageMu      sync.RWMutex
)

// SetDefaultStorageProvider sets the default storage provider for new FileService instances.
func SetDefaultStorageProvider(p StorageProvider) {
	storageMu.Lock()
	defer storageMu.Unlock()
	defaultStorage = p
}

// DefaultStorageProvider returns the current default storage provider.
func DefaultStorageProvider() StorageProvider {
	storageMu.RLock()
	defer storageMu.RUnlock()
	if defaultStorage == nil {
		return NewLocalStorageProvider(fileUploadDir)
	}
	return defaultStorage
}
