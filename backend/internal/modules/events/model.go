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
}

type Filter struct {
	EventTypes []string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
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
