package mailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const resendAPIURL = "https://api.resend.com/emails"

// Mailer — отправка HTML-писем через Resend API.
type Mailer struct {
	apiKey string
	from   string
	client *http.Client
}

// New создаёт Mailer. Если apiKey пустой — отправка отключена.
func New(apiKey, from string) *Mailer {
	return &Mailer{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type emailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func (m *Mailer) send(to []string, subject, html string) {
	if m.apiKey == "" {
		return
	}
	go func() {
		body, _ := json.Marshal(emailRequest{From: m.from, To: to, Subject: subject, HTML: html})
		req, err := http.NewRequest(http.MethodPost, resendAPIURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("[mailer] ошибка создания запроса: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
		resp, err := m.client.Do(req)
		if err != nil {
			log.Printf("[mailer] ошибка отправки письма: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			log.Printf("[mailer] предупреждение: Resend вернул статус %d", resp.StatusCode)
		}
	}()
}

// SendNewRequestToAdmins уведомляет администраторов о новой заявке.
func (m *Mailer) SendNewRequestToAdmins(adminEmails []string, requestID int, title, email, username, adminPanelURL string) {
	if len(adminEmails) == 0 {
		return
	}
	tg := "не указан"
	if username != "" {
		tg = "@" + username
	}
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f3f4f6; padding: 20px; margin: 0;">
  <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e5e7eb; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.05);">
    <div style="background-color: #0a192f; color: #ffffff; padding: 25px; text-align: center;">
      <h2 style="margin:0; letter-spacing: 1px; color: #ffffff; font-size: 18px;">НОВАЯ ЗАЯВКА НА МОДЕРАЦИЮ</h2>
    </div>
    <div style="padding: 30px; line-height: 1.6; color: #1f2937;">
      <p style="margin: 0 0 20px 0; font-size: 16px;">На платформе зарегистрировано новое историческое событие, ожидающее проверки.</p>
      <div style="background-color: #f9fafb; border-radius: 8px; padding: 20px; border: 1px solid #eee;">
        <p style="margin: 0 0 10px 0;"><strong>📦 Событие:</strong> %s</p>
        <p style="margin: 0 0 10px 0;"><strong>🆔 ID заявки:</strong> #%d</p>
        <p style="margin: 0 0 10px 0;"><strong>👤 Автор:</strong> %s</p>
        <p style="margin: 0;"><strong>📱 Telegram:</strong> %s</p>
      </div>
      <p style="margin: 20px 0 0 0; font-size: 14px; color: #6b7280;">Файлы и полная информация доступны в панели управления модератора.</p>
    </div>
    <div style="text-align: center; padding: 0 0 30px 0;">
      <a href="%s" target="_blank" style="display: inline-block; padding: 14px 32px; background-color: #c5a044; color: #ffffff !important; text-decoration: none; border-radius: 8px; font-weight: bold; font-size: 16px;">
        Открыть в админ-панели
      </a>
    </div>
    <div style="background-color: #f9fafb; color: #6b7280; padding: 15px; text-align: center; font-size: 12px; border-top: 1px solid #e5e7eb;">
      Системное уведомление проекта «РОССИЯ — СТРАНА ГЕРОЕВ»
    </div>
  </div>
</body>
</html>`, title, requestID, email, tg, adminPanelURL)
	m.send(adminEmails, fmt.Sprintf("📥 Новая заявка #%d: %s", requestID, title), html)
}

// SendUnderReview уведомляет пользователя о том, что заявка принята на проверку.
func (m *Mailer) SendUnderReview(toEmail string, requestID int, title string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f3f4f6; padding: 20px; margin: 0;">
  <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e5e7eb; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.05);">
    <div style="background-color: #0a192f; color: #ffffff; padding: 25px; text-align: center;">
      <h2 style="margin:0; letter-spacing: 1px; color: #ffffff; font-size: 20px;">РОССИЯ — СТРАНА ГЕРОЕВ</h2>
    </div>
    <div style="padding: 30px; line-height: 1.6; color: #1f2937;">
      <p style="margin: 0 0 15px 0;">Здравствуйте!</p>
      <p style="margin: 0 0 15px 0;">Ваша заявка на добавление события «<strong>%s</strong>» успешно получена и <strong>принята в работу</strong>.</p>
      <div style="background-color: #f0f7ff; border-left: 4px solid #3b82f6; padding: 15px; margin: 20px 0; border-radius: 4px;">
        <p style="margin: 0; color: #1e40af; font-size: 15px;">
          📍 <b>Статус:</b> На проверке модератором<br>
          🆔 <b>ID заявки:</b> #%d
        </p>
      </div>
      <p style="margin: 0 0 15px 0;">Обычно проверка занимает от 1 до 3 рабочих дней. Мы отправим вам уведомление, как только статус заявки изменится.</p>
      <p style="margin: 0;">Благодарим за участие в проекте!</p>
    </div>
    <div style="background-color: #f9fafb; color: #6b7280; padding: 15px; text-align: center; font-size: 13px; border-top: 1px solid #e5e7eb;">
      Это автоматическое уведомление. Пожалуйста, не отвечайте на него.
    </div>
  </div>
</body>
</html>`, title, requestID)
	m.send([]string{toEmail}, fmt.Sprintf("📥 Заявка принята в работу: %s", title), html)
}

// SendApproved уведомляет пользователя об одобрении и публикации события.
func (m *Mailer) SendApproved(toEmail, title, siteURL string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<style>
  .body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background-color: #f3f4f6; padding: 20px; margin: 0; }
  .card { max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e5e7eb; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.05); }
  .header { background-color: #0a192f; color: #ffffff; padding: 25px; text-align: center; }
  .content { padding: 30px; line-height: 1.6; color: #1f2937; }
  .btn-wrap { text-align: center; padding: 10px 0 30px 0; }
  .btn { display: inline-block; padding: 14px 32px; background-color: #c5a044; color: #ffffff !important; text-decoration: none; border-radius: 8px; font-weight: bold; font-size: 16px; min-width: 200px; }
  .footer { background-color: #f9fafb; color: #6b7280; padding: 15px; text-align: center; font-size: 13px; border-top: 1px solid #e5e7eb; }
</style>
</head>
<body class="body">
  <div class="card">
    <div class="header">
      <h2 style="margin:0; letter-spacing: 1px; color: #ffffff;">РОССИЯ — СТРАНА ГЕРОЕВ</h2>
    </div>
    <div class="content">
      <p>Здравствуйте!</p>
      <p>Ваше историческое событие «<strong>%s</strong>» успешно прошло проверку и теперь занимает своё место на интерактивной карте.</p>
      <p>Благодарим вас за вклад в сохранение истории и развитие платформы!</p>
    </div>
    <div class="btn-wrap">
      <a href="%s" class="btn" target="_blank">Посмотреть на карте</a>
    </div>
    <div class="footer">
      Это автоматическое уведомление. Пожалуйста, не отвечайте на него.
    </div>
  </div>
</body>
</html>`, title, siteURL)
	m.send([]string{toEmail}, "✅ Ваше событие опубликовано на интерактивной карте!", html)
}

// SendRejected уведомляет пользователя об отклонении заявки с комментарием модератора.
func (m *Mailer) SendRejected(toEmail, title, adminComment string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<style>
  .body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background-color: #f3f4f6; padding: 20px; margin: 0; }
  .card { max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e5e7eb; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.05); }
  .header { background-color: #0a192f; color: #ffffff; padding: 25px; text-align: center; }
  .content { padding: 30px; line-height: 1.6; color: #1f2937; }
  .comment-box { background-color: #fdfaf2; border-left: 4px solid #c5a044; padding: 15px; margin: 20px 0; border-radius: 4px; }
  .footer { background-color: #f9fafb; color: #6b7280; padding: 15px; text-align: center; font-size: 13px; border-top: 1px solid #e5e7eb; }
</style>
</head>
<body class="body">
  <div class="card">
    <div class="header">
      <h2 style="margin:0; letter-spacing: 1px;">РОССИЯ — СТРАНА ГЕРОЕВ</h2>
    </div>
    <div class="content">
      <p>Здравствуйте!</p>
      <p>Ваша заявка «<strong>%s</strong>» была рассмотрена модератором, но требует небольших уточнений для публикации.</p>
      <div class="comment-box">
        <strong style="color: #c5a044;">Комментарий для исправления:</strong><br>
        %s
      </div>
      <p>Пожалуйста, внесите необходимые изменения и отправьте форму повторно через главную страницу проекта.</p>
    </div>
    <div class="footer">
      Это автоматическое уведомление. Пожалуйста, не отвечайте на него.
    </div>
  </div>
</body>
</html>`, title, adminComment)
	m.send([]string{toEmail}, "📝 Требуются исправления по заявке", html)
}

// SendAdminCreated отправляет новому администратору его учётные данные.
func (m *Mailer) SendAdminCreated(toEmail, login, password, adminPanelURL string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f3f4f6; padding: 20px; margin: 0;">
  <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e5e7eb; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.05);">
    <div style="background-color: #0a192f; color: #ffffff; padding: 25px; text-align: center;">
      <h2 style="margin:0; letter-spacing: 1px; color: #ffffff; font-size: 18px;">РОССИЯ — СТРАНА ГЕРОЕВ</h2>
    </div>
    <div style="padding: 30px; line-height: 1.6; color: #1f2937;">
      <p style="margin: 0 0 15px 0;">Здравствуйте!</p>
      <p style="margin: 0 0 15px 0;">Вы назначены администратором в системе модерации проекта. Ниже указаны ваши данные для доступа:</p>
      <div style="background-color: #f9fafb; border-radius: 8px; padding: 20px; border: 1px solid #e5e7eb; margin: 20px 0;">
        <p style="margin: 0 0 10px 0;"><strong>🔑 Данные для входа:</strong></p>
        <p style="margin: 0 0 5px 0;"><b>Логин:</b> %s</p>
        <p style="margin: 0 0 5px 0;"><b>Пароль:</b> <span style="background-color: #eee; padding: 2px 5px; border-radius: 3px; font-family: monospace;">%s</span></p>
      </div>
      <p style="margin: 0;">Добро пожаловать в команду!</p>
    </div>
    <div style="text-align: center; padding: 0 0 30px 0;">
      <a href="%s" target="_blank" style="display: inline-block; padding: 14px 32px; background-color: #c5a044; color: #ffffff !important; text-decoration: none; border-radius: 8px; font-weight: bold; font-size: 16px;">
        Войти в панель управления
      </a>
    </div>
    <div style="background-color: #f9fafb; color: #6b7280; padding: 15px; text-align: center; font-size: 12px; border-top: 1px solid #e5e7eb;">
      Системное уведомление проекта. Сохраните это письмо в надёжном месте.
    </div>
  </div>
</body>
</html>`, login, password, adminPanelURL)
	m.send([]string{toEmail}, "🛡 Данные для входа в систему модерации", html)
}
