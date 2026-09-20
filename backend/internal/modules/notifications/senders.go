package notifications

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const telegramAPIURL = "https://api.telegram.org"

var (
	ErrInvalidMessage      = errors.New("invalid notification message")
	ErrInvalidSenderConfig = errors.New("invalid notification sender config")
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type TelegramSenderClient struct {
	botToken string
	chatID   string
	client   httpDoer
	endpoint string
}

func NewTelegramSender(botToken, chatID string, timeout time.Duration) (*TelegramSenderClient, error) {
	return newTelegramSender(botToken, chatID, timeout, &http.Client{Timeout: timeout}, telegramAPIURL)
}

func newTelegramSender(botToken, chatID string, timeout time.Duration, client httpDoer, endpoint string) (*TelegramSenderClient, error) {
	if strings.TrimSpace(botToken) == "" || strings.TrimSpace(chatID) == "" || timeout <= 0 || !validHTTPEndpoint(endpoint) {
		return nil, ErrInvalidSenderConfig
	}
	return &TelegramSenderClient{
		botToken: strings.TrimSpace(botToken), chatID: strings.TrimSpace(chatID), client: client,
		endpoint: strings.TrimRight(endpoint, "/"),
	}, nil
}

func (sender *TelegramSenderClient) Send(ctx context.Context, message string) error {
	if strings.TrimSpace(message) == "" {
		return ErrInvalidMessage
	}
	form := url.Values{}
	form.Set("chat_id", sender.chatID)
	form.Set("text", message)
	form.Set("parse_mode", "HTML")
	requestURL := sender.endpoint + "/bot" + url.PathEscape(sender.botToken) + "/sendMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return errors.New("create Telegram request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := sender.client.Do(request)
	if err != nil {
		return errors.New("send Telegram request")
	}
	return finishResponse(response, "Telegram")
}

func finishResponse(response *http.Response, provider string) error {
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	closeErr := response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return errors.Join(fmt.Errorf("%s returned HTTP %d", provider, response.StatusCode), readErr, closeErr)
	}
	return errors.Join(readErr, closeErr)
}

func validHTTPEndpoint(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
