package entity

import (
	"time"

	"github.com/google/uuid"
)

type Analytics struct {
	ID        uuid.UUID `json:"id"`
	URLId     uuid.UUID `json:"url_id"`
	UserAgent string    `json:"user_agent"`
	IPAddress string    `json:"ip_address"`
	Referer   string    `json:"referer,omitempty"`
	ClickedAt time.Time `json:"clicked_at"`
}

type AnalyticsStats struct {
	ShortCode    string           `json:"short_code"`
	OriginalURL  string           `json:"original_url"`
	TotalClicks  int64            `json:"total_clicks"`
	ClicksByDay  map[string]int64 `json:"clicks_by_day"`
	ClicksByUA   map[string]int64 `json:"clicks_by_user_agent"`
	RecentClicks []Analytics      `json:"recent_clicks"`
	CreatedAt    time.Time        `json:"created_at"`
}

func (a *Analytics) GetTime() time.Time {
	return a.ClickedAt
}
