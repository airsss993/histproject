# Backend Schema Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Подготовить и проверить forward-only миграцию legacy схемы PostgreSQL в модульные schemas без потери rows и ID.

**Architecture:** Legacy migration остается в корне `backend/migrations` и продолжает обслуживать `cmd/main.go`. V2 migration живет в `backend/migrations/v2` и запускается только явной CLI-командой. Новый API не мигрирует БД при startup.

**Tech Stack:** PostgreSQL 18, Goose 3.26, `database/sql` с pgx stdlib только для Goose, `pgx/v5` для integration assertions.

**Spec:** `docs/superpowers/specs/2026-09-13-backend-modular-monolith-design.md`

## Global Constraints

- Не применять V2 migration к `10.3.13.202` без PostgreSQL backup, реестра MinIO keys и рабочего SSH-доступа.
- Не удалять rows, колонки с данными, MinIO objects и bucket.
- Сохранить все ID, sequence values, timestamps, bcrypt hashes, object keys и URL.
- Удалить только cross-module foreign key `objects.request_id -> requests.id`.
- Не подключать V2 migration к legacy startup.
- Не печатать DSN, credentials, password hashes и MinIO object contents.

### Task 1: Изолировать V2 migration runner

**Files:**
- Modify: `backend/migrations/migrations.go`
- Test: `backend/migrations/migrations_test.go`

- [x] Написать failing tests: legacy runner видит только root migration и отклоняет nil database.
- [x] Подтвердить failure.
- [x] Заменить package-global Goose configuration на `goose.NewProvider`.
- [x] Сохранить `Run(*sql.DB)` для legacy entrypoint без изменения набора миграций.
- [x] Оставить V2 runner вне legacy package; он будет добавлен в `backend/migrations/v2` вместе с SQL.
- [x] Запустить package tests и закоммитить как `refactor(backend): изолировать V2 мигации`.

### Task 2: Описать модульную схему

**Files:**
- Create: `backend/migrations/v2/20260913000100_modular_schemas.sql`
- Create: `backend/migrations/v2/migrations.go`
- Test: `backend/migrations/v2/migrations_test.go`

**Target tables:**
- `auth.admins`
- `auth.legacy_sessions`
- `submissions.submissions`
- `submissions.audit_log`
- `events.events`
- `notifications.telegram_subscriptions`

- [ ] Создать schemas `auth`, `submissions`, `events`, `notifications`.
- [ ] Добавить отдельный embedded runner `v2.Run(context.Context, *sql.DB)`, который не импортируется legacy entrypoint.
- [ ] Перенести и переименовать таблицы, enum types, sequences, constraints и indexes без `CREATE TABLE AS` и без копирования rows.
- [ ] Переименовать status values в `processing`, `new`, `in_review`, `rejected`, `published`.
- [ ] Добавить `event_type_code`, backfill `1 -> political`, `2 -> military`, остальные значения -> `legacy_<id>`, затем установить `NOT NULL`.
- [ ] Сделать Down намеренно недоступным: migration forward-only.
- [ ] Закоммитить как `feat(backend): добавить модульную схему БД`.

### Task 3: Доказать сохранность данных

**Files:**
- Create: `backend/migrations/migrations_integration_test.go`

- [ ] Поднять временный PostgreSQL 18 без host volume.
- [ ] Применить legacy migration к пустой БД и вставить связанные rows во все таблицы.
- [ ] Зафиксировать counts, ID, sequence values, hashes, object keys, URL, statuses и event type IDs до V2 migration.
- [ ] Применить V2 migration и проверить те же values в target schemas.
- [ ] Проверить отсутствие public domain tables и cross-module foreign keys.
- [ ] Проверить idempotency runner и отказ Down.
- [ ] Запустить integration test и закоммитить как `test(backend): проверить сохранность миграции`.

### Task 4: Добавить явную migration CLI

**Files:**
- Create: `backend/cmd/migrate/main.go`
- Test: `backend/cmd/migrate/main_test.go`

- [ ] Написать failing tests на отсутствие `DATABASE_URL`, отказ без `--apply` и санитизацию errors.
- [ ] Подтвердить failure.
- [ ] Реализовать одноразовый `go run ./cmd/migrate --apply` с startup timeout.
- [ ] Не подключать CLI к `cmd/api` и `cmd/main.go`.
- [ ] Запустить tests и закоммитить как `feat(backend): добавить migration CLI`.

### Task 5: Runbook и quality gate

**Files:**
- Create: `backend/docs/migration-v2.md`
- Modify: `backend/README.md`

- [ ] Описать backup PostgreSQL, read-only inventory MinIO, dry checks, maintenance window, apply и post-migration checks без значений credentials.
- [ ] Зафиксировать, что live apply заблокирован до восстановления SSH и создания backup.
- [ ] Запустить `gofmt`, `go test -race ./...`, integration test, `go vet ./...`, сборки `cmd/api`, `cmd/main.go`, `cmd/migrate` и `git diff --check`.
- [ ] Закоммитить как `docs(backend): описать безопасную V2 миграцию`.
