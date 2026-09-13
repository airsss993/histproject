# Backend Platform Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Создать проверяемый platform-каркас нового backend с конфигом, логами, HTTP middleware, health endpoints и корректным shutdown.

**Architecture:** Новые platform-пакеты добавляются рядом со старыми бизнес-модулями. Переключение Docker и production entrypoint в этот этап не входит. Каждый пакет имеет узкий API и тесты без внешних services.

**Tech Stack:** Go 1.26, standard library, `log/slog`, `net/http`, `pgx/v5` на следующем этапе.

**Spec:** `docs/superpowers/specs/2026-09-13-backend-modular-monolith-design.md`

## Global Constraints

- Не менять PostgreSQL и MinIO в этом плане.
- Не переключать Dockerfile и compose на новый entrypoint.
- Не добавлять новые внешние зависимости.
- Ошибки возвращать вызывающему коду, а не завершать процесс через `log.Fatal`.
- Все тесты проходят с `-race`.

---

### Task 1: Путь Go-модуля

**Files:**
- Modify: `backend/go.mod`
- Modify: `backend/**/*.go`

**Interfaces:**
- Produces: единый import prefix `github.com/airsss993/histproject/backend`.

- [x] **Step 1: Заменить module path**

Run:

```bash
cd backend
go mod edit -module github.com/airsss993/histproject/backend
rg -l 'github.com/airsss993/histproject-backend' --glob '*.go' | xargs sed -i '' 's#github.com/airsss993/histproject-backend#github.com/airsss993/histproject/backend#g'
```

- [x] **Step 2: Проверить сборку**

Run: `go test ./...`

Expected: все пакеты компилируются.

- [x] **Step 3: Закоммитить**

```bash
git add backend/go.mod backend
git commit -m "chore(backend): обновить путь Go-модуля"
```

### Task 2: Строгая конфигурация

**Files:**
- Create: `backend/internal/platform/config/config.go`
- Test: `backend/internal/platform/config/config_test.go`

**Interfaces:**
- Produces: `config.Load() (config.Config, error)`.
- Produces: `config.LoadFrom(func(string) string) (config.Config, error)` для детерминированных тестов.

- [x] **Step 1: Написать failing table-driven tests**

Тесты проверяют:

```go
func TestLoadFromUsesDevelopmentDefaults(t *testing.T)
func TestLoadFromRequiresProductionSecrets(t *testing.T)
func TestLoadFromRejectsInvalidDuration(t *testing.T)
func TestLoadFromParsesExactCORSOrigins(t *testing.T)
```

Development defaults:

```go
Environment:     "development"
ServerAddress:   ":7666"
ShutdownTimeout: 10 * time.Second
DatabaseURL:     "postgres://postgres:postgres@localhost:5432/histproject?sslmode=disable"
RedisAddress:    "localhost:6379"
MinIOEndpoint:   "localhost:9000"
MinIOBucket:     "histproject"
CORSOrigins:     []string{"http://localhost:3000"}
```

- [x] **Step 2: Запустить tests и подтвердить failure**

Run: `go test ./internal/platform/config`

Expected: FAIL, пакет еще не существует.

- [x] **Step 3: Реализовать Config**

```go
type Config struct {
    Environment     string
    ServerAddress   string
    ShutdownTimeout time.Duration
    DatabaseURL     string
    RedisAddress    string
    RedisPassword   string
    MinIOEndpoint   string
    MinIOAccessKey  string
    MinIOSecretKey  string
    MinIOBucket     string
    MinIOUseSSL     bool
    CookieDomain    string
    CORSOrigins     []string
}

func Load() (Config, error) {
    return LoadFrom(os.Getenv)
}
```

