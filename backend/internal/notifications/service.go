package notifications

import (
	"context"
	"fmt"

	"github.com/airsss993/histproject-backend/pkg/mailer"
	"github.com/airsss993/histproject-backend/pkg/telegram"
)

// Service — оркестрирует отправку уведомлений о событиях с заявками.
type Service struct {
	mailer        *mailer.Mailer
	repo          *Repository
	tgBotToken    string
	tgAdminChatID string
	adminPanelURL string
}

// New создаёт сервис уведомлений.
func New(m *mailer.Mailer, repo *Repository, tgBotToken, tgAdminChatID, adminPanelURL string) *Service {
	return &Service{
		mailer:        m,
		repo:          repo,
		tgBotToken:    tgBotToken,
		tgAdminChatID: tgAdminChatID,
		adminPanelURL: adminPanelURL,
	}
}

// OnNewRequest уведомляет администраторов о новой заявке (email + Telegram-группа).
func (s *Service) OnNewRequest(ctx context.Context, requestID int, title, email, username string) {
	// Отправляем email всем администраторам
	adminEmails, _ := s.repo.GetAdminEmails(ctx)
	s.mailer.SendNewRequestToAdmins(adminEmails, requestID, title, email, username, s.adminPanelURL)

	// Отправляем уведомление в Telegram-группу админов
	tg := "не указан"
	if username != "" {
		tg = "@" + username
	}
	text := fmt.Sprintf(
		"<b>⚡️ НОВАЯ ЗАЯВКА #%d</b>\n\n<b>📂 СВЕДЕНИЯ</b>\n<b>Название:</b> «%s»\n<b>Статус:</b> Ожидает проверки\n\n<b>👤 КОНТАКТЫ</b>\n<b>Email:</b> %s\n<b>Telegram:</b> %s\n\n<b>📥 <a href=\"%s\">Рассмотреть в админке</a></b>",
		requestID, title, email, tg, s.adminPanelURL,
	)
	telegram.SendMessage(s.tgBotToken, s.tgAdminChatID, text)
}

// OnUnderReview уведомляет пользователя о том, что заявка взята на проверку.
func (s *Service) OnUnderReview(_ context.Context, requestID int, title, email, username string) {
	s.mailer.SendUnderReview(email, requestID, title)
}

// OnApproved уведомляет пользователя об одобрении заявки.
func (s *Service) OnApproved(_ context.Context, requestID int, title, email, username, siteURL string) {
	s.mailer.SendApproved(email, title, siteURL)
}

// OnRejected уведомляет пользователя об отклонении заявки.
func (s *Service) OnRejected(_ context.Context, requestID int, title, email, username, comment string) {
	s.mailer.SendRejected(email, title, comment)
}

// OnAdminCreated отправляет новому администратору его учётные данные.
func (s *Service) OnAdminCreated(_ context.Context, email, login, password, role string) {
	s.mailer.SendAdminCreated(email, login, password, s.adminPanelURL)
}
