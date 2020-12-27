package app

import (
	"net/http"

	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/core"
	"github.com/kubemod/kubemod/util"
	"sigs.k8s.io/yaml"
)

// DryRunRequest represents a /v1/dry-run request payload.
type DryRunRequest struct {
	ResourceManifest interface{}        `json:"resourceManifest" binding:"required"`
	ModRules         []*v1beta1.ModRule `json:"modRules" binding:"required"`
}

// DryRunResponse represents the resonse of a successful /v1/dry-run
type DryRunResponse struct {
	Patch      interface{} `json:"patch"`
	Diff       string      `json:"diff"`
	Rejections []string    `json:"rejections"`
}

const (
	dryRunNamespace string = "dry-run-namespace"
)

// Set up the web app HTTP routes.
func (app *KubeModWebApp) setupRoutes(router *gin.Engine) {
	v1 := router.Group("/v1")

	v1.PUT("/dry-run", app.dryRunHandler)
}

// Dry-runs a set of ModRules against a resource.
func (app *KubeModWebApp) dryRunHandler(c *gin.Context) {
	var payload DryRunRequest
	var err error

	if err = c.ShouldBind(&payload); err != nil {
		app.reportBadRequest(c, err)
		return
	}

	app.log.V(1).Info("processing request", "route", c.Request.URL, "payload", payload)

	// Instantiate a ModRuleStore for this request and populate it with the modrules.
	store := core.NewModRuleStore(app.modRuleStoreItemFactory, app.log)

	for _, modRule := range payload.ModRules {
		// Populate the modrule with its default values if missing.
		modRule.Default()

		// Fix the namespace of each modrule - we want to try them as if they all were created in the same namespace.
		modRule.Namespace = dryRunNamespace

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

	originalJSON, err := json.Marshal(payload.ResourceManifest)

	if err != nil {
		app.reportBadRequest(c, err)
		return
	}

	// First run the patch operations.
	patched, patch, err := store.CalculatePatch(dryRunNamespace, originalJSON, app.log)

	if err != nil {
		app.reportBadRequest(c, err)
		return
	}

	// Try rejections against the after-patch manifest.
	rejections := store.DetermineRejections(dryRunNamespace, patched, app.log)

	// If there is a valid patch, calculate the diff in unified diff format.
	var diff string

	if len(patch) > 0 {
		originalYAML, err := yaml.JSONToYAML(originalJSON)

		if err != nil {
			app.reportBadRequest(c, err)
			return
		}

		patchedYAML, err := yaml.Marshal(patched)

		if err != nil {
			app.reportBadRequest(c, err)
			return
		}

		diffBytes, err := util.Diff(originalYAML, patchedYAML)

		if err != nil {
			app.reportBadRequest(c, err)
			return
		}

		diff = string(diffBytes)
	}

	response := DryRunResponse{
		Patch:      patch,
		Diff:       diff,
		Rejections: rejections,
	}

	c.JSON(http.StatusOK, response)
}

// Common method to return an HTTP 400 bad request
func (app *KubeModWebApp) reportBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	app.log.Error(err, "request failed", "route", c.Request.URL)
	return
}
