package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks/services"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRestartService = "server:restart_service"

// RestartServicePayload contains data for restarting a service
type RestartServicePayload struct {
	ServerID  string              `json:"server_id"`
	Software  enums.Software      `json:"software"`
	Operation enums.ServiceOption `json:"operation"`
}

// RestartServiceJob handles restarting a service on a server
type RestartServiceJob struct {
	*JobContext
}

// NewRestartServiceTask creates a new asynq task for restarting a service
func NewRestartServiceTask(serverID string, software enums.Software, operation enums.ServiceOption) (*asynq.Task, error) {
	return jobs.NewTask(TypeRestartService, RestartServicePayload{
		ServerID:  serverID,
		Software:  software,
		Operation: operation,
	})
}

// Handle processes the restart service job
func (j *RestartServiceJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[RestartServicePayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Performing service operation")

	// Fetch the server
	server, err := j.FindServer(ctx, payload.ServerID)
	if err != nil {
		return err
	}

	j.broadcastProgress(payload.ServerID, "operating", fmt.Sprintf("%s %s...", payload.Operation.Label(), payload.Software.Label()))

	// Get the service name from the software
	serviceName := getServiceName(payload.Software)
	if serviceName == "" {
		return fmt.Errorf("unknown service for software: %s", payload.Software)
	}

	// Run the restart service task
	_, err = j.RunTask(server, services.NewRestartService(server, serviceName)).
		AsRoot().
		Throw().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	// Update service status to running
	if err := j.DB.Model(&models.InstalledService{}).
		Where("server_id = ? AND software = ?", payload.ServerID, payload.Software).
		Update("status", enums.ServiceStatusRunning).Error; err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "running", fmt.Sprintf("%s %s completed", payload.Operation.Label(), payload.Software.Label()))

	return nil
}

// Failed handles job failure
func (j *RestartServiceJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[RestartServicePayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("software", string(payload.Software)).
		Str("operation", string(payload.Operation)).
		Msg("Failed to perform service operation")

	j.broadcastProgress(payload.ServerID, "failed", fmt.Sprintf("Failed to %s %s", payload.Operation.Label(), payload.Software.Label()))
}

func (j *RestartServiceJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.service.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}

// getServiceName maps a software to its systemd service name
func getServiceName(software enums.Software) string {
	switch software {
	case enums.SoftwareMySql80:
		return "mysql"
	case enums.SoftwarePostgreSql16:
		return "postgresql"
	case enums.SoftwareRedis:
		return "redis-server"
	case enums.SoftwareCaddy2:
		return "caddy"
	case enums.SoftwareSupervisor:
		return "supervisor"
	case enums.SoftwarePhp56:
		return "php5.6-fpm"
	case enums.SoftwarePhp70:
		return "php7.0-fpm"
	case enums.SoftwarePhp71:
		return "php7.1-fpm"
	case enums.SoftwarePhp72:
		return "php7.2-fpm"
	case enums.SoftwarePhp73:
		return "php7.3-fpm"
	case enums.SoftwarePhp74:
		return "php7.4-fpm"
	case enums.SoftwarePhp80:
		return "php8.0-fpm"
	case enums.SoftwarePhp81:
		return "php8.1-fpm"
	case enums.SoftwarePhp82:
		return "php8.2-fpm"
	case enums.SoftwarePhp83:
		return "php8.3-fpm"
	case enums.SoftwarePhp84:
		return "php8.4-fpm"
	default:
		return ""
	}
}
