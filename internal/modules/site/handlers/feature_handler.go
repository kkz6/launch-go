package handlers

import (
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// EnableFeatureRequest represents optional body for enabling a feature
type EnableFeatureRequest struct {
	DeleteQueues    bool `json:"delete_queues"`
	ConfigureEnv    bool `json:"configure_env"`
	UpdateCaddyfile bool `json:"update_caddyfile"`
}

// FeatureHandler handles HTTP requests for Laravel feature management
type FeatureHandler struct {
	featureService *services.FeatureService
}

// NewFeatureHandler creates a new feature handler
func NewFeatureHandler(featureService *services.FeatureService) *FeatureHandler {
	return &FeatureHandler{featureService: featureService}
}

// EnableFeature enables a Laravel feature for a site
func (h *FeatureHandler) EnableFeature(r *fiberctx.Request) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(r.Ctx)
	if err != nil {
		return err
	}

	featureName := r.Params("feature")

	// Parse optional request body (allow empty body for simple toggles)
	var req EnableFeatureRequest
	_ = r.BodyParser(&req)

	opts := services.EnableFeatureOptions{
		DeleteQueues:    req.DeleteQueues,
		ConfigureEnv:    req.ConfigureEnv,
		UpdateCaddyfile: req.UpdateCaddyfile,
	}

	if err := h.featureService.EnableFeature(r.Context(), siteID, serverID, r.TeamID, &r.UserID, featureName, opts); err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	return fiberctx.OK(r.Ctx, "Feature is being enabled", nil)
}

// DisableFeature disables a Laravel feature for a site
func (h *FeatureHandler) DisableFeature(r *fiberctx.Request) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(r.Ctx)
	if err != nil {
		return err
	}

	featureName := r.Params("feature")

	if err := h.featureService.DisableFeature(r.Context(), siteID, serverID, r.TeamID, &r.UserID, featureName); err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	return fiberctx.OK(r.Ctx, "Feature is being disabled", nil)
}
