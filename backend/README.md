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

Внешние адреса: API `http://localhost:7666`, Caddy `http://localhost:8080`, MinIO API `http://localhost:9010`, MinIO Console `http://localhost:9011`. PostgreSQL и Redis доступны только на localhost. Buckets `archives` и `sites` создает `minio-init`.

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
- `MAX_UPLOAD_BYTES`: предел размера ZIP-архива в байтах, по умолчанию `20971520` (20 МБ) в development.
- `MAX_EXTRACTED_BYTES`, `MAX_ARCHIVE_FILES`, `MAX_ARCHIVE_FILE_BYTES`: пределы распакованного ZIP, количества записей и одного файла.
- `CHROME_URL`, `SCREENSHOT_TIMEOUT`: адрес headless Chrome и таймаут скриншота.
- `SITE_RENDER_BASE_URL`: внутренний URL bucket `sites`, доступный Chrome.
- `SITE_PREVIEW_BASE_URL`, `SITE_PUBLIC_BASE_URL`: внешние базовые URL preview и опубликованных сайтов.
- `POSTBOX_SMTP_ADDRESS`, `POSTBOX_KEY_ID`, `POSTBOX_SECRET`, `EMAIL_FROM`: отправка email через Yandex Cloud Postbox по SMTP. Логин - идентификатор API-ключа сервисного аккаунта с областью `yc.postbox.send`, пароль - его секретная часть. Домен отправителя должен быть подтверждён DKIM-записями в панели Postbox.
- Telegram-бот уведомлений админам настраивается не env, а в панели (вкладка "Бот" у супер-админа): токен и получатели лежат в базе.
- `ADMIN_PANEL_URL`, `NOTIFICATION_TIMEOUT`: ссылка на панель и timeout внешнего канала.
- `COOKIE_DOMAIN`: domain для auth cookie.
- `CORS_ALLOWED_ORIGINS`: список allowed origins через запятую.

В `production` все адреса, credentials, MinIO bucket names, лимиты архивов, worker URLs, `COOKIE_DOMAIN` и `CORS_ALLOWED_ORIGINS` обязательны. Значения из `.env.example` не подходят для production.

## API

Полный список - в `docs/openapi.yaml`, по группам:

| Префикс `/api/v1` | Что там | Доступ |
|---|---|---|
| `auth/session`, `users/*` | регистрация, подтверждение почты, вход и выход, профиль, согласие на распространение | все, `users/me*` - вошедший |
| `events`, `events/{id}` | события для карты и страницы события | все |
| `events/{id}/star`, `events/{id}/views*` | звёзды и учёт просмотров | звезда - вошедший, просмотры - все |
| `geocode` | поиск места для заявки через Nominatim | все |
| `submissions` | приём заявки с ZIP | вошедший |
| `admin/submissions*`, `admin/events/*` | модерация: review, reject, publish, координаты, удаление | `admin`, `super_admin` |
| `admin/admins*`, `admin/audit-log` | роли и журнал действий | `super_admin` |
| `admin/bot*`, `admin/mail*` | Telegram-бот и почта уведомлений | `super_admin` |

Login ограничен пятью попытками в минуту для пары login и client IP. Public submission form ограничен пятью попытками в час для client IP. IP в ключах Redis хранится как HMAC-SHA256 с ключом в памяти процесса: после рестарта счётчики обнуляются.

`GET /api/v1/events` принимает repeated `eventType`, `dateFrom`, `dateTo`, `page` и `pageSize`. Default page size равен `50`, maximum - `100`. Сортировка стабильная: `eventDate ASC`, затем `id ASC`.

`POST /api/v1/submissions` принимает `multipart/form-data`: `title`, `description`, `eventDate`, `eventType`, `email`, `telegramUsername` и ZIP `archive`. Архив хранится в bucket `archives` под raw UUID key. Queue task идемпотентна по submission ID.

Контракт API находится в `docs/openapi.yaml`.

## Требования к загружаемым сайтам

- Формат архива: `.zip`.
- Максимальный размер: 20 МБ.
- В корне архива должен быть `index.html`.
- Поддерживаются статические сайты без server-side rendering.
- React Router должен использовать `HashRouter`.
- Для Vite нужно указать `base: './'` до сборки.
