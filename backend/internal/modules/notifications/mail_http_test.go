package notifications

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMailStatusHidesSenderWhenNotConfigured(t *testing.T) {
	recipients := func(context.Context) ([]string, error) { return []string{"a@example.test"}, nil }
	for _, test := range []struct {
		sender EmailSender
		want   string
	}{
		{nil, `{"mail":{"configured":false,"recipients":["a@example.test"]}}`},
		{&emailSenderStub{}, `{"mail":{"configured":true,"from":"no-reply@example.test","recipients":["a@example.test"]}}`},
	} {
		response := serveMail(NewMailHandler(test.sender, "no-reply@example.test", recipients, slog.New(slog.NewTextHandler(io.Discard, nil))),
			http.MethodGet, "/api/v1/admin/mail", "")
		if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != test.want {
			t.Fatalf("status = %d, body = %s, want %s", response.Code, response.Body.String(), test.want)
		}
	}
}

func TestMailTestSendsCheckLetter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	noRecipients := func(context.Context) ([]string, error) { return nil, nil }
	sender := &emailSenderStub{}
	handler := NewMailHandler(sender, "no-reply@example.test", noRecipients, logger)

	if response := serveMail(handler, http.MethodPost, "/api/v1/admin/mail/test", `{"email":"bad"}`); response.Code != http.StatusBadRequest || len(sender.messages) != 0 {
		t.Fatalf("bad email: status = %d, sent = %#v", response.Code, sender.messages)
	}
	if response := serveMail(handler, http.MethodPost, "/api/v1/admin/mail/test", `{"email":" me@example.test "}`); response.Code != http.StatusNoContent {
		t.Fatalf("send: status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(sender.messages) != 1 || len(sender.messages[0].To) != 1 || sender.messages[0].To[0] != "me@example.test" || sender.messages[0].Subject != "Проверка почты" {
		t.Fatalf("sent = %#v", sender.messages)
	}

	sender.err = errors.New("postbox: авторизация, SMTP 535")
	response := serveMail(handler, http.MethodPost, "/api/v1/admin/mail/test", `{"email":"me@example.test"}`)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "SMTP 535") {
		t.Fatalf("failed send: status = %d, body = %s", response.Code, response.Body.String())
	}

	unconfigured := NewMailHandler(nil, "", noRecipients, logger)
	if response := serveMail(unconfigured, http.MethodPost, "/api/v1/admin/mail/test", `{"email":"me@example.test"}`); response.Code != http.StatusConflict {
		t.Fatalf("unconfigured: status = %d", response.Code)
	}
}

func serveMail(handler *MailHandler, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	handler.Register(mux, func(next http.Handler) http.Handler { return next })
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
	return response
}
