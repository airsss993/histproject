package notifications

import "errors"

type Channel string

const (
	ChannelEmail    Channel = "email"
	ChannelTelegram Channel = "telegram"
)

func (channel Channel) Valid() bool {
	return channel == ChannelEmail || channel == ChannelTelegram
}

type Action string

const (
	ActionCreated   Action = "created"
	ActionReady     Action = "ready"
	ActionReviewed  Action = "reviewed"
	ActionRejected  Action = "rejected"
	ActionPublished Action = "published"
)

var ErrInvalidAction = errors.New("invalid notification action")

func (action Action) Valid() bool {
	return action == ActionCreated || action == ActionReady || action == ActionReviewed ||
		action == ActionRejected || action == ActionPublished
}

type Submission struct {
	ID               int64
	Title            string
	Email            string
	TelegramUsername string
	SiteURL          string
	Comment          string
}

type Email struct {
	To      []string
	Subject string
	HTML    string
	// Text - text/plain версия. Пустая - письмо уходит одной HTML-частью.
	Text string
	// Images встраиваются в письмо и доступны в HTML как cid:ContentID.
	Images []InlineImage
}

type InlineImage struct {
	ContentID string
	FileName  string
	Content   []byte
}
