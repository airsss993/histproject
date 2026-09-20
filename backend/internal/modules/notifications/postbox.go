package notifications

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

const base64LineLength = 76

// PostboxSender отправляет письма через Yandex Cloud Postbox по SMTP.
// Логин - идентификатор API-ключа сервисного аккаунта, пароль - его секретная часть.
type PostboxSender struct {
	address      string
	host         string
	keyID        string
	secret       string
	from         string
	envelopeFrom string
	timeout      time.Duration
}

func NewPostboxSender(address, keyID, secret, from string, timeout time.Duration) (*PostboxSender, error) {
	address = strings.TrimSpace(address)
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return nil, ErrInvalidSenderConfig
	}
	parsedFrom, err := mail.ParseAddress(strings.TrimSpace(from))
	if err != nil {
		return nil, ErrInvalidSenderConfig
	}
	if strings.TrimSpace(keyID) == "" || strings.TrimSpace(secret) == "" || timeout <= 0 {
		return nil, ErrInvalidSenderConfig
	}
	return &PostboxSender{
		address: address, host: host,
		keyID: strings.TrimSpace(keyID), secret: strings.TrimSpace(secret),
		from: parsedFrom.String(), envelopeFrom: parsedFrom.Address, timeout: timeout,
	}, nil
}

func (sender *PostboxSender) Send(ctx context.Context, message Email) error {
	letter, err := buildLetter(sender.from, message)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, sender.timeout)
	defer cancel()

	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", sender.address)
	if err != nil {
		return errors.New("postbox: соединение не установлено")
	}
	// net/smtp не знает про context, дедлайн на соединении - единственный способ не зависнуть.
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(connection, sender.host)
	if err != nil {
		_ = connection.Close()
		return smtpFailure("приветствие", err)
	}
	defer func() { _ = client.Close() }()

	// Без STARTTLS ключ ушёл бы открытым текстом: отказываемся, а не продолжаем.
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("postbox: сервер не предлагает STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: sender.host, MinVersion: tls.VersionTLS12}); err != nil {
		return smtpFailure("STARTTLS", err)
	}
	if err := client.Auth(smtp.PlainAuth("", sender.keyID, sender.secret, sender.host)); err != nil {
		return smtpFailure("авторизация", err)
	}
	if err := deliverLetter(client, sender.envelopeFrom, message.To, letter); err != nil {
		return err
	}
	return client.Quit()
}

func deliverLetter(client *smtp.Client, envelopeFrom string, recipients []string, letter []byte) error {
	if err := client.Mail(envelopeFrom); err != nil {
		return smtpFailure("MAIL FROM", err)
	}
	for _, recipient := range recipients {
		parsed, err := mail.ParseAddress(strings.TrimSpace(recipient))
		if err != nil {
			return ErrInvalidMessage
		}
		if err := client.Rcpt(parsed.Address); err != nil {
			return smtpFailure("RCPT TO", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return smtpFailure("DATA", err)
	}
	if _, err := writer.Write(letter); err != nil {
		return smtpFailure("передача письма", err)
	}
	if err := writer.Close(); err != nil {
		return smtpFailure("завершение письма", err)
	}
	return nil
}

// buildLetter собирает MIME-письмо. Адреса проходят через mail.Address.String():
// имя с кириллицей кодируется по RFC 2047, заголовки остаются семибитными.
// Тело кодируется в base64: это разом снимает ограничение SMTP на длину строки,
// точку в начале строки и восьмибитные символы.
func buildLetter(from string, message Email) ([]byte, error) {
	if len(message.To) == 0 || strings.TrimSpace(message.Subject) == "" || strings.TrimSpace(message.HTML) == "" {
		return nil, ErrInvalidMessage
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return nil, ErrInvalidMessage
	}
	parsedFrom, err := mail.ParseAddress(strings.TrimSpace(from))
	if err != nil {
		return nil, ErrInvalidSenderConfig
	}
	recipients := make([]string, 0, len(message.To))
	for _, recipient := range message.To {
		parsed, err := mail.ParseAddress(strings.TrimSpace(recipient))
		if err != nil {
			return nil, ErrInvalidMessage
		}
		recipients = append(recipients, parsed.String())
	}

	var letter bytes.Buffer
	fmt.Fprintf(&letter, "From: %s\r\n", parsedFrom.String())
	fmt.Fprintf(&letter, "To: %s\r\n", strings.Join(recipients, ", "))
	fmt.Fprintf(&letter, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", message.Subject))
	letter.WriteString("MIME-Version: 1.0\r\n")
	letter.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	letter.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(message.HTML))
	for len(encoded) > base64LineLength {
		letter.WriteString(encoded[:base64LineLength] + "\r\n")
		encoded = encoded[base64LineLength:]
	}
	letter.WriteString(encoded + "\r\n")
	return letter.Bytes(), nil
}

// smtpFailure отдаёт код ответа сервера без его текста: в текст может попасть
// то, что мы сами отправили, включая ключ.
func smtpFailure(stage string, err error) error {
	var protocolError *textproto.Error
	if errors.As(err, &protocolError) {
		return fmt.Errorf("postbox: %s, SMTP %d", stage, protocolError.Code)
	}
	return fmt.Errorf("postbox: %s", stage)
}
