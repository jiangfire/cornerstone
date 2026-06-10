package services

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageProvider_UploadDownloadDelete(t *testing.T) {
	dir := t.TempDir()
	provider := NewLocalStorageProvider(dir)

	content := []byte("hello storage world")
	key, err := provider.Upload(context.Background(), "testfile.txt", bytesReader(content), int64(len(content)), "text/plain")
	require.NoError(t, err)
	assert.NotEmpty(t, key)

	fullPath := filepath.Join(dir, "testfile.txt")
	assert.FileExists(t, fullPath)
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	reader, err := provider.Download(context.Background(), key)
	require.NoError(t, err)

	downloaded, err := io.ReadAll(reader)
	reader.Close()
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)

	err = provider.Delete(context.Background(), key)
	require.NoError(t, err)
	assert.NoFileExists(t, fullPath)
}

func TestLocalStorageProvider_SupportsPresignedDownload(t *testing.T) {
	provider := NewLocalStorageProvider(t.TempDir())
	assert.False(t, provider.SupportsPresignedDownload())
}

func TestLocalStorageProvider_PresignedDownloadURL_Errors(t *testing.T) {
	provider := NewLocalStorageProvider(t.TempDir())
	_, err := provider.PresignedDownloadURL(context.Background(), "some-key")
	assert.Error(t, err)
}

func TestLocalStorageProvider_DeleteNonExistent(t *testing.T) {
	provider := NewLocalStorageProvider(t.TempDir())
	err := provider.Delete(context.Background(), "nonexistent-file.txt")
	assert.NoError(t, err)
}

func TestLocalStorageProvider_DownloadNonExistent(t *testing.T) {
	provider := NewLocalStorageProvider(t.TempDir())
	_, err := provider.Download(context.Background(), "nonexistent-file.txt")
	assert.Error(t, err)
}

func bytesReader(b []byte) io.Reader {
	return &byteReader{data: b, pos: 0}
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
