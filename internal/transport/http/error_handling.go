package handler

import (
	"errors"
	"net/http"

	"lshortener/internal/entity"
	"lshortener/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/logger"
)

func (h *ShortenerHandler) handleServiceError(c *gin.Context, op string, err error) {
	ctx := c.Request.Context()
	log := h.log.Ctx(ctx)

	switch {
	case errors.Is(err, service.ErrURLNotFound) || errors.Is(err, entity.ErrURLNotFound):
		log.LogAttrs(ctx, logger.WarnLevel, "url not found",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusNotFound, "not_found", "URL not found", err)
	case errors.Is(err, service.ErrURLExpired) || errors.Is(err, entity.ErrURLExpired):
		log.LogAttrs(ctx, logger.WarnLevel, "url expired",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusGone, "expired", "URL has expired", err)
	case errors.Is(err, service.ErrURLInactive) || errors.Is(err, entity.ErrURLInactive):
		log.LogAttrs(ctx, logger.WarnLevel, "url inactive",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusGone, "inactive", "URL is inactive", err)
	case errors.Is(err, service.ErrAliasAlreadyExists) || errors.Is(err, entity.ErrAliasAlreadyExists):
		log.LogAttrs(ctx, logger.WarnLevel, "alias already exists",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusConflict, "alias_exists", "Custom alias already exists", err)
	case errors.Is(err, service.ErrInvalidURL) || errors.Is(err, entity.ErrInvalidURL):
		log.LogAttrs(ctx, logger.WarnLevel, "invalid url",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusBadRequest, "invalid_url", "Invalid URL format", err)
	case errors.Is(err, entity.ErrInvalidData):
		log.LogAttrs(ctx, logger.WarnLevel, "invalid data",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusBadRequest, "invalid_data", "Invalid input data", err)
	case errors.Is(err, entity.ErrConflictingData):
		log.LogAttrs(ctx, logger.WarnLevel, "conflicting data",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusBadRequest, "conflict", "Data conflict occurred", err)
	default:
		log.LogAttrs(ctx, logger.ErrorLevel, "internal server error",
			logger.String("op", op),
			logger.Any("error", err),
		)
		h.respondError(c, http.StatusInternalServerError, "internal_error",
			"Internal server error occurred", err)
	}
}
