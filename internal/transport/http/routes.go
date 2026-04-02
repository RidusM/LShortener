package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Link Shortener API
// @version 1.0
// @description API для сокращения ссылок
// @termosOfService http://swagger.io/terms
// @contect.name RidusM
// @contect.email stormkillpeople@gmail.com
// @license.name MIT-0
// @license.url https://github.com/aws/mit-0
// @host localhost:8080
// @BasePath /
func (h *ShortenerHandler) setupRoutes() {
	h.router.GET("/health", h.Health)
	h.router.POST("/shorten", h.CreateShortURL)
	h.router.GET("/analytics/:short_code", h.GetAnalytics)

	h.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	h.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
