package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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
	GetObject(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error)
	StatObject(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error)
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

func (s *MinIO) GetArchive(ctx context.Context, objectKey string) (*minio.Object, int64, error) {
	info, err := s.objects.StatObject(ctx, s.archiveBucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("stat archive: %w", err)
	}
	object, err := s.objects.GetObject(ctx, s.archiveBucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("get archive: %w", err)
	}
	return object, info.Size, nil
}

func (s *MinIO) PutSiteFile(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	_, err := s.objects.PutObject(ctx, s.siteBucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put site file: %w", err)
	}
	return nil
}

func (s *MinIO) DeleteSiteFile(ctx context.Context, objectKey string) error {
	if err := s.objects.RemoveObject(ctx, s.siteBucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete site file: %w", err)
	}
	return nil
}

// DeleteSite удаляет все файлы сайта заявки: они лежат под префиксом slug/.
func (s *MinIO) DeleteSite(ctx context.Context, slug string) error {
	if slug == "" || strings.Contains(slug, "/") {
		return fmt.Errorf("delete site: bad slug %q", slug)
	}
	objects := s.client.ListObjects(ctx, s.siteBucket, minio.ListObjectsOptions{Prefix: slug + "/", Recursive: true})
	for result := range s.client.RemoveObjects(ctx, s.siteBucket, objects, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			return fmt.Errorf("delete site file %s: %w", result.ObjectName, result.Err)
		}
	}
	return nil
}

func (s *MinIO) Client() *minio.Client {
	return s.client
}

// RewriteSiteHTML прогоняет через rewrite каждый HTML в bucket сайтов и сохраняет те, что он изменил.
// Отдаёт число перезаписанных файлов.
// ponytail: читает весь bucket при каждом вызове; при тысячах сайтов - помечать обработанные.
func (s *MinIO) RewriteSiteHTML(ctx context.Context, isHTML func(string) bool, rewrite func([]byte) ([]byte, bool)) (int, error) {
	rewritten := 0
	for object := range s.client.ListObjects(ctx, s.siteBucket, minio.ListObjectsOptions{Recursive: true}) {
		if object.Err != nil {
			return rewritten, fmt.Errorf("list site files: %w", object.Err)
		}
		if !isHTML(object.Key) {
			continue
		}
		reader, err := s.objects.GetObject(ctx, s.siteBucket, object.Key, minio.GetObjectOptions{})
		if err != nil {
			return rewritten, fmt.Errorf("get site file %s: %w", object.Key, err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return rewritten, fmt.Errorf("read site file %s: %w", object.Key, err)
		}
		updated, changed := rewrite(content)
		if !changed {
			continue
		}
		if err := s.PutSiteFile(ctx, object.Key, bytes.NewReader(updated), int64(len(updated)), "text/html; charset=utf-8"); err != nil {
			return rewritten, err
		}
		rewritten++
	}
	return rewritten, nil
}
