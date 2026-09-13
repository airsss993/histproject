# Backend проекта "Россия - страна Героев"

Серверная часть интерактивной карты исторических объектов. Принимает заявки, обрабатывает загруженные архивы и предоставляет API для администраторов.

## Статус переписывания

Новый API собирается из `cmd/api`. Legacy entrypoint `cmd/main.go` остается до переноса всех модулей и переключения Docker.

```bash
go run ./cmd/api
```

Новый entrypoint подключает PostgreSQL, Redis и MinIO до запуска HTTP-сервера. Legacy buckets `requests` и `sites`, заданные через `MINIO_ARCHIVE_BUCKET` и `MINIO_SITE_BUCKET`, должны существовать заранее. Приложение не создает buckets автоматически.

Обработка ZIP и создание скриншотов запускаются отдельным процессом:

```bash
go run ./cmd/worker
```

V2 migration запускается только явной командой `go run ./cmd/migrate --apply`. Перед запуском выполнить `docs/migration-v2.md`. `cmd/api` не запускает migrations автоматически.

Первый `super_admin` создается только после V2 migration:

```bash
DATABASE_URL='postgres://...' \
BOOTSTRAP_ADMIN_LOGIN='root' \
BOOTSTRAP_ADMIN_PASSWORD='use-a-password-manager' \
BOOTSTRAP_ADMIN_EMAIL='root@example.com' \
go run ./cmd/bootstrap-admin --apply
```

Команда откажется запускаться без `--apply` и не создасть второго `super_admin`. Password и PostgreSQL DSN в output не печатаются.

## Env нового API

- `APP_ENV`: `development` или `production`.
- `SERVER_ADDRESS`: адрес HTTP-сервера, по умолчанию `:7666`.
- `SHUTDOWN_TIMEOUT`: таймаут graceful shutdown, по умолчанию `10s`.
- `MIGRATION_TIMEOUT`: таймаут migration CLI, по умолчанию `2m`.
- `SESSION_TTL`: TTL Redis session и auth cookie, по умолчанию `12h`.
- `DATABASE_URL`: PostgreSQL DSN.
- `REDIS_ADDRESS`, `REDIS_PASSWORD`, `REDIS_DB`: параметры Redis.
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_ARCHIVE_BUCKET`, `MINIO_SITE_BUCKET`, `MINIO_USE_SSL`: параметры MinIO.
- `MAX_UPLOAD_BYTES`: предел размера ZIP-архива в байтах, по умолчанию `52428800` в development.
- `MAX_EXTRACTED_BYTES`, `MAX_ARCHIVE_FILES`, `MAX_ARCHIVE_FILE_BYTES`: пределы распакованного ZIP, количества записей и одного файла.
- `CHROME_URL`, `SCREENSHOT_TIMEOUT`: адрес headless Chrome и таймаут скриншота.
- `SITE_RENDER_BASE_URL`: внутренний URL bucket `sites`, доступный Chrome.
- `SITE_PREVIEW_BASE_URL`, `SITE_PUBLIC_BASE_URL`: внешние базовые URL preview и опубликованных сайтов.
- `RESEND_API_KEY`, `EMAIL_FROM`: отправка email через Resend.
- `TG_BOT_TOKEN`, `TG_ADMIN_CHAT_ID`: уведомления администраторов через Telegram.
- `ADMIN_PANEL_URL`, `NOTIFICATION_TIMEOUT`: ссылка на панель и timeout внешнего канала.
- `COOKIE_DOMAIN`: domain для auth cookie.
- `CORS_ALLOWED_ORIGINS`: список allowed origins через запятую.

В `production` все адреса, credentials, MinIO bucket names, лимиты архивов, worker URLs, `COOKIE_DOMAIN` и `CORS_ALLOWED_ORIGINS` обязательны. Значения из `.env.example` не подходят для production.

## API

Новый API сейчас предоставляет:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/session`
- `GET /api/v1/events`
- `GET /api/v1/events/{id}`
- `POST /api/v1/submissions`
- `GET /api/v1/admin/admins`
- `POST /api/v1/admin/admins`
- `PATCH /api/v1/admin/admins/{id}/role`
- `DELETE /api/v1/admin/admins/{id}`
- `PATCH /api/v1/admin/events/{id}/coordinates`
- `DELETE /api/v1/admin/events/{id}`
- `GET /api/v1/admin/submissions`
- `GET /api/v1/admin/submissions/{id}`
- `POST /api/v1/admin/submissions/{id}/review`
- `POST /api/v1/admin/submissions/{id}/reject`
- `POST /api/v1/admin/submissions/{id}/publish`
- `GET /api/v1/admin/audit-log`

Admins routes требуют session с ролью `super_admin`. Events и submissions moderation routes доступны ролям `admin` и `super_admin`. Submissions audit доступен только `super_admin`.

Login ограничен пятью попытками в минуту для пары login и client IP. Public submission form ограничен пятью попытками в час для client IP. Redis keys содержат только SHA-256 digests, без login, email и Telegram username.

`GET /api/v1/events` принимает repeated `eventType`, `dateFrom`, `dateTo`, `page` и `pageSize`. Default page size равен `50`, maximum - `100`. Сортировка стабильная: `eventDate ASC`, затем `id ASC`.

`POST /api/v1/submissions` принимает `multipart/form-data`: `title`, `description`, `eventDate`, `eventType`, `email`, `telegramUsername` и ZIP `archive`. Архив хранится в bucket `requests` под raw UUID key. Queue task идемпотентна по submission ID.

Документация legacy API находится в `docs/api.html`. OpenAPI нового API будет добавлен после переноса всех модулей.

## Требования к загружаемым сайтам

- Формат архива: `.zip`.
- Максимальный размер: 50 МБ.
- В корне архива должен быть `index.html`.
- При `CLAMAV_ENABLED=true` legacy API проверяет архив антивирусом.
- Поддерживаются статические сайты без server-side rendering.
- React Router должен использовать `HashRouter`.
- Для Vite нужно указать `base: './'` до сборки.
