# Backend Platform Dependencies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Подключить новый API entrypoint к PostgreSQL, Redis и MinIO через явные lifecycle-адаптеры и реальный `/readyz`.

**Architecture:** Каждый adapter владеет своим клиентом, реализует `health.Checker` и закрывает ресурс. `app.Dependencies` создает их последовательно и откатывает уже открытые ресурсы при ошибке. Этап ничего не записывает в PostgreSQL и MinIO.

**Tech Stack:** Go 1.26, `pgx/v5`, `go-redis/v9`, `minio-go/v7`.

**Spec:** `docs/superpowers/specs/2026-09-13-backend-modular-monolith-design.md`

## Global Constraints

- Не запускать миграции и не создавать bucket автоматически.
- Ошибка startup завершает API с кодом 1.
- В логи не попадают DSN, Redis password, MinIO access key и MinIO secret key.
- Каждый startup и health вызов ограничен context timeout.

---

### Task 1: PostgreSQL adapter

**Files:**
- Create: `backend/internal/platform/database/database.go`
- Test: `backend/internal/platform/database/database_test.go`

**Interfaces:**
- Produces: `database.Open(context.Context, string) (*database.Database, error)`.
- Produces: `Name() string`, `Ping(context.Context) error`, `Close()` и `Pool() *pgxpool.Pool`.

- [x] Написать failing tests для имени checker, invalid DSN и canceled startup context.
- [x] Запустить `go test ./internal/platform/database` и подтвердить failure.
- [x] Реализовать adapter через `pgxpool.ParseConfig`, `pgxpool.NewWithConfig` и `Ping`.
- [x] Запустить `go test -race ./internal/platform/database ./...`.
- [x] Закоммитить как `feat(backend): добавить PostgreSQL adapter`.

### Task 2: Redis adapter

**Files:**
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/internal/platform/config/config_test.go`
- Create: `backend/internal/platform/redisstore/redis.go`
- Test: `backend/internal/platform/redisstore/redis_test.go`

**Interfaces:**
- Adds: `config.Config.RedisDB int` и строгий парсинг `REDIS_DB`.
- Produces: `redisstore.Open(context.Context, address, password string, db int) (*redisstore.Store, error)`.
- Produces: `Name() string`, `Ping(context.Context) error`, `Close() error` и `Client() *redis.Client`.

- [x] Написать failing tests для `REDIS_DB`, checker name и canceled startup context.
- [x] Подтвердить failure.
- [x] Реализовать config и adapter через `redis.NewClient` и `Ping`.
- [x] Запустить race-тесты пакетов и всего backend.
- [x] Закоммитить как `feat(backend): добавить Redis adapter`.

### Task 3: MinIO adapter

**Files:**
- Create: `backend/internal/platform/storage/minio.go`
- Test: `backend/internal/platform/storage/minio_test.go`

**Interfaces:**
- Produces: `storage.Open(context.Context, storage.Config) (*storage.MinIO, error)`.
- Produces: `Name() string`, `Ping(context.Context) error` и `Client() *minio.Client`.

- [x] Написать failing tests для checker name, отсутствующего bucket и ошибки health check.
- [x] Подтвердить failure.
- [x] Реализовать adapter через `minio.New` и `BucketExists` без `MakeBucket`.
- [x] Запустить race-тесты пакета и всего backend.
- [x] Закоммитить как `feat(backend): добавить MinIO adapter`.

### Task 4: Dependency lifecycle

**Files:**
- Create: `backend/internal/app/dependencies.go`
- Test: `backend/internal/app/dependencies_test.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/cmd/api/main_test.go`

**Interfaces:**
- Produces: `app.OpenDependencies(context.Context, config.Config) (*app.Dependencies, error)`.
- Produces: `(*app.Dependencies).Checkers() []health.Checker` и `Close() error`.

- [x] Написать failing tests на rollback уже открытых ресурсов, порядок Close и передачу трех checkers.
- [x] Подтвердить failure.
- [x] Реализовать lifecycle dependencies.
- [x] Внедрить opener в тестируемый `run`.
- [x] Запустить race-тесты и сборку `./cmd/api`.
- [x] Закоммитить как `feat(backend): подключить platform dependencies`.

### Task 5: Env contract и общая проверка

**Files:**
- Modify: `backend/.env.example`
- Modify: `backend/README.md`

- [ ] Заменить устаревшие env names на `SERVER_ADDRESS`, `DATABASE_URL`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `MINIO_USE_SSL` и `SHUTDOWN_TIMEOUT`.
- [ ] Описать, что MinIO bucket должен существовать до startup.
- [ ] Запустить `gofmt`, `go test -race ./...`, `go vet ./...`, обе сборки и `git diff --check`.
- [ ] Закоммитить как `docs(backend): описать env нового API`.
