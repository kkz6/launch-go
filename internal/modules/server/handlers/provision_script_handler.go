package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// ProvisionScriptRepository interface for provision script handler
type ProvisionScriptRepository interface {
	FindServerByID(ctx context.Context, id string) (*models.Server, error)
}

// ProvisionScriptService interface for generating provision scripts
type ProvisionScriptService interface {
	GetProvisionScript(ctx context.Context, serverID string) (string, error)
}

// ProvisionScriptHandler handles provision script requests
type ProvisionScriptHandler struct {
	repo    ProvisionScriptRepository
	service ProvisionScriptService
}

// NewProvisionScriptHandler creates a new provision script handler
func NewProvisionScriptHandler(repo ProvisionScriptRepository, service ProvisionScriptService) *ProvisionScriptHandler {
	return &ProvisionScriptHandler{
		repo:    repo,
		service: service,
	}
}

// RegisterRoutes registers provision script routes (with signed URL middleware)
func (h *ProvisionScriptHandler) RegisterRoutes(router fiber.Router) {
	// This route requires a valid signed URL
	router.Get("/servers/:id/provision-script",
		signedurl.RequireSignedURL(nil),
		h.GetProvisionScript,
	)
}

// GetProvisionScript returns the provision script for a server
func (h *ProvisionScriptHandler) GetProvisionScript(c *fiber.Ctx) error {
	serverID := c.Params("id")
	ctx := c.Context()

	// Find the server (including archived)
	server, err := h.repo.FindServerByID(ctx, serverID)
	if err != nil {
		return response.NotFound(c, "Server not found")
	}

	if server == nil {
		return response.NotFound(c, "Server not found")
	}

	// Get the provision script
	script, err := h.service.GetProvisionScript(ctx, serverID)
	if err != nil {
		return response.InternalError(c, "Failed to generate provision script")
	}

	// Return as plain text (bash script)
	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(script)
}
