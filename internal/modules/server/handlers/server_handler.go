package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// SiteCounter interface for counting sites by server
type SiteCounter interface {
	CountByServer(ctx context.Context, serverID string) (int64, error)
}

// UpstreamCounter interface for counting load balancer upstreams by server
type UpstreamCounter interface {
	CountByServerID(ctx context.Context, serverID string) (int64, error)
}

// Handler handles all server-related HTTP requests
type Handler struct {
	service         *services.Service
	taskRunner      *tasks.TaskRunnerDeps
	siteCounter     SiteCounter
	upstreamCounter UpstreamCounter
}

// NewHandler creates a new server handler
func NewHandler(service *services.Service, taskRunner *tasks.TaskRunnerDeps, siteCounter SiteCounter, upstreamCounter UpstreamCounter) *Handler {
	return &Handler{
		service:         service,
		taskRunner:      taskRunner,
		siteCounter:     siteCounter,
		upstreamCounter: upstreamCounter,
	}
}

// List returns all servers for the team
func (h *Handler) List(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	servers, err := h.service.ListServers(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Servers retrieved", pkgdto.TransformSlice(servers, dto.ToServerResponse))
}

// ListArchived returns all archived servers for the team
func (h *Handler) ListArchived(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	servers, err := h.service.ListArchivedServers(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Archived servers retrieved", pkgdto.TransformSlice(servers, dto.ToServerResponse))
}

// Create creates a new server
func (h *Handler) Create(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateServerRequest](c)
	if err != nil {
		return err
	}

	server, err := h.service.CreateServer(c.Context(), teamID, userID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Server created", dto.ToServerResponse(server))
}

// Show returns a single server
func (h *Handler) Show(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	server, err := h.service.GetServerWithRelations(c.Context(), id, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch server")
	}

	return fiberctx.OK(c, "Server retrieved", dto.ToServerResponse(server))
}

// Update updates a server
func (h *Handler) Update(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateServerRequest](c)
	if err != nil {
		return err
	}

	server, err := h.service.UpdateServer(c.Context(), id, teamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Server updated", dto.ToServerResponse(server))
}

// Delete deletes a server
func (h *Handler) Delete(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	if err := h.service.DeleteServer(c.Context(), id, teamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}

// Reboot reboots a server
func (h *Handler) Reboot(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	if err := h.service.RebootServer(c.Context(), id, teamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Server reboot initiated", nil)
}

// GetProvisionStatus returns the provision status for a server
func (h *Handler) GetProvisionStatus(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	server, err := h.service.GetServerWithRelations(c.Context(), id, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch server")
	}

	latestTask, _ := h.service.GetLatestTask(c.Context(), id, teamID)

	return fiberctx.OK(c, "Provision status retrieved", dto.BuildProvisionStatus(server, latestTask))
}

// Connect tests the connection to a server
func (h *Handler) Connect(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	if err := h.service.ConnectServer(c.Context(), id, teamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Server connection successful", nil)
}

// Archive archives a server
func (h *Handler) Archive(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	if err := h.service.ArchiveServer(c.Context(), id, teamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Server archived", nil)
}

// Unarchive unarchives a server
func (h *Handler) Unarchive(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	if err := h.service.UnarchiveServer(c.Context(), id, teamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Server unarchived", nil)
}

// ShowPage returns aggregated data for the server show page
func (h *Handler) ShowPage(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	data, err := h.service.GetShowPageData(c.Context(), id, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch server data")
	}

	return fiberctx.OK(c, "Server page data retrieved", data)
}

// RunVulnerabilityAudit runs a security vulnerability audit on a server
func (h *Handler) RunVulnerabilityAudit(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.VulnerabilityAuditRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.RunVulnerabilityAudit(c.Context(), id, teamID, userID, req.Email); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Vulnerability audit has been queued and will be sent to your email when completed.", nil)
}

// GetSiteCount returns the number of sites for a server
func (h *Handler) GetSiteCount(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	// Verify server exists and belongs to team
	if _, err := h.service.GetServer(c.Context(), serverID, teamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	// Check if site counter is configured
	if h.siteCounter == nil {
		return fiberctx.OK(c, "Site count retrieved", fiber.Map{"count": 0})
	}

	count, err := h.siteCounter.CountByServer(c.Context(), serverID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Site count retrieved", fiber.Map{"count": count})
}

// RetryProvision retries the provisioning of a failed server
func (h *Handler) RetryProvision(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	id, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	if err := h.service.RetryProvision(c.Context(), id, teamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Server provisioning has been queued", nil)
}

// GetProvisionScriptContent returns the provision script content for a server
// This endpoint is for authenticated users to get the script content directly (for local dev mode)
func (h *Handler) GetProvisionScriptContent(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	script, err := h.service.GetProvisionScript(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Provision script retrieved", fiber.Map{
		"script": script,
	})
}
