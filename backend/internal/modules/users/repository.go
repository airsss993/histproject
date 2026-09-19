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

const userColumns = `id, email, first_name, last_name, created_at`

func (repository *PostgresRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(repository.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users.users WHERE email = $1`, email))
}

func (repository *PostgresRepository) FindByID(ctx context.Context, id int64) (User, error) {
	return scanUser(repository.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users.users WHERE id = $1`, id))
}

func (repository *PostgresRepository) Create(ctx context.Context, user User) (User, error) {
	created, err := scanUser(repository.pool.QueryRow(ctx, `
		INSERT INTO users.users (email, first_name, last_name)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns, user.Email, user.FirstName, user.LastName))
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return User{}, ErrEmailTaken
	}
	return created, err
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}
