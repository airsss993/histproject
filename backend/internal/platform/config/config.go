package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	development = "development"
	production  = "production"
)

type Config struct {
	Environment         string
	ServerAddress       string
	ShutdownTimeout     time.Duration
	SessionTTL          time.Duration
	DatabaseURL         string
	RedisAddress        string
	RedisPassword       string
	RedisDB             int
	MinIOEndpoint       string
	MinIOAccessKey      string
	MinIOSecretKey      string
	MinIOArchiveBucket  string
	MinIOSiteBucket     string
	MinIOUseSSL         bool
	MaxUploadBytes      int64
	MaxExtractedBytes   int64
	MaxArchiveFiles     int
	MaxArchiveFileBytes int64
	ChromeURL           string
	ScreenshotTimeout   time.Duration
	SiteRenderBaseURL   string
	SitePreviewBaseURL  string
	SitePublicBaseURL   string
	ResendAPIKey        string
	EmailFrom           string
	TelegramBotToken    string
	TelegramAdminChatID string
	AdminPanelURL       string
	NotificationTimeout time.Duration
	CookieDomain        string
	CORSOrigins         []string
}

func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

func LoadFrom(getenv func(string) string) (Config, error) {
	environment := valueOrDefault(getenv, "APP_ENV", development)
	cfg := Config{
		Environment:         environment,
		ServerAddress:       valueOrDefault(getenv, "SERVER_ADDRESS", ":7666"),
		DatabaseURL:         strings.TrimSpace(getenv("DATABASE_URL")),
		RedisAddress:        strings.TrimSpace(getenv("REDIS_ADDRESS")),
		RedisPassword:       strings.TrimSpace(getenv("REDIS_PASSWORD")),
		MinIOEndpoint:       strings.TrimSpace(getenv("MINIO_ENDPOINT")),
		MinIOAccessKey:      strings.TrimSpace(getenv("MINIO_ACCESS_KEY")),
		MinIOSecretKey:      strings.TrimSpace(getenv("MINIO_SECRET_KEY")),
		MinIOArchiveBucket:  strings.TrimSpace(getenv("MINIO_ARCHIVE_BUCKET")),
		MinIOSiteBucket:     strings.TrimSpace(getenv("MINIO_SITE_BUCKET")),
		ChromeURL:           strings.TrimSpace(getenv("CHROME_URL")),
		SiteRenderBaseURL:   strings.TrimRight(strings.TrimSpace(getenv("SITE_RENDER_BASE_URL")), "/"),
		SitePreviewBaseURL:  strings.TrimRight(strings.TrimSpace(getenv("SITE_PREVIEW_BASE_URL")), "/"),
		SitePublicBaseURL:   strings.TrimRight(strings.TrimSpace(getenv("SITE_PUBLIC_BASE_URL")), "/"),
		ResendAPIKey:        strings.TrimSpace(getenv("RESEND_API_KEY")),
		EmailFrom:           strings.TrimSpace(getenv("EMAIL_FROM")),
		TelegramBotToken:    strings.TrimSpace(getenv("TG_BOT_TOKEN")),
		TelegramAdminChatID: strings.TrimSpace(getenv("TG_ADMIN_CHAT_ID")),
		AdminPanelURL:       strings.TrimRight(strings.TrimSpace(getenv("ADMIN_PANEL_URL")), "/"),
		CookieDomain:        strings.TrimSpace(getenv("COOKIE_DOMAIN")),
		CORSOrigins:         splitList(getenv("CORS_ALLOWED_ORIGINS")),
	}
	if environment != production {
		applyDevelopmentDefaults(&cfg)
	}

	shutdownTimeout, err := parseDuration(valueOrDefault(getenv, "SHUTDOWN_TIMEOUT", "10s"), "SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	cfg.ShutdownTimeout = shutdownTimeout

	sessionTTL, err := parseDuration(valueOrDefault(getenv, "SESSION_TTL", "12h"), "SESSION_TTL")
	if err != nil {
		return Config{}, err
	}
	cfg.SessionTTL = sessionTTL

	redisDB, err := parseNonNegativeInt(valueOrDefault(getenv, "REDIS_DB", "0"), "REDIS_DB")
	if err != nil {
		return Config{}, err
	}
	cfg.RedisDB = redisDB

	maxUploadBytesValue := strings.TrimSpace(getenv("MAX_UPLOAD_BYTES"))
	if environment != production && maxUploadBytesValue == "" {
		maxUploadBytesValue = "52428800"
	}
	if maxUploadBytesValue != "" {
		maxUploadBytes, err := parsePositiveInt64(maxUploadBytesValue, "MAX_UPLOAD_BYTES")
		if err != nil {
			return Config{}, err
		}
		cfg.MaxUploadBytes = maxUploadBytes
	}

	maxExtractedBytesValue := developmentValue(getenv, environment, "MAX_EXTRACTED_BYTES", "209715200")
	if maxExtractedBytesValue != "" {
		maxExtractedBytes, err := parsePositiveInt64(maxExtractedBytesValue, "MAX_EXTRACTED_BYTES")
		if err != nil {
			return Config{}, err
		}
		cfg.MaxExtractedBytes = maxExtractedBytes
	}

	maxArchiveFilesValue := developmentValue(getenv, environment, "MAX_ARCHIVE_FILES", "2000")
	if maxArchiveFilesValue != "" {
		maxArchiveFiles, err := parsePositiveInt(maxArchiveFilesValue, "MAX_ARCHIVE_FILES")
		if err != nil {
			return Config{}, err
		}
		cfg.MaxArchiveFiles = maxArchiveFiles
	}

	maxArchiveFileBytesValue := developmentValue(getenv, environment, "MAX_ARCHIVE_FILE_BYTES", "52428800")
	if maxArchiveFileBytesValue != "" {
		maxArchiveFileBytes, err := parsePositiveInt64(maxArchiveFileBytesValue, "MAX_ARCHIVE_FILE_BYTES")
		if err != nil {
			return Config{}, err
		}
		cfg.MaxArchiveFileBytes = maxArchiveFileBytes
	}

	screenshotTimeoutValue := developmentValue(getenv, environment, "SCREENSHOT_TIMEOUT", "60s")
	if screenshotTimeoutValue != "" {
		screenshotTimeout, err := parseDuration(screenshotTimeoutValue, "SCREENSHOT_TIMEOUT")
		if err != nil {
			return Config{}, err
		}
		cfg.ScreenshotTimeout = screenshotTimeout
	}

	notificationTimeoutValue := developmentValue(getenv, environment, "NOTIFICATION_TIMEOUT", "10s")
	if notificationTimeoutValue != "" {
		notificationTimeout, err := parseDuration(notificationTimeoutValue, "NOTIFICATION_TIMEOUT")
		if err != nil {
			return Config{}, err
		}
		cfg.NotificationTimeout = notificationTimeout
	}

	minIOUseSSL, err := parseBool(valueOrDefault(getenv, "MINIO_USE_SSL", "false"), "MINIO_USE_SSL")
	if err != nil {
		return Config{}, err
	}
	cfg.MinIOUseSSL = minIOUseSSL

	if cfg.Environment == production {
		if err := validateProduction(cfg); err != nil {
			return Config{}, err
		}
	}
	if (cfg.ResendAPIKey == "") != (cfg.EmailFrom == "") {
		return Config{}, errors.New("RESEND_API_KEY и EMAIL_FROM должны быть указаны вместе")
	}
	if (cfg.TelegramBotToken == "") != (cfg.TelegramAdminChatID == "") {
		return Config{}, errors.New("TG_BOT_TOKEN и TG_ADMIN_CHAT_ID должны быть указаны вместе")
	}

	return cfg, nil
}

func applyDevelopmentDefaults(cfg *Config) {
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = "postgres://postgres:postgres@localhost:5432/histproject?sslmode=disable"
	}
	if cfg.RedisAddress == "" {
		cfg.RedisAddress = "localhost:6379"
	}
	if cfg.MinIOEndpoint == "" {
		cfg.MinIOEndpoint = "localhost:9000"
	}
	if cfg.MinIOArchiveBucket == "" {
		cfg.MinIOArchiveBucket = "archives"
	}
	if cfg.MinIOSiteBucket == "" {
		cfg.MinIOSiteBucket = "sites"
	}
	if cfg.ChromeURL == "" {
		cfg.ChromeURL = "http://localhost:9222"
	}
	if cfg.SiteRenderBaseURL == "" {
		cfg.SiteRenderBaseURL = "http://localhost:9000/sites"
	}
	if cfg.SitePreviewBaseURL == "" {
		cfg.SitePreviewBaseURL = "http://localhost/preview"
	}
	if cfg.SitePublicBaseURL == "" {
		cfg.SitePublicBaseURL = "http://localhost/sites"
	}
	if cfg.AdminPanelURL == "" {
		cfg.AdminPanelURL = "http://localhost:3000/admin"
	}
	if len(cfg.CORSOrigins) == 0 {
		cfg.CORSOrigins = []string{"http://localhost:3000"}
	}
}

