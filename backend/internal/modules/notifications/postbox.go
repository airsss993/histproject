package notifications

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"path/filepath"
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
	body, err := letterBody(message)
	if err != nil {
		return nil, err
	}
	letter.WriteString("MIME-Version: 1.0\r\n")
	for _, key := range []string{"Content-Type", "Content-Transfer-Encoding"} {
		if value := body.header.Get(key); value != "" {
			fmt.Fprintf(&letter, "%s: %s\r\n", key, value)
		}
	}
	letter.WriteString("\r\n")
	letter.Write(body.content)
	return letter.Bytes(), nil
}

type mimePart struct {
	header  textproto.MIMEHeader
	content []byte
}

// letterBody раскладывает письмо по частям:
// multipart/alternative{text/plain, multipart/related{text/html, картинки}}.
// Без Text или без картинок лишний уровень вложенности не создаётся.
func letterBody(message Email) (mimePart, error) {
	body := base64Part("text/html; charset=UTF-8", []byte(message.HTML))
	if len(message.Images) > 0 {
		parts := []mimePart{body}
		for _, image := range message.Images {
			if image.ContentID == "" || len(image.Content) == 0 || strings.ContainsAny(image.ContentID+image.FileName, "\r\n\"<>") {
				return mimePart{}, ErrInvalidMessage
			}
			contentType := mime.TypeByExtension(filepath.Ext(image.FileName))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			part := base64Part(contentType, image.Content)
			part.header.Set("Content-ID", "<"+image.ContentID+">")
			part.header.Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", image.FileName))
			parts = append(parts, part)
		}
		var err error
		if body, err = multipartBody(`related; type="text/html"`, parts); err != nil {
			return mimePart{}, err
		}
	}
	if strings.TrimSpace(message.Text) != "" {
		return multipartBody("alternative", []mimePart{base64Part("text/plain; charset=UTF-8", []byte(message.Text)), body})
	}
	return body, nil
}

func multipartBody(subtype string, parts []mimePart) (mimePart, error) {
	var content bytes.Buffer
	writer := multipart.NewWriter(&content)
	for _, part := range parts {
		partWriter, err := writer.CreatePart(part.header)
		if err != nil {
			return mimePart{}, err
		}
		if _, err := partWriter.Write(part.content); err != nil {
			return mimePart{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return mimePart{}, err
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", fmt.Sprintf("multipart/%s; boundary=%s", subtype, writer.Boundary()))
	return mimePart{header: header, content: content.Bytes()}, nil
}

// base64Part кодирует тело в base64 строками по 76 символов: это разом снимает
// ограничение SMTP на длину строки, точку в начале строки и восьмибитные символы.
func base64Part(contentType string, content []byte) mimePart {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "base64")
	var body bytes.Buffer
	encoded := base64.StdEncoding.EncodeToString(content)
	for len(encoded) > base64LineLength {
		body.WriteString(encoded[:base64LineLength] + "\r\n")
		encoded = encoded[base64LineLength:]
	}
	body.WriteString(encoded + "\r\n")
	return mimePart{header: header, content: body.Bytes()}
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
