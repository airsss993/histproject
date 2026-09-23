package admins

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/airsss993/histproject/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRepositoryFindsIdentityByEmail(t *testing.T) {
	databaseURL := os.Getenv("TEST_ADMINS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_ADMINS_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	db := openTestDatabase(t, ctx, databaseURL)
	resetTestDatabase(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetTestDatabase(t, cleanupCtx, db)
		_ = db.Close()
	})

	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO auth.admins (login, password_hash, role, email)
		VALUES ('root', '$2a$12$test-hash', 'super_admin', 'root@example.test')
	`)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO users.users (email, first_name, last_name) VALUES ('root@example.test','Root',''), ('second@example.test','Second',''), ('third@example.test','Third','')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	adminID, login, _, role, err := repository.FindIdentityByEmail(ctx, "root@example.test")
	if err != nil {
		t.Fatalf("FindIdentityByEmail() error = %v", err)
	}
	if adminID != 1 || login != "root" || role != "super_admin" {
		t.Fatalf("credentials = (%d, %q, %q, %q)", adminID, login, "", role)
	}

	_, _, _, _, err = repository.FindIdentityByEmail(ctx, "missing@example.test")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing credentials error = %v, want pgx.ErrNoRows", err)
	}

	_, err = repository.Create(ctx, CreateRecord{
		Login:        "root",
		PasswordHash: "$2a$12$other-hash",
		Email:        "second@example.test",
		Role:         RoleAdmin,
	})
	if !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("duplicate Create() error = %v, want ErrLoginTaken", err)
	}

	if _, err := repository.ChangeRole(ctx, 1, RoleAdmin); !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("demote last super admin error = %v, want ErrLastSuperAdmin", err)
	}
	if err := repository.Delete(ctx, 1); !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("delete last super admin error = %v, want ErrLastSuperAdmin", err)
	}

	secondSuperAdmin, err := repository.Create(ctx, CreateRecord{
		Login:        "second-root",
		PasswordHash: "$2a$12$second-hash",
		Email:        "second@example.test",
		Role:         RoleSuperAdmin,
	})
	if err != nil {
		t.Fatalf("create second super admin: %v", err)
	}
	changedAdmin, err := repository.ChangeRole(ctx, 1, RoleAdmin)
	if err != nil {
		t.Fatalf("demote first super admin: %v", err)
	}
	if changedAdmin.Role != RoleAdmin {
		t.Fatalf("changed role = %q, want admin", changedAdmin.Role)
	}
	if err := repository.Delete(ctx, secondSuperAdmin.ID); !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("delete remaining super admin error = %v, want ErrLastSuperAdmin", err)
	}

	thirdSuperAdmin, err := repository.Create(ctx, CreateRecord{
		Login:        "third-root",
		PasswordHash: "$2a$12$third-hash",
		Email:        "third@example.test",
		Role:         RoleSuperAdmin,
	})
	if err != nil {
		t.Fatalf("create third super admin: %v", err)
	}
	if err := repository.Delete(ctx, secondSuperAdmin.ID); err != nil {
		t.Fatalf("delete super admin with replacement: %v", err)
	}

	name := "Анна"
	updated, err := repository.Update(ctx, 1, UpdateRecord{Name: &name})
	if err != nil || updated.Name != "Анна" || updated.Email != "root@example.test" {
		t.Fatalf("Update() name only = %#v, %v", updated, err)
	}
	email := "third@example.test"
	if _, err := repository.Update(ctx, 1, UpdateRecord{Email: &email}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("Update() duplicate email = %v", err)
	}
	email = "second@example.test"
	if _, err := repository.Update(ctx, 1, UpdateRecord{Email: &email}); err != nil {
		t.Fatalf("Update() email and password: %v", err)
	}
	if got, err := repository.Get(ctx, 1); err != nil || got.Name != "Анна" || got.Email != email {
		t.Fatalf("Get() after update = %#v, %v", got, err)
	}
	if _, err := repository.Update(ctx, 999, UpdateRecord{Name: &name}); !errors.Is(err, ErrAdminNotFound) {
		t.Fatalf("Update() missing admin error = %v, want ErrAdminNotFound", err)
	}

	admins, err := repository.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(admins) != 2 || admins[0].ID != 1 || admins[1].ID != thirdSuperAdmin.ID {
		t.Fatalf("admins after changes = %#v", admins)
	}
}

func TestRepositoryCreatesOnlyOneInitialSuperAdmin(t *testing.T) {
	databaseURL := os.Getenv("TEST_ADMINS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_ADMINS_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	db := openTestDatabase(t, ctx, databaseURL)
	resetTestDatabase(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetTestDatabase(t, cleanupCtx, db)
		_ = db.Close()
	})

	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)

	admin, err := repository.CreateInitialSuperAdmin(ctx, CreateRecord{
		Login:        "root",
		PasswordHash: "$2a$12$first-hash",
		Email:        "root@example.test",
		Role:         RoleSuperAdmin,
	})
	if err != nil {
		t.Fatalf("CreateInitialSuperAdmin() error = %v", err)
	}
	if admin.ID != 1 || admin.Role != RoleSuperAdmin {
		t.Fatalf("initial super admin = %#v", admin)
	}

	_, err = repository.CreateInitialSuperAdmin(ctx, CreateRecord{
		Login:        "second-root",
		PasswordHash: "$2a$12$second-hash",
		Role:         RoleSuperAdmin,
	})
	if !errors.Is(err, ErrSuperAdminExists) {
		t.Fatalf("second CreateInitialSuperAdmin() error = %v, want ErrSuperAdminExists", err)
	}
}

func openTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		_ = db.Close()
		t.Fatalf("read database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_test") {
		_ = db.Close()
		t.Fatalf("database %q is not an isolated test database", databaseName)
	}
	return db
}

func resetTestDatabase(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		DROP SCHEMA IF EXISTS auth CASCADE;
		DROP SCHEMA IF EXISTS submissions CASCADE;
		DROP SCHEMA IF EXISTS events CASCADE;
		DROP SCHEMA IF EXISTS notifications CASCADE;
		DROP SCHEMA IF EXISTS users CASCADE;
		DROP FUNCTION IF EXISTS public.rh_code();
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`)
	if err != nil {
		t.Fatalf("reset test database: %v", err)
	}
}
