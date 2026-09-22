package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/notifications"
	"github.com/airsss993/histproject/backend/internal/modules/submissions"
	"github.com/airsss993/histproject/backend/internal/modules/users"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
)

// NewCodeMailer шлёт код входа через Postbox. Без POSTBOX_KEY_ID (только dev, прод без него не стартует) код пишется в лог.
func NewCodeMailer(cfg config.Config, logger *slog.Logger, repository *notifications.Repository) users.Mailer {
	if cfg.PostboxKeyID == "" {
		return logCodeMailer{logger: logger}
	}
	sender, err := newEmailSender(cfg, repository)
	return postboxCodeMailer{sender: sender, err: err}
}

type postboxCodeMailer struct {
	sender notifications.EmailSender
	err    error
}

func (mailer postboxCodeMailer) SendCode(ctx context.Context, email, code string) error {
	if mailer.err != nil {
		return mailer.err
	}
	message, err := notifications.LoginCodeEmail(email, code)
	if err != nil {
		return err
	}
	return mailer.sender.Send(ctx, message)
}

type logCodeMailer struct {
	logger *slog.Logger
}

func (mailer logCodeMailer) SendCode(ctx context.Context, email, code string) error {
	mailer.logger.WarnContext(ctx, "POSTBOX_KEY_ID не задан, код входа только в логе", "email", email, "code", code)
	return nil
}

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
	repository := notifications.NewRepository(databaseClient.Pool())
	emailSender, err := newEmailSender(cfg, repository)
	if err != nil {
		return nil, err
	}
	return notifications.NewService(
		repository,
		notificationSubmissionSource{service: submissionService},
		notificationAdminSource{service: adminService},
		emailSender,
		notifications.NewBot(repository, cfg.NotificationTimeout),
		cfg.AdminPanelURL,
	), nil
}

// newEmailSender - Postbox из env. Без POSTBOX_KEY_ID отдаёт nil: почта не настроена.
// Отправителя, заданного в панели, читает перед каждым письмом, EMAIL_FROM - запасной.
func newEmailSender(cfg config.Config, repository *notifications.Repository) (notifications.EmailSender, error) {
	if cfg.PostboxKeyID == "" {
		return nil, nil
	}
	sender, err := notifications.NewPostboxSender(cfg.PostboxAddress, cfg.PostboxKeyID, cfg.PostboxSecret, cfg.EmailFrom, cfg.NotificationTimeout)
	if err != nil {
		return nil, err
	}
	return sender.WithFromOverride(repository.MailFrom), nil
}

// NewMailHandler - почта в панели. Ошибка настройки Postbox всплывает при отправке тестового письма.
func NewMailHandler(cfg config.Config, logger *slog.Logger, repository *notifications.Repository) *notifications.MailHandler {
	sender, err := newEmailSender(cfg, repository)
	if err != nil {
		sender = brokenSender{err: err}
	}
	return notifications.NewMailHandler(sender, cfg.EmailFrom, repository, logger)
}

type brokenSender struct {
	err error
}

func (sender brokenSender) Send(context.Context, notifications.Email) error {
	return sender.err
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
		PublicID: submission.PublicID, Title: submission.Title, Email: submission.Email,
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
