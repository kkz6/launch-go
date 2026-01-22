package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, DeleteServerPayload]
}

// Handle processes the job
func (j *DeleteServerJob) Handle(ctx context.Context) error {
	// Find the server with relations
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Check if server has provider data with a provider server ID
	providerServerID := ""
	if server.ProviderData != nil {
		if id, ok := server.ProviderData["provider_server_id"].(string); ok {
			providerServerID = id
		}
	}

	// Delete from cloud provider if not a custom server
	if server.Provider != types.ProviderCustom && providerServerID != "" {
		j.Ctx.LogInfo("Deleting server from cloud provider",
			"server_id", server.ID,
			"provider", server.Provider.String(),
			"provider_server_id", providerServerID,
		)

		provider, err := j.Ctx.ProviderFactory.Create(server.Provider)
		if err != nil {
			j.Ctx.LogError(err, "Failed to create provider, continuing with database deletion")
		} else {
			// Get credentials from server provider
			var credentials map[string]any
			if server.ServerProviderID != nil {
				serverProvider, err := j.Ctx.Repos().ServerProvider().FindByID(ctx, *server.ServerProviderID)
				if err == nil {
					credStr := serverProvider.Credentials.String()
					if credStr != "" {
						_ = json.Unmarshal([]byte(credStr), &credentials)
					}
				}
			}

			if len(credentials) > 0 {
				if err := provider.Delete(ctx, server, credentials); err != nil {
					j.Ctx.LogError(err, "Failed to delete server from provider, continuing with database deletion",
						"server_id", server.ID,
						"provider", server.Provider.String(),
					)
				} else {
					j.Ctx.LogInfo("Server deleted from cloud provider",
						"server_id", server.ID,
						"provider", server.Provider.String(),
					)
				}
			}
		}
	}

	// Delete the server record from database
	if err := j.Ctx.Repos().Server().Delete(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	// Log activity before deletion
	activity.RecordWithLogPtr(ctx, "server", "deleted", j.Payload.UserID, server, "Server was deleted")

	j.Ctx.LogInfo("Server deleted successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(server, "server.deleted", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *DeleteServerJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to delete server",
		"server_id", j.Payload.ServerID,
	)
}

// NewDeleteServerJob creates a new DeleteServerJob with the given context and payload.
func NewDeleteServerJob(ctx *JobContext, payload DeleteServerPayload) *DeleteServerJob {
	return &DeleteServerJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewDeleteServerTask creates an asynq task for deleting a server
// Uses TaskID for deduplication to prevent duplicate deletes
func NewDeleteServerTask(serverID, teamID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeDeleteServer, DeleteServerPayload{
		ServerID: serverID,
		TeamID:   teamID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("delete_server:%s", serverID)))
}
