package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

const (
	SitesBucketName    = "sites"
	RequestsBucketName = "requests"
)

// UploadArchive - функция загрузки архива в бакет MinIO
func (m *MinioClient) UploadArchive(file *multipart.FileHeader) (string, error) {
	objectId := uuid.New().String()

	fileReader, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("ошибка чтения файла: %w", err)
	}
	defer fileReader.Close()

	// Загрузка файла в бакет
	_, err = m.mc.PutObject(context.Background(), RequestsBucketName, objectId, fileReader, file.Size, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("ошибка при загрузки файла в бакет %s: %v", file.Filename, err)
	}

	// Возвращаем ID объекта
	return objectId, nil
}

// GetArchive - функция получения архива из бакета MinIO по ID объекта
func (m *MinioClient) GetArchive(archiveId string) (*minio.Object, int64, error) {
	// Загрузка файла в бакет
	object, err := m.mc.GetObject(context.Background(), RequestsBucketName, archiveId, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка при получении архива %s с бакета: %v", archiveId, err)
	}

	// Получаем информацию об объекте, чтобы узнать его размер
	objectInfo, err := object.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка при получении информации об объекте %s: %v", archiveId, err)
	}

	// Возвращаем полученный объект и его размер
	return object, objectInfo.Size, nil
}

// UploadFileSites - функция для загрузки файла в бакет сайтов
func (m *MinioClient) UploadFileSites(reader io.Reader, size int64, key string) error {
	contentType := mime.TypeByExtension(filepath.Ext(key))

	// Загрузка файла в бакет
	_, err := m.mc.PutObject(context.Background(), SitesBucketName, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("ошибка при загрузки файла в бакет: %v", err)
	}

	return nil
}

// DeleteSiteFolder удаляет все файлы папки {requestID}/ из бакета sites.
func (m *MinioClient) DeleteSiteFolder(requestID int) error {
	ctx := context.Background()
	prefix := fmt.Sprintf("%d/", requestID)

	// Собираем ключи всех объектов с нужным префиксом
	objectsCh := m.mc.ListObjects(ctx, SitesBucketName, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})

	var removeErr error
	for object := range objectsCh {
		if object.Err != nil {
			return fmt.Errorf("ошибка перечисления файлов сайта: %w", object.Err)
		}
		if err := m.mc.RemoveObject(ctx, SitesBucketName, object.Key, minio.RemoveObjectOptions{}); err != nil {
			removeErr = fmt.Errorf("ошибка удаления файла %s: %w", object.Key, err)
		}
	}

	return removeErr
}
