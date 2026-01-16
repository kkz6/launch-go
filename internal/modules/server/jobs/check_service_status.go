package jobs

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	ctx     *JobContext
	Payload CheckServiceStatusPayload
}

// Timeout returns the job timeout duration
func (j *CheckServiceStatusJob) Timeout() time.Duration {
	return 1 * time.Minute
}

// Handle processes the job
func (j *CheckServiceStatusJob) Handle(ctx context.Context) error {
	service, err := j.ctx.Repo.FindServiceByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	server, err := j.ctx.Repo.FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	task := tasks.GetServiceStatusTask(service.Software, service.Version)

	result, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		j.updateServiceStatus(ctx, service.ID, enums.ServiceStatusFailed, "", nil, err.Error())
		return fmt.Errorf("failed to check service status: %w", err)
	}

	output := result.GetOutput()
	status := j.parseServiceStatus(output)
	details := j.parseStatusDetails(output)

	j.updateServiceStatus(ctx, service.ID, status, output, details, "")

	j.ctx.LogInfo("Service status check completed",
		"service_id", service.ID,
		"server_id", server.ID,
		"status", status,
	)

	j.ctx.BroadcastServerEvent(server, "service.status_checked", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
		"status":     status.String(),
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *CheckServiceStatusJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to check service status",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)
}

func (j *CheckServiceStatusJob) updateServiceStatus(ctx context.Context, serviceID string, status enums.ServiceStatus, output string, details map[string]any, errorMsg string) {
	typeData := basemodels.JSONMap{
		"last_status_check": time.Now().Format(time.RFC3339),
		"status_output":     output,
	}

	if details != nil {
		typeData["status_details"] = details
	}

	if errorMsg != "" {
		typeData["status_error"] = errorMsg
	}

	if err := j.ctx.Repo.UpdateServiceWithTypeData(ctx, serviceID, status, typeData); err != nil {
		j.ctx.LogError(err, "Failed to update service status", "service_id", serviceID)
	}
}

func (j *CheckServiceStatusJob) parseServiceStatus(output string) enums.ServiceStatus {
	lowerOutput := strings.ToLower(output)

	if strings.Contains(lowerOutput, "active (running)") ||
		strings.Contains(lowerOutput, "is running") ||
		strings.Contains(lowerOutput, "status: started") {
		return enums.ServiceStatusRunning
	}

	if strings.Contains(lowerOutput, "inactive (dead)") ||
		strings.Contains(lowerOutput, "is stopped") ||
		strings.Contains(lowerOutput, "status: stopped") ||
		strings.Contains(lowerOutput, "not running") {
		return enums.ServiceStatusStopped
	}

	if strings.Contains(lowerOutput, "failed") ||
		strings.Contains(lowerOutput, "error") ||
		strings.Contains(lowerOutput, "could not") {
		return enums.ServiceStatusFailed
	}

	return enums.ServiceStatusInstalled
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
		details["memory_usage"] = formatBytes(matches[1])
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

func formatBytes(bytesStr string) string {
	var bytes int64
	fmt.Sscanf(bytesStr, "%d", &bytes)

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func NewCheckServiceStatusJob(ctx *JobContext, payload CheckServiceStatusPayload) *CheckServiceStatusJob {
	return &CheckServiceStatusJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewCheckServiceStatusTask creates an asynq task for checking service status
func NewCheckServiceStatusTask(serverID, serviceID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeCheckServiceStatus, CheckServiceStatusPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		UserID:    userID,
	})
}
