package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/events"
	"github.com/airsss993/histproject/backend/internal/modules/submissions"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
	platformqueue "github.com/airsss993/histproject/backend/internal/platform/queue"
	"github.com/airsss993/histproject/backend/internal/platform/redisstore"
	"github.com/airsss993/histproject/backend/internal/platform/storage"
	"github.com/airsss993/histproject/backend/migrations"
	"github.com/hibiken/asynq"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestAPIHTTPFlow(t *testing.T) {
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
	if _, err := adminService.Create(ctx, admins.CreateInput{Login: "editor", Password: "editor-password", Role: admins.RoleAdmin}); err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if _, err := databaseClient.Pool().Exec(ctx, `
		INSERT INTO submissions.submissions (
			id, title, description, event_date, event_type_code,
			email, telegram_username, archive_id, slug, status
		) VALUES (1, 'Submission', 'description', DATE '2020-01-02', 'military',
			'author@example.test', 'author', 'archives/1.zip', 'submission-1', 'published');
		INSERT INTO events.events (
			id, submission_id, title, description, latitude, longitude,
			event_date, event_type_code, site_url, preview_image_url
		) VALUES (1, 1, 'Event', 'description', 55.5, 60.5,
			DATE '2020-01-02', 'military', 'https://storage.test/sites/1', 'https://storage.test/previews/1.png');
	`); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	cfg := config.Config{SessionTTL: time.Hour, CORSOrigins: []string{"http://localhost:3000"}}
	logger := testLogger()
	handler := NewAPI(cfg, logger, NewModules(cfg, logger, databaseClient, redisClient, nil), databaseClient, redisClient)
	invalidSubmission := performJSONRequest(t, handler, http.MethodPost, "/api/v1/submissions", `{}`, nil)
	if invalidSubmission.Code != http.StatusBadRequest {
		t.Fatalf("invalid submission status = %d, want 400, body = %s", invalidSubmission.Code, invalidSubmission.Body.String())
	}
	unauthorizedSubmissions := performJSONRequest(t, handler, http.MethodGet, "/api/v1/admin/submissions", "", nil)
	if unauthorizedSubmissions.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized submissions status = %d, want 401", unauthorizedSubmissions.Code)
	}
	publicEvents := performJSONRequest(t, handler, http.MethodGet, "/api/v1/events?eventType=military&page=1&pageSize=10", "", nil)
	if publicEvents.Code != http.StatusOK || !strings.Contains(publicEvents.Body.String(), `"eventDate":"2020-01-02"`) {
		t.Fatalf("public events status = %d, body = %s", publicEvents.Code, publicEvents.Body.String())
	}
	publicEvent := performJSONRequest(t, handler, http.MethodGet, "/api/v1/events/1", "", nil)
	if publicEvent.Code != http.StatusOK || !strings.Contains(publicEvent.Body.String(), `"eventType":"military"`) {
		t.Fatalf("public event status = %d, body = %s", publicEvent.Code, publicEvent.Body.String())
	}
	unauthorizedUpdate := performJSONRequest(t, handler, http.MethodPatch, "/api/v1/admin/events/1/coordinates", `{"latitude":56,"longitude":61}`, nil)
	if unauthorizedUpdate.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized update status = %d, want 401", unauthorizedUpdate.Code)
	}

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
	if len(adminsBody.Items) != 2 || adminsBody.Items[0].Login != "root" || adminsBody.Items[1].Login != "editor" {
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

	editorLogin := performJSONRequest(t, handler, http.MethodPost, "/api/v1/auth/login", `{"login":"editor","password":"editor-password"}`, nil)
	if editorLogin.Code != http.StatusOK {
		t.Fatalf("editor login status = %d, body = %s", editorLogin.Code, editorLogin.Body.String())
	}
	editorResult := editorLogin.Result()
	t.Cleanup(func() { _ = editorResult.Body.Close() })
	editorCookies := editorResult.Cookies()
	if len(editorCookies) != 1 {
		t.Fatalf("editor cookies = %#v", editorCookies)
	}
	update := performJSONRequest(t, handler, http.MethodPatch, "/api/v1/admin/events/1/coordinates", `{"latitude":56,"longitude":61}`, editorCookies[0])
	if update.Code != http.StatusNoContent {
		t.Fatalf("update status = %d, body = %s", update.Code, update.Body.String())
	}
	var latitude, longitude float64
	if err := databaseClient.Pool().QueryRow(ctx, `SELECT latitude, longitude FROM events.events WHERE id = 1`).Scan(&latitude, &longitude); err != nil {
		t.Fatalf("read coordinates: %v", err)
	}
	if latitude != 56 || longitude != 61 {
		t.Fatalf("coordinates = (%f, %f)", latitude, longitude)
	}
	deleteEvent := performJSONRequest(t, handler, http.MethodDelete, "/api/v1/admin/events/1", "", editorCookies[0])
	if deleteEvent.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteEvent.Code, deleteEvent.Body.String())
	}
	var submissionCount int
	if err := databaseClient.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM submissions.submissions WHERE id = 1`).Scan(&submissionCount); err != nil {
		t.Fatalf("count submission: %v", err)
	}
	if submissionCount != 1 {
		t.Fatalf("submission count = %d, want 1", submissionCount)
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
	if len(keys) != 3 {
		t.Fatalf("rate limit keys = %#v, want root, editor and missing digests", keys)
	}
	for _, key := range keys {
		if strings.Contains(key, "root") || strings.Contains(key, "editor") || strings.Contains(key, "missing") {
			t.Fatalf("rate limit key exposes login: %q", key)
		}
	}
}

func TestAPISubmissionFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_APP_DATABASE_URL")
	redisAddress := os.Getenv("TEST_APP_REDIS_ADDRESS")
	minioEndpoint := os.Getenv("TEST_APP_MINIO_ENDPOINT")
	minioAccessKey := os.Getenv("TEST_APP_MINIO_ACCESS_KEY")
	minioSecretKey := os.Getenv("TEST_APP_MINIO_SECRET_KEY")
	if databaseURL == "" || redisAddress == "" || minioEndpoint == "" || minioAccessKey == "" || minioSecretKey == "" {
		t.Skip("app PostgreSQL, Redis and MinIO test variables are not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
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
	storageClient := prepareAppTestStorage(t, ctx, minioEndpoint, minioAccessKey, minioSecretKey)

	adminService := admins.NewService(admins.NewRepository(databaseClient.Pool()))
	if _, err := adminService.Bootstrap(ctx, admins.CreateInput{Login: "root", Password: "correct-password"}); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	if _, err := adminService.Create(ctx, admins.CreateInput{Login: "editor", Password: "editor-password", Role: admins.RoleAdmin}); err != nil {
		t.Fatalf("create editor: %v", err)
	}

	cfg := config.Config{SessionTTL: time.Hour, CORSOrigins: []string{"http://localhost:3000"}, MaxUploadBytes: 52_428_800, SitePublicBaseURL: "https://storage.test/sites"}
	logger := testLogger()
	handler := NewAPI(cfg, logger, NewModules(cfg, logger, databaseClient, redisClient, storageClient), databaseClient, redisClient, storageClient)
	editorCookie := appLogin(t, handler, "editor", "editor-password")
	rootCookie := appLogin(t, handler, "root", "correct-password")

	rejectedID := createSubmissionRequest(t, handler, "Rejected event", "rejected-author@example.test", "rejected_author")
	rejectedArchiveID := submissionArchiveID(t, ctx, databaseClient, rejectedID)
	assertRawArchiveKey(t, rejectedArchiveID)
	assertArchiveExists(t, ctx, storageClient, rejectedArchiveID)
	queueClient := platformqueue.NewClient(redisClient.Client())
	if err := queueClient.EnqueueArchive(ctx, rejectedID, rejectedArchiveID); !errors.Is(err, asynq.ErrTaskIDConflict) {
		t.Fatalf("duplicate EnqueueArchive() error = %v, want task ID conflict", err)
	}

	workflow := submissions.NewService(
		submissions.NewRepository(databaseClient.Pool()),
		submissionEventPublisher{service: events.NewService(events.NewRepository(databaseClient.Pool()))},
		storageClient,
		queueClient,
		nil,
		cfg.SitePublicBaseURL,
	)
	if _, err := workflow.MarkReady(ctx, rejectedID, "https://storage.test/preview/rejected/index.html", "https://storage.test/sites/rejected/screenshot.png"); err != nil {
		t.Fatalf("mark rejected submission ready: %v", err)
	}
	assertHTTPStatus(t, performJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/admin/submissions/%d/review", rejectedID), "", editorCookie), http.StatusOK)
	assertHTTPStatus(t, performJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/admin/submissions/%d/reject", rejectedID), `{"comment":"duplicate"}`, editorCookie), http.StatusOK)
	assertSubmissionState(t, ctx, databaseClient, rejectedID, statusExpectation{Status: "rejected", AuditCount: 2})
	assertArchiveExists(t, ctx, storageClient, rejectedArchiveID)

	publishedID := createSubmissionRequest(t, handler, "Published event", "published-author@example.test", "published_author")
	publishedArchiveID := submissionArchiveID(t, ctx, databaseClient, publishedID)
	assertRawArchiveKey(t, publishedArchiveID)
	assertArchiveExists(t, ctx, storageClient, publishedArchiveID)
	if _, err := workflow.MarkReady(ctx, publishedID, "https://storage.test/preview/published/index.html", "https://storage.test/sites/published/screenshot.png"); err != nil {
		t.Fatalf("mark published submission ready: %v", err)
	}
	assertHTTPStatus(t, performJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/admin/submissions/%d/review", publishedID), "", editorCookie), http.StatusOK)
	publishPath := fmt.Sprintf("/api/v1/admin/submissions/%d/publish", publishedID)
	assertHTTPStatus(t, performJSONRequest(t, handler, http.MethodPost, publishPath, `{"latitude":55.5,"longitude":60.5}`, editorCookie), http.StatusOK)
	assertHTTPStatus(t, performJSONRequest(t, handler, http.MethodPost, publishPath, `{"latitude":56.5,"longitude":61.5}`, editorCookie), http.StatusOK)
	assertSubmissionState(t, ctx, databaseClient, publishedID, statusExpectation{Status: "published", AuditCount: 2})

	var eventCount int
	var latitude float64
	var siteURL string
	if err := databaseClient.Pool().QueryRow(ctx, `
		SELECT COUNT(*), MAX(latitude), MAX(site_url)
		FROM events.events WHERE submission_id = $1
	`, publishedID).Scan(&eventCount, &latitude, &siteURL); err != nil {
		t.Fatalf("read published event: %v", err)
	}
	if eventCount != 1 || latitude != 56.5 || siteURL != "https://storage.test/sites/published-event-"+publishedArchiveID+"/index.html" {
		t.Fatalf("published event = count:%d latitude:%f site:%q", eventCount, latitude, siteURL)
	}
	assertArchiveExists(t, ctx, storageClient, publishedArchiveID)

	forbiddenAudit := performJSONRequest(t, handler, http.MethodGet, "/api/v1/admin/audit-log", "", editorCookie)
	assertHTTPStatus(t, forbiddenAudit, http.StatusForbidden)
	audit := performJSONRequest(t, handler, http.MethodGet, "/api/v1/admin/audit-log?page=1&pageSize=20", "", rootCookie)
	if audit.Code != http.StatusOK || !strings.Contains(audit.Body.String(), `"action":"published"`) || !strings.Contains(audit.Body.String(), `"action":"rejected"`) {
		t.Fatalf("audit status = %d, body = %s", audit.Code, audit.Body.String())
	}

	keys, err := redisClient.Client().Keys(ctx, "submissions:create:*").Result()
	if err != nil {
		t.Fatalf("list submission rate keys: %v", err)
	}
	if len(keys) != 1 || strings.Contains(keys[0], "rejected-author") || strings.Contains(keys[0], "published-author") {
		t.Fatalf("submission rate keys = %#v", keys)
	}

	for _, archiveID := range []string{rejectedArchiveID, publishedArchiveID} {
		if err := storageClient.Client().RemoveObject(ctx, "archives", archiveID, minio.RemoveObjectOptions{}); err != nil {
			t.Fatalf("remove test archive %q: %v", archiveID, err)
		}
	}
}

type statusExpectation struct {
	Status     string
	AuditCount int
}

func appLogin(t *testing.T, handler http.Handler, login, password string) *http.Cookie {
	t.Helper()
	response := performJSONRequest(t, handler, http.MethodPost, "/api/v1/auth/login", fmt.Sprintf(`{"login":%q,"password":%q}`, login, password), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("login %q status = %d, body = %s", login, response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login %q cookies = %#v", login, cookies)
	}
	return cookies[0]
}

func createSubmissionRequest(t *testing.T, handler http.Handler, title, email, telegram string) int64 {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fields := map[string]string{
		"title": title, "description": "Description", "eventDate": "2020-01-02",
		"eventType": "military", "email": email, "telegramUsername": telegram,
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field: %v", err)
		}
	}
	archive, err := writer.CreateFormFile("archive", "site.zip")
	if err != nil {
		t.Fatalf("create archive field: %v", err)
	}
	if _, err := archive.Write([]byte("test zip bytes")); err != nil {
		t.Fatalf("write archive field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create submission status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Submission struct {
			ID int64 `json:"id"`
		} `json:"submission"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode created submission: %v", err)
	}
	return payload.Submission.ID
}

