package admins

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) FindCredentialsByLogin(ctx context.Context, login string) (int64, string, string, string, error) {
	var adminID int64
	var storedLogin, passwordHash, role string
	err := repository.pool.QueryRow(ctx, `
		SELECT id, login, password_hash, role::TEXT
		FROM auth.admins
		WHERE login = $1
	`, login).Scan(&adminID, &storedLogin, &passwordHash, &role)
	if err != nil {
		return 0, "", "", "", fmt.Errorf("find admin credentials: %w", err)
	}
	return adminID, storedLogin, passwordHash, role, nil
}
