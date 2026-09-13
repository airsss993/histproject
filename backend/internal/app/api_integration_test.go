package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
	"github.com/airsss993/histproject/backend/internal/platform/redisstore"
	legacymigrations "github.com/airsss993/histproject/backend/migrations"
	v2migrations "github.com/airsss993/histproject/backend/migrations/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestAuthAndAdminsHTTPFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_APP_DATABASE_URL")
	redisAddress := os.Getenv("TEST_APP_REDIS_ADDRESS")
	if databaseURL == "" || redisAddress == "" {
		t.Skip("TEST_APP_DATABASE_URL and TEST_APP_REDIS_ADDRESS are not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	prepareAppTestDatabase(t, ctx, databaseURL)

	databaseClient, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(databaseClient.Close)
	redisClient, err := redisstore.Open(ctx, redisAddress, "", 0)
	if err != nil {
		t.Fatalf("open Redis: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })
	if err := redisClient.Client().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush Redis: %v", err)
	}

	adminService := admins.NewService(admins.NewRepository(databaseClient.Pool()))
	if _, err := adminService.Bootstrap(ctx, admins.CreateInput{Login: "root", Password: "correct-password"}); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	cfg := config.Config{SessionTTL: time.Hour, CORSOrigins: []string{"http://localhost:3000"}}
	logger := testLogger()
	handler := NewAPI(cfg, logger, NewModules(cfg, logger, databaseClient, redisClient), databaseClient, redisClient)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/v1/auth/login", `{"login":"root","password":"correct-password"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	result := login.Result()
	t.Cleanup(func() { _ = result.Body.Close() })
	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %#v", cookies)
	}
	sessionCookie := cookies[0]

	session := performJSONRequest(t, handler, http.MethodGet, "/api/v1/auth/session", "", sessionCookie)
	if session.Code != http.StatusOK {
		t.Fatalf("session status = %d, body = %s", session.Code, session.Body.String())
	}

	list := performJSONRequest(t, handler, http.MethodGet, "/api/v1/admin/admins", "", sessionCookie)
	if list.Code != http.StatusOK {
		t.Fatalf("admins status = %d, body = %s", list.Code, list.Body.String())
	}
	var adminsBody struct {
		Items []admins.Admin `json:"items"`
	}
	if err := json.NewDecoder(list.Body).Decode(&adminsBody); err != nil {
		t.Fatalf("decode admins: %v", err)
	}
	if len(adminsBody.Items) != 1 || adminsBody.Items[0].Login != "root" {
		t.Fatalf("admins = %#v", adminsBody.Items)
	}

	logout := performJSONRequest(t, handler, http.MethodPost, "/api/v1/auth/logout", "", sessionCookie)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logout.Code, logout.Body.String())
	}
	afterLogout := performJSONRequest(t, handler, http.MethodGet, "/api/v1/auth/session", "", sessionCookie)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("session after logout status = %d, want 401", afterLogout.Code)
	}

	for attempt := 1; attempt <= 6; attempt++ {
		response := performJSONRequest(t, handler, http.MethodPost, "/api/v1/auth/login", `{"login":"missing","password":"wrong-password"}`, nil)
		want := http.StatusUnauthorized
		if attempt == 6 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("login attempt %d status = %d, want %d", attempt, response.Code, want)
		}
	}
	keys, err := redisClient.Client().Keys(ctx, "auth:login:*").Result()
	if err != nil {
		t.Fatalf("list rate limit keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("rate limit keys = %#v, want root and missing digests", keys)
	}
	for _, key := range keys {
		if strings.Contains(key, "root") || strings.Contains(key, "missing") {
			t.Fatalf("rate limit key exposes login: %q", key)
		}
	}
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func prepareAppTestDatabase(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_test") {
		t.Fatalf("database %q is not an isolated test database", databaseName)
	}
	if _, err := db.ExecContext(ctx, `
		DROP SCHEMA IF EXISTS auth CASCADE;
		DROP SCHEMA IF EXISTS submissions CASCADE;
		DROP SCHEMA IF EXISTS events CASCADE;
		DROP SCHEMA IF EXISTS notifications CASCADE;
		DROP TABLE IF EXISTS public.request_audit_log CASCADE;
		DROP TABLE IF EXISTS public.bot_subscriptions CASCADE;
		DROP TABLE IF EXISTS public.objects CASCADE;
		DROP TABLE IF EXISTS public.requests CASCADE;
		DROP TABLE IF EXISTS public.admin_sessions CASCADE;
		DROP TABLE IF EXISTS public.admins CASCADE;
		DROP TYPE IF EXISTS public.request_status CASCADE;
		DROP TYPE IF EXISTS public.admin_role CASCADE;
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`); err != nil {
		t.Fatalf("reset database: %v", err)
	}
	if err := legacymigrations.Up(ctx, db); err != nil {
		t.Fatalf("run legacy migrations: %v", err)
	}
	if err := v2migrations.Run(ctx, db); err != nil {
		t.Fatalf("run V2 migrations: %v", err)
	}
}
