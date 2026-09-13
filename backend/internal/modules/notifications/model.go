package notifications

type Channel string

const (
	ChannelEmail    Channel = "email"
	ChannelTelegram Channel = "telegram"
)

func (channel Channel) Valid() bool {
	return channel == ChannelEmail || channel == ChannelTelegram
}
