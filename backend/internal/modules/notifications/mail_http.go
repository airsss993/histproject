package notifications

import (
	"context"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

// MailStatus - что панель показывает о почте. Ключи Postbox наружу не отдаются.
type MailStatus struct {
	Configured bool     `json:"configured"`
	From       string   `json:"from,omitempty"`
	Recipients []string `json:"recipients"`
}

// MailHandler - проверка почты из панели. Postbox задаётся env-переменными, здесь только статус и тестовое письмо.
type MailHandler struct {
	sender     EmailSender // nil - Postbox не настроен
	from       string
	recipients func(context.Context) ([]string, error)
	logger     *slog.Logger
}

func NewMailHandler(sender EmailSender, from string, recipients func(context.Context) ([]string, error), logger *slog.Logger) *MailHandler {
	return &MailHandler{sender: sender, from: from, recipients: recipients, logger: logger}
}

func (handler *MailHandler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/admin/mail", protect(http.HandlerFunc(handler.status)))
	mux.Handle("POST /api/v1/admin/mail/test", protect(http.HandlerFunc(handler.test)))
}

func (handler *MailHandler) status(w http.ResponseWriter, r *http.Request) {
	recipients, err := handler.recipients(r.Context())
	if err != nil {
		handler.internal(w, r, err)
		return
	}
	if recipients == nil {
		recipients = []string{}
	}
	status := MailStatus{Configured: handler.sender != nil, Recipients: recipients}
	if status.Configured {
		status.From = handler.from
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]MailStatus{"mail": status})
}

func (handler *MailHandler) test(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	to := strings.TrimSpace(request.Email)
	if address, err := mail.ParseAddress(to); err != nil || address.Address != to {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"email": "Некорректный email"})
		return
	}
	if handler.sender == nil {
		_ = httpserver.WriteError(w, http.StatusConflict, "mail_not_configured",
			"Почта не настроена: на сервере не заданы POSTBOX_KEY_ID, POSTBOX_SECRET и EMAIL_FROM", nil)
		return
	}
	message, err := checkEmail(to)
	if err != nil {
		handler.internal(w, r, err)
		return
	}
	// Текст ошибки отправителя безопасен для показа: smtpFailure оставляет только этап и код SMTP.
	if err := handler.sender.Send(r.Context(), message); err != nil {
		handler.logger.WarnContext(r.Context(), "test email failed",
			"request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
		_ = httpserver.WriteError(w, http.StatusBadGateway, "mail_unavailable", "Письмо не ушло: "+err.Error(), nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *MailHandler) internal(w http.ResponseWriter, r *http.Request, err error) {
	handler.logger.ErrorContext(r.Context(), "mail request failed",
		"request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
	_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
}
