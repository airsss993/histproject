package notifications

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

func allLetters(t *testing.T) map[string]Email {
	t.Helper()
	submission := Submission{
		PublicID: "RH-7K3F9Q", Title: `<script>alert(1)</script> Бородино`, Email: "author@example.test",
		TelegramUsername: "hero", SiteURL: "https://russia-heroes.ru/sites/7/", Comment: "Исправьте описание\nи даты",
	}
	letters := map[string]Email{}
	add := func(name string, message Email, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		letters[name] = message
	}
	message, err := LoginCodeEmail("user@example.test", "123456")
	add("code", message, err)
	for _, action := range []Action{ActionCreated, ActionReady} {
		message, err := adminEmail(action, submission, []string{"admin@example.test"}, "https://russia-heroes.ru/panel")
		add(string(action), message, err)
	}
	for _, action := range []Action{ActionReviewed, ActionRejected, ActionPublished} {
		message, err := authorEmail(action, submission)
		add(string(action), message, err)
	}
	return letters
}

func TestLettersRenderEscapedWithImages(t *testing.T) {
	for name, message := range allLetters(t) {
		if strings.Contains(message.HTML, "<script>") || strings.Contains(message.HTML, "{{") {
			t.Errorf("%s: неэкранированная вставка в HTML", name)
		}
		if strings.TrimSpace(message.Text) == "" {
			t.Errorf("%s: пустая text/plain версия", name)
		}
		for _, image := range message.Images {
			if !strings.Contains(message.HTML, "cid:"+image.ContentID) {
				t.Errorf("%s: картинка %s не используется в HTML", name, image.ContentID)
			}
		}
		if len(message.Images) != 2 {
			t.Errorf("%s: картинок %d, want 2", name, len(message.Images))
		}
	}
}

func TestLettersContent(t *testing.T) {
	letters := allLetters(t)
	checks := map[string][]string{
		"code":      {"123456", "10 минут"},
		"created":   {"author@example.test", "@hero", "https://russia-heroes.ru/panel"},
		"ready":     {"готова к проверке", "https://russia-heroes.ru/panel"},
		"reviewed":  {"на проверке"},
		"rejected":  {"Исправьте описание", "Комментарий модератора"},
		"published": {"https://russia-heroes.ru/sites/7/", "Открыть событие"},
	}
	for name, wants := range checks {
		for _, want := range wants {
			if !strings.Contains(letters[name].HTML, want) || !strings.Contains(letters[name].Text, want) {
				t.Errorf("%s: нет %q в HTML или тексте", name, want)
			}
		}
	}
	withoutURL, err := adminEmail(ActionReady, Submission{PublicID: "RH-000001", Title: "Т", Email: "a@example.test"}, []string{"a@example.test"}, "")
	if err != nil || strings.Contains(withoutURL.HTML, "Открыть панель") {
		t.Errorf("без ADMIN_PANEL_URL кнопки быть не должно: err=%v", err)
	}
}

func TestBuildLetterWithImagesParsesBack(t *testing.T) {
	message, err := LoginCodeEmail("user@example.test", "123456")
	if err != nil {
		t.Fatal(err)
	}
	letter, err := buildLetter("info@russia-heroes.ru", message)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(letter))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	var walk func(contentType string, body io.Reader)
	walk = func(contentType string, body io.Reader) {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			t.Fatalf("Content-Type %q: %v", contentType, err)
		}
		if !strings.HasPrefix(mediaType, "multipart/") {
			return
		}
		found[mediaType] = ""
		reader := multipart.NewReader(body, params["boundary"])
		for {
			part, err := reader.NextRawPart()
			if err == io.EOF {
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			partType := part.Header.Get("Content-Type")
			raw, _ := io.ReadAll(part)
			if strings.HasPrefix(partType, "multipart/") {
				walk(partType, bytes.NewReader(raw))
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(raw), "\r\n", ""))
			if err != nil {
				t.Fatalf("%s: не base64: %v", partType, err)
			}
			found[partType+part.Header.Get("Content-ID")] = string(decoded)
		}
	}
	walk(parsed.Header.Get("Content-Type"), parsed.Body)

	if _, ok := found["multipart/alternative"]; !ok {
		t.Error("нет multipart/alternative")
	}
	if found["text/plain; charset=UTF-8"] != message.Text || found["text/html; charset=UTF-8"] != message.HTML {
		t.Error("текст или HTML не совпадают после разбора")
	}
	for _, image := range message.Images {
		if found["image/png<"+image.ContentID+">"] != string(image.Content) {
			t.Errorf("картинка %s не совпадает после разбора", image.ContentID)
		}
	}
}
