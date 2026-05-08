package handler

import (
	"errors"
	"net/http"

	"lshortener/internal/entity"

	"github.com/gin-gonic/gin"
)

func (h *ShortenerHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, entity.ErrDataNotFound):
		h.respondError(c, http.StatusNotFound, "not_found",
			"Data not found", err)
	case errors.Is(err, entity.ErrInvalidData):
		h.respondError(c, http.StatusBadRequest, "invalid_data",
			"Invalid input data", err)
	case errors.Is(err, entity.ErrConflictingData):
		h.respondError(c, http.StatusConflict, "conflict",
			"Data conflict occurred", err)
	case errors.Is(err, entity.ErrURLExpired) || errors.Is(err, entity.ErrURLInactive):
		h.respondError(c, http.StatusGone, "expired",
			"URL has expired", err)
	case errors.Is(err, entity.ErrAliasAlreadyExists):
		h.respondError(c, http.StatusConflict, "alias_exists",
			"Custom alias already exists", err)
	case errors.Is(err, entity.ErrInvalidURL):
		h.respondError(c, http.StatusBadRequest, "invalid_url",
			"Invalid URL format", err)
	default:
		h.respondError(c, http.StatusInternalServerError, "internal_error",
			"Internal server error occurred", err)
	}
}
