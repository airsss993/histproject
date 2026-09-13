# Backend Events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Перенести публичное чтение и admin-управление опубликованными событиями в модуль `events` нового API.

**Architecture:** Модуль `events` владеет только `events.events` и не читае `submissions` через SQL JOIN. Repository использует `pgxpool`, service валидирует filters и coordinates, HTTP слой соблюдает общий JSON contract. Delete удаляет только event row; submission, audit и MinIO остаются во владении своих модулей.

**Tech Stack:** `net/http`, `http.ServeMux`, `pgx/v5`, PostgreSQL 18, `encoding/json`, `time`.

**Spec:** `docs/superpowers/specs/2026-09-13-backend-modular-monolith-design.md`

## Global Constraints

- Не менять existing event IDs, `submission_id`, URLs и MinIO keys.
- Не добавлять SQL JOIN между `events` и другими schemas.
- Event type в HTTP contract передается строковым `eventType`, который соответствует `event_type_code`.
- Date имеет формат `YYYY-MM-DD`; coordinates в JSON являются numbers.
- List сортируется по `event_date ASC, id ASC`.
- Pagination: `page` default `1`, `pageSize` default `50`, maximum `100`.
- Admin coordinate update и delete используют event ID, а не legacy request ID.
- `admin` и `super_admin` могут менять coordinates и удалять events.
- Внутренняя ошибка пишется в log с request ID; client получает безопасный error contract.

---

### Task 1: Event model, filters и service validation

**Files:**
- Create: `backend/internal/modules/events/model.go`
- Create: `backend/internal/modules/events/service.go`
- Test: `backend/internal/modules/events/service_test.go`

**Interfaces:**
- Produces: `Event`, `Filter`, `Page`, `Store`, `Service.List`, `Service.Get`, `Service.UpdateCoordinates`, `Service.Delete`.
- Consumes: standard `context.Context` and repository errors.

- [x] Написать failing tests на date range, repeated event type codes, pagination bounds, coordinate ranges и positive IDs.
- [x] Подтвердить expected failure.
- [x] Реализовать models, typed errors и validation без external validator.
- [x] Запустить race tests и закоммитить как `feat(backend): добавить events core`.

### Task 2: PostgreSQL event repository

**Files:**
- Create: `backend/internal/modules/events/repository.go`
- Test: `backend/internal/modules/events/repository_integration_test.go`

**Interfaces:**
- Consumes: `Event`, `Filter`, `Page`, `ErrEventNotFound` и `*pgxpool.Pool`.
- Produces: implementation of `events.Store` using only `events.events`.

- [ ] Написать failing integration tests на list filters, total count, stable pagination, detail, update и delete.
- [ ] Подтвердить failure на PostgreSQL 18.
- [ ] Реализовать parameterized queries без cross-schema JOIN.
- [ ] Проверить, что delete не изменяет `submissions.submissions` и не требует storage client.
- [ ] Запустить race и integration tests, закоммитить как `feat(backend): добавить events repository`.

### Task 3: Events HTTP API

**Files:**
- Create: `backend/internal/modules/events/http.go`
- Test: `backend/internal/modules/events/http_test.go`

**Interfaces:**
- Consumes: `events.Service`, shared `httpserver` JSON helpers, auth middleware passed as `events.Middleware`.
- Produces: `GET /api/v1/events`, `GET /api/v1/events/{id}`, `PATCH /api/v1/admin/events/{id}/coordinates`, `DELETE /api/v1/admin/events/{id}`.

- [ ] Написать failing HTTP tests на list response, detail, malformed query, unknown ID, coordinates, delete и authorization.
- [ ] Подтвердить expected failure.
- [ ] Реализовать handlers и unified errors.
- [ ] Не возвращать internal repository errors в HTTP body.
- [ ] Запустить race tests и закоммитить как `feat(backend): добавить events API`.

### Task 4: Wiring, E2E и docs

**Files:**
- Modify: `backend/internal/app/modules.go`
- Modify: `backend/internal/app/api_integration_test.go`
- Modify: `backend/README.md`

**Interfaces:**
- Consumes: `events.NewRepository`, `events.NewService`, `events.NewHandler`, auth session middleware and role checks.
- Produces: public events routes and authenticated admin events routes in `cmd/api`.

- [ ] Подключить events through explicit constructors and registrar.
- [ ] Добавить PostgreSQL 18 E2E для filters, pagination, detail, admin coordinate update и delete.
- [ ] Проверить session access для roles `admin` и `super_admin`.
- [ ] Обновить README без копирования legacy contract.
- [ ] Запустить `gofmt`, `go test -race ./...`, PostgreSQL 18 integration tests, `go vet ./...`, scoped `golangci-lint`, `go build ./cmd/...`, `git diff --check` и forbidden character scan.
- [ ] Закоммитить как `feat(backend): подключить events module`.
