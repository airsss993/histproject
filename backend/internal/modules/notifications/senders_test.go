package notifications

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendersValidateConfiguration(t *testing.T) {
	if _, err := NewPostboxSender("postbox.cloud.yandex.net:587", "", "key-secret", "info@russia-heroes.ru", time.Second); err == nil {
		t.Fatal("NewPostboxSender() error = nil")
	}
}

func TestTelegramSendsEscapedForm(t *testing.T) {
	var path, chatID, message, parseMode string
	api := telegramServer(t, func(response http.ResponseWriter, incoming *http.Request) {
		path = incoming.URL.Path
		if err := incoming.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		chatID = incoming.Form.Get("chat_id")
		message = incoming.Form.Get("text")
		parseMode = incoming.Form.Get("parse_mode")
		_, _ = io.WriteString(response, `{"ok":true,"result":{}}`)
	})

	if err := api.Send(t.Context(), "bot-token", -100123, "<b>Safe & text</b>"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if path != "/botbot-token/sendMessage" || chatID != "-100123" || message != "<b>Safe & text</b>" || parseMode != "HTML" {
		t.Fatalf("request = path:%q chat:%q message:%q parseMode:%q", path, chatID, message, parseMode)
	}
}

func TestTelegramClassifiesErrors(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrTelegramToken},
		{http.StatusForbidden, ErrTelegramChat},
		{http.StatusBadRequest, ErrTelegramChat},
		{http.StatusTooManyRequests, ErrTelegramUnavailable},
	}
	for _, test := range tests {
		api := telegramServer(t, func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(test.status)
			_, _ = io.WriteString(response, `{"ok":false,"description":"nope"}`)
		})
		if err := api.Send(t.Context(), "token", 1, "text"); !errors.Is(err, test.want) {
			t.Fatalf("HTTP %d: Send() error = %v, want %v", test.status, err, test.want)
		}
	}
}

func TestTelegramUpdatesCollectsUniqueChats(t *testing.T) {
	api := telegramServer(t, func(response http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(response, `{"ok":true,"result":[
			{"update_id":1,"message":{"chat":{"id":5,"first_name":"Иван","last_name":"Петров","username":"ivan"}}},
			{"update_id":2,"message":{"chat":{"id":5,"first_name":"Иван"}}},
			{"update_id":3,"my_chat_member":{"chat":{"id":-100,"title":"Модераторы"}}}
		]}`)
	})

	chats, err := api.Updates(t.Context(), "token")
	if err != nil {
		t.Fatalf("Updates() error = %v", err)
	}
	want := []TelegramChat{{ID: 5, Title: "Иван Петров", Username: "ivan"}, {ID: -100, Title: "Модераторы"}}
	if len(chats) != 2 || chats[0] != want[0] || chats[1] != want[1] {
		t.Fatalf("Updates() = %#v", chats)
	}
}

func TestTelegramHidesTokenOnTransportError(t *testing.T) {
	api := &telegramAPI{client: failingHTTPClient{}, endpoint: "https://api.telegram.test"}

	err := api.Send(t.Context(), "secret-bot-token", 123, "message")
	if err == nil || strings.Contains(err.Error(), "secret-bot-token") {
		t.Fatalf("Send() error = %v", err)
	}
}

func telegramServer(t *testing.T, handler http.HandlerFunc) *telegramAPI {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &telegramAPI{client: server.Client(), endpoint: server.URL}
}

type failingHTTPClient struct{}

func (failingHTTPClient) Do(request *http.Request) (*http.Response, error) {
	_, _ = io.Copy(io.Discard, request.Body)
	return nil, errors.New("transport failed for " + request.URL.String())
}
