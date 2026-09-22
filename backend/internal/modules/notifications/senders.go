package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const telegramAPIURL = "https://api.telegram.org"

var (
	ErrInvalidMessage      = errors.New("invalid notification message")
	ErrInvalidSenderConfig = errors.New("invalid notification sender config")
	// ErrTelegramToken - Telegram не знает такой токен (отозван или опечатка).
	ErrTelegramToken = errors.New("telegram rejected bot token")
	// ErrTelegramChat - бот не может писать в чат: человек не нажал /start, заблокировал бота или чата нет.
	ErrTelegramChat        = errors.New("telegram chat unavailable")
	ErrTelegramUnavailable = errors.New("telegram unavailable")
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// TelegramChat - чат, которому бот может писать: человек, группа или канал.
type TelegramChat struct {
	ID       int64  `json:"chatId"`
	Title    string `json:"title"`
	Username string `json:"username,omitempty"`
}

type telegramAPI struct {
	client   httpDoer
	endpoint string
}

func newTelegramAPI(timeout time.Duration) *telegramAPI {
	return &telegramAPI{client: &http.Client{Timeout: timeout}, endpoint: telegramAPIURL}
}

func (api *telegramAPI) Username(ctx context.Context, token string) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := api.call(ctx, token, "getMe", nil, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}

func (api *telegramAPI) Chat(ctx context.Context, token string, chatID int64) (TelegramChat, error) {
	var chat telegramChat
	err := api.call(ctx, token, "getChat", url.Values{"chat_id": {strconv.FormatInt(chatID, 10)}}, &chat)
	return chat.public(), err
}

// Updates отдаёт чаты, писавшие боту за последние сутки: Telegram держит необработанные апдейты 24 часа.
func (api *telegramAPI) Updates(ctx context.Context, token string) ([]TelegramChat, error) {
	type withChat struct {
		Chat telegramChat `json:"chat"`
	}
	var updates []struct {
		Message      *withChat `json:"message"`
		ChannelPost  *withChat `json:"channel_post"`
		MyChatMember *withChat `json:"my_chat_member"`
	}
	if err := api.call(ctx, token, "getUpdates", nil, &updates); err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	chats := []TelegramChat{}
	for _, update := range updates {
		for _, payload := range []*withChat{update.Message, update.ChannelPost, update.MyChatMember} {
			if payload == nil || payload.Chat.ID == 0 || seen[payload.Chat.ID] {
				continue
			}
			seen[payload.Chat.ID] = true
			chats = append(chats, payload.Chat.public())
		}
	}
	return chats, nil
}

func (api *telegramAPI) Send(ctx context.Context, token string, chatID int64, message string) error {
	if strings.TrimSpace(message) == "" {
		return ErrInvalidMessage
	}
	return api.call(ctx, token, "sendMessage", url.Values{
		"chat_id": {strconv.FormatInt(chatID, 10)}, "text": {message}, "parse_mode": {"HTML"},
	}, nil)
}

// call не кладёт в ошибку URL запроса: в его пути лежит токен.
func (api *telegramAPI) call(ctx context.Context, token, method string, form url.Values, result any) error {
	requestURL := api.endpoint + "/bot" + url.PathEscape(token) + "/" + method
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return errors.New("create Telegram request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := api.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: request failed", ErrTelegramUnavailable)
	}
	defer func() { _ = response.Body.Close() }()
	var body struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return fmt.Errorf("%w: HTTP %d, unreadable body", ErrTelegramUnavailable, response.StatusCode)
	}
	if !body.OK {
		switch response.StatusCode {
		case http.StatusUnauthorized, http.StatusNotFound:
			return ErrTelegramToken
		case http.StatusBadRequest, http.StatusForbidden:
			return fmt.Errorf("%w: %s", ErrTelegramChat, body.Description)
		}
		return fmt.Errorf("%w: HTTP %d: %s", ErrTelegramUnavailable, response.StatusCode, body.Description)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(body.Result, result); err != nil {
		return fmt.Errorf("%w: decode %s result", ErrTelegramUnavailable, method)
	}
	return nil
}

type telegramChat struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (chat telegramChat) public() TelegramChat {
	title := chat.Title
	if title == "" {
		title = strings.TrimSpace(chat.FirstName + " " + chat.LastName)
	}
	if title == "" {
		title = strconv.FormatInt(chat.ID, 10)
	}
	return TelegramChat{ID: chat.ID, Title: title, Username: chat.Username}
}
