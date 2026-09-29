package notifications

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"strings"
)

// Картинки перерисовывает templates/render_assets.py.
//
//go:embed templates/letter.html templates/*.png
var letterFiles embed.FS

var letterTemplate = template.Must(template.ParseFS(letterFiles, "templates/letter.html"))

// Content-ID совпадают с литералами cid: в templates/letter.html.
const (
	bannerContentID = "banner@histproject"
	iconContentID   = "icon@histproject"
)

var errUnknownLetterIcon = errors.New("unknown letter icon")

type letterField struct {
	Label string
	Value string
}

type letterAction struct {
	Label string
	URL   string
}

// letterView - содержимое письма. Пустые поля в письмо не попадают.
type letterView struct {
	Icon      string
	Preheader string
	Title     string
	Lead      string
	Code      string
	Fields    []letterField
	Quote     string
	Action    *letterAction
	Note      string
}

func (view letterView) email(to []string, subject string) (Email, error) {
	icon, err := letterFiles.ReadFile("templates/icon-" + view.Icon + ".png")
	if err != nil {
		return Email{}, fmt.Errorf("%w: %q", errUnknownLetterIcon, view.Icon)
	}
	banner, err := letterFiles.ReadFile("templates/banner.png")
	if err != nil {
		return Email{}, err
	}
	var html bytes.Buffer
	if err := letterTemplate.Execute(&html, view); err != nil {
		return Email{}, fmt.Errorf("render letter: %w", err)
	}
	return Email{
		To: append([]string(nil), to...), Subject: subject, HTML: html.String(), Text: view.text(),
		Images: []InlineImage{
			{ContentID: bannerContentID, FileName: "banner.png", Content: banner},
			{ContentID: iconContentID, FileName: "icon.png", Content: icon},
		},
	}, nil
}

// text - версия для клиентов без HTML. Без неё письмо из одной вёрстки чаще уходит в спам.
func (view letterView) text() string {
	var text strings.Builder
	text.WriteString(view.Title + "\n\n" + view.Lead + "\n")
	if view.Code != "" {
		text.WriteString("\nКод: " + view.Code + "\n")
	}
	if len(view.Fields) > 0 {
		text.WriteString("\n")
		for _, field := range view.Fields {
			text.WriteString(field.Label + ": " + field.Value + "\n")
		}
	}
	if view.Quote != "" {
		text.WriteString("\nКомментарий модератора:\n" + view.Quote + "\n")
	}
	if view.Action != nil {
		text.WriteString("\n" + view.Action.Label + ": " + view.Action.URL + "\n")
	}
	if view.Note != "" {
		text.WriteString("\n" + view.Note + "\n")
	}
	text.WriteString("\n--\nРоссия - страна Героев\nПисьмо отправлено автоматически, отвечать на него не нужно.\n")
	return text.String()
}

// LoginCodeEmail - письмо с кодом входа на сайт. Срок совпадает с codeTTL модуля users.
func LoginCodeEmail(to, code string) (Email, error) {
	return letterView{
		Icon:      "lock",
		Preheader: "Код действует 10 минут",
		Title:     "Код для входа",
		Lead:      "Введите этот код на сайте, чтобы войти в аккаунт.",
		Code:      code,
		Note:      "Код действует 10 минут. Если вы не запрашивали вход, удалите это письмо.",
	}.email([]string{to}, "Код входа: "+code)
}

// checkEmail - проверочное письмо, которое супер-админ отправляет из панели.
func checkEmail(to string) (Email, error) {
	return letterView{
		Icon:      "published",
		Preheader: "Почта сайта работает",
		Title:     "Проверка почты",
		Lead:      "Письмо отправлено из панели модерации. Раз оно дошло, письма о заявках и коды входа тоже доходят.",
		Note:      "Если письмо попало в спам, отметьте его как не спам: так следующие придут во входящие.",
	}.email([]string{to}, "Проверка почты")
}

func adminEmail(submission Submission, recipients []string, adminPanelURL string) (Email, error) {
	view := letterView{
		Icon:      "processing",
		Preheader: submission.Title,
		Title:     fmt.Sprintf("Новая заявка %s", submission.PublicID),
		Lead:      "Заявка создана, архив сайта обрабатывается. Проверяйте статус в панели.",
		Fields:    submissionFields(submission),
		Action:    linkAction("Открыть панель", adminPanelURL),
	}
	subject := fmt.Sprintf("Заявка %s создана и обрабатывается", submission.PublicID)
	return view.email(recipients, subject)
}

func authorEmail(action Action, submission Submission) (Email, error) {
	fields := []letterField{{Label: "Название", Value: submission.Title}}
	view := letterView{
		Icon:      "review",
		Preheader: submission.Title,
		Title:     "Заявка на проверке",
		Lead:      "Модератор начал проверять вашу заявку. Результат придёт отдельным письмом.",
		Fields:    fields,
	}
	subject := fmt.Sprintf("Заявка %s взята на проверку", submission.PublicID)
	switch action {
	case ActionRejected:
		view.Icon = "rejected"
		view.Title = "Заявка отклонена"
		view.Lead = "Модератор отклонил заявку. Исправьте замечание и отправьте её заново."
		view.Quote = strings.TrimSpace(submission.Comment)
		subject = fmt.Sprintf("Заявка %s отклонена", submission.PublicID)
	case ActionPublished:
		view.Icon = "published"
		view.Title = "Событие опубликовано"
		view.Lead = "Заявка прошла проверку, событие появилось на карте."
		view.Action = linkAction("Открыть событие", submission.SiteURL)
		subject = fmt.Sprintf("Заявка %s опубликована", submission.PublicID)
	}
	return view.email([]string{submission.Email}, subject)
}

func submissionFields(submission Submission) []letterField {
	return []letterField{
		{Label: "Название", Value: submission.Title},
		{Label: "Email", Value: submission.Email},
		{Label: "Telegram", Value: telegramName(submission.TelegramUsername)},
	}
}

// linkAction без адреса не рисует кнопку: в dev ADMIN_PANEL_URL и SiteURL бывают пустыми.
func linkAction(label, url string) *letterAction {
	if strings.TrimSpace(url) == "" {
		return nil
	}
	return &letterAction{Label: label, URL: url}
}
