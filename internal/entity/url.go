package entity

import (
	"time"

	"github.com/google/uuid"
)

type URL struct {
	ID          uuid.UUID
	ShortCode   string
	OriginalURL string
	CustomAlias *string
	ExpiresAt   *time.Time
	IsActive    bool
	ClickCount  int64
	CreatedAt   time.Time
}
