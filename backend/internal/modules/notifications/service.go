package notifications

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"
)

type DeliveryStore interface {
	Claim(context.Context, string, Channel) (bool, error)
	MarkSent(context.Context, string, Channel) error
	MarkFailed(context.Context, string, Channel, string) error
}

type SubmissionSource interface {
	NotificationSubmission(context.Context, int64) (Submission, error)
}

type AdminSource interface {
	NotificationAdminEmails(context.Context) ([]string, error)
}

type EmailSender interface {
	Send(context.Context, Email) error
}

type TelegramSender interface {
	Send(context.Context, string) error
}

type Service struct {
	store         DeliveryStore
	submissions   SubmissionSource
	admins        AdminSource
	email         EmailSender
	telegram      TelegramSender
	adminPanelURL string
}

func NewService(store DeliveryStore, submissions SubmissionSource, admins AdminSource, email EmailSender, telegram TelegramSender, adminPanelURL string) *Service {
	return &Service{
		store: store, submissions: submissions, admins: admins, email: email,
		telegram: telegram, adminPanelURL: strings.TrimRight(adminPanelURL, "/"),
	}
}

func (service *Service) Deliver(ctx context.Context, submissionID int64, action Action) error {
	if submissionID <= 0 || !action.Valid() {
		return ErrInvalidAction
	}
	submission, err := service.submissions.NotificationSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("load submission for notification: %w", err)
	}
	eventKey := fmt.Sprintf("submission:%d:%s", submissionID, action)

	if action == ActionCreated || action == ActionReady {
		return service.deliverToAdmins(ctx, eventKey, action, submission)
	}
	if service.email == nil || strings.TrimSpace(submission.Email) == "" {
		return nil
	}
	message := authorEmail(action, submission)
	return service.deliver(ctx, eventKey, ChannelEmail, func(ctx context.Context) error {
		return service.email.Send(ctx, message)
	})
}

func (service *Service) deliverToAdmins(ctx context.Context, eventKey string, action Action, submission Submission) error {
	var deliveryErr error
	if service.email != nil && service.admins != nil {
		emails, err := service.admins.NotificationAdminEmails(ctx)
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("load notification admin emails: %w", err))
		} else if len(emails) > 0 {
			message := adminEmail(action, submission, emails, service.adminPanelURL)
			deliveryErr = errors.Join(deliveryErr, service.deliver(ctx, eventKey, ChannelEmail, func(ctx context.Context) error {
				return service.email.Send(ctx, message)
			}))
		}
	}
	if service.telegram != nil {
		message := adminTelegram(action, submission, service.adminPanelURL)
		deliveryErr = errors.Join(deliveryErr, service.deliver(ctx, eventKey, ChannelTelegram, func(ctx context.Context) error {
			return service.telegram.Send(ctx, message)
		}))
	}
	return deliveryErr
}

func (service *Service) deliver(ctx context.Context, eventKey string, channel Channel, send func(context.Context) error) error {
	claimed, err := service.store.Claim(ctx, eventKey, channel)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if err := send(ctx); err != nil {
		journalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		markErr := service.store.MarkFailed(journalCtx, eventKey, channel, err.Error())
		cancel()
		return errors.Join(fmt.Errorf("send %s notification: %w", channel, err), markErr)
	}
	journalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	err = service.store.MarkSent(journalCtx, eventKey, channel)
	cancel()
	if err != nil {
		return err
	}
	return nil
}

func adminEmail(action Action, submission Submission, recipients []string, adminPanelURL string) Email {
	status := "создана и обрабатывается"
	if action == ActionReady {
		status = "готова к проверке"
	}
	return Email{
		To:      append([]string(nil), recipients...),
		Subject: fmt.Sprintf("Заявка #%d %s", submission.ID, status),
		HTML: fmt.Sprintf(
			"<p>Заявка #%d %s.</p><p><b>Название:</b> %s</p><p><b>Email:</b> %s</p><p><b>Telegram:</b> %s</p><p><a href=\"%s\">Открыть панель</a></p>",
			submission.ID, status, html.EscapeString(submission.Title), html.EscapeString(submission.Email),
			html.EscapeString(telegramName(submission.TelegramUsername)), html.EscapeString(adminPanelURL),
		),
	}
}

func authorEmail(action Action, submission Submission) Email {
	subject := fmt.Sprintf("Статус заявки #%d изменен", submission.ID)
	body := "Заявка взята на проверку."
	switch action {
	case ActionRejected:
		subject = fmt.Sprintf("Заявка #%d отклонена", submission.ID)
		body = "Заявка отклонена. Комментарий: " + html.EscapeString(submission.Comment)
	case ActionPublished:
		subject = fmt.Sprintf("Заявка #%d опубликована", submission.ID)
		body = fmt.Sprintf("Заявка опубликована. <a href=\"%s\">Открыть событие</a>", html.EscapeString(submission.SiteURL))
	}
	return Email{
		To:      []string{submission.Email},
		Subject: subject,
		HTML:    fmt.Sprintf("<p>%s</p><p><b>%s</b></p>", body, html.EscapeString(submission.Title)),
	}
}

func adminTelegram(action Action, submission Submission, adminPanelURL string) string {
	status := "создана и обрабатывается"
	if action == ActionReady {
		status = "готова к проверке"
	}
	return fmt.Sprintf(
		"<b>Заявка #%d %s</b>\nНазвание: %s\nEmail: %s\nTelegram: %s\n<a href=\"%s\">Открыть панель</a>",
		submission.ID, status, html.EscapeString(submission.Title), html.EscapeString(submission.Email),
		html.EscapeString(telegramName(submission.TelegramUsername)), html.EscapeString(adminPanelURL),
	)
}

func telegramName(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return "не указан"
	}
	return "@" + username
}
