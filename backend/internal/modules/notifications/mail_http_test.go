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

func TestMailStatusShowsSenderFromPanelOrEnv(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, test := range []struct {
		sender EmailSender
		store  *mailSenderStoreStub
		want   string
	}{
		{nil, &mailSenderStoreStub{}, `{"mail":{"configured":false}}`},
		{&emailSenderStub{}, &mailSenderStoreStub{}, `{"mail":{"configured":true,"name":"Герои","address":"no-reply@example.test","domain":"example.test"}}`},
		{&emailSenderStub{}, &mailSenderStoreStub{name: "", address: "news@example.test"}, `{"mail":{"configured":true,"address":"news@example.test","domain":"example.test"}}`},
	} {
		response := serveMail(NewMailHandler(test.sender, "Герои <no-reply@example.test>", test.store, logger), http.MethodGet, "/api/v1/admin/mail", "")
		if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != test.want {
			t.Fatalf("status = %d, body = %s, want %s", response.Code, response.Body.String(), test.want)
		}
	}
}

func TestMailSenderAcceptsOnlyVerifiedDomain(t *testing.T) {
	store := &mailSenderStoreStub{}
	handler := NewMailHandler(&emailSenderStub{}, "no-reply@example.test", store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for body, field := range map[string]string{
		`{"name":"Герои","address":"news@other.test"}`:      "address",
		`{"name":"Герои","address":"not-email"}`:            "address",
		`{"name":"Ге\nрои","address":"news@example.test"}`: "name",
	} {
		response := serveMail(handler, http.MethodPut, "/api/v1/admin/mail/sender", body)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"`+field+`"`) || store.saved {
			t.Fatalf("%s: status = %d, body = %s, saved = %v", body, response.Code, response.Body.String(), store.saved)
		}
	}

	response := serveMail(handler, http.MethodPut, "/api/v1/admin/mail/sender", `{"name":" Герои ","address":"news@EXAMPLE.test"}`)
	if response.Code != http.StatusOK || store.name != "Герои" || store.address != "news@EXAMPLE.test" {
		t.Fatalf("save: status = %d, body = %s, store = %#v", response.Code, response.Body.String(), store)
	}
	if !strings.Contains(response.Body.String(), `"address":"news@EXAMPLE.test"`) {
		t.Fatalf("save response = %s", response.Body.String())
	}

	unconfigured := NewMailHandler(nil, "", &mailSenderStoreStub{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if response := serveMail(unconfigured, http.MethodPut, "/api/v1/admin/mail/sender", `{"name":"","address":"a@example.test"}`); response.Code != http.StatusConflict {
		t.Fatalf("unconfigured: status = %d", response.Code)
	}
}

func TestMailTestSendsCheckLetter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sender := &emailSenderStub{}
	handler := NewMailHandler(sender, "no-reply@example.test", &mailSenderStoreStub{}, logger)

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

	unconfigured := NewMailHandler(nil, "", &mailSenderStoreStub{}, logger)
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

type mailSenderStoreStub struct {
	name, address string
	saved         bool
}

func (stub *mailSenderStoreStub) MailSender(context.Context) (string, string, error) {
	return stub.name, stub.address, nil
}

func (stub *mailSenderStoreStub) SaveMailSender(_ context.Context, name, address string) error {
	stub.name, stub.address, stub.saved = name, address, true
	return nil
}
