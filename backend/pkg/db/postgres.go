package db

import (
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB

func ConnDB(dsn string) *sqlx.DB {
	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("ошибка подключения к PostgreSQL: %s", err.Error())
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("ошибка пинга БД: %s", err.Error())
	}

	log.Println("Успешное подключение к БД!")

	DB = db

	return db
}
