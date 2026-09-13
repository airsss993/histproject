package app

import (
	"context"
	"strings"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/notifications"
	"github.com/airsss993/histproject/backend/internal/modules/submissions"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
)

type notificationQueue interface {
	EnqueueSubmissionNotification(context.Context, int64, string) error
}

type SubmissionNotifier struct {
	queue notificationQueue
}

func NewSubmissionNotifier(queue notificationQueue) *SubmissionNotifier {
	return &SubmissionNotifier{queue: queue}
}

func (notifier *SubmissionNotifier) SubmissionChanged(ctx context.Context, submission submissions.Submission, action string) error {
	return notifier.queue.EnqueueSubmissionNotification(ctx, submission.ID, action)
}

func NewNotificationService(cfg config.Config, databaseClient *database.Database, submissionService *submissions.Service, adminService *admins.Service) (*notifications.Service, error) {
	var emailSender notifications.EmailSender
	if cfg.ResendAPIKey != "" {
		sender, err := notifications.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom, cfg.NotificationTimeout)
		if err != nil {
			return nil, err
		}
		emailSender = sender
	}
	var telegramSender notifications.TelegramSender
	if cfg.TelegramBotToken != "" {
		sender, err := notifications.NewTelegramSender(cfg.TelegramBotToken, cfg.TelegramAdminChatID, cfg.NotificationTimeout)
		if err != nil {
			return nil, err
		}
		telegramSender = sender
	}
	return notifications.NewService(
		notifications.NewRepository(databaseClient.Pool()),
		notificationSubmissionSource{service: submissionService},
		notificationAdminSource{service: adminService},
		emailSender,
		telegramSender,
		cfg.AdminPanelURL,
	), nil
}

type notificationSubmissionSource struct {
	service *submissions.Service
}

func (source notificationSubmissionSource) NotificationSubmission(ctx context.Context, submissionID int64) (notifications.Submission, error) {
	submission, err := source.service.Get(ctx, submissionID)
	if err != nil {
		return notifications.Submission{}, err
	}
	return notifications.Submission{
		ID: submission.ID, Title: submission.Title, Email: submission.Email,
		TelegramUsername: submission.TelegramUsername, SiteURL: submission.SiteURL,
		Comment: submission.AdminComment,
	}, nil
}

type notificationAdminSource struct {
	service *admins.Service
}

func (source notificationAdminSource) NotificationAdminEmails(ctx context.Context) ([]string, error) {
	adminList, err := source.service.List(ctx)
	if err != nil {
		return nil, err
	}
	emails := make([]string, 0, len(adminList))
	for _, admin := range adminList {
		if email := strings.TrimSpace(admin.Email); email != "" {
			emails = append(emails, email)
		}
	}
	return emails, nil
}