`LoadFrom` обрезает пробелы, парсит `SHUTDOWN_TIMEOUT` через `time.ParseDuration`, парсит `MINIO_USE_SSL` через `strconv.ParseBool` и разделяет `CORS_ALLOWED_ORIGINS` по запятой. Для `production` он требует `DATABASE_URL`, `REDIS_ADDRESS`, `REDIS_PASSWORD`, `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `COOKIE_DOMAIN` и непустой список origins.

- [x] **Step 4: Запустить tests**

Run: `go test -race ./internal/platform/config`

Expected: PASS.

- [x] **Step 5: Закоммитить**

```bash
git add backend/internal/platform/config
git commit -m "feat(backend): добавить строгую конфигурацию"
```

### Task 3: HTTP middleware

**Files:**
- Create: `backend/internal/platform/httpserver/middleware.go`
- Test: `backend/internal/platform/httpserver/middleware_test.go`

**Interfaces:**
- Produces: `httpserver.Chain(http.Handler, ...func(http.Handler) http.Handler) http.Handler`.
- Produces: `httpserver.RequestID`, `httpserver.Recover`, `httpserver.SecurityHeaders`, `httpserver.CORS`, `httpserver.AccessLog`.
- Produces: `httpserver.RequestIDFromContext(context.Context) string`.

- [ ] **Step 1: Написать failing middleware tests**

```go
func TestRequestIDAddsHeaderAndContext(t *testing.T)
func TestRecoverReturnsJSONAndKeepsRequestID(t *testing.T)
func TestCORSMatchesWholeOrigin(t *testing.T)
func TestCORSRejectsSubstringOrigin(t *testing.T)
func TestSecurityHeaders(t *testing.T)
func TestAccessLogRecordsStatusAndDuration(t *testing.T)
```

Ключевая проверка CORS: origin `https://evil-example.com` не матчит `https://example.com`.

- [ ] **Step 2: Подтвердить failure**

Run: `go test ./internal/platform/httpserver`

Expected: FAIL.

- [ ] **Step 3: Реализовать middleware**

Request ID создается из 16 random bytes через `crypto/rand` и `hex.EncodeToString`. CORS преобразует origins в `map[string]struct{}`. Recovery возвращает:

```json
{"error":{"code":"internal_error","message":"Внутренняя ошибка"}}
```

Access log пишет `request_id`, `method`, `path`, `status`, `duration_ms` и `response_bytes`.

- [ ] **Step 4: Запустить tests**

Run: `go test -race ./internal/platform/httpserver`

Expected: PASS.

- [ ] **Step 5: Закоммитить**

```bash
git add backend/internal/platform/httpserver
git commit -m "feat(backend): добавить HTTP middleware"
```

### Task 4: Health endpoints

**Files:**
- Create: `backend/internal/platform/health/handler.go`
- Test: `backend/internal/platform/health/handler_test.go`

**Interfaces:**
- Produces: `health.Checker`.
- Produces: `health.NewHandler(time.Duration, ...health.Checker) *health.Handler`.
- Produces: `(*health.Handler).Register(*http.ServeMux)`.

- [ ] **Step 1: Написать failing tests**

```go
type Checker interface {
    Name() string
    Ping(context.Context) error
}

func TestHealthzAlwaysReportsLiveProcess(t *testing.T)
func TestReadyzReportsReadyWhenAllChecksPass(t *testing.T)
func TestReadyzReportsDependencyFailure(t *testing.T)
func TestReadyzAppliesTimeout(t *testing.T)
```

Expected readiness response:

```json
{"status":"ready","checks":{"postgres":"ok","redis":"ok","minio":"ok"}}
```

- [ ] **Step 2: Подтвердить failure**

Run: `go test ./internal/platform/health`

Expected: FAIL.

- [ ] **Step 3: Реализовать handlers**

`/healthz` всегда возвращает 200 и `{"status":"ok"}`. `/readyz` запускает checks последовательно с общим context timeout, возвращает 503 при любой ошибке и не печатает внутреннюю причину в JSON.

- [ ] **Step 4: Запустить tests**

Run: `go test -race ./internal/platform/health`

Expected: PASS.

- [ ] **Step 5: Закоммитить**

