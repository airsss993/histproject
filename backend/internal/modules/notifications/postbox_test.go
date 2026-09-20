package notifications

import (
	"bufio"
	"encoding/base64"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeSMTP struct {
	listener net.Listener
	offerTLS bool
	mutex    sync.Mutex
	commands []string
	message  string
}

func startFakeSMTP(t *testing.T, offerTLS bool) *fakeSMTP {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &fakeSMTP{listener: listener, offerTLS: offerTLS}
	go server.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

func (server *fakeSMTP) address() string { return server.listener.Addr().String() }

func (server *fakeSMTP) serve() {
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		go server.handle(connection)
	}
}

func (server *fakeSMTP) handle(connection net.Conn) {
	defer func() { _ = connection.Close() }()
	reader := bufio.NewReader(connection)
	write := func(line string) { _, _ = connection.Write([]byte(line + "\r\n")) }
	write("220 fake ESMTP")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		server.mutex.Lock()
		server.commands = append(server.commands, line)
		server.mutex.Unlock()
		switch {
		case strings.HasPrefix(line, "EHLO"):
			write("250-fake")
			if server.offerTLS {
				write("250-STARTTLS")
			}
			write("250 AUTH PLAIN")
		case line == "DATA":
			write("354 send it")
			var body strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				body.WriteString(dataLine)
			}
			server.mutex.Lock()
			server.message = body.String()
			server.mutex.Unlock()
			write("250 queued")
		case line == "QUIT":
			write("221 bye")
			return
		default:
			write("250 ok")
		}
	}
}

func (server *fakeSMTP) dialog() ([]string, string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return append([]string(nil), server.commands...), server.message
}

func decodeLetterBody(t *testing.T, letter string) string {
	t.Helper()
	_, body, found := strings.Cut(letter, "\r\n\r\n")
	if !found {
		t.Fatalf("письмо без пустой строки между заголовками и телом: %q", letter)
	}
	for _, line := range strings.Split(strings.TrimRight(body, "\r\n"), "\r\n") {
		if len(line) > base64LineLength {
			t.Fatalf("строка тела длиннее %d символов: %d", base64LineLength, len(line))
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimRight(body, "\r\n"), "\r\n", ""))
	if err != nil {
		t.Fatalf("тело не декодируется из base64: %v", err)
	}
	return string(decoded)
}

func TestBuildLetterEncodesHeadersAndBody(t *testing.T) {
	html := "<p>Ваш код для входа на сайт \"Россия - страна Героев\": 123456. Код действует 10 минут.</p>"

	letter, err := buildLetter("Россия - страна Героев <info@russia-heroes.ru>", Email{
		To:      []string{"user@example.test", "Модератор <admin@example.test>"},
		Subject: "Код входа: 123456",
		HTML:    html,
	})
	if err != nil {
		t.Fatalf("buildLetter() error = %v", err)
	}

	text := string(letter)
	for _, want := range []string{
		"From: =?utf-8?q?", "<info@russia-heroes.ru>",
		"To: <user@example.test>, ", "<admin@example.test>",
		"Subject: =?utf-8?q?",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/html; charset=UTF-8\r\n",
		"Content-Transfer-Encoding: base64\r\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("письмо без %q:\n%s", want, text)
		}
	}
	headers, _, _ := strings.Cut(text, "\r\n\r\n")
	for index := 0; index < len(headers); index++ {
		if headers[index] > 127 {
			t.Fatalf("восьмибитный символ в заголовках, позиция %d:\n%s", index, headers)
		}
	}
	if decoded := decodeLetterBody(t, text); decoded != html {
		t.Errorf("тело = %q, want %q", decoded, html)
	}
}