func valueOrDefault(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func developmentValue(getenv func(string) string, environment, key, fallback string) string {
	value := strings.TrimSpace(getenv(key))
	if value == "" && environment != production {
		return fallback
	}
	return value
}

func parseDuration(value, key string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s: ожидается положительная duration", key)
	}
	return duration, nil
}

func parseBool(value, key string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: ожидается boolean", key)
	}
	return parsed, nil
}

func parseNonNegativeInt(value, key string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s: ожидается неотрицательное целое число", key)
	}
	return parsed, nil
}

func parsePositiveInt(value, key string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s: ожидается положительное целое число", key)
	}
	return parsed, nil
}

func parsePositiveInt64(value, key string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s: ожидается положительное целое число", key)
	}
	return parsed, nil
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func validateProduction(cfg Config) error {
	values := []struct {
		key   string
		value string
	}{
		{"DATABASE_URL", cfg.DatabaseURL},
		{"REDIS_ADDRESS", cfg.RedisAddress},
		{"REDIS_PASSWORD", cfg.RedisPassword},
		{"MINIO_ENDPOINT", cfg.MinIOEndpoint},
		{"MINIO_ACCESS_KEY", cfg.MinIOAccessKey},
		{"MINIO_SECRET_KEY", cfg.MinIOSecretKey},
		{"MINIO_ARCHIVE_BUCKET", cfg.MinIOArchiveBucket},
		{"MINIO_SITE_BUCKET", cfg.MinIOSiteBucket},
		{"CHROME_URL", cfg.ChromeURL},
		{"SITE_RENDER_BASE_URL", cfg.SiteRenderBaseURL},
		{"SITE_PREVIEW_BASE_URL", cfg.SitePreviewBaseURL},
		{"SITE_PUBLIC_BASE_URL", cfg.SitePublicBaseURL},
		{"RESEND_API_KEY", cfg.ResendAPIKey},
		{"EMAIL_FROM", cfg.EmailFrom},
		{"TG_BOT_TOKEN", cfg.TelegramBotToken},
		{"TG_ADMIN_CHAT_ID", cfg.TelegramAdminChatID},
		{"ADMIN_PANEL_URL", cfg.AdminPanelURL},
		{"COOKIE_DOMAIN", cfg.CookieDomain},
	}

	var validationErrors []error
	for _, item := range values {
		if item.value == "" {
			validationErrors = append(validationErrors, fmt.Errorf("%s: обязательная переменная", item.key))
		}
	}
	if len(cfg.CORSOrigins) == 0 {
		validationErrors = append(validationErrors, errors.New("CORS_ALLOWED_ORIGINS: обязательная переменная"))
	}
	if cfg.MaxUploadBytes <= 0 {
		validationErrors = append(validationErrors, errors.New("MAX_UPLOAD_BYTES: обязательная переменная"))
	}
	if cfg.MaxExtractedBytes <= 0 {
		validationErrors = append(validationErrors, errors.New("MAX_EXTRACTED_BYTES: обязательная переменная"))
	}
	if cfg.MaxArchiveFiles <= 0 {
		validationErrors = append(validationErrors, errors.New("MAX_ARCHIVE_FILES: обязательная переменная"))
	}
	if cfg.MaxArchiveFileBytes <= 0 {
		validationErrors = append(validationErrors, errors.New("MAX_ARCHIVE_FILE_BYTES: обязательная переменная"))
	}
	if cfg.ScreenshotTimeout <= 0 {
		validationErrors = append(validationErrors, errors.New("SCREENSHOT_TIMEOUT: обязательная переменная"))
	}
	if cfg.NotificationTimeout <= 0 {
		validationErrors = append(validationErrors, errors.New("NOTIFICATION_TIMEOUT: обязательная переменная"))
	}

	return errors.Join(validationErrors...)
}
