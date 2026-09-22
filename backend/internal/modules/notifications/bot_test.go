package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
)

var validToken = "123456789:" + strings.Repeat("A", 35)

func TestBotSetTokenValidatesBeforeSaving(t *testing.T) {
	store := &botStoreStub{}
	bot := &Bot{store: store, api: &telegramStub{tokenErr: ErrTelegramToken}}

	if _, err := bot.SetToken(t.Context(), "not-a-token"); !errors.Is(err, ErrInvalidBotToken) {
		t.Fatalf("SetToken(bad format) error = %v", err)
	}
	if _, err := bot.SetToken(t.Context(), validToken); !errors.Is(err, ErrInvalidBotToken) {
		t.Fatalf("SetToken(rejected) error = %v", err)
	}
	if store.token != "" {
		t.Fatalf("token saved = %q, want nothing", store.token)
	}

	bot.api = &telegramStub{username: "heroes_bot"}
	status, err := bot.SetToken(t.Context(), " "+validToken+" ")
	if err != nil || !status.Configured || status.Username != "heroes_bot" || store.token != validToken {
		t.Fatalf("SetToken() = %#v, %v; stored %q", status, err, store.token)
	}

	status, err = bot.SetToken(t.Context(), "")
	if err != nil || status.Configured || store.token != "" {
		t.Fatalf("SetToken(empty) = %#v, %v", status, err)
	}
}

func TestBotCandidatesSkipGrantedChats(t *testing.T) {
	store := &botStoreStub{token: validToken, recipients: []TelegramChat{{ID: 1, Title: "Уже есть"}}}
	bot := &Bot{store: store, api: &telegramStub{updates: []TelegramChat{{ID: 1, Title: "Уже есть"}, {ID: 2, Title: "Новый"}}}}

	chats, err := bot.Candidates(t.Context())
	if err != nil || len(chats) != 1 || chats[0].ID != 2 {
		t.Fatalf("Candidates() = %#v, %v", chats, err)
	}
}

func TestBotWithoutTokenHasNoChats(t *testing.T) {
	bot := &Bot{store: &botStoreStub{recipients: []TelegramChat{{ID: 1}}}, api: &telegramStub{}}

	chats, err := bot.Chats(t.Context())
	if err != nil || len(chats) != 0 {
		t.Fatalf("Chats() = %v, %v", chats, err)
	}
	if _, err := bot.Candidates(t.Context()); !errors.Is(err, ErrBotNotConfigured) {
		t.Fatalf("Candidates() error = %v", err)
	}
}

func TestBotSendTestReportsUnreachableChats(t *testing.T) {
	store := &botStoreStub{token: validToken, recipients: []TelegramChat{{ID: 1, Title: "Дошло"}, {ID: 2, Title: "Заблокировал"}}}
	bot := &Bot{store: store, api: &telegramStub{sendErr: map[int64]error{2: ErrTelegramChat}}}

	failed, err := bot.SendTest(t.Context())
	if err != nil || len(failed) != 1 || failed[0].ID != 2 {
		t.Fatalf("SendTest() = %#v, %v", failed, err)
	}
}

type botStoreStub struct {
	token, username string
	recipients      []TelegramChat
}

func (stub *botStoreStub) BotToken(context.Context) (string, string, error) {
	return stub.token, stub.username, nil
}

func (stub *botStoreStub) SaveBot(_ context.Context, token, username string) error {
	stub.token, stub.username = token, username
	return nil
}

func (stub *botStoreStub) DeleteBot(context.Context) error {
	stub.token, stub.username = "", ""
	return nil
}

func (stub *botStoreStub) Recipients(context.Context) ([]TelegramChat, error) {
	return stub.recipients, nil
}

func (stub *botStoreStub) SaveRecipient(_ context.Context, chat TelegramChat) error {
	stub.recipients = append(stub.recipients, chat)
	return nil
}

func (stub *botStoreStub) DeleteRecipient(context.Context, int64) error { return nil }

type telegramStub struct {
	username string
	tokenErr error
	updates  []TelegramChat
	sendErr  map[int64]error
}

func (stub *telegramStub) Username(context.Context, string) (string, error) {
	return stub.username, stub.tokenErr
}

func (stub *telegramStub) Chat(_ context.Context, _ string, chatID int64) (TelegramChat, error) {
	return TelegramChat{ID: chatID, Title: "Чат"}, nil
}

func (stub *telegramStub) Updates(context.Context, string) ([]TelegramChat, error) {
	return stub.updates, nil
}

func (stub *telegramStub) Send(_ context.Context, _ string, chatID int64, _ string) error {
	return stub.sendErr[chatID]
}
