package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/core"
)

// TestRequest represents a /v1/test request payload.
type TestRequest struct {
	ResourceManifest interface{}        `json:"resourceManifest" binding:"required"`
	ModRules         []*v1beta1.ModRule `json:"modRules" binding:"required"`
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
	var err error

	if err = c.ShouldBind(&payload); err != nil {
		app.reportBadRequest(c, err)
		return
	}

	app.log.V(1).Info("processing request", "path", "/v1/test", "payload", payload)

	// Instantiate a ModRuleStore for this request and populate it with the modrules.
	store := core.NewModRuleStore(app.modRuleStoreItemFactory, app.log)

	for _, modRule := range payload.ModRules {
		// Populate the modrule with its default values if missing.
		modRule.Default()

		// Validate the modrule
		if err = modRule.ValidateCreate(); err != nil {
			app.reportBadRequest(c, err)
			return
		}

		// Add the modrule to the store.
		if err := store.Put(modRule); err != nil {
			app.reportBadRequest(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, payload)
}

// Common method to return an HTTP 400 bad request
func (app *KubeModWebApp) reportBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	app.log.Error(err, "request failed", "route", c.Request.URL)
	return
}
