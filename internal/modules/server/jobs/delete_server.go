package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/notification/notifications"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

const TypeDeleteServer = "server:delete"

type DeleteServerPayload struct {
	ServerID       string             `json:"server_id"`
	TeamID         string             `json:"team_id"`
	UserID         *string            `json:"user_id,omitempty"`
	PreviousStatus types.ServerStatus `json:"previous_status,omitempty"`
}

// DeleteServerJob deletes a server from the provider and database.
// Similar to Laravel's Modules\Server\Jobs\DeleteServer
type DeleteServerJob struct {
	Deps    *JobDeps
	Payload DeleteServerPayload

	server   *models.Server
	rejected bool
}

func NewDeleteServerJob(p DeleteServerPayload) pkgjobs.Handler {
	return &DeleteServerJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *DeleteServerJob) Handle(ctx context.Context) error {
	j.rejected = false
	// Hold the parent lock through the workload check and deletion. Foreign-key
	// checks for new sites/projects cannot race between the count and removal.
	err := j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Resolve the payload's team again before any destructive operation.
		j.server = &models.Server{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND team_id = ?", j.Payload.ServerID, j.Payload.TeamID).First(j.server).Error; err != nil {
			j.rejected = errors.Is(err, gorm.ErrRecordNotFound)
			return fmt.Errorf("failed to find server: %w", err)
		}
		repos := repositories.NewRegistry(tx)
		if j.server.Status != types.ServerStatusDeleting {
			j.rejected = true
			return errors.New("server is not awaiting deletion")
		}

		// Workloads may have changed after the request was accepted. Fail closed
		// when either count fails, before contacting the provider or deleting keys.
		var count int64
		if err := tx.Table("sites").Where("server_id = ?", j.server.ID).Count(&count).Error; err != nil {
			return fmt.Errorf("count server sites: %w", err)
		}
		if count > 0 {
			return errors.New("server still has sites")
		}
		if err := tx.Table("docker_projects").Where("server_id = ? AND deleted_at IS NULL", j.server.ID).Count(&count).Error; err != nil {
			return fmt.Errorf("count server Docker projects: %w", err)
		}
		if count > 0 {
			return errors.New("server still has Docker projects")
		}

		// A custom server is disconnected only; never operate on its remote host.
		if j.server.Provider != types.ProviderCustom {
			if j.Deps.ProviderFactory == nil {
				return errors.New("cloud provider factory is unavailable")
			}
			provider, err := j.Deps.ProviderFactory.Create(j.server.Provider)
			if err != nil {
				return errors.New("cloud provider is unsupported")
			}
			if j.server.ServerProviderID == nil {
				return errors.New("cloud provider credentials are unavailable")
			}
			account, err := repos.ServerProvider().FindByID(ctx, *j.server.ServerProviderID)
			if err != nil || account == nil || account.Provider != j.server.Provider ||
				(account.TeamID != nil && *account.TeamID != j.server.TeamID) {
				return errors.New("cloud provider credentials are unavailable")
			}
			var credentials map[string]any
			if strings.TrimSpace(account.Credentials.String()) == "" ||
				json.Unmarshal([]byte(account.Credentials.String()), &credentials) != nil || len(credentials) == 0 {
				return errors.New("cloud provider credentials are invalid")
			}
			if err := provider.Delete(ctx, j.server, credentials); err != nil {
				return fmt.Errorf("cloud provider deletion failed; server record retained: %w", err)
			}
		}

		// Delete server-specific (non-global) SSH keys before deleting the server
		if err := repos.SSHKey().DeleteNonGlobalByServer(ctx, j.server.ID); err != nil {
			return fmt.Errorf("failed to delete server-specific SSH keys: %w", err)
		}

		// Delete the server record from database
		if err := repos.Server().Delete(ctx, j.server.ID); err != nil {
			return fmt.Errorf("failed to delete server: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Record and broadcast only after the database transaction commits.
	activity.RecordWithLogPtr(ctx, "server", "deleted", j.Payload.UserID, j.server, "Server was deleted")

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("server_name", j.server.Name).
		Msg("server deleted successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.server, "server.deleted", map[string]any{
		"server_id": j.server.ID,
	})

	return nil
}

// Failed restores a retryable state and reports a safe terminal failure.
func (j *DeleteServerJob) Failed(ctx context.Context, _ error) {
	// A rejected stale task must not reset a later accepted deletion intent.
	if j.rejected {
		return
	}
	j.Deps.Logger.Error().Str("server_id", j.Payload.ServerID).Msg("failed to delete server; record retained")
	ctx = context.WithoutCancel(ctx)
	server, err := j.Deps.Repos.Server().FindByIDAndTeam(ctx, j.Payload.ServerID, j.Payload.TeamID)
	if err != nil || server == nil || server.Status != types.ServerStatusDeleting {
		return
	}
	previous := j.Payload.PreviousStatus
	if !previous.IsValid() || previous == types.ServerStatusDeleting || previous == types.ServerStatusStarting || previous == types.ServerStatusProvisioning {
		previous = types.ServerStatusFailed
	}
	restored := j.Deps.DB.WithContext(ctx).Model(&models.Server{}).
		Where("id = ? AND team_id = ? AND status = ?", server.ID, server.TeamID, types.ServerStatusDeleting).
		Update("status", previous)
	if restored.Error != nil || restored.RowsAffected != 1 {
		j.Deps.Logger.Error().Str("server_id", server.ID).Msg("failed to restore server status after deletion failure")
		return
	}
	server.Status = previous
	const message = "Server deletion failed. The server record has been retained; check the provider and try again."
	j.Deps.BroadcastServerEvent(server, "server.deletion_failed", map[string]any{
		"server_id": server.ID,
		"error":     message,
	})
	j.Deps.BroadcastServerEvent(server, "server.updated", dto.ToServerResponse(server))
	if j.Deps.TaskRunnerDeps != nil && j.Deps.TaskRunnerDeps.Notifier != nil {
		notification := notifications.NewFailedToDeleteServerNotification(server.Name, server.Provider.String(), message)
		_ = j.Deps.TaskRunnerDeps.Notifier.SendToTeam(ctx, server.TeamID, notification)
	}
}

// NewDeleteServerTask creates an asynq task for deleting a server
// Each accepted deletion intent receives a fresh ID so archived failures do not block retries.
func NewDeleteServerTask(serverID, teamID string, userID *string, previousStatus ...types.ServerStatus) (*asynq.Task, error) {
	previous := types.ServerStatusFailed
	if len(previousStatus) > 0 {
		previous = previousStatus[0]
	}
	return pkgjobs.Task(TypeDeleteServer, DeleteServerPayload{
		ServerID:       serverID,
		TeamID:         teamID,
		UserID:         userID,
		PreviousStatus: previous,
	}, asynq.TaskID(pkgjobs.Dedup("delete_server", serverID, util.NewULID())))
}
