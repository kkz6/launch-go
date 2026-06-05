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
// No MustGet — skipped; stays on plain fiber.Ctx.
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
func (h *SourceControlHandler) GetInstallations(r *fiberctx.Request) error {
	providerStr := r.Params("provider")
	providerType, err := fiberctx.MustParseEnum(r.Ctx, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}

	scs, err := h.service.ListSourceControlsByProvider(r.Context(), providerType, r.TeamID)
	if err != nil {
		return err
	}

	installations := make([]dto.AppInstallationData, len(scs))
	for i, sc := range scs {
		installations[i] = services.AppInstallationDataFromSourceControl(&sc)
	}
	return fiberctx.OK(r.Ctx, "Installations retrieved", dto.InstallationsResponse{Installations: installations, Provider: providerStr})
}

// GetInstallation returns a single installation scoped to the team.
func (h *SourceControlHandler) GetInstallation(r *fiberctx.Request) error {
	providerStr := r.Params("provider")
	providerType, err := fiberctx.MustParseEnum(r.Ctx, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}

	sc, err := h.service.GetSourceControlByInstallation(r.Context(), providerType, r.Params("installationId"), contracts.WithTeamID(r.TeamID))
	if err != nil {
		return fiberctx.NotFound("Installation not found")
	}

	return fiberctx.OK(r.Ctx, "Installation retrieved", fiber.Map{
		"installation": services.AppInstallationDataFromSourceControl(sc),
		"provider":     providerStr,
	})
}

// GetInstallationRepositories fetches repositories for an installation
// from the provider API after verifying team ownership.
func (h *SourceControlHandler) GetInstallationRepositories(r *fiberctx.Request) error {
	providerStr := r.Params("provider")
	providerType, err := fiberctx.MustParseEnum(r.Ctx, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}
	installationID := r.Params("installationId")

	if _, err := h.service.GetSourceControlByInstallation(r.Context(), providerType, installationID, contracts.WithTeamID(r.TeamID)); err != nil {
		return fiberctx.NotFound("Installation not found")
	}

	repos, err := h.service.GetInstallationRepositoriesFromAPI(r.Context(), providerType, installationID)
	if err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Repositories retrieved", fiber.Map{
		"repositories":    repos,
		"provider":        providerStr,
		"installation_id": installationID,
	})
}

// GetCachedInstallationRepositories fetches the locally cached list.
func (h *SourceControlHandler) GetCachedInstallationRepositories(r *fiberctx.Request) error {
	providerType, err := fiberctx.MustParseEnum(r.Ctx, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}
	installationID := r.Params("installationId")

	repos, err := h.service.GetCachedRepositories(r.Context(), providerType, installationID, r.TeamID)
	if err != nil {
		return err
	}
	out := make([]dto.RepositoryResponse, len(repos))
	for i := range repos {
		out[i] = dto.ToRepositoryResponse(&repos[i])
	}
	return fiberctx.OK(r.Ctx, "Cached repositories retrieved", fiber.Map{
		"repositories":     out,
		"repository_count": len(out),
	})
}

// RefreshInstallationRepositories triggers a background sync.
func (h *SourceControlHandler) RefreshInstallationRepositories(r *fiberctx.Request) error {
	providerType, err := fiberctx.MustParseEnum(r.Ctx, "provider", "Invalid provider", gittypes.ParseGitProviderType)
	if err != nil {
		return err
	}
	installationID := r.Params("installationId")

	sc, err := h.service.GetSourceControlByInstallation(r.Context(), providerType, installationID, contracts.WithTeamID(r.TeamID))
	if err != nil {
		return fiberctx.NotFound("Installation not found")
	}
	if err := h.service.RefreshInstallationRepositories(r.Context(), sc); err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Repositories are being synced in the background", fiber.Map{"status": "success"})
}

// HandleInstallationCallback handles the OAuth callback after a user
// installs the provider's app. Skipped: OAuth callback with redirect
// semantics — errors redirect rather than return JSON.
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
// No MustGet — skipped; stays on plain fiber.Ctx.
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
func (h *SourceControlHandler) GetInstallationsWithCounts(r *fiberctx.Request) error {
	installations, err := h.service.GetInstallationsWithRepositoryCounts(r.Context(), r.TeamID)
	if err != nil {
		return err
	}

	providers := make([]fiber.Map, 0, len(gittypes.AllGitProviders()))
	for _, p := range gittypes.AllGitProviders() {
		providers = append(providers, fiber.Map{"value": p.String(), "label": p.Label()})
	}

	return fiberctx.OK(r.Ctx, "Installations retrieved", fiber.Map{
		"appInstallations":   installations,
		"availableProviders": providers,
	})
}
