package handler

import (
	"context"
	"net/http"

	"lshortener/internal/entity"
	"lshortener/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/logger"
)

const _maxRequestBodySize = 1 << 20

type ShortenerService interface {
	CreateShortURL(ctx context.Context, req service.CreateURLRequest) (*service.CreateURLResponse, error)
	ResolveShortURL(ctx context.Context, shortCode string, clickInfo service.ClickInfo) (string, error)
	GetAnalytics(ctx context.Context, shortCode string) (*entity.AnalyticsStats, error)
}

type ShortenerHandler struct {
	svc    ShortenerService
	log    logger.Logger
	router *gin.Engine
}

func NewShortenerHandler(
	svc ShortenerService,
	log logger.Logger,
) *ShortenerHandler {
	h := &ShortenerHandler{
		svc: svc,
		log: log,
	}

	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, _maxRequestBodySize)
	})

	router.Use(h.loggingMiddleware())
	router.Use(h.requestIDMiddleware())
	router.Use(h.baseCORSMiddleware())
	router.Use(gin.Recovery())

	h.router = router

	h.router.LoadHTMLGlob("web/*.html")
	h.router.Static("/static", "./web")

	h.setupRoutes()

	return h
}

func (h *ShortenerHandler) Engine() *gin.Engine {
	return h.router
}
