package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TestRequest represents a /v1/test request payload.
type TestRequest struct {
	ResourceManifest interface{} `json:"resourceManifest" binding:"required"`
	ModRules         interface{} `json:"modRules" binding:"required"`
}

// Set up the web app HTTP routes.
func (app *KubeModWebApp) setupRoutes(router *gin.Engine) {
	v1 := router.Group("/v1")

	// TODO - implement test handler.
	v1.POST("/test", app.v1Test)
}

// Tests a set of ModRules against a resource.
func (app *KubeModWebApp) v1Test(c *gin.Context) {
	var payload TestRequest

	if err := c.ShouldBind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		app.log.Error(err, "/v1/test failed")
		return
	}

	app.log.V(1).Info("processing request", "path", "/v1/test", "payload", payload)

	c.JSON(http.StatusOK, payload)
}