func prepareAppTestStorage(t *testing.T, ctx context.Context, endpoint, accessKey, secretKey string) *storage.MinIO {
	t.Helper()
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatalf("create MinIO setup client: %v", err)
	}
	for _, bucket := range []string{"archives", "sites"} {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			t.Fatalf("check MinIO bucket %q: %v", bucket, err)
		}
		if !exists {
			if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				t.Fatalf("create MinIO bucket %q: %v", bucket, err)
			}
		}
	}
	storageClient, err := storage.Open(ctx, storage.Config{
		Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey,
		ArchiveBucket: "archives", SiteBucket: "sites",
	})
	if err != nil {
		t.Fatalf("open test storage: %v", err)
	}
	return storageClient
}

func submissionArchiveID(t *testing.T, ctx context.Context, databaseClient *database.Database, submissionID int64) string {
	t.Helper()
	var archiveID string
	if err := databaseClient.Pool().QueryRow(ctx, `SELECT archive_id FROM submissions.submissions WHERE id = $1`, submissionID).Scan(&archiveID); err != nil {
		t.Fatalf("read submission archive ID: %v", err)
	}
	return archiveID
}

func assertRawArchiveKey(t *testing.T, archiveID string) {
	t.Helper()
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(archiveID) {
		t.Fatalf("archive ID = %q, want raw UUID", archiveID)
	}
}

