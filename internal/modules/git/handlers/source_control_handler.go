package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// SourceControlHandler holds the source-control endpoints that do not
// fit the team-scoped route helpers. Standard CRUD on /source-controls
// is wired directly to the framework helpers in routes.go; the bespoke
// handlers below cover provider-discriminated reads (`:provider`-scoped
// installation lookups, OAuth callbacks, dropdown payloads).
type SourceControlHandler struct {
	service *services.SourceControlService
}

// NewSourceControlHandler creates a new source control handler.
func NewSourceControlHandler(service *services.SourceControlService) *SourceControlHandler {
	return &SourceControlHandler{service: service}
}

// GetInstallationURL returns the URL the user should visit to install
// the provider's app for their account.
func (h *SourceControlHandler) GetInstallationURL(c *fiber.Ctx) error {
	providerStr := c.Params("provider")
	providerType, err := fiberctx.MustParseEnum(c, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}

	url, err := h.service.GetInstallationURL(providerType)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Installation URL retrieved", dto.InstallationURLResponse{URL: url, Provider: providerStr})
}

// GetInstallations returns all installations for a provider scoped to
// the team.
func (h *SourceControlHandler) GetInstallations(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	providerStr := c.Params("provider")
	providerType, err := fiberctx.MustParseEnum(c, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}

	scs, err := h.service.ListSourceControlsByProvider(c.Context(), providerType, teamID)
	if err != nil {
		return err
	}

	installations := make([]dto.AppInstallationData, len(scs))
	for i, sc := range scs {
		installations[i] = services.AppInstallationDataFromSourceControl(&sc)
	}
	return fiberctx.OK(c, "Installations retrieved", dto.InstallationsResponse{Installations: installations, Provider: providerStr})
}

// GetInstallation returns a single installation scoped to the team.
func (h *SourceControlHandler) GetInstallation(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	providerStr := c.Params("provider")
	providerType, err := fiberctx.MustParseEnum(c, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}

	sc, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, c.Params("installationId"), contracts.WithTeamID(teamID))
	if err != nil {
		return fiberctx.NotFound("Installation not found")
	}

	return fiberctx.OK(c, "Installation retrieved", fiber.Map{
		"installation": services.AppInstallationDataFromSourceControl(sc),
		"provider":     providerStr,
	})
}

// GetInstallationRepositories fetches repositories for an installation
// from the provider API after verifying team ownership.
func (h *SourceControlHandler) GetInstallationRepositories(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	providerStr := c.Params("provider")
	providerType, err := fiberctx.MustParseEnum(c, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}
	installationID := c.Params("installationId")

	if _, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, installationID, contracts.WithTeamID(teamID)); err != nil {
		return fiberctx.NotFound("Installation not found")
	}

	repos, err := h.service.GetInstallationRepositoriesFromAPI(c.Context(), providerType, installationID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Repositories retrieved", fiber.Map{
		"repositories":    repos,
		"provider":        providerStr,
		"installation_id": installationID,
	})
}

// GetCachedInstallationRepositories fetches the locally cached list.
func (h *SourceControlHandler) GetCachedInstallationRepositories(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	providerType, err := fiberctx.MustParseEnum(c, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}
	installationID := c.Params("installationId")

	repos, err := h.service.GetCachedRepositories(c.Context(), providerType, installationID, teamID)
	if err != nil {
		return err
	}
	out := make([]dto.RepositoryResponse, len(repos))
	for i := range repos {
		out[i] = dto.ToRepositoryResponse(&repos[i])
	}
	return fiberctx.OK(c, "Cached repositories retrieved", fiber.Map{
		"repositories":     out,
		"repository_count": len(out),
	})
}

// RefreshInstallationRepositories triggers a background sync.
func (h *SourceControlHandler) RefreshInstallationRepositories(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	providerType, err := fiberctx.MustParseEnum(c, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}
	installationID := c.Params("installationId")

	sc, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, installationID, contracts.WithTeamID(teamID))
	if err != nil {
		return fiberctx.NotFound("Installation not found")
	}
	if err := h.service.RefreshInstallationRepositories(c.Context(), sc); err != nil {
		return err
	}
	return fiberctx.OK(c, "Repositories are being synced in the background", fiber.Map{"status": "success"})
}

// HandleInstallationCallback handles the OAuth callback after a user
// installs the provider's app. On any error it redirects the user back
// to the settings page rather than rendering a JSON error — that's why
// it can't use the standard error-propagation convention.
func (h *SourceControlHandler) HandleInstallationCallback(c *fiber.Ctx) error {
	const redirectTo = "/settings/git-providers"
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return c.Redirect(redirectTo)
	}
	installationID := c.Query("installation_id")
	setupAction := c.Query("setup_action")
	if installationID == "" {
		return c.Redirect(redirectTo)
	}
	providerType, err := gittypes.ParseGitProviderType(c.Params("provider"))
	if err != nil {
		return c.Redirect(redirectTo)
	}

	existingSC, err := h.service.GetSourceControlByInstallation(c.Context(), providerType, installationID, contracts.WithTeamID(teamID))
	if setupAction == "install" {
		if err == nil && existingSC != nil {
			_ = h.service.RefreshInstallationRepositories(c.Context(), existingSC)
		} else {
			_ = h.service.SyncUserInstallation(c.Context(), providerType, installationID, teamID, userID)
		}
	}
	return c.Redirect(redirectTo)
}

// TestConnection probes the provider for connectivity.
func (h *SourceControlHandler) TestConnection(c *fiber.Ctx) error {
	providerStr := c.Params("provider")
	providerType, err := fiberctx.MustParseEnum(c, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}

	if err := h.service.TestConnection(c.Context(), providerType); err != nil {
		return fiberctx.OK(c, "Connection test failed", fiber.Map{
			"success":  false,
			"error":    err.Error(),
			"provider": providerStr,
		})
	}
	return fiberctx.OK(c, "Connection test successful", fiber.Map{"success": true, "provider": providerStr})
}

// GetInstallationsWithCounts powers the settings page: installations
// for the team plus the catalogue of available provider types.
func (h *SourceControlHandler) GetInstallationsWithCounts(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	installations, err := h.service.GetInstallationsWithRepositoryCounts(c.Context(), teamID)
	if err != nil {
		return err
	}

	providers := make([]fiber.Map, 0, len(gittypes.AllGitProviders()))
	for _, p := range gittypes.AllGitProviders() {
		providers = append(providers, fiber.Map{"value": p.String(), "label": p.Label()})
	}

	return fiberctx.OK(c, "Installations retrieved", fiber.Map{
		"appInstallations":   installations,
		"availableProviders": providers,
	})
}
