package users

import (
	"context"
	"errors"
	"strings"
	"time"
)

const statusPublished = "published"

// Work - заявка автора в профиле. Заявка привязана к учётке по почте: её подтверждает код при входе,
// и письма о статусе идут на неё же, так что автор видит в профиле ровно то, о чём ему пишут.
// ponytail: связь по почте, а не колонкой user_id. Появится смена почты в учётке - тогда и колонка.
type Work struct {
	PublicID      string    `json:"publicId"`
	Title         string    `json:"title"`
	EventDate     string    `json:"eventDate"`
	EventType     string    `json:"eventType"`
	Status        string    `json:"status"`
	SiteURL       string    `json:"siteUrl"`
	ScreenshotURL string    `json:"screenshotUrl"`
	Comment       string    `json:"comment,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	// EventID, Stars, Starred, Visitors есть у опубликованной работы: у неё есть событие на карте.
	EventID  *int64 `json:"eventId,omitempty"`
	Stars    int64  `json:"stars"`
	Starred  bool   `json:"starred"`
	Visitors int64  `json:"visitors"`
	// Stats - просмотры сайта, только владельцу профиля.
	Stats *WorkStats `json:"stats,omitempty"`
}

type WorkStats struct {
	Views         int64 `json:"views"`
	Visitors      int64 `json:"visitors"`
	ActiveSeconds int64 `json:"activeSeconds"`
}

// Works отдаёт заявки автора по почте. viewerID - смотрящий для признака Starred, 0 у гостя.
// Реализуют модули submissions и events через адаптер в internal/app.
type Works interface {
	ByAuthor(ctx context.Context, email string, viewerID int64) ([]Work, error)
}

// Author - автор в чужом профиле: без почты и числового id, их видеть посторонним незачем.
type Author struct {
	PublicID  string    `json:"publicId"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

type Profile struct {
	User Author `json:"user"`
	// Owner - смотрит сам автор. Ему видны все заявки, гостю - только опубликованные.
	Owner bool   `json:"owner"`
	Works []Work `json:"works"`
}

// Profile отдаёт публичный профиль автора по коду RHU-XXXXXX. token - кука смотрящего, пустая у гостя.
func (service *Service) Profile(ctx context.Context, publicID, token string) (Profile, error) {
	code, ok := parsePublicID(publicID)
	if !ok {
		return Profile{}, ErrUserNotFound
	}
	user, err := service.repository.FindByPublicID(ctx, code)
	if err != nil {
		return Profile{}, err
	}
	owner := false
	var viewerID int64
	viewer, viewerErr := service.Authenticate(ctx, token)
	switch {
	case viewerErr == nil:
		owner = viewer.ID == user.ID
		viewerID = viewer.ID
	case !errors.Is(viewerErr, ErrNoSession):
		// Сбой хранилища сессий отдаём ошибкой: иначе владелец молча получает гостевой профиль без своих заявок.
		return Profile{}, viewerErr
	}
	works := []Work{}
	if service.works != nil {
		works, err = service.works.ByAuthor(ctx, strings.ToLower(user.Email), viewerID)
		if err != nil {
			return Profile{}, err
		}
	}
	if !owner {
		works = onlyPublished(works)
	}
	return Profile{
		User:  Author{PublicID: user.PublicID, FirstName: user.FirstName, LastName: user.LastName, CreatedAt: user.CreatedAt},
		Owner: owner,
		Works: works,
	}, nil
}

// onlyPublished оставляет опубликованные работы и снимает с них то, что только для автора:
// комментарий модератора и статистику просмотров.
func onlyPublished(works []Work) []Work {
	public := make([]Work, 0, len(works))
	for _, work := range works {
		if work.Status != statusPublished {
			continue
		}
		work.Comment = ""
		work.Stats = nil
		public = append(public, work)
	}
	return public
}

const publicIDAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// parsePublicID принимает код с префиксом и без, в любом регистре. Похожие буквы O, I, L
// читает как цифры 0 и 1: так декодирует Crockford base32, и продиктованный код найдётся.
func parsePublicID(value string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(value))
	code = strings.TrimPrefix(code, "RHU-")
	code = strings.NewReplacer("O", "0", "I", "1", "L", "1").Replace(code)
	if len(code) != 6 || strings.Trim(code, publicIDAlphabet) != "" {
		return "", false
	}
	return code, true
}