```bash
git add backend/internal/platform/health
git commit -m "feat(backend): добавить health endpoints"
```

### Task 5: HTTP server lifecycle

**Files:**
- Replace: `backend/internal/server/server.go`
- Test: `backend/internal/server/server_test.go`

**Interfaces:**
- Produces: `server.New(server.Config, http.Handler, *slog.Logger) *server.Server`.
- Produces: `(*server.Server).Run(context.Context) error`.

- [ ] **Step 1: Написать failing tests**

```go
type Config struct {
    Address         string
    ShutdownTimeout time.Duration
    ReadTimeout     time.Duration
    ReadHeaderTimeout time.Duration
    WriteTimeout    time.Duration
    IdleTimeout     time.Duration
}

func TestNewAppliesTimeouts(t *testing.T)
func TestRunReturnsListenError(t *testing.T)
func TestRunStopsAfterContextCancellation(t *testing.T)
```

- [ ] **Step 2: Подтвердить failure**

Run: `go test ./internal/server`

Expected: FAIL из-за отсутствия нового API.

- [ ] **Step 3: Заменить lifecycle**

`Run` слушает в отдельной goroutine, ожидает ошибку или `ctx.Done()`, вызывает `Shutdown` с новым timeout context и возвращает ошибку. Внутри нет `log.Fatal`.

- [ ] **Step 4: Запустить tests**

Run: `go test -race ./internal/server`

Expected: PASS.

- [ ] **Step 5: Закоммитить**

```bash
git add backend/internal/server
git commit -m "refactor(backend): исправить lifecycle HTTP-сервера"
```

### Task 6: Новая API composition root

**Files:**
- Create: `backend/internal/app/api.go`
- Create: `backend/cmd/api/main.go`
- Test: `backend/internal/app/api_test.go`

**Interfaces:**
- Produces: `app.NewAPI(config.Config, *slog.Logger, ...health.Checker) http.Handler`.
- Consumes: config, middleware, health handler и server lifecycle из предыдущих tasks.

- [ ] **Step 1: Написать failing composition tests**

```go
func TestNewAPIExposesHealthEndpoints(t *testing.T)
func TestNewAPIAppliesRequestIDAndSecurityHeaders(t *testing.T)
func TestNewAPIReturnsJSONForUnknownRoute(t *testing.T)
```

- [ ] **Step 2: Подтвердить failure**

Run: `go test ./internal/app`

Expected: FAIL, `NewAPI` еще не существует.

- [ ] **Step 3: Реализовать composition root**

`NewAPI` регистрирует health routes в `http.ServeMux`, добавляет JSON 404 handler и оборачивает mux middleware в порядке из спецификации. `cmd/api/main.go` загружает config, создает logger, подписывается на `SIGINT` и `SIGTERM`, запускает server и выходит с кодом 1 только из `main`.

- [ ] **Step 4: Проверить новый entrypoint**

Run:

```bash
go test -race ./internal/app ./internal/platform/... ./internal/server
go build ./cmd/api
```

Expected: PASS.

- [ ] **Step 5: Закоммитить**

```bash
git add backend/cmd/api backend/internal/app/api.go backend/internal/app/api_test.go
git commit -m "feat(backend): добавить новый API entrypoint"
```

### Task 7: Общая проверка этапа

**Files:**
- Modify: только файлы, требующие форматирования.

**Interfaces:**
- Verifies: весь backend и новый entrypoint.

- [ ] **Step 1: Отформатировать**

Run: `gofmt -w cmd internal`

- [ ] **Step 2: Запустить quality gates**

Run:

```bash
go test -race ./...
go vet ./...
go build ./cmd/main.go
go build ./cmd/api
git diff --check
```

Expected: все команды завершаются с кодом 0.

- [ ] **Step 3: Проверить историю и рабочее дерево**

Run:

```bash
git status --short
git log --oneline -8
```

Expected: рабочее дерево чистое, история содержит отдельные conventional commits.
