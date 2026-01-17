package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	ctx     *JobContext
	Payload DeleteServerPayload
}

// Handle processes the job
func (j *DeleteServerJob) Handle(ctx context.Context) error {
	// Find the server with relations
	server, err := j.ctx.Repos.Server().FindByID(ctx, j.Payload.ServerID)
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
	if server.Provider != enums.ProviderCustom && providerServerID != "" {
		j.ctx.LogInfo("Deleting server from cloud provider",
			"server_id", server.ID,
			"provider", server.Provider.String(),
			"provider_server_id", providerServerID,
		)

		provider, err := j.ctx.ProviderFactory.Create(server.Provider)
		if err != nil {
			j.ctx.LogError(err, "Failed to create provider, continuing with database deletion")
		} else {
			// Get credentials from server provider
			var credentials map[string]any
			if server.ServerProviderID != nil {
				serverProvider, err := j.ctx.Repos.ServerProvider().FindByID(ctx, *server.ServerProviderID)
				if err == nil {
					credStr := serverProvider.Credentials.String()
					if credStr != "" {
						_ = json.Unmarshal([]byte(credStr), &credentials)
					}
				}
			}

			if credentials != nil && len(credentials) > 0 {
				if err := provider.Delete(ctx, server, credentials); err != nil {
					j.ctx.LogError(err, "Failed to delete server from provider, continuing with database deletion",
						"server_id", server.ID,
						"provider", server.Provider.String(),
					)
				} else {
					j.ctx.LogInfo("Server deleted from cloud provider",
						"server_id", server.ID,
						"provider", server.Provider.String(),
					)
				}
			}
		}
	}

	// Delete the server record from database
	if err := j.ctx.Repos.Server().Delete(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	// Log activity before deletion
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(server).
		WithEvent("deleted")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Server was deleted")

	j.ctx.LogInfo("Server deleted successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.ctx.BroadcastServerEvent(server, "server.deleted", map[string]any{
		"server_id": server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *DeleteServerJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to delete server",
		"server_id", j.Payload.ServerID,
	)
}

// NewDeleteServerJob creates a new DeleteServerJob with the given context and payload.
func NewDeleteServerJob(ctx *JobContext, payload DeleteServerPayload) *DeleteServerJob {
	return &DeleteServerJob{
		ctx:     ctx,
		Payload: payload,
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
