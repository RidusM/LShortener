package handler

import (
	"net/http"

	_ "lshortener/docs" // required for Swagger

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Link Shortener API
// @version 1.0
// @description Link shortening API
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
	h.router.GET("/:short_code", h.RedirectToOriginal)
	h.router.GET("/:short_code/analytics", h.GetAnalytics)

	h.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	h.router.GET("/expired", func(c *gin.Context) {
		c.HTML(http.StatusOK, "expired.html", gin.H{})
	})
	h.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
