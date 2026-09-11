package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"log"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var embedMigrations embed.FS

// Run выполняет все миграции при старте приложения
func Run(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	log.Println("[INFO] Миграции применены успешно!")

	return nil
}
