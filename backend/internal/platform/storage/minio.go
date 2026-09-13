package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrBucketNotFound = errors.New("MinIO bucket not found")

type Config struct {
	Endpoint      string
	AccessKey     string
	SecretKey     string
	ArchiveBucket string
	SiteBucket    string
	Secure        bool
}

type objectClient interface {
	BucketExists(context.Context, string) (bool, error)
	PutObject(context.Context, string, string, io.Reader, int64, minio.PutObjectOptions) (minio.UploadInfo, error)
	RemoveObject(context.Context, string, string, minio.RemoveObjectOptions) error
}

type MinIO struct {
	client        *minio.Client
	objects       objectClient
	archiveBucket string
	siteBucket    string
}

func Open(ctx context.Context, cfg Config) (*MinIO, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}

	storage := &MinIO{
		client:        client,
		objects:       client,
		archiveBucket: cfg.ArchiveBucket,
		siteBucket:    cfg.SiteBucket,
	}
	if err := storage.Ping(ctx); err != nil {
		return nil, fmt.Errorf("check MinIO bucket: %w", err)
	}
	return storage, nil
}

func (s *MinIO) Name() string {
	return "minio"
}

func (s *MinIO) Ping(ctx context.Context) error {
	for _, bucket := range []string{s.archiveBucket, s.siteBucket} {
		exists, err := s.objects.BucketExists(ctx, bucket)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: %s", ErrBucketNotFound, bucket)
		}
	}
	return nil
}

func (s *MinIO) PutArchive(ctx context.Context, reader io.Reader, size int64) (string, error) {
	objectKey := uuid.NewString()
	if _, err := s.objects.PutObject(ctx, s.archiveBucket, objectKey, reader, size, minio.PutObjectOptions{}); err != nil {
		return "", fmt.Errorf("put archive: %w", err)
	}
	return objectKey, nil
}

func (s *MinIO) DeleteArchive(ctx context.Context, objectKey string) error {
	if err := s.objects.RemoveObject(ctx, s.archiveBucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete archive: %w", err)
	}
	return nil
}

func (s *MinIO) Client() *minio.Client {
	return s.client
}
