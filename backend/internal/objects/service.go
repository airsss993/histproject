package objects

import "context"

// SiteStorage — удаление файлов сайта из хранилища.
type SiteStorage interface {
	DeleteSiteFolder(requestID int) error
}

// Service — бизнес-логика объектов и типов событий.
type Service struct {
	repo    Repository
	storage SiteStorage
}

// NewService создаёт сервис объектов.
func NewService(repo Repository, storage SiteStorage) *Service {
	return &Service{repo: repo, storage: storage}
}

// GetObjectData возвращает данные одного объекта по ID.
func (s *Service) GetObjectData(ctx context.Context, id int) (*SingleObjectInfo, error) {
	return s.repo.GetByID(ctx, id)
}

// GetObjectsList возвращает список объектов с фильтрами.
func (s *Service) GetObjectsList(ctx context.Context, eventTypeIDs []int, dateFrom, dateTo string) ([]ObjectInfo, error) {
	return s.repo.List(ctx, eventTypeIDs, dateFrom, dateTo)
}

// DeleteObject удаляет объект (метку) с карты и файлы сайта из хранилища.
func (s *Service) DeleteObject(ctx context.Context, requestID int) error {
	// Удаляем запись из БД
	if err := s.repo.DeleteObject(ctx, requestID); err != nil {
		return err
	}

	// Удаляем файлы сайта из хранилища
	_ = s.storage.DeleteSiteFolder(requestID)

	return nil
}

// UpdateCoordinates обновляет координаты метки по ID заявки.
func (s *Service) UpdateCoordinates(ctx context.Context, requestID int, latitude, longitude float64) error {
	return s.repo.UpdateCoordinates(ctx, requestID, latitude, longitude)
}

// IsPublished проверяет, опубликован ли сайт заявки (есть ли объект на карте).
func (s *Service) IsPublished(ctx context.Context, slug string) (bool, error) {
	return s.repo.IsPublished(ctx, slug)
}

