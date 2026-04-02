// nolint: revive,staticcheck
package handler

import (
	"net/http"
	"time"

	"lshortener/internal/service"

	"github.com/gin-gonic/gin"
)

// @Summary Создать сокращенную ссылку
// @Description Создает сокращенную ссылку из оригинальной
// @Tags ShortURL
// @Accept json
// @Produce json
// @Param request body CreateShortURLRequest true "Данные для создания короткой ссылки"
// @Success 201 {object} CreateShortURLResponse "Короткая ссылка создана"
// @Failure 400 {object} ErrorResponse "Ошибка валидации"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /shorten [post]
func (h *ShortenerHandler) CreateShortURL(c *gin.Context) {
	const op = "transport.http.CreateShortURL"

	ctx := c.Request.Context()

	var req CreateShortURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_request", "Invalid request body", err)
		return
	}

	serviceReq := service.CreateURLRequest{
		OriginalURL: req.OriginalURL,
		CustomAlias: req.CustomAlias,
		ExpiresAt:   req.ExpiresAt,
	}

	result, err := h.svc.CreateShortURL(ctx, serviceReq)
	if err != nil {
		h.handleServiceError(c, op, err)
		return
	}

	response := CreateShortURLResponse{
		ID:          result.ID,
		ShortCode:   result.ShortCode,
		ShortURL:    result.ShortURL,
		OriginalURL: result.OriginalURL,
		CustomAlias: result.CustomAlias,
		ExpiresAt:   result.ExpiresAt,
		CreatedAt:   result.CreatedAt,
	}

	h.respondSuccess(c, http.StatusCreated, response)
}

// @Summary Переход по короткой ссылке
// @Description Перенаправляет на оригинальную ссылку по короткому коду
// @Tags ShortURL
// @Param short_code path string true "Короткий код ссылки" minlength(1)
// @Success 301 {string} string "Redirect to original URL"
// @Failure 400 {object} ErrorResponse "Отсутствует short_code"
// @Failure 404 {object} ErrorResponse "Ссылка не найдена"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка"
// @Router /{short_code} [get]
func (h *ShortenerHandler) RedirectToOriginal(c *gin.Context) {
	const op = "transport.http.RedirectToOriginal"

	ctx := c.Request.Context()

	shortCode := c.Param("short_code")
	if shortCode == "" {
		h.respondError(c, http.StatusBadRequest, "missing_code", "Short code is required", nil)
		return
	}

	clickInfo := service.ClickInfo{
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
		Referer:   c.Request.Referer(),
	}

	originalURL, err := h.svc.ResolveShortURL(ctx, shortCode, clickInfo)
	if err != nil {
		h.handleServiceError(c, op, err)
		return
	}

	c.Redirect(http.StatusMovedPermanently, originalURL)
}

// @Summary Получить аналитику по короткой ссылке
// @Description Возвращает статистику переходов по короткому коду
// @Tags ShortURL
// @Accept json
// @Produce json
// @Param short_code path string true "Короткий код ссылки" minlength(1)
// @Success 200 {object} AnalyticsResponse "Аналитика получена"
// @Failure 400 {object} ErrorResponse "Отсутствует short_code"
// @Failure 404 {object} ErrorResponse "Ссылка не найдена"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка"
// @Router /{short_code}/analytics [get]
func (h *ShortenerHandler) GetAnalytics(c *gin.Context) {
	const op = "transport.http.GetAnalytics"

	ctx := c.Request.Context()

	shortCode := c.Param("short_code")
	if shortCode == "" {
		h.respondError(c, http.StatusBadRequest, "missing_code", "Short code is required", nil)
		return
	}

	stats, err := h.svc.GetAnalytics(ctx, shortCode)
	if err != nil {
		h.handleServiceError(c, op, err)
		return
	}

	recentClicks := make([]ClickDetail, 0, len(stats.RecentClicks))
	for _, click := range stats.RecentClicks {
		recentClicks = append(recentClicks, ClickDetail{
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
	h.respondSuccess(c, http.StatusOK, response)
}

func (h *ShortenerHandler) Health(c *gin.Context) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}
	h.respondJSON(c, http.StatusOK, response)
}

func (h *ShortenerHandler) respondSuccess(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, data)
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
