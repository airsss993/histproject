package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const userColumns = `id, 'RHU-' || public_id, email, first_name, last_name, created_at`

func (repository *PostgresRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(repository.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users.users WHERE email = $1`, email))
}

func (repository *PostgresRepository) FindByID(ctx context.Context, id int64) (User, error) {
	return scanUser(repository.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users.users WHERE id = $1`, id))
}

// FindByPublicID ищет по коду без префикса: в базе лежит только "7K3F9Q".
func (repository *PostgresRepository) FindByPublicID(ctx context.Context, code string) (User, error) {
	return scanUser(repository.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users.users WHERE public_id = $1`, code))
}

func (repository *PostgresRepository) Create(ctx context.Context, user User) (User, error) {
	created, err := scanUser(repository.pool.QueryRow(ctx, `
		INSERT INTO users.users (email, first_name, last_name)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns, user.Email, user.FirstName, user.LastName))
	var postgresError *pgconn.PgError
	// ponytail: совпадение public_id (шанс порядка n/10^9) отдаётся 500, а не повтором вставки.
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "users_email_key" {
		return User{}, ErrEmailTaken
	}
	return created, err
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.PublicID, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}