func TestBuildLetterRejectsInvalidMessages(t *testing.T) {
	tests := []struct {
		name    string
		message Email
	}{
		{"без получателей", Email{Subject: "Тема", HTML: "<p>Тело</p>"}},
		{"пустая тема", Email{To: []string{"user@example.test"}, Subject: "   ", HTML: "<p>Тело</p>"}},
		{"пустое тело", Email{To: []string{"user@example.test"}, Subject: "Тема", HTML: ""}},
		{"кривой адрес", Email{To: []string{"не адрес"}, Subject: "Тема", HTML: "<p>Тело</p>"}},
		{"перенос строки в теме", Email{
			To:      []string{"user@example.test"},
			Subject: "Тема\r\nBcc: attacker@example.test",
			HTML:    "<p>Тело</p>",
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildLetter("info@russia-heroes.ru", test.message); err != ErrInvalidMessage {
				t.Fatalf("buildLetter() error = %v, want %v", err, ErrInvalidMessage)
			}
		})
	}
}

func TestDeliverLetterSendsEnvelopeAndBody(t *testing.T) {
	server := startFakeSMTP(t, false)
	client, err := smtp.Dial(server.address())
	if err != nil {
		t.Fatalf("smtp.Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	letter, err := buildLetter("info@russia-heroes.ru", Email{
		To:      []string{"user@example.test", "Модератор <admin@example.test>"},
		Subject: "Заявка #7 опубликована",
		HTML:    "<p>Заявка опубликована.</p>",
	})
	if err != nil {
		t.Fatalf("buildLetter() error = %v", err)
	}
	if err := deliverLetter(client, "info@russia-heroes.ru", []string{"user@example.test", "Модератор <admin@example.test>"}, letter); err != nil {
		t.Fatalf("deliverLetter() error = %v", err)
	}

	commands, message := server.dialog()
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"MAIL FROM:<info@russia-heroes.ru>",
		"RCPT TO:<user@example.test>",
		"RCPT TO:<admin@example.test>",
		"DATA",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("диалог без команды %q:\n%s", want, joined)
		}
	}
	if decoded := decodeLetterBody(t, message); decoded != "<p>Заявка опубликована.</p>" {
		t.Errorf("доставленное тело = %q", decoded)
	}
}

func TestPostboxSenderRefusesWithoutSTARTTLS(t *testing.T) {
	server := startFakeSMTP(t, false)
	sender, err := NewPostboxSender(server.address(), "key-id", "key-secret", "info@russia-heroes.ru", 5*time.Second)
	if err != nil {
		t.Fatalf("NewPostboxSender() error = %v", err)
	}

	err = sender.Send(t.Context(), Email{To: []string{"user@example.test"}, Subject: "Тема", HTML: "<p>Тело</p>"})
	if err == nil {
		t.Fatal("Send() error = nil, ключ ушёл бы по открытому соединению")
	}
	if strings.Contains(err.Error(), "key-secret") {
		t.Errorf("ошибка раскрывает секрет: %v", err)
	}
	commands, message := server.dialog()
	if joined := strings.Join(commands, "\n"); strings.Contains(joined, "AUTH") || message != "" {
		t.Errorf("отправитель продолжил диалог без STARTTLS:\n%s", joined)
	}
}

func TestNewPostboxSenderValidatesConfig(t *testing.T) {
	tests := []struct {
		name    string
		address string
		keyID   string
		secret  string
		from    string
		timeout time.Duration
	}{
		{"адрес без порта", "postbox.cloud.yandex.net", "key-id", "key-secret", "info@russia-heroes.ru", time.Second},
		{"пустой ключ", DefaultPostboxAddress, "", "key-secret", "info@russia-heroes.ru", time.Second},
		{"пустой секрет", DefaultPostboxAddress, "key-id", "", "info@russia-heroes.ru", time.Second},
		{"кривой отправитель", DefaultPostboxAddress, "key-id", "key-secret", "не адрес", time.Second},
		{"нулевой таймаут", DefaultPostboxAddress, "key-id", "key-secret", "info@russia-heroes.ru", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewPostboxSender(test.address, test.keyID, test.secret, test.from, test.timeout); err != ErrInvalidSenderConfig {
				t.Fatalf("NewPostboxSender() error = %v, want %v", err, ErrInvalidSenderConfig)
			}
		})
	}
}
