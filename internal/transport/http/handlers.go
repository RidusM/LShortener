// nolint: revive,staticcheck
package handler

import (
	"errors"
	"net/http"
	"time"

	"lshortener/internal/entity"
	"lshortener/internal/service"

	"github.com/gin-gonic/gin"
)

// @Summary Create a shortened link
// @Description Creates a shortened link from the original
// @Tags ShortURL
// @Accept json
// @Produce json
// @Param request body CreateURLRequest true "Data for creating a short link"
// @Success 201 {object} CreateURLResponse "Short link created"
// @Failure 400 {object} ErrorResponse "Validation error"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /shorten [post]
func (h *ShortenerHandler) CreateShortURL(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_request", "Invalid request body", err)
		return
	}

	serviceReq := service.CreateURLRequest{
		OriginalURL: req.OriginalURL,
		CustomAlias: req.CustomAlias,
		ExpiresAt:   req.ExpiresAt,
	}

	url, err := h.svc.CreateShortURL(ctx, serviceReq)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response := CreateURLResponse{
		ID:          url.ID,
		ShortCode:   url.ShortCode,
		ShortURL:    url.ShortURL,
		OriginalURL: url.OriginalURL,
		CustomAlias: url.CustomAlias,
		ExpiresAt:   url.ExpiresAt,
		CreatedAt:   url.CreatedAt,
	}

	h.respondJSON(c, http.StatusCreated, response)
}

// @Summary Follow a short link
// @Description Redirects to the original link using a short code
// @Tags ShortURL
// @Param short_code path string true "Link short code" minlength(1)
// @Success 301 {string} string "Redirect to original URL"
// @Failure 400 {object} ErrorResponse "Missing short_code"
// @Failure 404 {object} ErrorResponse "Link not found"
// @Failure 500 {object} ErrorResponse "Internal error"
// @Router /{short_code} [get]
func (h *ShortenerHandler) RedirectToOriginal(c *gin.Context) {
	ctx := c.Request.Context()

	shortCode := c.Param("short_code")
	if shortCode == "" {
		h.respondError(c, http.StatusBadRequest, "missing_code", "Short code is required", nil)
		return
	}

	serviceReq := service.ClickInfo{
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
		Referer:   c.Request.Referer(),
	}

	originalURL, err := h.svc.ResolveShortURL(ctx, shortCode, serviceReq)
	if err != nil {
		if errors.Is(err, entity.ErrURLExpired) {
			c.Redirect(http.StatusFound, "/expired")
			return
		}
		h.handleServiceError(c, err)
		return
	}

	c.Redirect(http.StatusMovedPermanently, originalURL)
	c.Abort()
}

// @Summary Get analytics for a short link
// @Description Returns click statistics for a short code
// @Tags ShortURL
// @Accept json
// @Produce json
// @Param short_code path string true "Link short code" minlength(1)
// @Success 200 {object} AnalyticsResponse "Analytics received"
// @Failure 400 {object} ErrorResponse "Missing short_code"
// @Failure 404 {object} ErrorResponse "Link not found"
// @Failure 500 {object} ErrorResponse "Internal error"
// @Router /{short_code}/analytics [get]
func (h *ShortenerHandler) GetAnalytics(c *gin.Context) {
	ctx := c.Request.Context()

	shortCode := c.Param("short_code")
	if shortCode == "" {
		h.respondError(c, http.StatusBadRequest, "missing_code", "Short code is required", nil)
		return
	}

	stats, err := h.svc.GetAnalytics(ctx, shortCode)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	recentClicks := make([]ClickInfo, 0, len(stats.RecentClicks))
	for _, click := range stats.RecentClicks {
		recentClicks = append(recentClicks, ClickInfo{
			UserAgent: click.UserAgent,
			IPAddress: click.IPAddress,
			Referer:   click.Referer,
			ClickedAt: click.ClickedAt,
		})
	}

	response := AnalyticsResponse{
		ShortCode:         stats.ShortCode,
		OriginalURL:       stats.OriginalURL,
		TotalClicks:       stats.TotalClicks,
		ClicksByDay:       stats.ClicksByDay,
		ClicksByUserAgent: stats.ClicksByUA,
		RecentClicks:      recentClicks,
		CreatedAt:         stats.CreatedAt,
	}
	h.respondJSON(c, http.StatusOK, response)
}

// @Summary Health check endpoint
// @Description Return service status and current timestamp. No authentication required.
// @Tags System
// @Produce json
// @Success 200 {object} map[string]string "Service is healthy"
// @Router /health [get]
func (h *ShortenerHandler) Health(c *gin.Context) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}
	h.respondJSON(c, http.StatusOK, response)
}

func (h *ShortenerHandler) respondJSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

func (h *ShortenerHandler) respondError(c *gin.Context, status int, code, message string, err error) {
	response := ErrorResponse{
		Error: message,
		Code:  code,
	}
	if err != nil {
		response.Details = err.Error()
	}
	h.respondJSON(c, status, response)
}
