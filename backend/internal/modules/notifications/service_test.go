package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestServiceDeliversCreatedNotificationOncePerChannel(t *testing.T) {
	store := &deliveryStoreStub{claims: map[Channel]bool{ChannelEmail: true, ChannelTelegram: true}}
	emailSender := &emailSenderStub{}
	telegramSender := &telegramSenderStub{}
	service := NewService(
		store,
		&submissionSourceStub{submission: notificationSubmission()},
		&adminSourceStub{emails: []string{"admin@example.test"}},
		emailSender,
		telegramSender,
		"https://admin.example.test",
	)

	if err := service.Deliver(t.Context(), 7, ActionCreated); err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if len(emailSender.messages) != 1 || len(telegramSender.messages) != 1 {
		t.Fatalf("messages = email:%d telegram:%d", len(emailSender.messages), len(telegramSender.messages))
	}
	if len(emailSender.messages[0].To) != 1 || emailSender.messages[0].To[0] != "admin@example.test" {
		t.Fatalf("email recipients = %#v", emailSender.messages[0].To)
	}
	if strings.Contains(telegramSender.messages[0], "author@example.test") || strings.Contains(telegramSender.messages[0], "@author") {
		t.Fatalf("telegram message exposes author contacts: %q", telegramSender.messages[0])
	}
	if strings.Contains(emailSender.messages[0].HTML, "<script>") || strings.Contains(telegramSender.messages[0], "<script>") {
		t.Fatal("notification content contains unescaped title")
	}
	if store.sent[ChannelEmail] != 1 || store.sent[ChannelTelegram] != 1 {
		t.Fatalf("sent marks = %#v", store.sent)
	}
}

func TestServiceSkipsReadyNotification(t *testing.T) {
	source := &submissionSourceStub{}
	emailSender := &emailSenderStub{}
	telegramSender := &telegramSenderStub{}
	service := NewService(&deliveryStoreStub{}, source, &adminSourceStub{}, emailSender, telegramSender, "")
	if err := service.Deliver(t.Context(), 7, ActionReady); err != nil {
		t.Fatal(err)
	}
	if source.called || len(emailSender.messages) != 0 || len(telegramSender.messages) != 0 {
		t.Fatal("ready notification reached recipients")
	}
}

func TestServiceSkipsAlreadyDeliveredChannel(t *testing.T) {
	store := &deliveryStoreStub{claims: map[Channel]bool{ChannelEmail: false}}
	emailSender := &emailSenderStub{}
	service := NewService(
		store,
		&submissionSourceStub{submission: notificationSubmission()},
		&adminSourceStub{},
		emailSender,
		nil,
		"https://admin.example.test",
	)

	if err := service.Deliver(t.Context(), 7, ActionReviewed); err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if len(emailSender.messages) != 0 || store.sent[ChannelEmail] != 0 {
		t.Fatalf("email calls = %d, sent marks = %d", len(emailSender.messages), store.sent[ChannelEmail])
	}
}

func TestServiceRetriesDeliveryAlreadyInProgress(t *testing.T) {
	store := &deliveryStoreStub{claimErr: ErrDeliveryInProgress}
	emailSender := &emailSenderStub{}
	service := NewService(
		store,
		&submissionSourceStub{submission: notificationSubmission()},
		&adminSourceStub{},
		emailSender,
		nil,
		"https://admin.example.test",
	)

	err := service.Deliver(t.Context(), 7, ActionReviewed)
	if !errors.Is(err, ErrDeliveryInProgress) {
		t.Fatalf("Deliver() error = %v, want ErrDeliveryInProgress", err)
	}
	if len(emailSender.messages) != 0 {
		t.Fatalf("email calls = %d, want 0", len(emailSender.messages))
	}
}

