package notifications

import (
	"context"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

// MailStatus - что панель показывает о почте. Ключи Postbox наружу не отдаются.
// Name и Address - текущий отправитель, Domain - домен EMAIL_FROM, подтверждённый в Postbox.
type MailStatus struct {
	Configured bool   `json:"configured"`
	Name       string `json:"name,omitempty"`
	Address    string `json:"address,omitempty"`
	Domain     string `json:"domain,omitempty"`
}

// MailSenderStore хранит отправителя, заданного в панели. Пустой address - не задан.
type MailSenderStore interface {
	MailSender(context.Context) (string, string, error)
	SaveMailSender(ctx context.Context, name, address string) error
}

// MailHandler - почта в панели: статус, отправитель и тестовое письмо. Ключи Postbox задаются env-переменными.
type MailHandler struct {
	sender      EmailSender // nil - Postbox не настроен
	defaultFrom *mail.Address
	store       MailSenderStore
	logger      *slog.Logger
}

func NewMailHandler(sender EmailSender, defaultFrom string, store MailSenderStore, logger *slog.Logger) *MailHandler {
	handler := &MailHandler{sender: sender, store: store, logger: logger}
	if parsed, err := mail.ParseAddress(strings.TrimSpace(defaultFrom)); err == nil {
		handler.defaultFrom = parsed
	}
	return handler
}

func (handler *MailHandler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/admin/mail", protect(http.HandlerFunc(handler.status)))
	mux.Handle("PUT /api/v1/admin/mail/sender", protect(http.HandlerFunc(handler.saveSender)))
	mux.Handle("POST /api/v1/admin/mail/test", protect(http.HandlerFunc(handler.test)))
}

func (handler *MailHandler) configured() bool {
	return handler.sender != nil && handler.defaultFrom != nil
}

func (handler *MailHandler) status(w http.ResponseWriter, r *http.Request) {
	status, err := handler.current(r.Context())
	if err != nil {
		handler.internal(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]MailStatus{"mail": status})
}

func (handler *MailHandler) current(ctx context.Context) (MailStatus, error) {
	if !handler.configured() {
		return MailStatus{}, nil
	}
	status := MailStatus{Configured: true, Name: handler.defaultFrom.Name, Address: handler.defaultFrom.Address, Domain: domainOf(handler.defaultFrom.Address)}
	name, address, err := handler.store.MailSender(ctx)
	if err != nil {
		return MailStatus{}, err
	}
	if address != "" {
		status.Name, status.Address = name, address
	}
	return status, nil
}

func (handler *MailHandler) saveSender(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	if !handler.configured() {
		handler.notConfigured(w)
		return
	}
	name, address := strings.TrimSpace(request.Name), strings.TrimSpace(request.Address)
	fields := map[string]string{}
	if utf8.RuneCountInString(name) > 100 {
		fields["name"] = "Не длиннее 100 символов"
	} else if strings.ContainsFunc(name, unicode.IsControl) {
		fields["name"] = "Недопустимые символы"
	}
	domain := domainOf(handler.defaultFrom.Address)
	if parsed, err := mail.ParseAddress(address); err != nil || parsed.Address != address {
		fields["address"] = "Некорректный email"
	} else if !strings.EqualFold(domainOf(address), domain) {
		// Postbox принимает письма только от подтверждённого домена.
		fields["address"] = "Адрес должен быть на домене " + domain
	}
	if len(fields) > 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
		return
	}
	if err := handler.store.SaveMailSender(r.Context(), name, address); err != nil {
		handler.internal(w, r, err)
		return
	}
	handler.status(w, r)
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
	if !handler.configured() {
		handler.notConfigured(w)
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

func (handler *MailHandler) notConfigured(w http.ResponseWriter) {
	_ = httpserver.WriteError(w, http.StatusConflict, "mail_not_configured",
		"Почта не настроена: на сервере не заданы POSTBOX_KEY_ID, POSTBOX_SECRET и EMAIL_FROM", nil)
}

func (handler *MailHandler) internal(w http.ResponseWriter, r *http.Request, err error) {
	handler.logger.ErrorContext(r.Context(), "mail request failed",
		"request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
	_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
}

func domainOf(address string) string {
	return address[strings.LastIndex(address, "@")+1:]
}
