# Backend проекта "Россия - страна Героев"

Серверная часть интерактивной карты исторических объектов. Принимает заявки, обрабатывает загруженные архивы и предоставляет API для администраторов.

## Статус переписывания

Новый API собирается из `cmd/api`. Legacy entrypoint `cmd/main.go` остается до переноса всех модулей и переключения Docker.

```bash
go run ./cmd/api
```

Новый entrypoint подключает PostgreSQL, Redis и MinIO до запуска HTTP-сервера. MinIO bucket из `MINIO_BUCKET` должен существовать заранее. Приложение не создает bucket автоматически.

V2 migration запускается только явной командой `go run ./cmd/migrate --apply`. Перед запуском выполнить `docs/migration-v2.md`. `cmd/api` не запускает migrations автоматически.

## Env нового API

- `APP_ENV`: `development` или `production`.
- `SERVER_ADDRESS`: адрес HTTP-сервера, по умолчанию `:7666`.
- `SHUTDOWN_TIMEOUT`: таймаут graceful shutdown, по умолчанию `10s`.
- `MIGRATION_TIMEOUT`: таймаут migration CLI, по умолчанию `2m`.
- `DATABASE_URL`: PostgreSQL DSN.
- `REDIS_ADDRESS`, `REDIS_PASSWORD`, `REDIS_DB`: параметры Redis.
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `MINIO_USE_SSL`: параметры MinIO.
- `COOKIE_DOMAIN`: domain для auth cookie.
- `CORS_ALLOWED_ORIGINS`: список allowed origins через запятую.

В `production` все адреса, credentials, `COOKIE_DOMAIN` и `CORS_ALLOWED_ORIGINS` обязательны. Значения из `.env.example` не подходят для production.

## API

Документация legacy API находится в `docs/api.html`. Контракт нового API будет добавляться по мере переноса модулей.

## Требования к загружаемым сайтам

- Формат архива: `.zip`.
- Максимальный размер: 50 МБ.
- В корне архива должен быть `index.html`.
- При `CLAMAV_ENABLED=true` legacy API проверяет архив антивирусом.
- Поддерживаются статические сайты без server-side rendering.
- React Router должен использовать `HashRouter`.
- Для Vite нужно указать `base: './'` до сборки.
