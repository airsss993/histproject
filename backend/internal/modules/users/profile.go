package users

import (
	"context"
	"strings"
	"time"
)

const statusPublished = "published"

// Work - заявка автора в профиле. Заявка привязана к учётке по почте: её подтверждает код при входе,
// и письма о статусе идут на неё же, так что автор видит в профиле ровно то, о чём ему пишут.
// ponytail: связь по почте, а не колонкой user_id. Появится смена почты в учётке - тогда и колонка.
type Work struct {
	ID            int64     `json:"id"`
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

// Author - автор в чужом профиле: без почты, её видеть посторонним незачем.
type Author struct {
	ID        int64     `json:"id"`
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

// Profile отдаёт публичный профиль автора. token - кука смотрящего, пустая у гостя.
func (service *Service) Profile(ctx context.Context, id int64, token string) (Profile, error) {
	if id <= 0 {
		return Profile{}, ErrUserNotFound
	}
	user, err := service.repository.FindByID(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	owner := false
	if viewer, viewerErr := service.Authenticate(ctx, token); viewerErr == nil {
		owner = viewer.ID == user.ID
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
		User:  Author{ID: user.ID, FirstName: user.FirstName, LastName: user.LastName, CreatedAt: user.CreatedAt},
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
