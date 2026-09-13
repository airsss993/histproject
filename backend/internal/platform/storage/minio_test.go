package storage

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

type objectClientStub struct {
	buckets      []string
	putContext   context.Context
	putBucket    string
	putObject    string
	putSize      int64
	removeCtx    context.Context
	removeBucket string
	removeObject string
	exists       bool
	err          error
}

func (stub *objectClientStub) BucketExists(_ context.Context, bucket string) (bool, error) {
	stub.buckets = append(stub.buckets, bucket)
	return stub.exists, stub.err
}

func (stub *objectClientStub) PutObject(ctx context.Context, bucket, object string, _ io.Reader, size int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	stub.putContext = ctx
	stub.putBucket = bucket
	stub.putObject = object
	stub.putSize = size
	return minio.UploadInfo{}, stub.err
}

func (stub *objectClientStub) RemoveObject(ctx context.Context, bucket, object string, _ minio.RemoveObjectOptions) error {
	stub.removeCtx = ctx
	stub.removeBucket = bucket
	stub.removeObject = object
	return stub.err
}

func TestMinIOName(t *testing.T) {
	storage := &MinIO{}
	if storage.Name() != "minio" {
		t.Fatalf("Name() = %q, want minio", storage.Name())
	}
}

func TestMinIOPingRejectsMissingBucket(t *testing.T) {
	client := &objectClientStub{}
	storage := &MinIO{
		objects:       client,
		archiveBucket: "requests",
		siteBucket:    "sites",
	}

	err := storage.Ping(context.Background())

	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("Ping() error = %v, want ErrBucketNotFound", err)
	}
}

func TestMinIOPingReturnsClientError(t *testing.T) {
	want := errors.New("connection failed")
	client := &objectClientStub{err: want}
	storage := &MinIO{
		objects:       client,
		archiveBucket: "requests",
		siteBucket:    "sites",
	}

	err := storage.Ping(context.Background())

	if !errors.Is(err, want) {
		t.Fatalf("Ping() error = %v, want client error", err)
	}
}

func TestMinIOPingChecksLegacyBuckets(t *testing.T) {
	client := &objectClientStub{exists: true}
	storage := &MinIO{objects: client, archiveBucket: "requests", siteBucket: "sites"}

	if err := storage.Ping(t.Context()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if len(client.buckets) != 2 || client.buckets[0] != "requests" || client.buckets[1] != "sites" {
		t.Fatalf("checked buckets = %#v", client.buckets)
	}
}

func TestMinIOPutArchiveForwardsContextAndPreservesObjectKeySemantics(t *testing.T) {
	client := &objectClientStub{exists: true}
	storage := &MinIO{objects: client, archiveBucket: "requests", siteBucket: "sites"}
	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "trace-id")

	objectKey, err := storage.PutArchive(ctx, strings.NewReader("zip"), 3)
	if err != nil {
		t.Fatalf("PutArchive() error = %v", err)
	}
	if client.putContext != ctx || client.putBucket != "requests" || client.putSize != 3 {
		t.Fatalf("upload = context:%v bucket:%q size:%d", client.putContext == ctx, client.putBucket, client.putSize)
	}
	if objectKey != client.putObject {
		t.Fatalf("object key = %q, uploaded = %q", objectKey, client.putObject)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(objectKey) {
		t.Fatalf("object key = %q, want raw UUID", objectKey)
	}
}

func TestMinIODeleteArchiveUsesLegacyBucket(t *testing.T) {
	client := &objectClientStub{exists: true}
	storage := &MinIO{objects: client, archiveBucket: "requests", siteBucket: "sites"}
	ctx := t.Context()

	if err := storage.DeleteArchive(ctx, "archive-id"); err != nil {
		t.Fatalf("DeleteArchive() error = %v", err)
	}
	if client.removeCtx != ctx || client.removeBucket != "requests" || client.removeObject != "archive-id" {
		t.Fatalf("remove = context:%v bucket:%q object:%q", client.removeCtx == ctx, client.removeBucket, client.removeObject)
	}
}

func TestOpenDoesNotLeakCredentialsOnInvalidEndpoint(t *testing.T) {
	_, err := Open(context.Background(), Config{
		Endpoint:      "://bad",
		AccessKey:     "access-secret",
		SecretKey:     "secret-value",
		ArchiveBucket: "requests",
		SiteBucket:    "sites",
	})

	if err == nil {
		t.Fatal("Open() error = nil")
	}
	if strings.Contains(err.Error(), "access-secret") || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Open() leaks credentials: %v", err)
	}
}
