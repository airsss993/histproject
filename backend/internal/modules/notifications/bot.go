package notifications

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidBotToken   = errors.New("invalid bot token")
	ErrBotNotConfigured  = errors.New("bot token is not set")
	ErrRecipientNotFound = errors.New("telegram recipient not found")
	ErrNoRecipients      = errors.New("no telegram recipients")
)

// Токен из @BotFather: числовой id бота, двоеточие, секрет.
var botTokenPattern = regexp.MustCompile(`^[0-9]{5,20}:[A-Za-z0-9_-]{30,100}$`)

type BotStore interface {
	BotToken(context.Context) (token, username string, err error)
	SaveBot(context.Context, string, string) error
	DeleteBot(context.Context) error
	Recipients(context.Context) ([]TelegramChat, error)
	SaveRecipient(context.Context, TelegramChat) error
	DeleteRecipient(context.Context, int64) error
}

type telegramClient interface {
	Username(context.Context, string) (string, error)
	Chat(context.Context, string, int64) (TelegramChat, error)
	Updates(context.Context, string) ([]TelegramChat, error)
	Send(context.Context, string, int64, string) error
}

// BotStatus уходит в панель. Токен наружу не отдаётся никогда.
type BotStatus struct {
	Configured bool           `json:"configured"`
	Username   string         `json:"username,omitempty"`
	Recipients []TelegramChat `json:"recipients"`
}

// Bot - Telegram-бот уведомлений админам. Токен и получатели лежат в базе, их меняет супер-админ.
type Bot struct {
	store BotStore
	api   telegramClient
}

func NewBot(store BotStore, timeout time.Duration) *Bot {
	return &Bot{store: store, api: newTelegramAPI(timeout)}
}

func (bot *Bot) Status(ctx context.Context) (BotStatus, error) {
	token, username, err := bot.store.BotToken(ctx)
	if err != nil {
		return BotStatus{}, err
	}
	recipients, err := bot.store.Recipients(ctx)
	if err != nil {
		return BotStatus{}, err
	}
	if recipients == nil {
		recipients = []TelegramChat{}
	}
	return BotStatus{Configured: token != "", Username: username, Recipients: recipients}, nil
}

// SetToken проверяет токен через getMe и сохраняет. Пустой токен отключает бота, получатели остаются.
func (bot *Bot) SetToken(ctx context.Context, token string) (BotStatus, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		if err := bot.store.DeleteBot(ctx); err != nil {
			return BotStatus{}, err
		}
		return bot.Status(ctx)
	}
	if !botTokenPattern.MatchString(token) {
		return BotStatus{}, ErrInvalidBotToken
	}
	username, err := bot.api.Username(ctx, token)
	if errors.Is(err, ErrTelegramToken) {
		return BotStatus{}, ErrInvalidBotToken
	}
	if err != nil {
		return BotStatus{}, err
	}
	if err := bot.store.SaveBot(ctx, token, username); err != nil {
		return BotStatus{}, err
	}
	return bot.Status(ctx)
}

// Candidates - кто писал боту за сутки и ещё не получил доступ.
func (bot *Bot) Candidates(ctx context.Context) ([]TelegramChat, error) {
	token, err := bot.token(ctx)
	if err != nil {
		return nil, err
	}
	chats, err := bot.api.Updates(ctx, token)
	if err != nil {
		return nil, err
	}
	recipients, err := bot.store.Recipients(ctx)
	if err != nil {
		return nil, err
	}
	granted := map[int64]bool{}
	for _, recipient := range recipients {
		granted[recipient.ID] = true
	}
	candidates := []TelegramChat{}
	for _, chat := range chats {
		if !granted[chat.ID] {
			candidates = append(candidates, chat)
		}
	}
	return candidates, nil
}

// Grant берёт имя чата у Telegram, а не у клиента: заодно видно, что бот может туда писать.
func (bot *Bot) Grant(ctx context.Context, chatID int64) (BotStatus, error) {
	token, err := bot.token(ctx)
	if err != nil {
		return BotStatus{}, err
	}
	chat, err := bot.api.Chat(ctx, token, chatID)
	if err != nil {
		return BotStatus{}, err
	}
	if err := bot.store.SaveRecipient(ctx, chat); err != nil {
		return BotStatus{}, err
	}
	return bot.Status(ctx)
}

func (bot *Bot) Revoke(ctx context.Context, chatID int64) (BotStatus, error) {
	if err := bot.store.DeleteRecipient(ctx, chatID); err != nil {
		return BotStatus{}, err
	}
	return bot.Status(ctx)
}

// SendTest шлёт проверочное сообщение всем получателям и возвращает тех, до кого оно не дошло.
func (bot *Bot) SendTest(ctx context.Context) ([]TelegramChat, error) {
	token, err := bot.token(ctx)
	if err != nil {
		return nil, err
	}
	recipients, err := bot.store.Recipients(ctx)
	if err != nil {
		return nil, err
	}
	if len(recipients) == 0 {
		return nil, ErrNoRecipients
	}
	failed := []TelegramChat{}
	for _, recipient := range recipients {
		err := bot.api.Send(ctx, token, recipient.ID, "Проверка связи: уведомления о заявках будут приходить сюда.")
		if errors.Is(err, ErrTelegramChat) {
			failed = append(failed, recipient)
		} else if err != nil {
			return nil, err
		}
	}
	return failed, nil
}

// Chats - получатели уведомлений. Без токена бот выключен, и слать некому.
func (bot *Bot) Chats(ctx context.Context) ([]int64, error) {
	token, _, err := bot.store.BotToken(ctx)
	if err != nil || token == "" {
		return nil, err
	}
	recipients, err := bot.store.Recipients(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		ids = append(ids, recipient.ID)
	}
	return ids, nil
}

func (bot *Bot) Send(ctx context.Context, chatID int64, message string) error {
	token, err := bot.token(ctx)
	if err != nil {
		return err
	}
	return bot.api.Send(ctx, token, chatID, message)
}

func (bot *Bot) token(ctx context.Context) (string, error) {
	token, _, err := bot.store.BotToken(ctx)
	if err != nil {
		return "", fmt.Errorf("load bot token: %w", err)
	}
	if token == "" {
		return "", ErrBotNotConfigured
	}
	return token, nil
}
