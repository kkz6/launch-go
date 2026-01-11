package git

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for git operations
type Handler struct {
	service *Service
}

// NewHandler creates a new git handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListSourceControls lists all source controls for the team
func (h *Handler) ListSourceControls(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	sourceControls, err := h.service.ListSourceControls(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch source controls")
	}

	result := make([]SourceControlResponse, len(sourceControls))
	for i, sc := range sourceControls {
		result[i] = ToSourceControlResponse(&sc)
	}

	return response.OK(c, "Source controls retrieved", result)
}

// GetSourceControl gets a single source control
func (h *Handler) GetSourceControl(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	sc, err := h.service.GetSourceControl(c.Context(), id, teamID)
	if err != nil {
		return response.NotFound(c, "Source control not found")
	}

	return response.OK(c, "Source control retrieved", ToSourceControlResponse(sc))
}

// Connect connects a git provider
func (h *Handler) Connect(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)

	var req ConnectProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	providerType, err := ParseGitProviderType(req.Provider)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	sc, err := h.service.Connect(c.Context(), userID, teamID, providerType, req.InstallationID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Provider connected successfully", ToSourceControlResponse(sc))
}

// Disconnect disconnects a source control
func (h *Handler) Disconnect(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	if err := h.service.Disconnect(c.Context(), id, teamID); err != nil {
		if err == ErrSourceControlNotFound {
			return response.NotFound(c, "Source control not found")
		}
		if err == ErrHasSites {
			return response.Error(c, fiber.StatusConflict, "Cannot disconnect provider with associated sites")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// GetInstallationURL gets the installation URL for a provider
func (h *Handler) GetInstallationURL(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	url, err := h.service.GetInstallationURL(providerType)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.OK(c, "Installation URL retrieved", InstallationURLResponse{
		URL:      url,
		Provider: providerStr,
	})
}

// GetInstallations gets all installations for a provider
func (h *Handler) GetInstallations(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	installations, err := h.service.GetInstallations(c.Context(), providerType)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.OK(c, "Installations retrieved", InstallationsResponse{
		Installations: installations,
		Provider:      providerStr,
	})
}

// GetInstallation gets a single installation
func (h *Handler) GetInstallation(c *fiber.Ctx) error {
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	installations, err := h.service.GetInstallations(c.Context(), providerType)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	for _, inst := range installations {
		if inst.ID == installationID {
			return response.OK(c, "Installation retrieved", fiber.Map{
				"installation": inst,
				"provider":     providerStr,
			})
		}
	}

	return response.NotFound(c, "Installation not found")
}

// GetInstallationRepositories gets repositories for an installation from the API
func (h *Handler) GetInstallationRepositories(c *fiber.Ctx) error {
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	repos, err := h.service.GetInstallationRepositoriesFromAPI(c.Context(), providerType, installationID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.OK(c, "Repositories retrieved", fiber.Map{
		"repositories":    repos,
		"provider":        providerStr,
		"installation_id": installationID,
	})
}

// GetCachedInstallationRepositories gets cached repositories for an installation
func (h *Handler) GetCachedInstallationRepositories(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	repos, err := h.service.GetCachedRepositories(c.Context(), providerType, installationID, teamID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	result := make([]RepositoryResponse, len(repos))
	for i, repo := range repos {
		result[i] = ToRepositoryResponse(&repo)
	}

	return response.OK(c, "Cached repositories retrieved", fiber.Map{
		"repositories":     result,
		"repository_count": len(result),
	})
}

// RefreshInstallationRepositories refreshes repositories for an installation
func (h *Handler) RefreshInstallationRepositories(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	sc, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, installationID, WithUserID(userID))
	if err != nil {
		return response.NotFound(c, "Installation not found")
	}

	if err := h.service.RefreshInstallationRepositories(c.Context(), sc); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.OK(c, "Repositories are being synced in the background", fiber.Map{
		"status": "success",
	})
}

// HandleInstallationCallback handles the callback after installing a GitHub App
func (h *Handler) HandleInstallationCallback(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	providerStr := c.Params("provider")

	installationID := c.Query("installation_id")
	setupAction := c.Query("setup_action")

	if installationID == "" {
		return c.Redirect("/settings/git-providers")
	}

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return c.Redirect("/settings/git-providers")
	}

	// Check if installation already exists
	existingSC, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, installationID, WithUserID(userID))

	if setupAction == "install" {
		if err == nil && existingSC != nil {
			// Update existing installation
			if refreshErr := h.service.RefreshInstallationRepositories(c.Context(), existingSC); refreshErr != nil {
				// Log error but continue
			}
		} else {
			// Sync new installation
			if syncErr := h.service.SyncUserInstallation(c.Context(), providerType, installationID, teamID, userID); syncErr != nil {
				// Log error but continue
			}
		}
	}

	return c.Redirect("/settings/git-providers")
}

// TestConnection tests the connection to a provider
func (h *Handler) TestConnection(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := ParseGitProviderType(providerStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider")
	}

	if err := h.service.TestConnection(c.Context(), providerType); err != nil {
		return response.OK(c, "Connection test failed", fiber.Map{
			"success":  false,
			"error":    err.Error(),
			"provider": providerStr,
		})
	}

	return response.OK(c, "Connection test successful", fiber.Map{
		"success":  true,
		"provider": providerStr,
	})
}

// GetInstallationsWithCounts gets installations with repository counts for the settings page
func (h *Handler) GetInstallationsWithCounts(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)

	installations, err := h.service.GetInstallationsWithRepositoryCounts(c.Context(), teamID, userID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch installations")
	}

	// Get available providers
	providers := make([]fiber.Map, 0)
	for _, p := range AllGitProviders() {
		providers = append(providers, fiber.Map{
			"value": p.String(),
			"label": p.Label(),
		})
	}

	return response.OK(c, "Installations retrieved", fiber.Map{
		"app_installations":    installations,
		"available_providers": providers,
	})
}
