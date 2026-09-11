package storage

import (
	"context"
	"fmt"

	"github.com/airsss993/histproject-backend/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var Client *MinioClient

type MinioClient struct {
	cfg config.Storage
	mc  *minio.Client
}

// NewMinioClient создает новый экземпляр Minio Client
func NewMinioClient(cfg config.Storage) *MinioClient {
	return &MinioClient{
		cfg: cfg,
	}
}

// InitMinio подключается к Minio и создает бакет, если не существует
func (m *MinioClient) InitMinio() error {
	// Создание контекста с возможностью отмены операции
	ctx := context.Background()

	// Подключение к Minio с использованием имени пользователя и пароля
	client, err := minio.New(m.cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(m.cfg.MinioUsername, m.cfg.MinioPassword, ""),
		Secure: false,
	})
	if err != nil {
		return fmt.Errorf("ошибка подключения к MinIO: %w", err)
	}

	// Установка подключения Minio
	m.mc = client

	// Проверка наличия бакета и его создание, если не существует
	exists, err := m.mc.BucketExists(ctx, RequestsBucketName)
	if err != nil {
		return fmt.Errorf("ошибка проверки наличия бакета requests: %w", err)
	}
	if !exists {
		err := m.mc.MakeBucket(ctx, RequestsBucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("ошибка создания бакета requests: %w", err)
		}
	}

	// Проверка наличия бакета и его создание, если не существует
	exists, err = m.mc.BucketExists(ctx, SitesBucketName)
	if err != nil {
		return fmt.Errorf("ошибка проверки наличия бакета sites: %w", err)
	}
	if !exists {
		err := m.mc.MakeBucket(ctx, SitesBucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("ошибка создания бакета sites: %w", err)
		}
	}

	// Устанавливаем публичную политику для бакета sites
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + SitesBucketName + `/*"]}]}`
	if err := m.mc.SetBucketPolicy(ctx, SitesBucketName, policy); err != nil {
		return fmt.Errorf("ошибка установки политики бакета sites: %w", err)
	}

	return nil
}
