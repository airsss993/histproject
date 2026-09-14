package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	resendAPIURL   = "https://api.resend.com/emails"
	telegramAPIURL = "https://api.telegram.org"
)

var (
	ErrInvalidMessage      = errors.New("invalid notification message")
	ErrInvalidSenderConfig = errors.New("invalid notification sender config")
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type ResendSender struct {
	apiKey   string
	from     string
	client   httpDoer
	endpoint string
}

func NewResendSender(apiKey, from string, timeout time.Duration) (*ResendSender, error) {
	return newResendSender(apiKey, from, timeout, &http.Client{Timeout: timeout}, resendAPIURL)
}

func newResendSender(apiKey, from string, timeout time.Duration, client httpDoer, endpoint string) (*ResendSender, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(from) == "" || timeout <= 0 || !validHTTPEndpoint(endpoint) {
		return nil, ErrInvalidSenderConfig
	}
	return &ResendSender{
		apiKey: strings.TrimSpace(apiKey), from: strings.TrimSpace(from), client: client,
		endpoint: strings.TrimRight(endpoint, "/"),
	}, nil
}

func (sender *ResendSender) Send(ctx context.Context, message Email) error {
	if len(message.To) == 0 || strings.TrimSpace(message.Subject) == "" || strings.TrimSpace(message.HTML) == "" {
		return ErrInvalidMessage
	}
	body, err := json.Marshal(resendRequest{
		From: sender.from, To: message.To, Subject: message.Subject, HTML: message.HTML,
	})
	if err != nil {
		return ErrInvalidMessage
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("create Resend request")
	}
	request.Header.Set("Authorization", "Bearer "+sender.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := sender.client.Do(request)
	if err != nil {
		return errors.New("send Resend request")
	}
	return finishResponse(response, "Resend")
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
