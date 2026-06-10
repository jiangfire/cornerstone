package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3StorageProvider stores files in an S3-compatible object storage.
type S3StorageProvider struct {
	client     *minio.Client
	bucket     string
	preSignExp time.Duration
}

// S3Config holds configuration for S3 storage.
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Secure    bool
}

// NewS3StorageProvider creates a new S3StorageProvider.
func NewS3StorageProvider(cfg S3Config) (*S3StorageProvider, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	exists, err := client.BucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check S3 bucket: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("S3 bucket %q does not exist", cfg.Bucket)
	}

	return &S3StorageProvider{
		client:     client,
		bucket:     cfg.Bucket,
		preSignExp: time.Hour,
	}, nil
}

func (p *S3StorageProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := p.client.PutObject(ctx, p.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}
	return key, nil
}

func (p *S3StorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := p.client.GetObject(ctx, p.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	return obj, nil
}

func (p *S3StorageProvider) Delete(ctx context.Context, key string) error {
	err := p.client.RemoveObject(ctx, p.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}
	return nil
}

func (p *S3StorageProvider) SupportsPresignedDownload() bool {
	return true
}

func (p *S3StorageProvider) PresignedDownloadURL(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("storage key is empty")
	}
	url, err := p.client.PresignedGetObject(ctx, p.bucket, key, p.preSignExp, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return url.String(), nil
}
