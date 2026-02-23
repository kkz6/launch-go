package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// FeatureHandler handles HTTP requests for Laravel feature management
type FeatureHandler struct {
	featureService *services.FeatureService
}

// NewFeatureHandler creates a new feature handler
func NewFeatureHandler(featureService *services.FeatureService) *FeatureHandler {
	return &FeatureHandler{featureService: featureService}
}

// EnableFeature enables a Laravel feature for a site
func (h *FeatureHandler) EnableFeature(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	featureName := c.Params("feature")

	if err := h.featureService.EnableFeature(c.Context(), siteID, serverID, teamID, &userID, featureName); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Feature is being enabled", nil)
}

// DisableFeature disables a Laravel feature for a site
func (h *FeatureHandler) DisableFeature(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	featureName := c.Params("feature")

	if err := h.featureService.DisableFeature(c.Context(), siteID, serverID, teamID, &userID, featureName); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Feature is being disabled", nil)
}
