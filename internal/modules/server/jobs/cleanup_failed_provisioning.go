package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeCleanupFailedProvisioning = "server:cleanup_failed_provisioning"

type CleanupFailedProvisioningPayload struct {
	ServerID         string  `json:"server_id"`
	TeamID           string  `json:"team_id"`
	UserID           *string `json:"user_id,omitempty"`
	ServerProviderID string  `json:"server_provider_id,omitempty"`
	Reason           string  `json:"reason,omitempty"`
	DeleteRecord     bool    `json:"delete_record,omitempty"`
}

// CleanupFailedProvisioningJob cleans up resources after a failed server provisioning.
// This job is triggered when WaitForServerToConnect or ProvisionServer fails.
// It removes the server from the cloud provider and optionally deletes the database record.
type CleanupFailedProvisioningJob struct {
	Deps    *JobDeps
	Payload CleanupFailedProvisioningPayload

	server   *models.Server
	rejected bool
}

func NewCleanupFailedProvisioningJob(p CleanupFailedProvisioningPayload) pkgjobs.Handler {
	return &CleanupFailedProvisioningJob{Deps: deps, Payload: p}
}

func (j *CleanupFailedProvisioningJob) Handle(ctx context.Context) error {
	j.rejected = false
	cleaned := false
	// Serialize cleanup with server state changes and workload insertion, as
	// normal deletion does. A recovered server must not be removed by old work.
	err := j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		j.server = &models.Server{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND team_id = ?", j.Payload.ServerID, j.Payload.TeamID).First(j.server).Error; err != nil {
			j.rejected = errors.Is(err, gorm.ErrRecordNotFound)
			return fmt.Errorf("find server: %w", err)
		}
		if j.server.Status != types.ServerStatusFailed {
			j.rejected = true
			return nil
		}
		if j.Payload.ServerProviderID != "" &&
			(j.server.ServerProviderID == nil || *j.server.ServerProviderID != j.Payload.ServerProviderID) {
			j.rejected = true
			return errors.New("server provider changed since cleanup was requested")
		}

		repos := repositories.NewRegistry(tx)
		cloud := j.server.Provider != types.ProviderCustom
		if cloud || j.Payload.DeleteRecord {
			if err := requireServerWithoutWorkloads(ctx, tx, j.server.ID); err != nil {
				return err
			}
		}
		if cloud {
			provider, credentials, err := resolveServerProvider(ctx, j.server, repos, j.Deps.ProviderFactory)
			if err != nil {
				return err
			}
			// Providers own instance-ID validation and any secondary cleanup.
			if err := provider.Delete(ctx, j.server, credentials); err != nil {
				return fmt.Errorf("delete server from provider: %w", err)
			}
			if !j.Payload.DeleteRecord {
				// The removed host can no longer be reached or trusted. Preserve
				// provider metadata so a repeated deletion remains idempotent.
				if err := tx.Model(j.server).Updates(map[string]any{
					"connected":    false,
					"host_key":     "",
					"public_ipv4":  nil,
					"private_ipv4": nil,
				}).Error; err != nil {
					return fmt.Errorf("mark removed server disconnected: %w", err)
				}
			}
		}
		if j.Payload.DeleteRecord {
			if err := repos.SSHKey().DeleteNonGlobalByServer(ctx, j.server.ID); err != nil {
				return fmt.Errorf("delete server-specific SSH keys: %w", err)
			}
			if err := repos.Server().Delete(ctx, j.server.ID); err != nil {
				return fmt.Errorf("delete server record: %w", err)
			}
		}
		cleaned = true
		return nil
	})
	if err != nil || !cleaned {
		return err
	}

	activity.RecordWithLogAndPropsPtr(ctx, "server", "provisioning_failed", j.Payload.UserID, j.server, "Server provisioning cleanup completed", map[string]any{
		"reason": j.Payload.Reason,
	})
	j.Deps.BroadcastServerEvent(j.server, "server.provisioning_cleanup_complete", map[string]any{
		"server_id": j.server.ID,
		"reason":    j.Payload.Reason,
		"deleted":   j.Payload.DeleteRecord,
	})
	if j.Payload.DeleteRecord {
		j.Deps.BroadcastServerEvent(j.server, "server.deleted", map[string]any{
			"server_id": j.server.ID,
			"reason":    "provisioning_failed",
		})
	}
	return nil
}

func (j *CleanupFailedProvisioningJob) Failed(ctx context.Context, err error) {
	if j.rejected {
		return
	}
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to cleanup failed provisioning")

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	server, findErr := j.Deps.Repos.Server().FindByIDAndTeam(ctx, j.Payload.ServerID, j.Payload.TeamID)
	if findErr != nil || server == nil || server.Status != types.ServerStatusFailed {
		return
	}
	j.Deps.BroadcastServerEvent(server, "server.cleanup_failed", map[string]any{
		"server_id": server.ID,
		"error":     "Provisioning cleanup failed. The server record has been retained; check the provider and try again.",
	})
}

func NewCleanupFailedProvisioningTask(serverID, teamID string, serverProviderID string, userID *string, reason string, deleteRecord bool) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeCleanupFailedProvisioning,
		CleanupFailedProvisioningPayload{
			ServerID:         serverID,
			TeamID:           teamID,
			ServerProviderID: serverProviderID,
			UserID:           userID,
			Reason:           reason,
			DeleteRecord:     deleteRecord,
		},
		pkgjobs.Dedup("cleanup_provisioning", serverID),
	)
}
