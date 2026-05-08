package entity

import (
	"time"

	"github.com/google/uuid"
)

type Analytics struct {
	ID        uuid.UUID
	URLID     uuid.UUID
	UserAgent string
	IPAddress string
	Referer   string
	ClickedAt time.Time
}

type AnalyticsStats struct {
	ShortCode    string
	OriginalURL  string
	TotalClicks  int64
	ClicksByDay  map[string]int64
	ClicksByUA   map[string]int64
	RecentClicks []Analytics
	CreatedAt    time.Time
}
