package database

import (
	"context"
	"strings"
	"testing"
)

func TestDatabaseName(t *testing.T) {
	database := &Database{}
	if database.Name() != "postgres" {
		t.Fatalf("Name() = %q, want postgres", database.Name())
	}
}

func TestOpenRejectsInvalidDSN(t *testing.T) {
	database, err := Open(context.Background(), "postgres://%")
	if err == nil {
		database.Close()
		t.Fatal("Open() error = nil")
	}
	if database != nil {
		t.Fatal("Open() database is not nil")
	}
}

func TestOpenHonorsCanceledContextWithoutLeakingDSN(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	const dsn = "postgres://user:secret-value@127.0.0.1:1/histproject?sslmode=disable"

	database, err := Open(ctx, dsn)

	if err == nil {
		database.Close()
		t.Fatal("Open() error = nil")
	}
	if database != nil {
		t.Fatal("Open() database is not nil")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Open() leaks DSN: %v", err)
	}
}
