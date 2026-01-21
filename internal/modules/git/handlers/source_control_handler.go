package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// SourceControlHandler handles HTTP requests for git operations
type SourceControlHandler struct {
	service *services.SourceControlService
}

// NewSourceControlHandler creates a new source control handler
func NewSourceControlHandler(service *services.SourceControlService) *SourceControlHandler {
	return &SourceControlHandler{service: service}
}

// ListSourceControls lists all source controls for the team
func (h *SourceControlHandler) ListSourceControls(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	sourceControls, err := h.service.ListSourceControls(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
	}

	result := make([]dto.SourceControlResponse, len(sourceControls))
	for i, sc := range sourceControls {
		result[i] = dto.ToSourceControlResponse(&sc)
	}

	return response.OK(c, "Source controls retrieved", result)
}

// GetSourceControl gets a single source control
func (h *SourceControlHandler) GetSourceControl(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	sc, err := h.service.GetSourceControl(c.Context(), id, teamID)
	if err != nil {
		return response.NotFound(c, response.MsgSourceControlNotFound)
	}

	return response.OK(c, "Source control retrieved", dto.ToSourceControlResponse(sc))
}

// GetSourceControlRepositories gets all repositories for a source control
func (h *SourceControlHandler) GetSourceControlRepositories(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	repos, err := h.service.GetRepositoriesBySourceControlID(c.Context(), id, teamID)
	if err != nil {
		if err == repositories.ErrSourceControlNotFound {
			return response.NotFound(c, response.MsgSourceControlNotFound)
		}
		return response.InternalError(c, response.MsgInternalError)
	}

	result := make([]dto.RepositoryResponse, len(repos))
	for i, repo := range repos {
		result[i] = dto.ToRepositoryResponse(&repo)
	}

	return response.OK(c, "Repositories retrieved", result)
}

// Connect connects a git provider
func (h *SourceControlHandler) Connect(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.ConnectProviderRequest](c)
	if err != nil {
		return err
	}

	providerType, err := enums.ParseGitProviderType(req.Provider)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
	}

	sc, err := h.service.Connect(c.Context(), userID, teamID, providerType, req.InstallationID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Provider connected successfully", dto.ToSourceControlResponse(sc))
}

// Disconnect disconnects a source control
func (h *SourceControlHandler) Disconnect(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	if err := h.service.Disconnect(c.Context(), id, teamID); err != nil {
		if err == repositories.ErrSourceControlNotFound {
			return response.NotFound(c, response.MsgSourceControlNotFound)
		}
		if err == services.ErrHasSites {
			return response.Conflict(c, "Cannot disconnect provider with associated sites")
		}

		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// GetInstallationURL gets the installation URL for a provider
func (h *SourceControlHandler) GetInstallationURL(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
	}

	url, err := h.service.GetInstallationURL(providerType)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.OK(c, "Installation URL retrieved", dto.InstallationURLResponse{
		URL:      url,
		Provider: providerStr,
	})
}

// GetInstallations gets all installations for a provider
func (h *SourceControlHandler) GetInstallations(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
	}

	installations, err := h.service.GetInstallations(c.Context(), providerType)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.OK(c, "Installations retrieved", dto.InstallationsResponse{
		Installations: installations,
		Provider:      providerStr,
	})
}

// GetInstallation gets a single installation
func (h *SourceControlHandler) GetInstallation(c *fiber.Ctx) error {
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
	}

	installations, err := h.service.GetInstallations(c.Context(), providerType)
	if err != nil {
		return response.InternalError(c, err.Error())
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
func (h *SourceControlHandler) GetInstallationRepositories(c *fiber.Ctx) error {
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
	}

	repos, err := h.service.GetInstallationRepositoriesFromAPI(c.Context(), providerType, installationID)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.OK(c, "Repositories retrieved", fiber.Map{
		"repositories":    repos,
		"provider":        providerStr,
		"installation_id": installationID,
	})
}

// GetCachedInstallationRepositories gets cached repositories for an installation
func (h *SourceControlHandler) GetCachedInstallationRepositories(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
	}

	repos, err := h.service.GetCachedRepositories(c.Context(), providerType, installationID, teamID)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	result := make([]dto.RepositoryResponse, len(repos))
	for i, repo := range repos {
		result[i] = dto.ToRepositoryResponse(&repo)
	}

	return response.OK(c, "Cached repositories retrieved", fiber.Map{
		"repositories":     result,
		"repository_count": len(result),
	})
}

// RefreshInstallationRepositories refreshes repositories for an installation
func (h *SourceControlHandler) RefreshInstallationRepositories(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	providerStr := c.Params("provider")
	installationID := c.Params("installationId")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
	}

	sc, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, installationID, contracts.WithUserID(userID))
	if err != nil {
		return response.NotFound(c, "Installation not found")
	}

	if err := h.service.RefreshInstallationRepositories(c.Context(), sc); err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.OK(c, "Repositories are being synced in the background", fiber.Map{
		"status": "success",
	})
}

// HandleInstallationCallback handles the callback after installing a GitHub App
func (h *SourceControlHandler) HandleInstallationCallback(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return c.Redirect("/settings/git-providers")
	}
	providerStr := c.Params("provider")

	installationID := c.Query("installation_id")
	setupAction := c.Query("setup_action")

	if installationID == "" {
		return c.Redirect("/settings/git-providers")
	}

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return c.Redirect("/settings/git-providers")
	}

	// Check if installation already exists
	existingSC, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, installationID, contracts.WithUserID(userID))

	if setupAction == "install" {
		if err == nil && existingSC != nil {
			// Update existing installation - ignore refresh errors and continue
			_ = h.service.RefreshInstallationRepositories(c.Context(), existingSC)
		} else {
			// Sync new installation - ignore sync errors and continue
			_ = h.service.SyncUserInstallation(c.Context(), providerType, installationID, teamID, userID)
		}
	}

	return c.Redirect("/settings/git-providers")
}

// TestConnection tests the connection to a provider
func (h *SourceControlHandler) TestConnection(c *fiber.Ctx) error {
	providerStr := c.Params("provider")

	providerType, err := enums.ParseGitProviderType(providerStr)
	if err != nil {
		return response.BadRequest(c, "Invalid provider")
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
func (h *SourceControlHandler) GetInstallationsWithCounts(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	installations, err := h.service.GetInstallationsWithRepositoryCounts(c.Context(), teamID, userID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
	}

	// Get available providers (matches Laravel's format)
	providers := make([]fiber.Map, 0)
	for _, p := range enums.AllGitProviders() {
		providers = append(providers, fiber.Map{
			"value": p.String(),
			"label": p.Label(),
		})
	}

	// Return with camelCase keys to match Laravel's Inertia response format
	return response.OK(c, "Installations retrieved", fiber.Map{
		"appInstallations":   installations,
		"availableProviders": providers,
	})
}
