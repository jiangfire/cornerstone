package services

import (
	"context"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func s3ConfigFromEnv() (S3Config, bool) {
	endpoint := os.Getenv("FILE_STORAGE_S3_ENDPOINT")
	bucket := os.Getenv("FILE_STORAGE_S3_BUCKET")
	if endpoint == "" || bucket == "" {
		return S3Config{}, false
	}
	secure := false
	if v := os.Getenv("FILE_STORAGE_S3_SECURE"); v != "" {
		secure, _ = strconv.ParseBool(v)
	}
	return S3Config{
		Endpoint:  endpoint,
		Bucket:    bucket,
		Region:    os.Getenv("FILE_STORAGE_S3_REGION"),
		AccessKey: os.Getenv("FILE_STORAGE_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("FILE_STORAGE_S3_SECRET_KEY"),
		Secure:    secure,
	}, true
}

func TestS3StorageProvider_UploadDownloadDelete(t *testing.T) {
	cfg, ok := s3ConfigFromEnv()
	if !ok {
		t.Skip("S3 not configured, set FILE_STORAGE_S3_ENDPOINT and FILE_STORAGE_S3_BUCKET")
	}

	provider, err := NewS3StorageProvider(cfg)
	require.NoError(t, err)

	content := []byte("hello s3 storage")
	key := "test/s3-upload-test.txt"

	storageKey, err := provider.Upload(context.Background(), key, bytesReader(content), int64(len(content)), "text/plain")
	require.NoError(t, err)
	assert.Equal(t, key, storageKey)

	reader, err := provider.Download(context.Background(), key)
	require.NoError(t, err)

	downloaded, err := io.ReadAll(reader)
	reader.Close()
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)

	err = provider.Delete(context.Background(), key)
	require.NoError(t, err)

	_, err = provider.Download(context.Background(), key)
	assert.Error(t, err)
}

func TestS3StorageProvider_PresignedDownload(t *testing.T) {
	cfg, ok := s3ConfigFromEnv()
	if !ok {
		t.Skip("S3 not configured")
	}

	provider, err := NewS3StorageProvider(cfg)
	require.NoError(t, err)

	content := []byte("presigned test")
	key := "test/presigned-test.txt"

	_, err = provider.Upload(context.Background(), key, bytesReader(content), int64(len(content)), "text/plain")
	require.NoError(t, err)
	defer provider.Delete(context.Background(), key)

	assert.True(t, provider.SupportsPresignedDownload())

	url, err := provider.PresignedDownloadURL(context.Background(), key)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, key)
}

func TestS3StorageProvider_PresignedEmptyKey(t *testing.T) {
	cfg, ok := s3ConfigFromEnv()
	if !ok {
		t.Skip("S3 not configured")
	}

	provider, err := NewS3StorageProvider(cfg)
	require.NoError(t, err)

	_, err = provider.PresignedDownloadURL(context.Background(), "")
	assert.Error(t, err)
}
