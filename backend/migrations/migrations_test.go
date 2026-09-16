package migrations

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestProviderStartsWithInitialSchema(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://unused")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	provider, err := newProvider(db, embedMigrations)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	sources := provider.ListSources()
	if len(sources) == 0 {
		t.Fatal("no migration sources")
	}
	if !strings.HasSuffix(sources[0].Path, "20260914000100_initial_schema.sql") {
		t.Fatalf("source path = %q, want initial schema migration", sources[0].Path)
	}
}

func TestRunRejectsNilDatabase(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Fatal("Run(nil) error = nil, want error")
	}
}