func assertArchiveExists(t *testing.T, ctx context.Context, storageClient *storage.MinIO, archiveID string) {
	t.Helper()
	if _, err := storageClient.Client().StatObject(ctx, "archives", archiveID, minio.StatObjectOptions{}); err != nil {
		t.Fatalf("stat archive %q: %v", archiveID, err)
	}
}

func assertSubmissionState(t *testing.T, ctx context.Context, databaseClient *database.Database, submissionID int64, want statusExpectation) {
	t.Helper()
	var status string
	var auditCount int
	if err := databaseClient.Pool().QueryRow(ctx, `SELECT status::TEXT FROM submissions.submissions WHERE id = $1`, submissionID).Scan(&status); err != nil {
		t.Fatalf("read submission status: %v", err)
	}
	if err := databaseClient.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM submissions.audit_log WHERE submission_id = $1`, submissionID).Scan(&auditCount); err != nil {
		t.Fatalf("count submission audit: %v", err)
	}
	if status != want.Status || auditCount != want.AuditCount {
		t.Fatalf("submission state = status:%q audit:%d, want status:%q audit:%d", status, auditCount, want.Status, want.AuditCount)
	}
}

func assertHTTPStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("HTTP status = %d, want %d, body = %s", response.Code, want, response.Body.String())
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
		DROP SCHEMA IF EXISTS users CASCADE;
		DROP FUNCTION IF EXISTS public.rh_code();
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`); err != nil {
		t.Fatalf("reset database: %v", err)
	}
	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}
