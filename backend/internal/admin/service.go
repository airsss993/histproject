package admin

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/airsss993/histproject-backend/internal/notifications"
	"github.com/airsss993/histproject-backend/internal/requests"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

// Service — бизнес-логика аутентификации администраторов.
type Service struct {
	repo         Repository
	requestsRepo requests.RequestRepository
	notifSvc     *notifications.Service
	jwtSecret    []byte
}

// NewService создаёт сервис администраторов.
func NewService(repo Repository, requestsRepo requests.RequestRepository, notifSvc *notifications.Service, jwtSecret string) *Service {
	return &Service{repo: repo, requestsRepo: requestsRepo, notifSvc: notifSvc, jwtSecret: []byte(jwtSecret)}
}

// Login проверяет логин/пароль и возвращает пару токенов.
func (s *Service) Login(ctx context.Context, login, password string) (*TokenResponse, error) {
	a, err := s.repo.GetByLogin(ctx, login)
	if err != nil {
		return nil, errors.New("неверный логин или пароль")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("неверный логин или пароль")
	}

	return s.generateTokenPair(ctx, a.ID, a.Role, a.Login)
}

// Refresh валидирует refresh-токен и возвращает новую пару токенов.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	session, err := s.repo.GetSessionByToken(ctx, refreshToken)
	if err != nil {
		return nil, errors.New("невалидный refresh-токен")
	}

	a, err := s.repo.GetByID(ctx, session.AdminID)
	if err != nil {
		return nil, errors.New("администратор не найден")
	}

	if err := s.repo.DeleteSession(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("ошибка ротации токена: %w", err)
	}

	return s.generateTokenPair(ctx, a.ID, a.Role, a.Login)
}

// Logout удаляет сессию по refresh-токену.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.DeleteSession(ctx, refreshToken)
}

// SeedSuperAdmin создаёт super_admin, если его нет в БД, и выводит данные в консоль.
func (s *Service) SeedSuperAdmin(ctx context.Context) error {
	exists, err := s.repo.HasSuperAdmin(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	password, err := generateRandomPassword(16)
	if err != nil {
		return fmt.Errorf("ошибка генерации пароля: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("ошибка хеширования пароля: %w", err)
	}

	login, err := generateAdminLogin()
	if err != nil {
		return fmt.Errorf("ошибка генерации логина: %w", err)
	}
	inserted, err := s.repo.Create(ctx, login, string(hash), "super_admin", "")
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}

	log.Printf("\n========================================\n  SUPER ADMIN СОЗДАН\n  Логин:  %s\n  Пароль: %s\n========================================", login, password)

	return nil
}

// CreateAdmin создаёт нового администратора, генерирует login и password.
func (s *Service) CreateAdmin(ctx context.Context, role, email string) (*CreateAdminResponse, error) {
	login, err := generateAdminLogin()
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации логина: %w", err)
	}

	password, err := generateRandomPassword(16)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации пароля: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("ошибка хеширования пароля: %w", err)
	}

	inserted, err := s.repo.Create(ctx, login, string(hash), role, email)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return nil, errors.New("ошибка создания админа")
	}

	// Отправляем уведомление с учётными данными на почту нового администратора
	if email != "" {
		s.notifSvc.OnAdminCreated(ctx, email, login, password, role)
	}

	return &CreateAdminResponse{Login: login, Password: password}, nil
}

// ListAdmins возвращает список всех администраторов.
func (s *Service) ListAdmins(ctx context.Context) ([]AdminListItem, error) {
	return s.repo.ListAll(ctx)
}

// UpdateAdminRole меняет роль администратора. Нельзя изменить свою собственную роль.
func (s *Service) UpdateAdminRole(ctx context.Context, callerID, targetID int, role string) error {
	// Запрещаем смену роли самому себе
	if callerID == targetID {
		return errors.New("нельзя изменить свою собственную роль")
	}
	return s.repo.UpdateRole(ctx, targetID, role)
}

// DeleteAdmin удаляет администратора по ID. Нельзя удалить самого себя.
func (s *Service) DeleteAdmin(ctx context.Context, callerID, targetID int) error {
	if callerID == targetID {
		return errors.New("нельзя удалить самого себя")
	}

	return s.repo.Delete(ctx, targetID)
}

// generateTokenPair генерирует access-токен и случайный refresh-токен, сохраняя сессию в БД.
func (s *Service) generateTokenPair(ctx context.Context, adminID int, role, login string) (*TokenResponse, error) {
	accessToken, err := s.createAccessToken(adminID, role, login)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации access-токена: %w", err)
	}

	refreshUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации refresh-токена: %w", err)
	}
	refreshToken := refreshUUID.String()

	expiresAt := time.Now().Add(refreshTokenTTL)
	if err := s.repo.CreateSession(ctx, adminID, refreshToken, expiresAt); err != nil {
		return nil, fmt.Errorf("ошибка сохранения сессии: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// createAccessToken создаёт JWT access-токен.
func (s *Service) createAccessToken(adminID int, role, login string) (string, error) {
	claims := jwt.MapClaims{
		"adminId": adminID,
		"role":    role,
		"login":   login,
		"type":    "access",
		"exp":     time.Now().Add(accessTokenTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// parseToken разбирает и валидирует JWT-токен.
func (s *Service) parseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("невалидный токен")
	}

	return claims, nil
}

// generateRandomPassword генерирует случайный пароль заданной длины.
func generateRandomPassword(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range raw {
		raw[i] = charset[int(raw[i])%len(charset)]
	}
	return string(raw), nil
}

// ListRequests возвращает список заявок с опциональными фильтром по статусу, поиском и пагинацией.
func (s *Service) ListRequests(ctx context.Context, status, q string, page, limit int) (*requests.RequestListResult, error) {
	return s.requestsRepo.List(ctx, status, q, page, limit)
}

// GetRequest возвращает полную карточку заявки по ID.
func (s *Service) GetRequest(ctx context.Context, id int) (*requests.RequestDetail, error) {
	return s.requestsRepo.GetByID(ctx, id)
}

// GetRequestHistory возвращает полный аудит-лог действий администраторов по всем заявкам.
func (s *Service) GetRequestHistory(ctx context.Context, q string, page, limit int) (*requests.HistoryResult, error) {
	return s.requestsRepo.GetHistory(ctx, q, page, limit)
}

// generateAdminLogin генерирует логин вида admin_XxXxXx из 8 случайных букв.
func generateAdminLogin() (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return "admin_" + string(b), nil
}
