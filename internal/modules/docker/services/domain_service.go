package services

import (
	"context"
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DomainService manages application domains and the Traefik dynamic-
// config writes that back them.
//
// Lives alongside ApplicationService rather than as a separate module
// because every mutation needs to reach into the application's container
// name + internal port to render the Traefik file.
type DomainService struct {
	*BaseService
}

// NewDomainService wires the service.
func NewDomainService(deps *ServiceDeps) *DomainService {
	return &DomainService{BaseService: NewBaseService(deps)}
}

// ListDomains returns the live domains attached to an application,
// after validating the (server, project, app) chain.
func (s *DomainService) ListDomains(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]dto.DomainResponse, error) {
	if _, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Domain().ListForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DomainResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToDomainResponse(&rows[i]))
	}
	return out, nil
}

// CreateDomain attaches a new hostname to the application. After the row
// is persisted we enqueue a write of the Traefik config file so the new
// route takes effect — the asynq job below dispatches a one-shot SSH
// task that re-renders the YAML.
func (s *DomainService) CreateDomain(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.CreateDomainRequest,
) (dto.DomainResponse, error) {
	_ = userID
	app, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.DomainResponse{}, err
	}

	host := strings.TrimSpace(strings.ToLower(req.Host))
	host = strings.TrimSuffix(host, ".")
	if !validHostname.MatchString(host) {
		return dto.DomainResponse{}, fiberutil.BadRequest("Invalid hostname")
	}

	taken, err := s.Repos().Domain().ExistsByHost(ctx, applicationID, host)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	if taken {
		return dto.DomainResponse{}, fiberutil.Conflict("This host is already attached to the application")
	}

	https := true
	if req.HTTPS != nil {
		https = *req.HTTPS
	}

	d := &models.ApplicationDomain{
		ApplicationID: applicationID,
		Host:          host,
		Path:          trimEmpty(req.Path),
		HTTPS:         https,
	}
	if err := s.Repos().Domain().Create(ctx, d); err != nil {
		return dto.DomainResponse{}, err
	}

	if err := s.dispatchTraefikSync(ctx, app, teamID); err != nil {
		s.LogError(err, "failed to enqueue traefik sync", "application_id", app.ID)
	}

	resp := dto.ToDomainResponse(d)
	s.BroadcastToTeam(teamID, "docker.application.domain.added", resp)
	return *resp, nil
}

// UpdateDomain toggles HTTPS or updates the path prefix. Host changes
// aren't allowed — see UpdateDomainRequest for the reasoning.
func (s *DomainService) UpdateDomain(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
	req *dto.UpdateDomainRequest,
) (dto.DomainResponse, error) {
	_ = userID
	app, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	d, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	if d.ApplicationID != applicationID {
		return dto.DomainResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.HTTPS != nil {
		updates["https"] = *req.HTTPS
	}
	if req.Path != nil {
		updates["path"] = trimEmpty(req.Path)
	}
	if len(updates) > 0 {
		if err := s.Repos().Domain().UpdateFields(ctx, id, updates); err != nil {
			return dto.DomainResponse{}, err
		}
	}

	if err := s.dispatchTraefikSync(ctx, app, teamID); err != nil {
		s.LogError(err, "failed to enqueue traefik sync", "application_id", app.ID)
	}

	reloaded, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	resp := dto.ToDomainResponse(reloaded)
	s.BroadcastToTeam(teamID, "docker.application.domain.updated", resp)
	return *resp, nil
}

// DeleteDomain soft-deletes a domain and triggers a Traefik resync.
func (s *DomainService) DeleteDomain(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	app, err := s.scopedApp(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	d, err := s.Repos().Domain().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if d.ApplicationID != applicationID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Domain().Delete(ctx, id); err != nil {
		return err
	}
	if err := s.dispatchTraefikSync(ctx, app, teamID); err != nil {
		s.LogError(err, "failed to enqueue traefik sync", "application_id", app.ID)
	}
	s.BroadcastToTeam(teamID, "docker.application.domain.deleted", map[string]any{
		"id":             id,
		"application_id": applicationID,
		"team_id":        teamID,
		"server_id":      serverID,
	})
	return nil
}

// scopedApp resolves the (server, project, application) triple inside the
// caller's team. Returns the application so the caller can dispatch
// follow-up work without an extra DB hit.
func (s *DomainService) scopedApp(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) (*models.Application, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if app.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return app, nil
}

// dispatchTraefikSync enqueues an async job that re-renders the Traefik
// config for the application. Idempotent: re-running it with the same
// inputs produces the same file content, so we can safely fire it on
// every domain mutation without worrying about ordering.
func (s *DomainService) dispatchTraefikSync(
	ctx context.Context, app *models.Application, teamID string,
) error {
	task, err := jobs.NewSyncTraefikConfigTask(app.ID, app.ServerID, teamID)
	if err != nil {
		return err
	}
	return s.EnqueueTask(task)
}

// validHostname accepts a minimal RFC-1035-ish hostname: labels of
// alphanumerics and hyphens (no leading/trailing hyphen), separated by
// dots, total length up to 253. Rejects underscores, schemes, and
// trailing slashes — which is most of the malformed input we've seen
// from copy-paste mistakes.
var validHostname = regexp.MustCompile(
	`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
)

func trimEmpty(p *string) *string {
	if p == nil {
		return nil
	}
	t := strings.TrimSpace(*p)
	if t == "" {
		return nil
	}
	return &t
}
