package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrBucketNotFound = errors.New("MinIO bucket not found")

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool
}

type bucketChecker interface {
	BucketExists(context.Context, string) (bool, error)
}

type MinIO struct {
	client  *minio.Client
	checker bucketChecker
	bucket  string
}

func Open(ctx context.Context, cfg Config) (*MinIO, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}

	storage := &MinIO{client: client, checker: client, bucket: cfg.Bucket}
	if err := storage.Ping(ctx); err != nil {
		return nil, fmt.Errorf("check MinIO bucket: %w", err)
	}
	return storage, nil
}

func (s *MinIO) Name() string {
	return "minio"
}

func (s *MinIO) Ping(ctx context.Context) error {
	exists, err := s.checker.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return ErrBucketNotFound
	}
	return nil
}

func (s *MinIO) Client() *minio.Client {
	return s.client
}
