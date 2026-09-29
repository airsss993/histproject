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
	Chats(context.Context) ([]int64, error)
	Send(context.Context, int64, string) error
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
	if action == ActionReady {
		return nil
	}
	submission, err := service.submissions.NotificationSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("load submission for notification: %w", err)
	}
	eventKey := fmt.Sprintf("submission:%d:%s", submissionID, action)

	if action == ActionCreated {
		return service.deliverToAdmins(ctx, eventKey, submission)
	}
	if service.email == nil || strings.TrimSpace(submission.Email) == "" {
		return nil
	}
	message, err := authorEmail(action, submission)
	if err != nil {
		return err
	}
	return service.deliver(ctx, eventKey, ChannelEmail, func(ctx context.Context) error {
		return service.email.Send(ctx, message)
	})
}

func (service *Service) deliverToAdmins(ctx context.Context, eventKey string, submission Submission) error {
	var deliveryErr error
	if service.email != nil && service.admins != nil {
		emails, err := service.admins.NotificationAdminEmails(ctx)
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("load notification admin emails: %w", err))
		} else if len(emails) > 0 {
			message, err := adminEmail(submission, emails, service.adminPanelURL)
			if err != nil {
				deliveryErr = errors.Join(deliveryErr, err)
			} else {
				deliveryErr = errors.Join(deliveryErr, service.deliver(ctx, eventKey, ChannelEmail, func(ctx context.Context) error {
					return service.email.Send(ctx, message)
				}))
			}
		}
	}
	if service.telegram != nil {
		chats, err := service.telegram.Chats(ctx)
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("load telegram recipients: %w", err))
		}
		message := adminTelegram(submission, service.adminPanelURL)
		// Журнал доставки ведётся по каждому чату: повтор после сбоя не шлёт дубль тем, до кого уже дошло.
		for _, chatID := range chats {
			deliveryErr = errors.Join(deliveryErr, service.deliver(ctx, fmt.Sprintf("%s:%d", eventKey, chatID), ChannelTelegram, func(ctx context.Context) error {
				return service.telegram.Send(ctx, chatID, message)
			}))
		}
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

func adminTelegram(submission Submission, adminPanelURL string) string {
	// Контакты автора сюда не идут: Telegram - трансграничная передача, они есть в панели.
	return fmt.Sprintf(
		"<b>Заявка %s создана и обрабатывается</b>\nНазвание: %s\n<a href=\"%s\">Открыть панель</a>",
		submission.PublicID, html.EscapeString(submission.Title), html.EscapeString(adminPanelURL),
	)
}

func telegramName(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return "не указан"
	}
	return "@" + username
}
