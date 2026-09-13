package v2

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestProviderContainsOnlyModularMigrations(t *testing.T) {
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
	want := []string{
		"20260913000100_modular_schemas.sql",
		"20260913000200_submission_contract.sql",
	}
	if len(sources) != len(want) {
		t.Fatalf("source count = %d, want %d", len(sources), len(want))
	}
	for index, suffix := range want {
		if !strings.HasSuffix(sources[index].Path, suffix) {
			t.Fatalf("source %d path = %q, want suffix %q", index, sources[index].Path, suffix)
		}
	}
}

func TestRunRejectsNilDatabase(t *testing.T) {
	if err := Run(t.Context(), nil); err == nil {
		t.Fatal("Run(nil) error = nil, want error")
	}
}
