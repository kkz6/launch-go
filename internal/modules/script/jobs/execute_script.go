package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/script/enums"
	"github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/modules/script/support"
	"github.com/kkz6/launch-go/internal/modules/script/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeExecuteScript is the task type for executing a script
const TypeExecuteScript = "script:execute"

// ExecuteScriptPayload represents the payload for executing a script
type ExecuteScriptPayload struct {
	ExecutionID uint64 `json:"execution_id"`
	ScriptID    string `json:"script_id"`
	ServerID    string `json:"server_id"`
	TeamID      string `json:"team_id"`
}

// ExecuteScriptJob handles script execution on a server
type ExecuteScriptJob struct {
	ctx     *JobContext
	Payload ExecuteScriptPayload
}

// NewExecuteScriptJob creates a new ExecuteScriptJob instance
func NewExecuteScriptJob(ctx *JobContext, payload ExecuteScriptPayload) *ExecuteScriptJob {
	return &ExecuteScriptJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the script on the server
func (j *ExecuteScriptJob) Handle(ctx context.Context) error {
	j.ctx.LogInfo("ExecuteScript job started",
		"execution_id", j.Payload.ExecutionID,
		"script_id", j.Payload.ScriptID,
		"server_id", j.Payload.ServerID,
	)

	// Get execution record
	execution, err := j.ctx.Repos().Execution().FindByID(ctx, j.Payload.ExecutionID)
	if err != nil {
		return fmt.Errorf("failed to find execution: %w", err)
	}

	// Get script
	script, err := j.ctx.Repos().Script().FindByID(ctx, j.Payload.ScriptID)
	if err != nil {
		return fmt.Errorf("failed to find script: %w", err)
	}

	// Get server
	server, err := j.ctx.Repos().Server().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update status to running
	now := time.Now()
	if err := j.ctx.Repos().Execution().UpdateFields(ctx, execution.ID, map[string]any{
		"status":     models.ExecutionStatusRunning,
		"started_at": now,
	}); err != nil {
		j.ctx.LogError(err, "Failed to update execution status")
	}

	// Broadcast started event
	j.ctx.BroadcastToTeam(j.Payload.TeamID, "script.execution.started", map[string]any{
		"execution_id": execution.ID,
		"batch_id":     execution.BatchID,
		"server_id":    server.ID,
		"script_id":    script.ID,
	})

	// Interpolate variables in the script content
	interpolatedContent := support.InterpolateVariables(script.Content, server)

	// Create the task
	task := tasks.RunScript(tasks.RunScriptConfig{
		Content: interpolatedContent,
	})

	// Resolve the run-as type to actual username from server
	runAsUser := resolveRunAsUser(execution.RunAs, server)

	// Execute the task on the server
	result, err := j.ctx.RunTaskOnServer(server, task).
		AsUser(runAsUser).
		Dispatch(ctx)

	// Prepare final status
	finalStatus := models.ExecutionStatusFinished
	var exitCode *int
	var output *string

	if result != nil {
		ec := result.GetExitCode()
		exitCode = &ec
		out := result.GetOutput()
		output = &out

		if !result.IsSuccessful() {
			finalStatus = models.ExecutionStatusFailed
		}
	} else if err != nil {
		finalStatus = models.ExecutionStatusFailed
		errMsg := err.Error()
		output = &errMsg
	}

	// Update execution record with result
	finishedAt := time.Now()
	if updateErr := j.ctx.Repos().Execution().UpdateFields(ctx, execution.ID, map[string]any{
		"status":      finalStatus,
		"exit_code":   exitCode,
		"output":      output,
		"finished_at": finishedAt,
	}); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update execution result")
	}

	// Broadcast completion event
	j.ctx.BroadcastToTeam(j.Payload.TeamID, "script.execution.completed", map[string]any{
		"execution_id": execution.ID,
		"batch_id":     execution.BatchID,
		"server_id":    server.ID,
		"script_id":    script.ID,
		"status":       string(finalStatus),
		"exit_code":    exitCode,
	})

	j.ctx.LogInfo("ExecuteScript job completed",
		"execution_id", execution.ID,
		"status", finalStatus,
	)

	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

// Failed handles job failure
func (j *ExecuteScriptJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "ExecuteScript job failed",
		"execution_id", j.Payload.ExecutionID,
		"script_id", j.Payload.ScriptID,
		"server_id", j.Payload.ServerID,
	)

	errMsg := err.Error()

	// Update execution status to failed
	if updateErr := j.ctx.Repos().Execution().UpdateFields(ctx, j.Payload.ExecutionID, map[string]any{
		"status":      models.ExecutionStatusFailed,
		"output":      errMsg,
		"finished_at": time.Now(),
	}); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update execution status on failure")
	}

	// Broadcast failure event
	j.ctx.BroadcastToTeam(j.Payload.TeamID, "script.execution.completed", map[string]any{
		"execution_id": j.Payload.ExecutionID,
		"server_id":    j.Payload.ServerID,
		"script_id":    j.Payload.ScriptID,
		"status":       string(models.ExecutionStatusFailed),
		"error":        errMsg,
	})
}

// NewExecuteScriptTask creates an asynq task for executing a script
func NewExecuteScriptTask(executionID uint64, scriptID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeExecuteScript, ExecuteScriptPayload{
		ExecutionID: executionID,
		ScriptID:    scriptID,
		ServerID:    serverID,
		TeamID:      teamID,
	}, asynq.TaskID(fmt.Sprintf("script-exec:%d", executionID)))
}

// resolveRunAsUser resolves the run-as type to the actual username from the server
// - "root" → server.RootUsername() (e.g., "root" or provider-specific root user)
// - "local" → server.GetUsername() (e.g., "launch" or custom username)
func resolveRunAsUser(runAs *enums.RunAsUser, server *servermodels.Server) string {
	if runAs == nil {
		return server.RootUsername()
	}

	switch *runAs {
	case enums.RunAsUserLocal:
		return server.GetUsername()
	case enums.RunAsUserRoot:
		return server.RootUsername()
	default:
		return server.RootUsername()
	}
}
