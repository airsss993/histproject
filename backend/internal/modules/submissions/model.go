package submissions

import "time"

type Status string

const (
	StatusProcessing Status = "processing"
	StatusNew        Status = "new"
	StatusReview     Status = "review"
	StatusRejected   Status = "rejected"
	StatusPublished  Status = "published"
)

type Submission struct {
	ID               int64
	Title            string
	Description      string
	EventDate        time.Time
	EventType        string
	Email            string
	TelegramUsername string
	ArchiveID        string
	SiteURL          string
	ScreenshotURL    string
	Slug             string
	Status           Status
	AdminComment     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
	Audit         Audit
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
