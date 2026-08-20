package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// SiteCounter interface for counting sites by server.
type SiteCounter interface {
	CountByServer(ctx context.Context, serverID string) (int64, error)
}

// UpstreamCounter interface for counting load balancer upstreams by server.
type UpstreamCounter interface {
	CountByServerID(ctx context.Context, serverID string) (int64, error)
}

// Handler holds the server endpoints that do not fit a generic route
// helper. Standard CRUD and the simple actions are wired directly to
// the framework helpers in routes.go; the bespoke handlers below cover
// composite reads (provision-status, site-count) and actions with
// non-standard inputs (vulnerability-audit, optional body).
type Handler struct {
	service         *services.Service
	taskRunner      *tasks.TaskRunnerDeps
	siteCounter     SiteCounter
	upstreamCounter UpstreamCounter
}

// NewHandler creates a new server handler.
func NewHandler(service *services.Service, taskRunner *tasks.TaskRunnerDeps, siteCounter SiteCounter, upstreamCounter UpstreamCounter) *Handler {
	return &Handler{
		service:         service,
		taskRunner:      taskRunner,
		siteCounter:     siteCounter,
		upstreamCounter: upstreamCounter,
	}
}

// GetProvisionStatus returns a composite payload (server + latest task)
// for the provisioning UI. Combines two service calls, so it is wired
// as a small bespoke handler rather than a service method.
func (h *Handler) GetProvisionStatus(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	server, err := h.service.GetServerWithRelations(c.Context(), serverID, teamID)
	if err != nil {
		return err
	}
	latestTask, _ := h.service.GetLatestTaskRaw(c.Context(), serverID, teamID)
	status := dto.BuildProvisionStatus(server, latestTask)
	localizeProvisionStatus(c, &status)
	return fiberctx.OK(c, "Provision status retrieved", status)
}

var provisionStepNames = map[string]struct{}{
	"connecting_server":          {},
	"detect_os":                  {},
	"configure_firewall":         {},
	"configure_swap":             {},
	"install_essential_packages": {},
	"setup_default_user":         {},
	"setup_root":                 {},
	"ssh_security":               {},
	"validate_ports":             {},
	"install_docker":             {},
	"setup_docker_network":       {},
	"setup_launch_dirs":          {},
	"install_traefik":            {},
	"setup_swarm_network":        {},
}

func localizeProvisionStep(c *fiber.Ctx, step *dto.ProvisionStatusStep) {
	if step == nil {
		return
	}
	if _, fixed := provisionStepNames[step.Name]; fixed {
		step.Description = i18n.T(c, step.Description)
		return
	}
	if step.Status == "completed" {
		step.Description = i18n.T(c, "Installed %s", step.Name)
		return
	}
	step.Description = i18n.T(c, "Installing %s", step.Name)
}

func localizeProvisionStatus(c *fiber.Ctx, status *dto.ProvisionStatusResponse) {
	if status == nil {
		return
	}
	for index := range status.Steps {
		localizeProvisionStep(c, &status.Steps[index])
	}
	localizeProvisionStep(c, status.CurrentStep)
	if status.ErrorMessage == "" {
		return
	}
	translated := i18n.T(c, status.ErrorMessage)
	// Stored provider errors contain provider names and sometimes status
	// values, so old rows cannot be exact catalog keys. Never leak English into
	// a Japanese response; use the stable, actionable fallback instead.
	if i18n.Locale(c) == i18n.LocaleJapanese && translated == status.ErrorMessage {
		translated = i18n.T(c, "We couldn't finish provisioning this server. Please try again, or contact support if it keeps happening.")
	}
	status.ErrorMessage = translated
}

// RunVulnerabilityAudit queues a security audit. POST with a body
// carrying the email recipient — does not fit ActionFunc.
func (h *Handler) RunVulnerabilityAudit(r *fiberctx.Request, req *dto.VulnerabilityAuditRequest) error {
	id, err := fiberctx.GetID(r.Ctx)
	if err != nil {
		return err
	}
	if err := h.service.RunVulnerabilityAudit(r.Context(), id, r.TeamID, r.UserID, req.Email); err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Vulnerability audit has been queued and will be sent to your email when completed.", nil)
}

// GetSiteCount returns the number of sites for a server. Uses an
// external SiteCounter dep, so the response is composed here.
func (h *Handler) GetSiteCount(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	if _, err := h.service.GetServerRaw(c.Context(), serverID, teamID); err != nil {
		return err
	}
	if h.siteCounter == nil {
		return fiberctx.OK(c, "Site count retrieved", fiber.Map{"count": 0})
	}
	count, err := h.siteCounter.CountByServer(c.Context(), serverID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Site count retrieved", fiber.Map{"count": count})
}

// GetProvisionScriptContent returns the script content (as a wrapped
// payload) for the local-dev mode. Bespoke because the response is a
// shaped fiber.Map, not a typed DTO.
func (h *Handler) GetProvisionScriptContent(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	script, err := h.service.GetProvisionScript(c.Context(), serverID, teamID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Provision script retrieved", fiber.Map{"script": script})
}
