package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/platform/dto"
	"github.com/kkz6/launch-go/internal/modules/platform/models"
	"github.com/kkz6/launch-go/internal/modules/platform/repositories"
	"github.com/kkz6/launch-go/internal/modules/platform/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

const (
	renameUsernameUpdateKey         = "rename_username_captain"
	renameUsernameUpdateTitle       = "Rename Default Username to Captain"
	renameUsernameUpdateDescription = "The default UNIX username on provisioned servers is being renamed from `launcher` to `captain`. This update renames the system user, moves the home directory, and updates all related configuration files (Caddyfile, systemd services, crontab) on your servers.\n\nThis is a safe, idempotent operation — if a server already has the `captain` user, it will be skipped automatically."
)

// ServiceDeps holds all dependencies needed for platform services
type ServiceDeps struct {
	service.Dependencies
	Repos *repositories.Registry
}

// PlatformUpdateService provides business logic for platform updates
type PlatformUpdateService struct {
	service.Base
	repos *repositories.Registry
}

// localizePlatformUpdateResponse replaces metadata only for platform updates
// identified by a stable application-owned key. Database-authored content for
// unknown keys remains untouched.
func localizePlatformUpdateResponse(ctx context.Context, response dto.PlatformUpdateResponse) dto.PlatformUpdateResponse {
	if response.Key != renameUsernameUpdateKey {
		return response
	}

	localized := response
	localized.Title = i18n.TContext(ctx, renameUsernameUpdateTitle)
	localized.Description = i18n.TContext(ctx, renameUsernameUpdateDescription)
	return localized
}

// NewPlatformUpdateService creates a new PlatformUpdateService
func NewPlatformUpdateService(deps ServiceDeps) *PlatformUpdateService {
	return &PlatformUpdateService{
		Base:  service.NewBase(deps.Dependencies),
		repos: deps.Repos,
	}
}

// GetPendingUpdates returns undismissed updates with pending servers for the team
func (s *PlatformUpdateService) GetPendingUpdates(ctx context.Context, teamID, userID string) ([]dto.PlatformUpdateResponse, error) {
	updates, err := s.repos.ServerPlatformUpdate().FindPendingForTeam(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.PlatformUpdateResponse, len(updates))
	for i, u := range updates {
		resp := localizePlatformUpdateResponse(ctx, dto.ToPlatformUpdateResponse(&u))

		counts, err := s.repos.ServerPlatformUpdate().CountByStatus(ctx, u.ID, teamID)
		if err == nil {
			resp.StatusCounts = make(map[string]int64)
			for status, count := range counts {
				resp.StatusCounts[status.String()] = count
			}
		}

		result[i] = resp
	}

	return result, nil
}

// GetUpdateDetail returns full update details with all server statuses for the team
func (s *PlatformUpdateService) GetUpdateDetail(ctx context.Context, updateID, teamID string) (*dto.PlatformUpdateDetailResponse, error) {
	update, err := s.repos.PlatformUpdate().FindByID(ctx, updateID)
	if err != nil {
		return nil, err
	}

	rows, err := s.repos.ServerPlatformUpdate().GetServerStatuses(ctx, updateID, teamID)
	if err != nil {
		return nil, err
	}

	serverStatuses := make([]dto.ServerUpdateStatusResponse, len(rows))
	for i, row := range rows {
		serverStatuses[i] = dto.ToServerUpdateStatusResponse(row)
	}

	counts, _ := s.repos.ServerPlatformUpdate().CountByStatus(ctx, updateID, teamID)
	statusCounts := make(map[string]int64)
	for status, count := range counts {
		statusCounts[status.String()] = count
	}

	resp := &dto.PlatformUpdateDetailResponse{
		PlatformUpdateResponse: localizePlatformUpdateResponse(ctx, dto.ToPlatformUpdateResponse(update)),
		ServerStatuses:         serverStatuses,
	}
	resp.StatusCounts = statusCounts

	return resp, nil
}

// RunUpdate dispatches the update task on a specific server
func (s *PlatformUpdateService) RunUpdate(ctx context.Context, updateID, serverID, teamID string) error {
	serverUpdate, err := s.repos.ServerPlatformUpdate().FindByServerAndUpdateForTeam(ctx, serverID, updateID, teamID)
	if err != nil {
		return err
	}

	if serverUpdate.Status != types.UpdateStatusPending && serverUpdate.Status != types.UpdateStatusFailed {
		return fiberutil.BadRequest("Update is not in a runnable state")
	}

	return s.dispatchUpdateJob(updateID, serverID, serverUpdate.ID, teamID)
}

// RunUpdateAll dispatches the update task for all pending servers in the team
func (s *PlatformUpdateService) RunUpdateAll(ctx context.Context, updateID, teamID string) error {
	rows, err := s.repos.ServerPlatformUpdate().GetServerStatuses(ctx, updateID, teamID)
	if err != nil {
		return err
	}

	for _, row := range rows {
		if row.Status != types.UpdateStatusPending && row.Status != types.UpdateStatusFailed {
			continue
		}

		if err := s.dispatchUpdateJob(updateID, row.ServerID, row.ID, teamID); err != nil {
			s.LogError(err, "Failed to dispatch update job",
				"server_id", row.ServerID,
				"update_id", updateID,
			)
		}
	}

	return nil
}

// DismissBanner marks the update banner as dismissed for the user
func (s *PlatformUpdateService) DismissBanner(ctx context.Context, updateID, userID string) error {
	// Verify the update exists
	if _, err := s.repos.PlatformUpdate().FindByID(ctx, updateID); err != nil {
		return err
	}

	return s.repos.Dismissal().Dismiss(ctx, userID, updateID)
}

// SeedUpdate creates a platform update and generates server_platform_updates rows
func (s *PlatformUpdateService) SeedUpdate(ctx context.Context, update *models.PlatformUpdate) error {
	// Check if already exists
	existing, err := s.repos.PlatformUpdate().FindByKey(ctx, update.Key)
	if err == nil && existing != nil {
		return nil
	}

	if err := s.repos.PlatformUpdate().Create(ctx, update); err != nil {
		return fmt.Errorf("failed to create platform update: %w", err)
	}

	return nil
}

// dispatchUpdateJob enqueues a job to run the platform update on a server
func (s *PlatformUpdateService) dispatchUpdateJob(updateID, serverID, serverUpdateID, teamID string) error {
	payload, _ := json.Marshal(map[string]string{
		"server_id":                 serverID,
		"platform_update_id":        updateID,
		"server_platform_update_id": serverUpdateID,
		"team_id":                   teamID,
	})

	task := asynq.NewTask("platform:run_update", payload, asynq.MaxRetry(0))

	return s.EnqueueTask(task)
}
