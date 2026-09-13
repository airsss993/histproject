# Backend Submissions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Перенести прием, просмотр, аудит и модерацию заявок в модуль `submissions` нового API без потери PostgreSQL и MinIO data.

**Architecture:** `submissions` владеет `submissions.submissions` и `submissions.audit_log`. State transitions и audit выполняются одной PostgreSQL transaction с row lock. Archive storage, queue, event publication и notifications подключаются через узкие interfaces, объявленные в `submissions`.

**Tech Stack:** `net/http`, `http.ServeMux`, `pgx/v5`, PostgreSQL 18, MinIO SDK, Asynq, Redis 8, `mime/multipart`.

**Spec:** `docs/superpowers/specs/2026-09-13-backend-modular-monolith-design.md`

## Global Constraints

- Статусы: `processing`, `new`, `review`, `rejected`, `published`.
- Allowed transitions: `processing -> new`, `processing -> rejected`, `new -> review`, `review -> rejected`, `review -> published`.
- Review, reject и publish пишут audit в той же transaction, что и status update.
- Publish передает event snapshot без SQL JOIN между modules.
- Event publication и job enqueue идемпотентны по submission ID.
- Existing IDs, archive IDs, slugs, site URLs, screenshot URLs и MinIO object keys не меняются.
- Public form принимает только ZIP и ограничивает request body до парсинга multipart.
- Тип события передается стабильным `eventType`, а не integer ID.
- Notification failure не откатывает завершенную transaction.
- Client не получает SQL, MinIO, Redis и internal error details.

---

### Task 1: V3 status и idempotency migration

**Files:**
- Create: `backend/migrations/v2/20260913000200_submission_contract.sql`
- Modify: `backend/migrations/v2/migrations_integration_test.go`

- [x] Написать failing migration tests на `in_review -> review`, duplicate `submission_id` guard и preserved rows.
- [x] Подтвердить failure на PostgreSQL 18.
- [x] Добавить forward-only migration, которая переименует enum value и создает unique index `events_submission_id_key`.
- [x] Проверить row counts, IDs, sequences, URLs и повторный `Run`.
- [x] Закоммитить как `feat(backend): добавить submissions contract migration`.

### Task 2: Submission model и state machine

**Files:**
- Create: `backend/internal/modules/submissions/model.go`
- Create: `backend/internal/modules/submissions/service.go`
- Test: `backend/internal/modules/submissions/service_test.go`

- [x] Написать failing tests на moderation input validation, transition matrix, required reject comment, coordinates и event snapshot.
- [x] Подтвердить expected failure.
- [x] Объявить consumer interfaces `ArchiveStore`, `JobQueue`, `EventPublisher`, `Notifier` и typed errors.
- [x] Реализовать state machine без hidden side effects и cross-module imports.
- [x] Запустить race tests и закоммитить как `feat(backend): добавить submissions core`.

### Task 3: PostgreSQL submissions repository

**Files:**
- Create: `backend/internal/modules/submissions/repository.go`
- Test: `backend/internal/modules/submissions/repository_integration_test.go`

- [ ] Написать failing PostgreSQL 18 tests на create, list, detail, status filters, search, pagination и audit list.
- [ ] Добавить concurrent transition tests с `FOR UPDATE` и exact previous status condition.
- [ ] Реализовать status update и audit insert в одной transaction.
- [ ] Исключить cross-schema JOIN; audit title получать из submissions schema.
- [ ] Запустить race и integration tests, закоммитить как `feat(backend): добавить submissions repository`.

### Task 4: Archive intake и queue adapters

**Files:**
- Modify: `backend/internal/platform/storage/minio.go`
- Test: `backend/internal/platform/storage/minio_test.go`
- Create: `backend/internal/platform/queue/client.go`
- Test: `backend/internal/platform/queue/client_test.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/internal/platform/config/config_test.go`
- Modify: `backend/.env.example`

- [ ] Добавить failing tests на context-aware archive upload, preserved object key, compensation delete и unique Asynq task ID.
- [ ] Добавить `MAX_UPLOAD_BYTES` с production validation и safe default `52428800`.
- [ ] Реализовать adapters без global clients и `context.Background()`.
- [ ] Не менять existing archive bucket и object key semantics.
- [ ] Запустить race tests и закоммитить как `feat(backend): добавить archive intake adapters`.

### Task 5: Submissions HTTP API

**Files:**
- Create: `backend/internal/modules/submissions/http.go`
- Test: `backend/internal/modules/submissions/http_test.go`

- [ ] Написать failing HTTP tests на public multipart create, body limit, list, detail, review, reject, publish и audit log.
- [ ] Проверить generic errors, required fields, ZIP extension, dates, email, Telegram username и coordinates.
- [ ] Подключить Redis rate limit для public form без email и Telegram username в key.
- [ ] Защитить admin routes session middleware для `admin` и `super_admin`; audit log - только `super_admin`.
- [ ] Запустить race tests и закоммитить как `feat(backend): добавить submissions API`.

### Task 6: Event publisher и idempotent publish

**Files:**
- Modify: `backend/internal/modules/events/model.go`
- Modify: `backend/internal/modules/events/repository.go`
- Modify: `backend/internal/modules/events/repository_integration_test.go`
- Modify: `backend/internal/modules/submissions/service.go`
- Modify: `backend/internal/modules/submissions/service_test.go`

- [ ] Написать failing tests на event upsert by submission ID и retry после partial publish failure.
- [ ] Реализовать `events.Publish(ctx, Snapshot)` с `ON CONFLICT (submission_id) DO UPDATE`.
- [ ] После status/audit transaction вызвать idempotent event publisher; retry status `published` доводит event до нужного snapshot.
- [ ] Не отправлять notification до успешной event publication.
- [ ] Запустить PostgreSQL 18 integration tests и закоммитить как `feat(backend): добавить idempotent publication`.

### Task 7: Wiring, E2E и docs

**Files:**
- Modify: `backend/internal/app/modules.go`
- Modify: `backend/internal/app/api_integration_test.go`
- Modify: `backend/README.md`

- [ ] Подключить submissions, storage, queue и event publisher through explicit constructors.
- [ ] Проверить PostgreSQL 18, Redis 8 и MinIO E2E: create -> processing -> new -> review -> reject и create -> processing -> new -> review -> publish.
- [ ] Проверить audit atomicity, duplicate job suppression, event idempotency и preserved MinIO key.
- [ ] Обновить README и `.env.example` без secrets.
- [ ] Запустить `gofmt`, `go test -race ./...`, PostgreSQL/Redis/MinIO integration tests, `go vet ./...`, scoped `golangci-lint`, `go build ./cmd/...`, `git diff --check` и forbidden character scan.
- [ ] Закоммитить как `feat(backend): подключить submissions module`.
