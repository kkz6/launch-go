package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// DeleteServerJob deletes a server from the provider and database.
// Similar to Laravel's Modules\Server\Jobs\DeleteServer
type DeleteServerJob struct {
	ServerJobBase
	Payload DeleteServerPayload
}

// Type returns the job type identifier
func (j *DeleteServerJob) Type() string {
	return TypeDeleteServer
}

// Handle processes the job
func (j *DeleteServerJob) Handle(ctx context.Context) error {
	// Find the server with relations
	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
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
		j.LogInfo("Deleting server from cloud provider",
			"server_id", server.ID,
			"provider", server.Provider.String(),
			"provider_server_id", providerServerID,
		)

		provider, err := j.ProviderFactory().Create(server.Provider)
		if err != nil {
			j.LogError(err, "Failed to create provider, continuing with database deletion")
		} else {
			// Get credentials from server provider
			var credentials map[string]interface{}
			if server.ServerProviderID != nil {
				serverProvider, err := j.Repo().FindServerProviderByID(ctx, *server.ServerProviderID)
				if err == nil {
					credStr := serverProvider.Credentials.String()
					if credStr != "" {
						_ = json.Unmarshal([]byte(credStr), &credentials)
					}
				}
			}

			if credentials != nil && len(credentials) > 0 {
				if err := provider.Delete(ctx, server, credentials); err != nil {
					j.LogError(err, "Failed to delete server from provider, continuing with database deletion",
						"server_id", server.ID,
						"provider", server.Provider.String(),
					)
				} else {
					j.LogInfo("Server deleted from cloud provider",
						"server_id", server.ID,
						"provider", server.Provider.String(),
					)
				}
			}
		}
	}

	// Delete the server record from database
	if err := j.Repo().DeleteServer(ctx, server.ID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	j.LogInfo("Server deleted successfully",
		"server_id", server.ID,
		"server_name", server.Name,
	)

	// Broadcast event
	j.BroadcastServerEvent(server.ID, "server.deleted", map[string]interface{}{
		"server_id": server.ID,
	})

	return nil
}

// NewDeleteServerTask creates an asynq task for deleting a server
func NewDeleteServerTask(serverID, teamID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeDeleteServer, DeleteServerPayload{
		ServerID: serverID,
		TeamID:   teamID,
		UserID:   userID,
	})
}

// Failed is called when the job fails after all retries
func (j *DeleteServerJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to delete server",
		"server_id", j.Payload.ServerID,
	)
}
