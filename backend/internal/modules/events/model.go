package events

import "time"

type Event struct {
	ID              int64
	SubmissionID    int64
	Title           string
	Description     string
	Latitude        float64
	Longitude       float64
	EventDate       time.Time
	EventType       string
	SiteURL         string
	PreviewImageURL *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// Stars - число звёзд, Starred - стоит ли звезда смотрящего. Заполняют только List и Get.
	Stars   int64
	Starred bool
	// Visitors - уникальные посетители сайта, видны всем. Заполняют только List и Get.
	Visitors int64
	// Author заполняет только Service.Get. Пустой PublicID - у почты заявки нет учётки на сайте.
	Author Author
}

// Author - автор события: учётка посетителя с почтой заявки. Email нужен только сервису.
type Author struct {
	Email     string
	PublicID  string
	FirstName string
	LastName  string
}

// Viewer - кто смотрит. Нулевой - гость.
type Viewer struct {
	UserID int64
	Email  string
}

type StarState struct {
	Stars   int64 `json:"stars"`
	Starred bool  `json:"starred"`
}

// Summary - звёзды и просмотры опубликованной работы для профиля автора.
type Summary struct {
	EventID       int64
	SubmissionID  int64
	Stars         int64
	Starred       bool
	Views         int64
	Visitors      int64
	ActiveSeconds int64
}

type Snapshot struct {
	SubmissionID    int64
	Title           string
	Description     string
	Latitude        float64
	Longitude       float64
	EventDate       time.Time
	EventType       string
	SiteURL         string
	PreviewImageURL string
}

type Filter struct {
	EventTypes []string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	// ViewerID - для признака Starred, 0 у гостя.
	ViewerID int64
}

type Page struct {
	Items    []Event
	Page     int
	PageSize int
	Total    int64
}

type ListInput struct {
	EventTypes []string
	DateFrom   string
	DateTo     string
	Page       int
	PageSize   int
}
