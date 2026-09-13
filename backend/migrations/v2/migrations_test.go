package v2

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestProviderContainsOnlyModularSchemaMigration(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://unused")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	provider, err := newProvider(db, embeddedMigrations)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	sources := provider.ListSources()
	if len(sources) != 1 {
		t.Fatalf("source count = %d, want 1", len(sources))
	}
	if !strings.HasSuffix(sources[0].Path, "20260913000100_modular_schemas.sql") {
		t.Fatalf("source path = %q, want modular schema migration", sources[0].Path)
	}
}

func TestRunRejectsNilDatabase(t *testing.T) {
	if err := Run(t.Context(), nil); err == nil {
		t.Fatal("Run(nil) error = nil, want error")
	}
}
