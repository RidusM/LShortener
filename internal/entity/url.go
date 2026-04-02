package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	millisPerSecond  = 1000
	nanosPerMilli    = 1_000_000
	maxUnixTimestamp = 1<<63 - 1
)

type URL struct {
	ID          uuid.UUID  `json:"id"                     validate:"required,uuid_strict"`
	ShortCode   string     `json:"short_code"             validate:"required"`
	OriginalURL string     `json:"original_url"           validate:"required"`
	CustomAlias *string    `json:"custom_alias,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool       `json:"is_active"              validate:"required"`
	ClickCount  int64      `json:"click_count"            validate:"required,min=0"`
}

func ExtractTimestampFromUUIDv7(id uuid.UUID) time.Time {
	timestamp := uint64(id[0])<<40 | uint64(id[1])<<32 | uint64(id[2])<<24 |
		uint64(id[3])<<16 | uint64(id[4])<<8 | uint64(id[5])

	seconds := timestamp / millisPerSecond
	nanos := (timestamp % millisPerSecond) * nanosPerMilli

	if seconds > maxUnixTimestamp {
		return time.Time{}
	}

	// nolint:gosec
	return time.Unix(int64(seconds), int64(nanos)).UTC()
}
