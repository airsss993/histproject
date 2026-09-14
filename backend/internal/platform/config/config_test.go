package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromUsesDevelopmentDefaults(t *testing.T) {
	cfg, err := LoadFrom(env(nil))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if cfg.Environment != "development" {
		t.Errorf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.ServerAddress != ":7666" {
		t.Errorf("ServerAddress = %q, want :7666", cfg.ServerAddress)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Errorf("SessionTTL = %s, want 12h", cfg.SessionTTL)
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/histproject?sslmode=disable" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.RedisAddress != "localhost:6379" {
		t.Errorf("RedisAddress = %q, want localhost:6379", cfg.RedisAddress)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("RedisDB = %d, want 0", cfg.RedisDB)
	}
	if cfg.MinIOEndpoint != "localhost:9000" {
		t.Errorf("MinIOEndpoint = %q, want localhost:9000", cfg.MinIOEndpoint)
	}
	if cfg.MinIOArchiveBucket != "archives" {
		t.Errorf("MinIOArchiveBucket = %q, want archives", cfg.MinIOArchiveBucket)
	}
	if cfg.MinIOSiteBucket != "sites" {
		t.Errorf("MinIOSiteBucket = %q, want sites", cfg.MinIOSiteBucket)
	}
	if cfg.MinIOUseSSL {
		t.Error("MinIOUseSSL = true, want false")
	}
	if cfg.MaxUploadBytes != 52_428_800 {
		t.Errorf("MaxUploadBytes = %d, want 52428800", cfg.MaxUploadBytes)
	}
	if cfg.MaxExtractedBytes != 209_715_200 {
		t.Errorf("MaxExtractedBytes = %d, want 209715200", cfg.MaxExtractedBytes)
	}
	if cfg.MaxArchiveFiles != 2_000 {
		t.Errorf("MaxArchiveFiles = %d, want 2000", cfg.MaxArchiveFiles)
	}
	if cfg.MaxArchiveFileBytes != 52_428_800 {
		t.Errorf("MaxArchiveFileBytes = %d, want 52428800", cfg.MaxArchiveFileBytes)
	}
	if cfg.ChromeURL != "http://localhost:9222" {
		t.Errorf("ChromeURL = %q, want http://localhost:9222", cfg.ChromeURL)
	}
	if cfg.ScreenshotTimeout != 60*time.Second {
		t.Errorf("ScreenshotTimeout = %s, want 60s", cfg.ScreenshotTimeout)
	}
	if cfg.SiteRenderBaseURL != "http://localhost:9000/sites" {
		t.Errorf("SiteRenderBaseURL = %q", cfg.SiteRenderBaseURL)
	}
	if cfg.SitePreviewBaseURL != "http://localhost/preview" {
		t.Errorf("SitePreviewBaseURL = %q", cfg.SitePreviewBaseURL)
	}
	if cfg.SitePublicBaseURL != "http://localhost/sites" {
		t.Errorf("SitePublicBaseURL = %q", cfg.SitePublicBaseURL)
	}
	if cfg.NotificationTimeout != 10*time.Second {
		t.Errorf("NotificationTimeout = %s, want 10s", cfg.NotificationTimeout)
	}
	if cfg.AdminPanelURL != "http://localhost:3000/admin" {
		t.Errorf("AdminPanelURL = %q", cfg.AdminPanelURL)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:3000" {
		t.Errorf("CORSOrigins = %#v, want localhost origin", cfg.CORSOrigins)
	}
}

func TestLoadFromRequiresProductionSecrets(t *testing.T) {
	required := []string{
		"DATABASE_URL",
		"REDIS_ADDRESS",
		"REDIS_PASSWORD",
		"MINIO_ENDPOINT",
		"MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY",
		"MINIO_ARCHIVE_BUCKET",
		"MINIO_SITE_BUCKET",
		"MAX_UPLOAD_BYTES",
		"MAX_EXTRACTED_BYTES",
		"MAX_ARCHIVE_FILES",
		"MAX_ARCHIVE_FILE_BYTES",
		"CHROME_URL",
		"SCREENSHOT_TIMEOUT",
		"SITE_RENDER_BASE_URL",
		"SITE_PREVIEW_BASE_URL",
		"SITE_PUBLIC_BASE_URL",
		"RESEND_API_KEY",
		"EMAIL_FROM",
		"TG_BOT_TOKEN",
		"TG_ADMIN_CHAT_ID",
		"ADMIN_PANEL_URL",
		"NOTIFICATION_TIMEOUT",
		"COOKIE_DOMAIN",
		"CORS_ALLOWED_ORIGINS",
	}

	for _, missing := range required {
		t.Run(missing, func(t *testing.T) {
			values := map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://database",
				"REDIS_ADDRESS":          "redis:6379",
				"REDIS_PASSWORD":         "redis-password",
				"MINIO_ENDPOINT":         "minio:9000",
				"MINIO_ACCESS_KEY":       "access-key",
				"MINIO_SECRET_KEY":       "secret-key",
				"MINIO_ARCHIVE_BUCKET":   "archives",
				"MINIO_SITE_BUCKET":      "sites",
				"MAX_UPLOAD_BYTES":       "52428800",
				"MAX_EXTRACTED_BYTES":    "209715200",
				"MAX_ARCHIVE_FILES":      "2000",
				"MAX_ARCHIVE_FILE_BYTES": "52428800",
				"CHROME_URL":             "http://chrome:9222",
				"SCREENSHOT_TIMEOUT":     "60s",
				"SITE_RENDER_BASE_URL":   "http://minio:9000/sites",
				"SITE_PREVIEW_BASE_URL":  "https://example.com/preview",
				"SITE_PUBLIC_BASE_URL":   "https://example.com/sites",
				"RESEND_API_KEY":         "resend-key",
				"EMAIL_FROM":             "mail@example.com",
				"TG_BOT_TOKEN":           "bot-token",
				"TG_ADMIN_CHAT_ID":       "-100123",
				"ADMIN_PANEL_URL":        "https://example.com/admin",
				"NOTIFICATION_TIMEOUT":   "10s",
				"COOKIE_DOMAIN":          "example.com",
				"CORS_ALLOWED_ORIGINS":   "https://example.com",
				"SHUTDOWN_TIMEOUT":       "10s",
				"MINIO_USE_SSL":          "true",
			}
			delete(values, missing)

			_, err := LoadFrom(env(values))
			if err == nil {
				t.Fatalf("LoadFrom() error = nil, want missing %s", missing)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Fatalf("LoadFrom() error = %q, want %s", err, missing)
			}
		})
	}
}

func TestLoadFromRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		field  string
	}{
		{
			name:   "duration",
			values: map[string]string{"SHUTDOWN_TIMEOUT": "soon"},
			field:  "SHUTDOWN_TIMEOUT",
		},
		{
			name:   "session duration",
			values: map[string]string{"SESSION_TTL": "forever"},
			field:  "SESSION_TTL",
		},
		{
			name:   "boolean",
			values: map[string]string{"MINIO_USE_SSL": "sometimes"},
			field:  "MINIO_USE_SSL",
		},
		{
			name:   "redis database",
			values: map[string]string{"REDIS_DB": "cache"},
			field:  "REDIS_DB",
		},
		{
			name:   "upload limit",
			values: map[string]string{"MAX_UPLOAD_BYTES": "0"},
			field:  "MAX_UPLOAD_BYTES",
		},
		{
			name:   "extracted limit",
			values: map[string]string{"MAX_EXTRACTED_BYTES": "0"},
			field:  "MAX_EXTRACTED_BYTES",
		},
		{
			name:   "archive file count",
			values: map[string]string{"MAX_ARCHIVE_FILES": "0"},
			field:  "MAX_ARCHIVE_FILES",
		},
		{
			name:   "archive file limit",
			values: map[string]string{"MAX_ARCHIVE_FILE_BYTES": "0"},
			field:  "MAX_ARCHIVE_FILE_BYTES",
		},
		{
			name:   "screenshot timeout",
			values: map[string]string{"SCREENSHOT_TIMEOUT": "0s"},
			field:  "SCREENSHOT_TIMEOUT",
		},
		{
			name:   "notification timeout",
			values: map[string]string{"NOTIFICATION_TIMEOUT": "0s"},
			field:  "NOTIFICATION_TIMEOUT",
		},
		{
			name:   "incomplete Resend config",
			values: map[string]string{"RESEND_API_KEY": "key"},
			field:  "EMAIL_FROM",
		},
		{
			name:   "incomplete Telegram config",
			values: map[string]string{"TG_BOT_TOKEN": "token"},
			field:  "TG_ADMIN_CHAT_ID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFrom(env(test.values))
			if err == nil {
				t.Fatal("LoadFrom() error = nil")
			}
			if !strings.Contains(err.Error(), test.field) {
				t.Fatalf("LoadFrom() error = %q, want field %s", err, test.field)
			}
		})
	}
}

func TestLoadFromParsesExactCORSOrigins(t *testing.T) {
	cfg, err := LoadFrom(env(map[string]string{
		"CORS_ALLOWED_ORIGINS": " https://one.example,https://two.example ,, ",
	}))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	want := []string{"https://one.example", "https://two.example"}
	if len(cfg.CORSOrigins) != len(want) {
		t.Fatalf("CORSOrigins = %#v, want %#v", cfg.CORSOrigins, want)
	}
	for index := range want {
		if cfg.CORSOrigins[index] != want[index] {
			t.Errorf("CORSOrigins[%d] = %q, want %q", index, cfg.CORSOrigins[index], want[index])
		}
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
