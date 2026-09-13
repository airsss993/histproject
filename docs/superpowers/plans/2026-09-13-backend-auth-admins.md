# Backend Auth And Admins Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Перенести auth и admins в модули нового API с Redis sessions, bcrypt и явным bootstrap super admin.

**Architecture:** `admins` владеет таблицей `auth.admins`. `auth` не импортирует admins и получает credentials через узкий interface. Session token генерируется из `crypto/rand`, в Redis key попадает только SHA-256 token digest. HTTP handlers использую общий JSON error contract.

**Tech Stack:** `net/http`, `pgx/v5`, `go-redis/v9`, `bcrypt`, `crypto/rand`, `encoding/json`.

**Spec:** `docs/superpowers/specs/2026-09-13-backend-modular-monolith-design.md`

## Global Constraints

- Не использовать JWT и `auth.legacy_sessions` в новом API.
- Не логировать password, bcrypt hash, session token и Redis session value.
- Не возвращать сгенерированный password из API.
- Cookie: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, TTL совпадает с Redis TTL.
- Login не раскрывает, существует ли admin.
- Только `super_admin` управляет admins.
- Нельзя понизить или удалить себя, а также последнего `super_admin`.
- Первый `super_admin` создается только CLI-командой.

### Task 1: JSON HTTP contract и auth config

**Files:**
- Create: `backend/internal/platform/httpserver/response.go`
- Test: `backend/internal/platform/httpserver/response_test.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/internal/platform/config/config_test.go`
- Modify: `backend/.env.example`

- [x] Написать failing tests на strict JSON decode, body limit, unknown fields, unified error body и `SESSION_TTL`.
- [x] Подтвердить failure.
- [x] Реализовать helpers и config без новой dependency.
- [x] Запустить race tests и закоммитить как `feat(backend): добавить JSON contract и auth config`.

### Task 2: Admin repository и auth core

**Files:**
- Create: `backend/internal/modules/admins/model.go`
- Create: `backend/internal/modules/admins/repository.go`
- Test: `backend/internal/modules/admins/repository_integration_test.go`
- Create: `backend/internal/modules/auth/session.go`
- Create: `backend/internal/modules/auth/service.go`
- Test: `backend/internal/modules/auth/session_test.go`
- Test: `backend/internal/modules/auth/service_test.go`

- [ ] Написать failing auth unit tests на generic invalid credentials, bcrypt, random token, digest key, TTL, get и delete.
- [ ] Подтвердить failure.
- [ ] Реализовать admin credential query без cross-module import.
- [ ] Реализовать Redis session store и auth service.
- [ ] Проверить repository на PostgreSQL 18 и session store на Redis 8.
- [ ] Запустить race tests и закоммитить как `feat(backend): добавить Redis sessions`.

### Task 3: Auth HTTP endpoints и middleware

**Files:**
- Create: `backend/internal/modules/auth/http.go`
- Test: `backend/internal/modules/auth/http_test.go`

- [ ] Написать failing HTTP tests для login, logout, session, cookie flags, unauthorized и malformed JSON.
- [ ] Подтвердить failure.
- [ ] Реализовать `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/session`.
- [ ] Добавить middleware, который кладет immutable identity в request context.
- [ ] Запустить race tests и закоммитить как `feat(backend): добавить session auth API`.

### Task 4: Admin management

**Files:**
- Create: `backend/internal/modules/admins/service.go`
- Test: `backend/internal/modules/admins/service_test.go`
- Create: `backend/internal/modules/admins/http.go`
- Test: `backend/internal/modules/admins/http_test.go`

- [ ] Написать failing tests на roles, self-change, last super admin, duplicate login, password hashing и authorization.
- [ ] Подтвердить failure.
- [ ] Реализовать service и repository transactions для role/delete invariants.
- [ ] Реализовать admins endpoints и `super_admin` middleware.
- [ ] Запустить race и integration tests, закоммитить как `feat(backend): добавить admins API`.

### Task 5: Bootstrap super admin

**Files:**
- Create: `backend/cmd/bootstrap-admin/main.go`
- Test: `backend/cmd/bootstrap-admin/main_test.go`

- [ ] Написать failing tests на explicit `--apply`, env validation, existing super admin и secret-safe stderr.
- [ ] Подтвердить failure.
- [ ] Реализовать CLI без вывода password и hash.
- [ ] Проверить CLI на PostgreSQL 18.
- [ ] Запустить race tests и закоммитить как `feat(backend): добавить bootstrap super admin`.

### Task 6: Wiring и quality gate

**Files:**
- Modify: `backend/internal/app/api.go`
- Modify: `backend/internal/app/api_test.go`
- Create: `backend/internal/app/modules.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/cmd/api/main_test.go`
- Modify: `backend/README.md`

- [ ] Внедрить modules через explicit constructors и route registrars.
- [ ] Проверить HTTP flow login -> session -> admins -> logout на PostgreSQL 18 и Redis 8.
- [ ] Повторить login rate limit в Redis без хранения password и login в key.
- [ ] Удалить `golang-jwt/jwt` и `google/uuid` из direct dependencies, если их больше не использует legacy code на этом этапе.
- [ ] Обновить README и `.env.example` без secrets.
- [ ] Запустить `gofmt`, `go test -race ./...`, PostgreSQL/Redis integration tests, `go vet ./...`, scoped `golangci-lint`, сборки всех commands и `git diff --check`.
- [ ] Закоммитить как `feat(backend): подключить auth и admins modules`.
