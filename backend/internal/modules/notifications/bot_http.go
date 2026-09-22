package notifications

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

type BotService interface {
	Status(context.Context) (BotStatus, error)
	SetToken(context.Context, string) (BotStatus, error)
	Candidates(context.Context) ([]TelegramChat, error)
	Grant(context.Context, int64) (BotStatus, error)
	Revoke(context.Context, int64) (BotStatus, error)
	SendTest(context.Context) ([]TelegramChat, error)
}

type BotHandler struct {
	bot    BotService
	logger *slog.Logger
}

func NewBotHandler(bot BotService, logger *slog.Logger) *BotHandler {
	return &BotHandler{bot: bot, logger: logger}
}

func (handler *BotHandler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/admin/bot", protect(http.HandlerFunc(handler.status)))
	mux.Handle("PUT /api/v1/admin/bot/token", protect(http.HandlerFunc(handler.setToken)))
	mux.Handle("GET /api/v1/admin/bot/candidates", protect(http.HandlerFunc(handler.candidates)))
	mux.Handle("POST /api/v1/admin/bot/recipients", protect(http.HandlerFunc(handler.grant)))
	mux.Handle("DELETE /api/v1/admin/bot/recipients/{chatId}", protect(http.HandlerFunc(handler.revoke)))
	mux.Handle("POST /api/v1/admin/bot/test", protect(http.HandlerFunc(handler.test)))
}

func (handler *BotHandler) status(w http.ResponseWriter, r *http.Request) {
	status, err := handler.bot.Status(r.Context())
	handler.writeStatus(w, r, status, err)
}

func (handler *BotHandler) setToken(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	status, err := handler.bot.SetToken(r.Context(), request.Token)
	handler.writeStatus(w, r, status, err)
}

func (handler *BotHandler) candidates(w http.ResponseWriter, r *http.Request) {
	chats, err := handler.bot.Candidates(r.Context())
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string][]TelegramChat{"items": chats})
}

func (handler *BotHandler) grant(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ChatID int64 `json:"chatId"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	if request.ChatID == 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"chatId": "Укажите чат"})
		return
	}
	status, err := handler.bot.Grant(r.Context(), request.ChatID)
	handler.writeStatus(w, r, status, err)
}

func (handler *BotHandler) revoke(w http.ResponseWriter, r *http.Request) {
	chatID, err := strconv.ParseInt(r.PathValue("chatId"), 10, 64)
	if err != nil || chatID == 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"chatId": "Ожидается ID чата"})
		return
	}
	status, err := handler.bot.Revoke(r.Context(), chatID)
	handler.writeStatus(w, r, status, err)
}

func (handler *BotHandler) test(w http.ResponseWriter, r *http.Request) {
	failed, err := handler.bot.SendTest(r.Context())
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	if len(failed) > 0 {
		names := make([]string, 0, len(failed))
		for _, chat := range failed {
			names = append(names, chat.Title)
		}
		_ = httpserver.WriteError(w, http.StatusBadGateway, "telegram_chat_unavailable",
			"Не дошло: "+strings.Join(names, ", ")+". Этим людям нужно открыть бота и нажать Start", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *BotHandler) writeStatus(w http.ResponseWriter, r *http.Request, status BotStatus, err error) {
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]BotStatus{"bot": status})
}

func (handler *BotHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidBotToken):
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля",
			map[string]string{"token": "Telegram не принял токен. Скопируйте его из @BotFather целиком"})
	case errors.Is(err, ErrBotNotConfigured):
		_ = httpserver.WriteError(w, http.StatusConflict, "bot_not_configured", "Сначала задайте токен бота", nil)
	case errors.Is(err, ErrTelegramToken):
		_ = httpserver.WriteError(w, http.StatusConflict, "bot_token_rejected", "Telegram больше не принимает токен. Задайте новый из @BotFather", nil)
	case errors.Is(err, ErrNoRecipients):
		_ = httpserver.WriteError(w, http.StatusConflict, "no_recipients", "Получателей пока нет", nil)
	case errors.Is(err, ErrRecipientNotFound):
		_ = httpserver.WriteError(w, http.StatusNotFound, "recipient_not_found", "Получатель не найден", nil)
	case errors.Is(err, ErrTelegramChat):
		_ = httpserver.WriteError(w, http.StatusBadRequest, "telegram_chat_unavailable", "Бот не может писать в этот чат. Пусть человек откроет бота и нажмёт Start", nil)
	case errors.Is(err, ErrTelegramUnavailable):
		handler.logger.WarnContext(r.Context(), "telegram request failed",
			"request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
		_ = httpserver.WriteError(w, http.StatusBadGateway, "telegram_unavailable", "Telegram не отвечает. Попробуйте ещё раз", nil)
	default:
		handler.logger.ErrorContext(r.Context(), "bot request failed",
			"request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
		_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
	}
}
