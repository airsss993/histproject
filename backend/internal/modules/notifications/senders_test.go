package notifications

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResendSenderSendsJSONWithAuthorization(t *testing.T) {
	var request resendRequest
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, incoming *http.Request) {
		if incoming.Method != http.MethodPost || incoming.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("request = method:%s authorization:%q", incoming.Method, incoming.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(incoming.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		response.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	sender, err := newResendSender("secret-key", "Project <mail@example.test>", time.Second, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("newResendSender() error = %v", err)
	}

	err = sender.Send(t.Context(), Email{To: []string{"admin@example.test"}, Subject: "Subject", HTML: "<p>Body</p>"})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if request.From != "Project <mail@example.test>" || len(request.To) != 1 || request.Subject != "Subject" || request.HTML != "<p>Body</p>" {
		t.Fatalf("request = %#v", request)
	}
}

func TestResendSenderDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "provider-secret-details", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	sender, err := newResendSender("secret-key", "mail@example.test", time.Second, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("newResendSender() error = %v", err)
	}

	err = sender.Send(t.Context(), Email{To: []string{"admin@example.test"}, Subject: "Subject", HTML: "Body"})
	if err == nil || strings.Contains(err.Error(), "provider-secret-details") || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestTelegramSenderSendsEscapedForm(t *testing.T) {
	var path, chatID, message, parseMode string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, incoming *http.Request) {
		path = incoming.URL.Path
		if err := incoming.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		chatID = incoming.Form.Get("chat_id")
		message = incoming.Form.Get("text")
		parseMode = incoming.Form.Get("parse_mode")
		response.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	sender, err := newTelegramSender("bot-token", "-100123", time.Second, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("newTelegramSender() error = %v", err)
	}

	if err := sender.Send(t.Context(), "<b>Safe & text</b>"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if path != "/botbot-token/sendMessage" || chatID != "-100123" || message != "<b>Safe & text</b>" || parseMode != "HTML" {
		t.Fatalf("request = path:%q chat:%q message:%q parseMode:%q", path, chatID, message, parseMode)
	}
}

func TestTelegramSenderHidesTokenOnTransportError(t *testing.T) {
	sender, err := newTelegramSender("secret-bot-token", "123", time.Second, failingHTTPClient{}, "https://api.telegram.test")
	if err != nil {
		t.Fatalf("newTelegramSender() error = %v", err)
	}

	err = sender.Send(t.Context(), "message")
	if err == nil || strings.Contains(err.Error(), "secret-bot-token") {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestSendersValidateConfiguration(t *testing.T) {
	if _, err := NewResendSender("", "mail@example.test", time.Second); err == nil {
		t.Fatal("NewResendSender() error = nil")
	}
	if _, err := NewTelegramSender("token", "", time.Second); err == nil {
		t.Fatal("NewTelegramSender() error = nil")
	}
}

type failingHTTPClient struct{}

func (failingHTTPClient) Do(request *http.Request) (*http.Response, error) {
	_, _ = io.Copy(io.Discard, request.Body)
	return nil, errors.New("transport failed for " + request.URL.String())
}

var _ interface {
	Do(*http.Request) (*http.Response, error)
} = failingHTTPClient{}
