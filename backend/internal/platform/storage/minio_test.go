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
	getContext   context.Context
	getBucket    string
	getObject    string
	getResult    *minio.Object
	statContext  context.Context
	statBucket   string
	statObject   string
	statResult   minio.ObjectInfo
	putContext   context.Context
	putBucket    string
	putObject    string
	putSize      int64
	putOptions   minio.PutObjectOptions
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

func (stub *objectClientStub) GetObject(ctx context.Context, bucket, object string, _ minio.GetObjectOptions) (*minio.Object, error) {
	stub.getContext = ctx
	stub.getBucket = bucket
	stub.getObject = object
	return stub.getResult, stub.err
}

func (stub *objectClientStub) StatObject(ctx context.Context, bucket, object string, _ minio.StatObjectOptions) (minio.ObjectInfo, error) {
	stub.statContext = ctx
	stub.statBucket = bucket
	stub.statObject = object
	return stub.statResult, stub.err
}

func (stub *objectClientStub) PutObject(ctx context.Context, bucket, object string, _ io.Reader, size int64, options minio.PutObjectOptions) (minio.UploadInfo, error) {
	stub.putContext = ctx
	stub.putBucket = bucket
	stub.putObject = object
	stub.putSize = size
	stub.putOptions = options
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
		archiveBucket: "archives",
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
		archiveBucket: "archives",
		siteBucket:    "sites",
	}

	err := storage.Ping(context.Background())

	if !errors.Is(err, want) {
		t.Fatalf("Ping() error = %v, want client error", err)
	}
}

func TestMinIOPingChecksLegacyBuckets(t *testing.T) {
	client := &objectClientStub{exists: true}
	storage := &MinIO{objects: client, archiveBucket: "archives", siteBucket: "sites"}

	if err := storage.Ping(t.Context()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if len(client.buckets) != 2 || client.buckets[0] != "archives" || client.buckets[1] != "sites" {
		t.Fatalf("checked buckets = %#v", client.buckets)
	}
}

func TestMinIOPutArchiveForwardsContextAndPreservesObjectKeySemantics(t *testing.T) {
	client := &objectClientStub{exists: true}
	storage := &MinIO{objects: client, archiveBucket: "archives", siteBucket: "sites"}
	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "trace-id")

	objectKey, err := storage.PutArchive(ctx, strings.NewReader("zip"), 3)
	if err != nil {
		t.Fatalf("PutArchive() error = %v", err)
	}
	if client.putContext != ctx || client.putBucket != "archives" || client.putSize != 3 {
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
	storage := &MinIO{objects: client, archiveBucket: "archives", siteBucket: "sites"}
	ctx := t.Context()

	if err := storage.DeleteArchive(ctx, "archive-id"); err != nil {
		t.Fatalf("DeleteArchive() error = %v", err)
	}
	if client.removeCtx != ctx || client.removeBucket != "archives" || client.removeObject != "archive-id" {
		t.Fatalf("remove = context:%v bucket:%q object:%q", client.removeCtx == ctx, client.removeBucket, client.removeObject)
	}
}

func TestMinIOGetArchiveUsesRawKeyAndReturnsSize(t *testing.T) {
	object := &minio.Object{}
	client := &objectClientStub{exists: true, getResult: object, statResult: minio.ObjectInfo{Size: 123}}
	storage := &MinIO{objects: client, archiveBucket: "archives", siteBucket: "sites"}
	ctx := t.Context()

	result, size, err := storage.GetArchive(ctx, "archive-id")
	if err != nil {
		t.Fatalf("GetArchive() error = %v", err)
	}
	if result != object || size != 123 {
		t.Fatalf("GetArchive() = object:%v size:%d", result == object, size)
	}
	if client.statContext != ctx || client.statBucket != "archives" || client.statObject != "archive-id" {
		t.Fatalf("stat = context:%v bucket:%q object:%q", client.statContext == ctx, client.statBucket, client.statObject)
	}
	if client.getContext != ctx || client.getBucket != "archives" || client.getObject != "archive-id" {
		t.Fatalf("get = context:%v bucket:%q object:%q", client.getContext == ctx, client.getBucket, client.getObject)
	}
}

func TestMinIOPutSiteFileUsesLegacyBucketAndContentType(t *testing.T) {
	client := &objectClientStub{exists: true}
	storage := &MinIO{objects: client, archiveBucket: "archives", siteBucket: "sites"}
	ctx := t.Context()

	err := storage.PutSiteFile(ctx, "event/index.html", strings.NewReader("site"), 4, "text/html")
	if err != nil {
		t.Fatalf("PutSiteFile() error = %v", err)
	}
	if client.putContext != ctx || client.putBucket != "sites" || client.putObject != "event/index.html" || client.putSize != 4 {
		t.Fatalf("upload = context:%v bucket:%q object:%q size:%d", client.putContext == ctx, client.putBucket, client.putObject, client.putSize)
	}
	if client.putOptions.ContentType != "text/html" {
		t.Fatalf("ContentType = %q, want text/html", client.putOptions.ContentType)
	}
}

func TestMinIODeleteSiteFileUsesLegacyBucket(t *testing.T) {
	client := &objectClientStub{exists: true}
	storage := &MinIO{objects: client, archiveBucket: "archives", siteBucket: "sites"}
	ctx := t.Context()

	if err := storage.DeleteSiteFile(ctx, "event/index.html"); err != nil {
		t.Fatalf("DeleteSiteFile() error = %v", err)
	}
	if client.removeCtx != ctx || client.removeBucket != "sites" || client.removeObject != "event/index.html" {
		t.Fatalf("remove = context:%v bucket:%q object:%q", client.removeCtx == ctx, client.removeBucket, client.removeObject)
	}
}

func TestOpenDoesNotLeakCredentialsOnInvalidEndpoint(t *testing.T) {
	_, err := Open(context.Background(), Config{
		Endpoint:      "://bad",
		AccessKey:     "access-secret",
		SecretKey:     "secret-value",
		ArchiveBucket: "archives",
		SiteBucket:    "sites",
	})

	if err == nil {
		t.Fatal("Open() error = nil")
	}
	if strings.Contains(err.Error(), "access-secret") || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Open() leaks credentials: %v", err)
	}
}
