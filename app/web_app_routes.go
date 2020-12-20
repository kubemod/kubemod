package app

import "github.com/gin-gonic/gin"

// Set up the web app HTTP routes.
func setupRoutes(router *gin.Engine) {
	v1 := router.Group("/v1")

	// TODO - implement test handler.
	v1.POST("/test")
}
