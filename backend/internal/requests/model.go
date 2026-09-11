package requests

import "time"

// RequestListItem — краткая информация о заявке для списка.
type RequestListItem struct {
	ID        int       `db:"id" json:"id"`
	Title     string    `db:"title" json:"title"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// RequestListResult — результат запроса списка заявок с метаданными пагинации.
type RequestListResult struct {
	Items []RequestListItem
	Total int
}

// HistoryResult — результат запроса истории с метаданными пагинации.
type HistoryResult struct {
	Items []RequestHistoryItem
	Total int
}

// RequestHistoryItem — запись аудит-лога действия администратора над заявкой.
type RequestHistoryItem struct {
	ID           int       `db:"id"            json:"id"`
	RequestID    int       `db:"request_id"    json:"requestId"`
	RequestTitle string    `db:"request_title" json:"requestTitle"`
	Action       string    `db:"action"        json:"action"`
	Comment      string    `db:"comment"       json:"comment"`
	AdminLogin   string    `db:"admin_login"   json:"adminLogin"`
	AdminRole    string    `db:"admin_role"    json:"adminRole"`
	CreatedAt    time.Time `db:"created_at"    json:"createdAt"`
}

// RequestDetail — полная информация о заявке для карточки.
type RequestDetail struct {
	ID               int       `db:"id" json:"id"`
	Title            string    `db:"title" json:"title"`
	Description      string    `db:"description" json:"description"`
	EventDate        string    `db:"event_date" json:"eventDate"`
	EventTypeID      int       `db:"event_type_id" json:"eventTypeId"`
	Email            string    `db:"email" json:"email"`
	TelegramUsername string    `db:"telegram_username" json:"telegramUsername"`
	Status           string    `db:"status" json:"status"`
	AdminComment     *string   `db:"admin_comment" json:"adminComment"`
	Slug             string    `db:"slug"           json:"slug"`
	SiteURL          string    `db:"site_url"       json:"siteUrl"`
	ScreenshotURL    string    `db:"screenshot_url" json:"-"`
	CreatedAt        time.Time `db:"created_at"     json:"createdAt"`
}
