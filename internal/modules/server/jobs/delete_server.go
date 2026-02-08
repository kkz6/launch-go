package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeDeleteServer = "server:delete"

type DeleteServerPayload struct {
	ServerID string  `json:"server_id"`
	TeamID   string  `json:"team_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// DeleteServerJob deletes a server from the provider and database.
// Similar to Laravel's Modules\Server\Jobs\DeleteServer
type DeleteServerJob struct {
	Deps    *JobDeps
	Payload DeleteServerPayload

	server *models.Server
}

func NewDeleteServerJob(p DeleteServerPayload) pkgjobs.Handler {
	return &DeleteServerJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *DeleteServerJob) Handle(ctx context.Context) error {
	var err error

	// Find the server with relations
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Check if server has provider data with a provider server ID
	providerServerID := ""
	if j.server.ProviderData != nil {
		if id, ok := j.server.ProviderData["provider_server_id"].(string); ok {
			providerServerID = id
		}
	}

	// Delete from cloud provider if not a custom server
	if j.server.Provider != types.ProviderCustom && providerServerID != "" {
		j.Deps.Logger.Info().
			Str("server_id", j.server.ID).
			Str("provider", j.server.Provider.String()).
			Str("provider_server_id", providerServerID).
			Msg("deleting server from cloud provider")

		provider, err := j.Deps.ProviderFactory.Create(j.server.Provider)
		if err != nil {
			j.Deps.Logger.Error().Err(err).Msg("failed to create provider, continuing with database deletion")
		} else {
			// Get credentials from server provider
			var credentials map[string]any
			if j.server.ServerProviderID != nil {
				serverProvider, err := j.Deps.Repos.ServerProvider().FindByID(ctx, *j.server.ServerProviderID)
				if err == nil {
					credStr := serverProvider.Credentials.String()
					if credStr != "" {
						_ = json.Unmarshal([]byte(credStr), &credentials)
					}
				}
			}

			if len(credentials) > 0 {
				if err := provider.Delete(ctx, j.server, credentials); err != nil {
					j.Deps.Logger.Error().Err(err).
						Str("server_id", j.server.ID).
						Str("provider", j.server.Provider.String()).
						Msg("failed to delete server from provider, continuing with database deletion")
				} else {
					j.Deps.Logger.Info().
						Str("server_id", j.server.ID).
						Str("provider", j.server.Provider.String()).
						Msg("server deleted from cloud provider")
				}
			}
		}
	}

	// Delete server-specific (non-global) SSH keys before deleting the server
	if err := j.Deps.Repos.SSHKey().DeleteNonGlobalByServer(ctx, j.server.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Str("server_id", j.server.ID).Msg("failed to delete server-specific SSH keys")
	}

	// Delete the server record from database
	if err := j.Deps.Repos.Server().Delete(ctx, j.server.ID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	// Log activity before deletion
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

// Failed is called when the job fails after all retries
func (j *DeleteServerJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to delete server")
}

// NewDeleteServerTask creates an asynq task for deleting a server
// Uses TaskID for deduplication to prevent duplicate deletes
func NewDeleteServerTask(serverID, teamID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeDeleteServer, DeleteServerPayload{
		ServerID: serverID,
		TeamID:   teamID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("delete_server", serverID)))
}
