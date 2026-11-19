package handlers

import (
	"strconv"

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

	repos, err := h.service.GetCachedRepositories(r.Context(), providerType, installationID, r.TeamID)
	if err != nil {
		return err
	}
	selected := make([]dto.RepositoryResponse, len(repos))
	for i := range repos {
		selected[i] = dto.ToRepositoryResponse(&repos[i])
	}
	return fiberctx.OK(r.Ctx, "Repositories retrieved", fiber.Map{
		"repositories":    selected,
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

// RefreshInstallationRepositories waits for inventory reconciliation.
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
	return fiberctx.OK(r.Ctx, "Repositories refreshed", fiber.Map{"status": "success"})
}

// HandleInstallationCallback verifies one session-bound OAuth or setup state.
// The frontend handles navigation from this structured, non-sensitive result.
func (h *SourceControlHandler) HandleInstallationCallback(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	if c.Params("provider") != "github" {
		return fiberctx.BadRequest("Provider not supported")
	}
	sessionID, _ := c.Locals("sessionID").(string)
	result, err := h.service.CompleteGitHubCallback(c.UserContext(), teamID, userID, sessionID, c.Query("state"), c.Query("code"), c.Query("installation_id"))
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "GitHub identity verified", result)
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

func (h *SourceControlHandler) BeginGitHub(r *fiberctx.Request) error {
	var req services.BeginGitHubRequest
	if err := r.BodyParser(&req); err != nil {
		return fiberctx.BadRequest("Invalid connection request")
	}
	session, _ := r.Locals("sessionID").(string)
	result, err := h.service.BeginGitHub(r.Context(), r.TeamID, r.UserID, session, req)
	if err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "GitHub verification started", result)
}
func (h *SourceControlHandler) GitHubAttempt(r *fiberctx.Request) error {
	session, _ := r.Locals("sessionID").(string)
	result, err := h.service.GitHubAttempt(r.Context(), r.Params("attemptId"), r.TeamID, r.UserID, session)
	if err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "GitHub connection", result)
}
func (h *SourceControlHandler) GitHubCandidates(r *fiberctx.Request) error {
	session, _ := r.Locals("sessionID").(string)
	var pages []int
	if r.Ctx.Context().QueryArgs().Has("page") {
		page, err := strconv.Atoi(r.Query("page"))
		if err != nil || page < 1 || page > 10000 {
			return fiberctx.BadRequest("Invalid repository page")
		}
		pages = append(pages, page)
	}
	result, err := h.service.GitHubCandidates(r.Context(), r.Params("attemptId"), r.TeamID, r.UserID, session, r.Params("installationId"), pages...)
	if err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Choose repositories for this account", result)
}
func (h *SourceControlHandler) SelectGitHub(r *fiberctx.Request) error {
	var req services.GitHubSelectionRequest
	if err := r.BodyParser(&req); err != nil {
		return fiberctx.BadRequest("Invalid repository selection")
	}
	session, _ := r.Locals("sessionID").(string)
	result, err := h.service.SelectGitHubRepositories(r.Context(), r.Params("attemptId"), r.TeamID, r.UserID, session, req)
	if err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Repository selection saved", result)
}
