# Backend проекта "Россия - страна Героев"

Серверная часть интерактивной карты исторических объектов. Принимает заявки, обрабатывает загруженные архивы и предоставляет API для администраторов.

## Запуск

Скопировать `.env.example` в `.env`, заменить тестовые credentials и запустить development stack:

```bash
docker compose -f docker-compose.dev.yml build
docker compose -f docker-compose.dev.yml up -d db redis minio chrome
docker compose -f docker-compose.dev.yml run --rm migrate
docker compose -f docker-compose.dev.yml run --rm bootstrap-admin
docker compose -f docker-compose.dev.yml up -d
```

Внешние адреса: API `http://localhost:7666`, Caddy `http://localhost:8080`, MinIO API `http://localhost:9010`, MinIO Console `http://localhost:9011`. PostgreSQL и Redis доступны только на localhost. Buckets `requests` и `sites` создает `minio-init`.

Без Docker API и worker запускаются отдельно. Для этого заменить в `.env` container hostnames `db`, `redis`, `minio` и `chrome` на `localhost`.

```bash
go run ./cmd/api
go run ./cmd/worker
```

Проект рассчитан на чистую PostgreSQL database. Migration запускается только явно и не импортирует схему старого backend.

```bash
docker compose -f docker-compose.prod.yml --profile tools run --rm migrate
docker compose -f docker-compose.prod.yml --profile tools run --rm bootstrap-admin
```

Первый `super_admin` создается только после migration:

```bash
DATABASE_URL='postgres://...' \
BOOTSTRAP_ADMIN_LOGIN='root' \
BOOTSTRAP_ADMIN_PASSWORD='use-a-password-manager' \
BOOTSTRAP_ADMIN_EMAIL='root@example.com' \
go run ./cmd/bootstrap-admin --apply
```

Команда откажется запускаться без `--apply` и не создаст второго `super_admin`. Password и PostgreSQL DSN в output не печатаются.

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

API предоставляет:

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

Контракт API находится в `docs/openapi.yaml`.

## Требования к загружаемым сайтам

- Формат архива: `.zip`.
- Максимальный размер: 50 МБ.
- В корне архива должен быть `index.html`.
- Поддерживаются статические сайты без server-side rendering.
- React Router должен использовать `HashRouter`.
- Для Vite нужно указать `base: './'` до сборки.
