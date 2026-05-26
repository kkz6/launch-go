package services

import (
	"context"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ScheduleService manages cron-style commands that run inside the
// container. Phase-2 ships CRUD only — wiring the schedules into a
// host-side cron entry lives in a follow-up job (the model + UI land
// first so the design is testable).
type ScheduleService struct {
	*BaseService
}

func NewScheduleService(deps *ServiceDeps) *ScheduleService {
	return &ScheduleService{BaseService: NewBaseService(deps)}
}

func (s *ScheduleService) ListSchedules(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]dto.ScheduleResponse, error) {
	if _, err := s.scopedAppForSchedule(ctx, applicationID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Schedule().ListForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ScheduleResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToScheduleResponse(&rows[i]))
	}
	return out, nil
}

func (s *ScheduleService) CreateSchedule(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.CreateScheduleRequest,
) (dto.ScheduleResponse, error) {
	_ = userID
	app, err := s.scopedAppForSchedule(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.ScheduleResponse{}, err
	}

	cron := strings.TrimSpace(req.Cron)
	if !looksLikeCron(cron) {
		return dto.ScheduleResponse{}, fiberutil.BadRequest(
			"Cron must be a 5-field expression (minute hour day month weekday)",
		)
	}
	cmd := strings.TrimSpace(req.Command)
	if cmd == "" {
		return dto.ScheduleResponse{}, fiberutil.BadRequest("Command is required")
	}

	// Default shell is sh (universal across alpine + minimal images);
	// caller can override to bash for debian/ubuntu apps that need
	// brace-expansion / array syntax. Default enabled=true so a new
	// row picks up on the next poller tick without an extra toggle.
	shellType := "sh"
	if req.ShellType != nil {
		shellType = *req.ShellType
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	sc := &models.ApplicationSchedule{
		ApplicationID: applicationID,
		Cron:          cron,
		Command:       cmd,
		Enabled:       enabled,
		ShellType:     shellType,
	}
	if err := s.Repos().Schedule().Create(ctx, sc); err != nil {
		return dto.ScheduleResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.application.schedule.added", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"schedule_id":    sc.ID,
	})
	return *dto.ToScheduleResponse(sc), nil
}

func (s *ScheduleService) UpdateSchedule(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
	req *dto.UpdateScheduleRequest,
) (dto.ScheduleResponse, error) {
	_ = userID
	app, err := s.scopedAppForSchedule(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.ScheduleResponse{}, err
	}
	sc, err := s.Repos().Schedule().FindByID(ctx, id)
	if err != nil {
		return dto.ScheduleResponse{}, err
	}
	if sc.ApplicationID != applicationID {
		return dto.ScheduleResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.Cron != nil {
		c := strings.TrimSpace(*req.Cron)
		if !looksLikeCron(c) {
			return dto.ScheduleResponse{}, fiberutil.BadRequest(
				"Cron must be a 5-field expression (minute hour day month weekday)",
			)
		}
		updates["cron"] = c
	}
	if req.Command != nil {
		c := strings.TrimSpace(*req.Command)
		if c == "" {
			return dto.ScheduleResponse{}, fiberutil.BadRequest("Command cannot be empty")
		}
		updates["command"] = c
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.ShellType != nil {
		updates["shell_type"] = *req.ShellType
	}
	if len(updates) > 0 {
		if err := s.Repos().Schedule().UpdateFields(ctx, id, updates); err != nil {
			return dto.ScheduleResponse{}, err
		}
	}
	reloaded, err := s.Repos().Schedule().FindByID(ctx, id)
	if err != nil {
		return dto.ScheduleResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.application.schedule.updated", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"schedule_id":    id,
	})
	return *dto.ToScheduleResponse(reloaded), nil
}

func (s *ScheduleService) DeleteSchedule(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	app, err := s.scopedAppForSchedule(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	sc, err := s.Repos().Schedule().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if sc.ApplicationID != applicationID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Schedule().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.application.schedule.deleted", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"schedule_id":    id,
	})
	return nil
}

func (s *ScheduleService) scopedAppForSchedule(
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

// looksLikeCron is a *very* light syntactic check — exactly 5 whitespace-
// separated fields. We don't validate the field-internal grammar (*/5,
// 0-23, etc.) because cron implementations vary on extensions — the
// host-side cron daemon will reject anything truly broken when we
// install the entry.
func looksLikeCron(s string) bool {
	parts := strings.Fields(s)
	return len(parts) == 5
}
