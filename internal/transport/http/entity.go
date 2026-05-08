// nolint: revive,staticcheck
// swagger:meta
package handler

import (
	"time"

	"github.com/google/uuid"
)

// swagger:model CreateShortURLRequest
type CreateURLRequest struct {
	OriginalURL string     `json:"original_url"           binding:"required,url"                           example:"https://example.com/very/long/url"`
	CustomAlias *string    `json:"custom_alias,omitempty" binding:"omitempty,min=3,max=50,alphanumunicode" example:"mylink"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"   binding:"omitempty,gt=0"                         example:"2026-12-31T23:59:59Z"`
}

// swagger:model CreateShortURLResponse
type CreateURLResponse struct {
	ID          uuid.UUID  `json:"id"                     example:"550e8400-e29b-41d4-a716-446655440001"`
	ShortCode   string     `json:"short_code"             example:"abc123"`
	ShortURL    string     `json:"short_url"              example:"https://short.link/abc123"`
	OriginalURL string     `json:"original_url"           example:"https://example.com/very/long/url"`
	CustomAlias *string    `json:"custom_alias,omitempty" example:"mylink"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"   example:"2026-12-31T23:59:59Z"`
	CreatedAt   time.Time  `json:"created_at"             example:"2026-05-07T10:00:00Z"`
}

// swagger:model AnalyticsResponse
type AnalyticsResponse struct {
	ShortCode         string           `json:"short_code"           example:"abc123"`
	OriginalURL       string           `json:"original_url"         example:"https://example.com/very/long/url"`
	TotalClicks       int64            `json:"total_clicks"         example:"1234"`
	ClicksByDay       map[string]int64 `json:"clicks_by_day"        example:"2024-01-01:12,2024-01-02:45"       swaggertype:"object,integer"`
	ClicksByUserAgent map[string]int64 `json:"clicks_by_user_agent" example:"Mozilla/5.0:89,curl/7.68.0:12"     swaggertype:"object,integer"`
	RecentClicks      []ClickInfo      `json:"recent_clicks"`
	CreatedAt         time.Time        `json:"created_at"           example:"2026-05-07T10:00:00Z"`
}

// swagger:model ClickDetail
type ClickInfo struct {
	UserAgent string    `json:"user_agent"        example:"Mozilla/5.0 (Windows NT 10.0; Win64; x64)"`
	IPAddress string    `json:"ip_address"        example:"192.168.1.100"`
	Referer   string    `json:"referer,omitempty" example:"https://google.com"`
	ClickedAt time.Time `json:"clicked_at"        example:"2026-05-07T10:00:00Z"`
}

// swagger:model ErrorResponse
type ErrorResponse struct {
	Error   string `json:"error"             example:"link not found"`
	Code    string `json:"code,omitempty"    example:"not_found"`
	Details string `json:"details,omitempty" example:"link with short_code abc123 does not exist"`
}

// swagger:model SuccessResponse
type SuccessResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}
