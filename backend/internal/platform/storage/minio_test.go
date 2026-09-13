package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type bucketCheckerFunc func(context.Context, string) (bool, error)

func (f bucketCheckerFunc) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return f(ctx, bucket)
}

func TestMinIOName(t *testing.T) {
	storage := &MinIO{}
	if storage.Name() != "minio" {
		t.Fatalf("Name() = %q, want minio", storage.Name())
	}
}

func TestMinIOPingRejectsMissingBucket(t *testing.T) {
	storage := &MinIO{
		bucket: "histproject",
		checker: bucketCheckerFunc(func(_ context.Context, bucket string) (bool, error) {
			if bucket != "histproject" {
				t.Fatalf("bucket = %q", bucket)
			}
			return false, nil
		}),
	}

	err := storage.Ping(context.Background())

	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("Ping() error = %v, want ErrBucketNotFound", err)
	}
}

func TestMinIOPingReturnsClientError(t *testing.T) {
	want := errors.New("connection failed")
	storage := &MinIO{
		bucket: "histproject",
		checker: bucketCheckerFunc(func(context.Context, string) (bool, error) {
			return false, want
		}),
	}

	err := storage.Ping(context.Background())

	if !errors.Is(err, want) {
		t.Fatalf("Ping() error = %v, want client error", err)
	}
}

func TestOpenDoesNotLeakCredentialsOnInvalidEndpoint(t *testing.T) {
	_, err := Open(context.Background(), Config{
		Endpoint:  "://bad",
		AccessKey: "access-secret",
		SecretKey: "secret-value",
		Bucket:    "histproject",
	})

	if err == nil {
		t.Fatal("Open() error = nil")
	}
	if strings.Contains(err.Error(), "access-secret") || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Open() leaks credentials: %v", err)
	}
}
