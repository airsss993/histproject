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

const userColumns = `id, 'RHU-' || public_id, email, first_name, last_name, created_at,
	EXISTS (SELECT 1 FROM users.consents c WHERE c.user_id = users.id AND c.kind = 'distribution' AND c.revoked_at IS NULL)`

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

// Create заводит учётку. consentVersion - версия согласия на обработку, пустая у учёток без регистрации
// через сайт (модератор, тесты): тогда строки согласия нет.
func (repository *PostgresRepository) Create(ctx context.Context, user User, consentVersion string) (User, error) {
	created, err := scanUser(repository.pool.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO users.users (email, first_name, last_name)
			VALUES ($1, $2, $3)
			RETURNING *
		), consent AS (
			INSERT INTO users.consents (user_id, kind, version)
			SELECT id, 'registration', $4 FROM created WHERE $4 <> ''
		)
		SELECT `+userColumns+` FROM created AS users`, user.Email, user.FirstName, user.LastName, consentVersion))
	var postgresError *pgconn.PgError
	// ponytail: совпадение public_id (шанс порядка n/10^9) отдаётся 500, а не повтором вставки.
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "users_email_key" {
		return User{}, ErrEmailTaken
	}
	return created, err
}

// SetDistribution выдаёт или отзывает согласие на распространение фамилии. Повтор ничего не меняет.
func (repository *PostgresRepository) SetDistribution(ctx context.Context, userID int64, enabled bool, version string) error {
	var err error
	if enabled {
		_, err = repository.pool.Exec(ctx, `
			INSERT INTO users.consents (user_id, kind, version) VALUES ($1, 'distribution', $2)
			ON CONFLICT (user_id, kind) WHERE revoked_at IS NULL DO NOTHING`, userID, version)
	} else {
		_, err = repository.pool.Exec(ctx, `
			UPDATE users.consents SET revoked_at = NOW()
			WHERE user_id = $1 AND kind = 'distribution' AND revoked_at IS NULL`, userID)
	}
	if err != nil {
		return fmt.Errorf("set distribution consent: %w", err)
	}
	return nil
}

// Delete удаляет учётку с согласиями, звёздами и просмотрами. Заявки привязаны к почте и остаются:
// у них своё согласие и свой срок.
func (repository *PostgresRepository) Delete(ctx context.Context, userID int64) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete user: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Без этого просмотры остались бы с user_id = NULL (ON DELETE SET NULL), а надо удалить.
	if _, err := tx.Exec(ctx, `DELETE FROM events.views WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete user views: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users.users WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete user: %w", err)
	}
	return nil
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.PublicID, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt, &user.ShowLastName)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}
