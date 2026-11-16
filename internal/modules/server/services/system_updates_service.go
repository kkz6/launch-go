package services

import (
	"context"
	"time"

	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

func (s *Service) GetSystemUpdates(ctx context.Context, serverID, teamID string) (tasks.SystemUpdateStatus, error) {
	return s.systemUpdates(ctx, serverID, teamID, "", nil)
}

func (s *Service) UpdateSystemUpdates(ctx context.Context, serverID, teamID, userID string, settings *tasks.SystemUpdateSettings) (tasks.SystemUpdateStatus, error) {
	parsed, err := time.Parse("15:04", settings.Schedule)
	if err != nil || parsed.Format("15:04") != settings.Schedule {
		return tasks.SystemUpdateStatus{}, fiberutil.BadRequest("Enter a maintenance time in HH:MM (UTC)")
	}
	return s.systemUpdates(ctx, serverID, teamID, userID, settings)
}

type SystemUpdateLogs struct {
	Logs string `json:"logs"`
}

func (s *Service) GetSystemUpdateLogs(ctx context.Context, serverID, teamID string) (SystemUpdateLogs, error) {
	result, err := s.systemUpdateAction(ctx, serverID, teamID, "", "logs")
	return SystemUpdateLogs{Logs: result.Logs}, err
}

func (s *Service) RetrySystemUpdates(ctx context.Context, serverID, teamID, userID string) (tasks.SystemUpdateStatus, error) {
	result, err := s.systemUpdateAction(ctx, serverID, teamID, userID, "retry")
	return result.SystemUpdateStatus, err
}

func (s *Service) RepairSystemUpdates(ctx context.Context, serverID, teamID, userID string) (tasks.SystemUpdateStatus, error) {
	result, err := s.systemUpdateAction(ctx, serverID, teamID, userID, "repair")
	return result.SystemUpdateStatus, err
}

func systemUpdateResponseError(result tasks.SystemUpdateResult) error {
	switch {
	case result.Error == "busy":
		return fiberutil.Conflict("Security updates are running. Try again after the run finishes.")
	case result.Error == "unattended_upgrades_missing":
		return fiberutil.BadRequest("Install unattended-upgrades on this Ubuntu server, refresh this page, then try again.")
	case result.Error == "schedule_disabled":
		return fiberutil.Conflict("Enable automatic security updates and save a maintenance time before repairing the schedule.")
	case !result.Supported:
		return fiberutil.BadRequest("System updates require Ubuntu 22.04 or 24.04 with systemd")
	case result.Error != "":
		return fiberutil.BadRequest("Could not complete this action. View update logs, resolve the reported issue, and try again.")
	default:
		return nil
	}
}

func (s *Service) systemUpdateAction(ctx context.Context, serverID, teamID, userID, action string) (tasks.SystemUpdateResult, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return tasks.SystemUpdateResult{}, err
	}
	if server.IsArchived() {
		return tasks.SystemUpdateResult{}, fiberutil.Conflict("System updates are unavailable on archived servers")
	}
	if action != "logs" {
		if err := s.ensureNoServerRestart(ctx, serverID); err != nil {
			return tasks.SystemUpdateResult{}, err
		}
		if err := s.DB().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.SystemUpdateMonitor{ServerID: serverID}).Error; err != nil {
			return tasks.SystemUpdateResult{}, err
		}
	}
	result, err := tasks.SystemUpdateAction(ctx, server, s.taskRunnerDeps.Dispatcher, s.Logger, action)
	if err != nil {
		s.Logger.Warn().Err(err).Str("server_id", serverID).Str("action", action).Msg("System update action failed")
		return tasks.SystemUpdateResult{}, fiberutil.BadRequest("Could not complete this action. Check that the server is reachable, view update logs, and try again.")
	}
	if err := systemUpdateResponseError(result); err != nil {
		return tasks.SystemUpdateResult{}, err
	}
	if action != "logs" {
		activity.RecordEvent(ctx, "system_updates_"+action, userID, server, "System update action was requested")
		s.BroadcastToTeam(teamID, "server.system_updates.updated", map[string]any{"server_id": serverID})
	}
	return result, nil
}

func (s *Service) systemUpdates(ctx context.Context, serverID, teamID, userID string, settings *tasks.SystemUpdateSettings) (tasks.SystemUpdateStatus, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return tasks.SystemUpdateStatus{}, err
	}
	if server.IsArchived() {
		return tasks.SystemUpdateStatus{}, fiberutil.Conflict("System updates are unavailable on archived servers")
	}
	// Ensure monitoring is registered before changing the host. A database
	// outage must not leave an enabled timer without report/reboot monitoring.
	if settings != nil {
		if err := s.ensureNoServerRestart(ctx, serverID); err != nil {
			return tasks.SystemUpdateStatus{}, err
		}
		if err := s.DB().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.SystemUpdateMonitor{ServerID: serverID}).Error; err != nil {
			return tasks.SystemUpdateStatus{}, err
		}
	}

	status, err := tasks.SystemUpdates(ctx, server, s.taskRunnerDeps.Dispatcher, s.Logger, settings)
	if err != nil {
		s.Logger.Warn().Err(err).Str("server_id", serverID).Msg("Failed to read or configure system updates")
		return tasks.SystemUpdateStatus{}, fiberutil.BadRequest("Could not read or configure system updates on this server")
	}
	if settings != nil {
		if err := systemUpdateResponseError(status); err != nil {
			return tasks.SystemUpdateStatus{}, err
		}
		activity.RecordEvent(ctx, "system_updates_configured", userID, server, "System update settings were saved")
		s.BroadcastToTeam(teamID, "server.system_updates.updated", map[string]any{"server_id": serverID})
	}
	return status.SystemUpdateStatus, nil
}
