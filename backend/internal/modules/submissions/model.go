package submissions

import (
	"io"
	"time"
)

type Status string

const (
	StatusProcessing Status = "processing"
	StatusNew        Status = "new"
	StatusReview     Status = "review"
	StatusRejected   Status = "rejected"
	StatusPublished  Status = "published"
)

type Submission struct {
	// ID внутренний. Людям (письма, профиль, панель) показывается PublicID вида RH-7K3F9Q.
	ID               int64
	PublicID         string
	Title            string
	Description      string
	EventDate        time.Time
	EventType        string
	Email            string
	TelegramUsername string
	// Latitude и Longitude пусты у заявок, созданных до того, как точку стал ставить автор.
	Latitude      *float64
	Longitude     *float64
	ArchiveID     string
	SiteURL       string
	ScreenshotURL string
	Slug          string
	Status        Status
	AdminComment  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Actor struct {
	Login string
	Role  string
}

type Audit struct {
	AdminLogin string
	AdminRole  string
	Action     string
	Comment    string
}

type Transition struct {
	ID            int64
	From          Status
	To            Status
	Comment       string
	SiteURL       string
	ScreenshotURL string
	// EventType пуст, если тип не меняется.
	EventType string
	Audit     Audit
}

type EventSnapshot struct {
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

type CreateRecord struct {
	Title            string
	Description      string
	EventDate        time.Time
	EventType        string
	Email            string
	TelegramUsername string
	Latitude         float64
	Longitude        float64
	ArchiveID        string
	Slug             string
}

type CreateInput struct {
	Title            string
	Description      string
	EventDate        time.Time
	EventType        string
	Email            string
	TelegramUsername string
	Latitude         float64
	Longitude        float64
	ArchiveName      string
	ArchiveSize      int64
	Archive          io.Reader
}

type ListFilter struct {
	Status   Status
	Search   string
	Page     int
	PageSize int
}

type SubmissionPage struct {
	Items    []Submission
	Page     int
	PageSize int
	Total    int64
}

type AuditEntry struct {
	ID                 int64
	SubmissionID       int64
	SubmissionPublicID string
	SubmissionTitle    string
	AdminLogin         string
	AdminRole          string
	Action             string
	Comment            string
	CreatedAt          time.Time
}

type AuditFilter struct {
	Search   string
	Page     int
	PageSize int
}

type AuditPage struct {
	Items    []AuditEntry
	Page     int
	PageSize int
	Total    int64
}
