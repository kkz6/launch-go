package services

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// GetServerRestart is a local read, so progress remains available while SSH is offline.
func (s *Service) GetServerRestart(ctx context.Context, id, teamID string) (models.ServerRestart, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID); err != nil {
		return models.ServerRestart{}, err
	}
	var state models.ServerRestart
	err := s.DB().WithContext(ctx).First(&state, "server_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ServerRestart{Status: "idle"}, nil
	}
	if err != nil {
		return state, err
	}
	if state.Active() && state.DeadlineAt != nil && time.Now().After(*state.DeadlineAt) {
		result := s.DB().WithContext(ctx).Model(&models.ServerRestart{}).
			Where("server_id = ? AND operation_id = ? AND status IN ?", id, state.OperationID, []string{"queued", "restarting", "verifying"}).
			Updates(map[string]any{"status": "failed", "completed_at": time.Now().UTC(), "check_after": nil,
				"message": "The restart could not be confirmed within five minutes. Check the server in your provider console, then check again."})
		if result.Error != nil {
			return state, result.Error
		}
		if result.RowsAffected != 0 {
			s.BroadcastToTeam(teamID, "server.reboot.updated", map[string]any{"server_id": id})
		}
		if err := s.DB().WithContext(ctx).First(&state, "server_id = ?", id).Error; err != nil {
			return state, err
		}
	}
	state.CanVerify = state.Status == "failed" && state.BootID != ""
	return state, nil
}

// RebootServer records a unique operation before dispatching any remote command.
func (s *Service) RebootServer(ctx context.Context, id, teamID, userID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}
	if server.IsArchived() {
		return fiberutil.Conflict("Archived servers cannot be restarted. Restore the server first.")
	}
	db := s.DB().WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ServerRestart{ServerID: id, Status: "idle"}).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	deadline := now.Add(5 * time.Minute)
	operationID := util.NewULID()
	result := db.Model(&models.ServerRestart{}).Where("server_id = ? AND status NOT IN ?", id, []string{"queued", "restarting", "verifying"}).Updates(map[string]any{
		"operation_id": operationID, "status": "queued", "boot_id": "", "requested_at": now,
		"completed_at": nil, "deadline_at": deadline, "check_after": nil, "message": "Restart queued. Waiting for the server to restart.",
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fiberutil.Conflict("A restart is already in progress. Wait for its result before restarting again.")
	}
	task, err := jobs.NewTrackedRebootServerTask(id, operationID, &userID)
	if err == nil {
		err = s.EnqueueTask(task)
	}
	if err != nil {
		_ = db.Model(&models.ServerRestart{}).Where("server_id = ? AND operation_id = ?", id, operationID).Updates(map[string]any{
			"status": "failed", "completed_at": time.Now().UTC(), "message": "The restart could not be queued. Try again.",
		}).Error
		s.BroadcastToTeam(teamID, "server.reboot.updated", map[string]any{"server_id": id})
		return err
	}
	s.BroadcastToTeam(teamID, "server.reboot.updated", map[string]any{"server_id": id})
	return nil
}

// VerifyServerRestart checks the original boot identity again, without issuing another reboot.
func (s *Service) VerifyServerRestart(ctx context.Context, id, teamID string) (models.ServerRestart, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return models.ServerRestart{}, err
	}
	if server.IsArchived() {
		return models.ServerRestart{}, fiberutil.Conflict("Restore the archived server before checking its restart.")
	}
	state, err := s.GetServerRestart(ctx, id, teamID)
	if err != nil {
		return models.ServerRestart{}, err
	}
	if state.Active() || state.Status == "succeeded" {
		return state, nil
	}
	if !state.CanVerify {
		return models.ServerRestart{}, fiberutil.Conflict("No previous restart can be verified. Check the connection and request a restart again.")
	}
	now := time.Now().UTC()
	// A recheck gets its own generation while retaining the original boot
	// identity. Old probes and timeout snapshots must not finish this new check.
	operationID := util.NewULID()
	result := s.DB().WithContext(ctx).Model(&models.ServerRestart{}).
		Where("server_id = ? AND operation_id = ? AND status = ?", id, state.OperationID, "failed").
		Updates(map[string]any{"operation_id": operationID, "status": "verifying", "deadline_at": now.Add(5 * time.Minute), "check_after": now, "completed_at": nil,
			"message": "Checking whether the earlier restart completed. No restart command will be sent."})
	if result.Error != nil {
		return models.ServerRestart{}, result.Error
	}
	if result.RowsAffected != 0 {
		task, taskErr := jobs.NewVerifyServerRestartTask(id, operationID)
		if taskErr == nil {
			taskErr = s.EnqueueTask(task)
		}
		if taskErr != nil {
			_ = s.DB().WithContext(ctx).Model(&models.ServerRestart{}).Where("server_id = ? AND operation_id = ?", id, operationID).
				Updates(map[string]any{"status": "failed", "completed_at": time.Now().UTC(), "message": "The restart check could not be queued. Check again."}).Error
			s.BroadcastToTeam(teamID, "server.reboot.updated", map[string]any{"server_id": id})
			return models.ServerRestart{}, taskErr
		}
		s.BroadcastToTeam(teamID, "server.reboot.updated", map[string]any{"server_id": id})
	}
	return s.GetServerRestart(ctx, id, teamID)
}

func (s *Service) ensureNoServerRestart(ctx context.Context, serverID string) error {
	var count int64
	if err := s.DB().WithContext(ctx).Model(&models.ServerRestart{}).Where("server_id = ? AND status IN ?", serverID,
		[]string{"queued", "restarting", "verifying"}).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return fiberutil.Conflict("A server restart is in progress. Wait for confirmation before changing system updates.")
	}
	return nil
}
