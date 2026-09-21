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
}

// Works отдаёт заявки автора по почте. Реализует модуль submissions через адаптер в internal/app.
type Works interface {
	ByAuthor(ctx context.Context, email string) ([]Work, error)
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
	viewer, viewerErr := service.Authenticate(ctx, token)
	switch {
	case viewerErr == nil:
		owner = viewer.ID == user.ID
	case !errors.Is(viewerErr, ErrNoSession):
		// Сбой хранилища сессий отдаём ошибкой: иначе владелец молча получает гостевой профиль без своих заявок.
		return Profile{}, viewerErr
	}
	works := []Work{}
	if service.works != nil {
		works, err = service.works.ByAuthor(ctx, strings.ToLower(user.Email))
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

// onlyPublished оставляет опубликованные работы и снимает с них комментарий модератора: он для автора.
func onlyPublished(works []Work) []Work {
	public := make([]Work, 0, len(works))
	for _, work := range works {
		if work.Status != statusPublished {
			continue
		}
		work.Comment = ""
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
