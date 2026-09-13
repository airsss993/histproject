package admins

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (repository *Repository) Create(ctx context.Context, input CreateRecord) (Admin, error) {
	admin, err := scanAdmin(repository.pool.QueryRow(ctx, `
		INSERT INTO auth.admins (login, password_hash, email, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, login, email, role::TEXT, created_at
	`, input.Login, input.PasswordHash, input.Email, input.Role))
	if isUniqueViolation(err) {
		return Admin{}, ErrLoginTaken
	}
	if err != nil {
		return Admin{}, fmt.Errorf("create admin: %w", err)
	}
	return admin, nil
}

func (repository *Repository) CreateInitialSuperAdmin(ctx context.Context, input CreateRecord) (Admin, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Admin{}, fmt.Errorf("begin super admin bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `LOCK TABLE auth.admins IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return Admin{}, fmt.Errorf("lock admins for bootstrap: %w", err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth.admins WHERE role = 'super_admin')`).Scan(&exists); err != nil {
		return Admin{}, fmt.Errorf("check super admin for bootstrap: %w", err)
	}
	if exists {
		return Admin{}, ErrSuperAdminExists
	}

	admin, err := scanAdmin(tx.QueryRow(ctx, `
		INSERT INTO auth.admins (login, password_hash, email, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, login, email, role::TEXT, created_at
	`, input.Login, input.PasswordHash, input.Email, input.Role))
	if isUniqueViolation(err) {
		return Admin{}, ErrLoginTaken
	}
	if err != nil {
		return Admin{}, fmt.Errorf("create initial super admin: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Admin{}, fmt.Errorf("commit super admin bootstrap: %w", err)
	}
	return admin, nil
}

func (repository *Repository) List(ctx context.Context) ([]Admin, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id, login, email, role::TEXT, created_at
		FROM auth.admins
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}

	admins := make([]Admin, 0)
	for rows.Next() {
		admin, err := scanAdmin(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan admin: %w", err)
		}
		admins = append(admins, admin)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate admins: %w", err)
	}
	rows.Close()
	return admins, nil
}

func (repository *Repository) ChangeRole(ctx context.Context, targetID int64, role Role) (Admin, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Admin{}, fmt.Errorf("begin admin role transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	currentRole, err := lockAdminRole(ctx, tx, targetID)
	if err != nil {
		return Admin{}, err
	}
	if currentRole == RoleSuperAdmin && role != RoleSuperAdmin {
		if err := ensureAnotherSuperAdmin(ctx, tx); err != nil {
			return Admin{}, err
		}
	}

	admin, err := scanAdmin(tx.QueryRow(ctx, `
		UPDATE auth.admins
		SET role = $1
		WHERE id = $2
		RETURNING id, login, email, role::TEXT, created_at
	`, role, targetID))
	if err != nil {
		return Admin{}, fmt.Errorf("change admin role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Admin{}, fmt.Errorf("commit admin role: %w", err)
	}
	return admin, nil
}

func (repository *Repository) Delete(ctx context.Context, targetID int64) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin admin delete transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	currentRole, err := lockAdminRole(ctx, tx, targetID)
	if err != nil {
		return err
	}
	if currentRole == RoleSuperAdmin {
		if err := ensureAnotherSuperAdmin(ctx, tx); err != nil {
			return err
		}
	}

	commandTag, err := tx.Exec(ctx, `DELETE FROM auth.admins WHERE id = $1`, targetID)
	if err != nil {
		return fmt.Errorf("delete admin: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return ErrAdminNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit admin delete: %w", err)
	}
	return nil
}

func (repository *Repository) HasSuperAdmin(ctx context.Context) (bool, error) {
	var exists bool
	if err := repository.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM auth.admins WHERE role = 'super_admin')
	`).Scan(&exists); err != nil {
		return false, fmt.Errorf("check super admin: %w", err)
	}
	return exists, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanAdmin(row rowScanner) (Admin, error) {
	var admin Admin
	var role string
	if err := row.Scan(&admin.ID, &admin.Login, &admin.Email, &role, &admin.CreatedAt); err != nil {
		return Admin{}, err
	}
	admin.Role = Role(role)
	return admin, nil
}

func lockAdminRole(ctx context.Context, tx pgx.Tx, targetID int64) (Role, error) {
	var role string
	err := tx.QueryRow(ctx, `SELECT role::TEXT FROM auth.admins WHERE id = $1 FOR UPDATE`, targetID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAdminNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock admin: %w", err)
	}
	return Role(role), nil
}

func ensureAnotherSuperAdmin(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, `
		SELECT id
		FROM auth.admins
		WHERE role = 'super_admin'
		ORDER BY id
		FOR UPDATE
	`)
	if err != nil {
		return fmt.Errorf("lock super admins: %w", err)
	}
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate super admins: %w", err)
	}
	rows.Close()
	if count <= 1 {
		return ErrLastSuperAdmin
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
