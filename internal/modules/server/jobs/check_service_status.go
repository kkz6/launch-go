package jobs

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/status"
)

const TypeCheckServiceStatus = "server:check_service_status"

type CheckServiceStatusPayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	UserID    *string `json:"user_id,omitempty"`
}

// CheckServiceStatusJob checks the status of a service and updates the database.
// Similar to Laravel's Modules\Server\Jobs\CheckServiceStatusOnServer
type CheckServiceStatusJob struct {
	Deps    *JobDeps
	Payload CheckServiceStatusPayload

	server  *models.Server
	service *models.InstalledService
}

func NewCheckServiceStatusJob(p CheckServiceStatusPayload) pkgjobs.Handler {
	return &CheckServiceStatusJob{Deps: deps, Payload: p}
}

// Timeout returns the job timeout duration
func (j *CheckServiceStatusJob) Timeout() time.Duration {
	return 1 * time.Minute
}

func (j *CheckServiceStatusJob) Handle(ctx context.Context) error {
	var err error

	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("find service: %w", err)
	}

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	task := tasks.GetServiceStatusTask(j.service.Software, j.service.Version)

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		j.updateServiceStatus(ctx, j.service.ID, types.ServiceStatusFailed, "", nil, err.Error())
		return fmt.Errorf("check service status: %w", err)
	}

	output := result.GetOutput()
	svcStatus := j.parseServiceStatus(output)
	details := j.parseStatusDetails(output)

	j.updateServiceStatus(ctx, j.service.ID, svcStatus, output, details, "")

	j.Deps.Logger.Info().
		Str("service_id", j.service.ID).
		Str("server_id", j.server.ID).
		Str("status", svcStatus.String()).
		Msg("service status check completed")

	j.Deps.BroadcastServerEvent(j.server, "service.status_checked", map[string]any{
		"service_id": j.service.ID,
		"server_id":  j.server.ID,
		"status":     svcStatus.String(),
	})

	return nil
}

func (j *CheckServiceStatusJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("service_id", j.Payload.ServiceID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to check service status")
}

func (j *CheckServiceStatusJob) updateServiceStatus(ctx context.Context, serviceID string, svcStatus types.ServiceStatus, output string, details map[string]any, errorMsg string) {
	typeData := dbtype.JSONMap{
		"last_status_check": time.Now().Format(time.RFC3339),
		"status_output":     output,
	}

	if details != nil {
		typeData["status_details"] = details
	}

	if errorMsg != "" {
		typeData["status_error"] = errorMsg
	}

	if err := j.Deps.Repos.Service().UpdateWithTypeData(ctx, serviceID, svcStatus, typeData); err != nil {
		j.Deps.Logger.Error().Err(err).
			Str("service_id", serviceID).
			Msg("failed to update service status")
	}
}

func (j *CheckServiceStatusJob) parseServiceStatus(output string) types.ServiceStatus {
	lowerOutput := strings.ToLower(output)

	if strings.Contains(lowerOutput, "active (running)") ||
		strings.Contains(lowerOutput, "is running") ||
		strings.Contains(lowerOutput, "status: started") {
		return types.ServiceStatusRunning
	}

	if strings.Contains(lowerOutput, "inactive (dead)") ||
		strings.Contains(lowerOutput, "is stopped") ||
		strings.Contains(lowerOutput, "status: stopped") ||
		strings.Contains(lowerOutput, "not running") {
		return types.ServiceStatusStopped
	}

	if strings.Contains(lowerOutput, "failed") ||
		strings.Contains(lowerOutput, "error") ||
		strings.Contains(lowerOutput, "could not") {
		return types.ServiceStatusFailed
	}

	return types.ServiceStatusInstalled
}

func (j *CheckServiceStatusJob) parseStatusDetails(output string) map[string]any {
	details := map[string]any{
		"uptime":          nil,
		"pid":             nil,
		"memory_usage":    nil,
		"cpu_usage":       nil,
		"connections":     []string{},
		"processes":       []string{},
		"started_at":      nil,
		"additional_info": map[string]any{},
	}

	pidRegex := regexp.MustCompile(`Main PID:\s*(\d+)`)
	if matches := pidRegex.FindStringSubmatch(output); len(matches) > 1 {
		details["pid"] = matches[1]
	}

	activeSinceRegex := regexp.MustCompile(`Active:\s*active \(running\) since (.+?);`)
	if matches := activeSinceRegex.FindStringSubmatch(output); len(matches) > 1 {
		details["started_at"] = strings.TrimSpace(matches[1])
	}

	memoryRegex := regexp.MustCompile(`MemoryCurrent=(\d+)`)
	if matches := memoryRegex.FindStringSubmatch(output); len(matches) > 1 {
		details["memory_usage"] = status.ParseBytesString(matches[1])
	}

	sections := strings.Split(output, "===")
	for i := 1; i < len(sections); i++ {
		section := sections[i]
		lines := strings.Split(section, "\n")
		if len(lines) == 0 {
			continue
		}

		sectionName := strings.TrimSpace(lines[0])
		sectionContent := make([]string, 0)

		for _, line := range lines[1:] {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				sectionContent = append(sectionContent, trimmed)
			}
		}

		switch sectionName {
		case "PROCESSES":
			if len(sectionContent) > 5 {
				sectionContent = sectionContent[:5]
			}
			details["processes"] = sectionContent
		case "CONNECTIONS":
			if len(sectionContent) > 10 {
				sectionContent = sectionContent[:10]
			}
			details["connections"] = sectionContent
		case "REDIS_INFO":
			additionalInfo := details["additional_info"].(map[string]any)
			additionalInfo["redis"] = strings.Join(sectionContent, "\n")
		case "SUPERVISOR_STATUS":
			additionalInfo := details["additional_info"].(map[string]any)
			additionalInfo["supervisor"] = strings.Join(sectionContent, "\n")
		case "CONFIG_TEST":
			additionalInfo := details["additional_info"].(map[string]any)
			additionalInfo["config_test"] = strings.Join(sectionContent, "\n")
		case "FPM_STATUS":
			additionalInfo := details["additional_info"].(map[string]any)
			additionalInfo["php_version"] = strings.Join(sectionContent, "\n")
		case "POOL_CONFIG":
			additionalInfo := details["additional_info"].(map[string]any)
			additionalInfo["pool_config"] = strings.Join(sectionContent, "\n")
		case "PROGRAMS":
			additionalInfo := details["additional_info"].(map[string]any)
			additionalInfo["programs"] = sectionContent
		}
	}

	return details
}

func NewCheckServiceStatusTask(serverID, serviceID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCheckServiceStatus, CheckServiceStatusPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		UserID:    userID,
	})
}