func TestServiceContinuesOtherChannelAfterFailure(t *testing.T) {
	want := errors.New("email unavailable")
	store := &deliveryStoreStub{claims: map[Channel]bool{ChannelEmail: true, ChannelTelegram: true}}
	emailSender := &emailSenderStub{err: want}
	telegramSender := &telegramSenderStub{}
	service := NewService(
		store,
		&submissionSourceStub{submission: notificationSubmission()},
		&adminSourceStub{emails: []string{"admin@example.test"}},
		emailSender,
		telegramSender,
		"https://admin.example.test",
	)

	err := service.Deliver(t.Context(), 7, ActionCreated)
	if !errors.Is(err, want) {
		t.Fatalf("Deliver() error = %v, want email error", err)
	}
	if store.failed[ChannelEmail] != 1 || store.sent[ChannelTelegram] != 1 {
		t.Fatalf("delivery marks = failed:%#v sent:%#v", store.failed, store.sent)
	}
	if len(telegramSender.messages) != 1 {
		t.Fatalf("telegram calls = %d, want 1", len(telegramSender.messages))
	}
}

func TestServiceBuildsStatusEmailForAuthor(t *testing.T) {
	store := &deliveryStoreStub{claims: map[Channel]bool{ChannelEmail: true}}
	emailSender := &emailSenderStub{}
	service := NewService(
		store,
		&submissionSourceStub{submission: notificationSubmission()},
		&adminSourceStub{},
		emailSender,
		nil,
		"https://admin.example.test",
	)

	if err := service.Deliver(t.Context(), 7, ActionRejected); err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	message := emailSender.messages[0]
	if len(message.To) != 1 || message.To[0] != "author@example.test" {
		t.Fatalf("email recipients = %#v", message.To)
	}
	if !strings.Contains(message.HTML, "Исправьте описание") || !strings.Contains(message.Subject, "отклонена") {
		t.Fatalf("status email = subject:%q html:%q", message.Subject, message.HTML)
	}
}

func TestServiceRejectsUnknownActionBeforeLoadingSubmission(t *testing.T) {
	source := &submissionSourceStub{}
	service := NewService(&deliveryStoreStub{}, source, &adminSourceStub{}, nil, nil, "")

	err := service.Deliver(t.Context(), 7, Action("unknown"))
	if !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("Deliver() error = %v, want ErrInvalidAction", err)
	}
	if source.called {
		t.Fatal("submission loaded for invalid action")
	}
}

func notificationSubmission() Submission {
	return Submission{
		PublicID: "RH-7K3F9Q", Title: "Event <script>", Email: "author@example.test",
		TelegramUsername: "author", SiteURL: "https://example.test/sites/event",
		Comment: "Исправьте описание",
	}
}

type deliveryStoreStub struct {
	claims   map[Channel]bool
	claimErr error
	sent     map[Channel]int
	failed   map[Channel]int
}

func (stub *deliveryStoreStub) Claim(_ context.Context, _ string, channel Channel) (bool, error) {
	return stub.claims[channel], stub.claimErr
}

func (stub *deliveryStoreStub) MarkSent(_ context.Context, _ string, channel Channel) error {
	if stub.sent == nil {
		stub.sent = make(map[Channel]int)
	}
	stub.sent[channel]++
	return nil
}

func (stub *deliveryStoreStub) MarkFailed(_ context.Context, _ string, channel Channel, _ string) error {
	if stub.failed == nil {
		stub.failed = make(map[Channel]int)
	}
	stub.failed[channel]++
	return nil
}

type submissionSourceStub struct {
	submission Submission
	err        error
	called     bool
}

func (stub *submissionSourceStub) NotificationSubmission(context.Context, int64) (Submission, error) {
	stub.called = true
	return stub.submission, stub.err
}

type adminSourceStub struct {
	emails []string
	err    error
}

func (stub *adminSourceStub) NotificationAdminEmails(context.Context) ([]string, error) {
	return stub.emails, stub.err
}

type emailSenderStub struct {
	messages []Email
	err      error
}

func (stub *emailSenderStub) Send(_ context.Context, message Email) error {
	stub.messages = append(stub.messages, message)
	return stub.err
}

type telegramSenderStub struct {
	messages []string
	err      error
}

func (stub *telegramSenderStub) Chats(context.Context) ([]int64, error) {
	return []int64{-100123}, nil
}

func (stub *telegramSenderStub) Send(_ context.Context, _ int64, message string) error {
	stub.messages = append(stub.messages, message)
	return stub.err
}
