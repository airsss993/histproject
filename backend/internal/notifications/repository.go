package notifications

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Repository — получение данных для уведомлений.
type Repository struct {
	db *sqlx.DB
}

// NewRepository создаёт Repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// GetAdminEmails возвращает список email-адресов всех администраторов.
func (r *Repository) GetAdminEmails(ctx context.Context) ([]string, error) {
	var emails []string
	query := `SELECT email FROM admins WHERE email != ''`
	err := r.db.SelectContext(ctx, &emails, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения email-адресов администраторов: %w", err)
	}
	return emails, nil
}
