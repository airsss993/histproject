package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Repository — доступ к данным администраторов.
type Repository interface {
	GetByLogin(ctx context.Context, login string) (*Admin, error)
	GetByID(ctx context.Context, id int) (*Admin, error)
	Create(ctx context.Context, login, passwordHash, role, email string) (bool, error)
	HasSuperAdmin(ctx context.Context) (bool, error)

	Delete(ctx context.Context, id int) error
	ListAll(ctx context.Context) ([]AdminListItem, error)
	UpdateRole(ctx context.Context, id int, role string) error

	CreateSession(ctx context.Context, adminID int, refreshToken string, expiresAt time.Time) error
	GetSessionByToken(ctx context.Context, refreshToken string) (*AdminSession, error)
	DeleteSession(ctx context.Context, refreshToken string) error
}

type repository struct {
	db *sqlx.DB
}

// NewRepository создаёт реализацию Repository для PostgreSQL.
func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// GetByLogin возвращает администратора по логину.
func (r *repository) GetByLogin(ctx context.Context, login string) (*Admin, error) {
	var a Admin
	query := `SELECT id, login, password_hash, role, created_at FROM admins WHERE login = $1`
	err := r.db.GetContext(ctx, &a, query, login)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения админа по логину: %w", err)
	}
	return &a, nil
}

// GetByID возвращает администратора по ID.
func (r *repository) GetByID(ctx context.Context, id int) (*Admin, error) {
	var a Admin
	query := `SELECT id, login, password_hash, role, created_at FROM admins WHERE id = $1`
	err := r.db.GetContext(ctx, &a, query, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения админа по id: %w", err)
	}
	return &a, nil
}

// Create создаёт нового администратора. Возвращает false, если запись не была вставлена (конфликт).
func (r *repository) Create(ctx context.Context, login, passwordHash, role, email string) (bool, error) {
	query := `INSERT INTO admins (login, password_hash, role, email) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`
	res, err := r.db.ExecContext(ctx, query, login, passwordHash, role, email)
	if err != nil {
		return false, fmt.Errorf("ошибка создания админа: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// HasSuperAdmin проверяет, существует ли super_admin в БД.
func (r *repository) HasSuperAdmin(ctx context.Context) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM admins WHERE role = 'super_admin')`
	err := r.db.GetContext(ctx, &exists, query)
	if err != nil {
		return false, fmt.Errorf("ошибка проверки наличия super_admin: %w", err)
	}
	return exists, nil
}

// CreateSession сохраняет refresh-токен сессии в БД.
func (r *repository) CreateSession(ctx context.Context, adminID int, refreshToken string, expiresAt time.Time) error {
	query := `INSERT INTO admin_sessions (admin_id, refresh_token, expires_at) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, adminID, refreshToken, expiresAt)
	if err != nil {
		return fmt.Errorf("ошибка создания сессии: %w", err)
	}
	return nil
}

// GetSessionByToken возвращает сессию по refresh-токену, если она не истекла.
func (r *repository) GetSessionByToken(ctx context.Context, refreshToken string) (*AdminSession, error) {
	var s AdminSession
	query := `SELECT id, admin_id, refresh_token, expires_at, created_at FROM admin_sessions WHERE refresh_token = $1 AND expires_at > NOW()`
	err := r.db.GetContext(ctx, &s, query, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("сессия не найдена: %w", err)
	}
	return &s, nil
}

// ListAll возвращает всех администраторов без хешей паролей.
func (r *repository) ListAll(ctx context.Context) ([]AdminListItem, error) {
	var items []AdminListItem
	query := `SELECT id, login, email, role, created_at FROM admins ORDER BY created_at ASC`
	err := r.db.SelectContext(ctx, &items, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка администраторов: %w", err)
	}
	return items, nil
}

// UpdateRole обновляет роль администратора по ID.
func (r *repository) UpdateRole(ctx context.Context, id int, role string) error {
	query := `UPDATE admins SET role = $1 WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, role, id)
	if err != nil {
		return fmt.Errorf("ошибка обновления роли: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("администратор не найден")
	}
	return nil
}

// Delete удаляет администратора по ID.
func (r *repository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM admins WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("ошибка удаления админа: %w", err)
	}
	return nil
}

// DeleteSession удаляет сессию по refresh-токену.
func (r *repository) DeleteSession(ctx context.Context, refreshToken string) error {
	query := `DELETE FROM admin_sessions WHERE refresh_token = $1`
	_, err := r.db.ExecContext(ctx, query, refreshToken)
	if err != nil {
		return fmt.Errorf("ошибка удаления сессии: %w", err)
	}
	return nil
}
