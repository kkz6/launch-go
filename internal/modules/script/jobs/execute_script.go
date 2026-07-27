package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/modules/script/support"
	"github.com/kkz6/launch-go/internal/modules/script/tasks"
	scripttypes "github.com/kkz6/launch-go/internal/modules/script/types"
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
	Deps    *JobDeps
	Payload ExecuteScriptPayload

	// Model fields for Failed() callback
	execution *models.ScriptExecution
	script    *models.Script
	server    *servermodels.Server
}

func NewExecuteScriptJob(p ExecuteScriptPayload) pkgjobs.Handler {
	return &ExecuteScriptJob{Deps: deps, Payload: p}
}

// Handle executes the script on the server
func (j *ExecuteScriptJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Uint64("execution_id", j.Payload.ExecutionID).
		Str("script_id", j.Payload.ScriptID).
		Str("server_id", j.Payload.ServerID).
		Msg("ExecuteScript job started")

	var err error

	// Get execution record
	j.execution, err = j.Deps.Repos.Execution().FindByID(ctx, j.Payload.ExecutionID)
	if err != nil {
		return fmt.Errorf("failed to find execution: %w", err)
	}

	// Get script
	j.script, err = j.Deps.Repos.Script().FindByID(ctx, j.Payload.ScriptID)
	if err != nil {
		return fmt.Errorf("failed to find script: %w", err)
	}

	// Get server
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update status to running
	now := time.Now()
	if err := j.Deps.Repos.Execution().UpdateFields(ctx, j.execution.ID, map[string]any{
		"status":     models.ExecutionStatusRunning,
		"started_at": now,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update execution status")
	}

	// Broadcast started event
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "script.execution.started", map[string]any{
		"execution_id": j.execution.ID,
		"batch_id":     j.execution.BatchID,
		"server_id":    j.server.ID,
		"script_id":    j.script.ID,
	})

	// Interpolate variables in the script content
	interpolatedContent := support.InterpolateVariables(j.script.Content, j.server)

	// Create the task
	task := tasks.RunScript(tasks.RunScriptConfig{
		Name:    "Run " + j.script.Name,
		Content: interpolatedContent,
	})

	// Resolve the run-as type to actual username from server
	runAsUser := resolveRunAsUser(j.execution.RunAs, j.server)

	// Execute the task on the server
	result, err := j.Deps.RunTask(j.server, task).
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
	if updateErr := j.Deps.Repos.Execution().UpdateFields(ctx, j.execution.ID, map[string]any{
		"status":      finalStatus,
		"exit_code":   exitCode,
		"output":      output,
		"finished_at": finishedAt,
	}); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update execution result")
	}

	// Broadcast completion event
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "script.execution.completed", map[string]any{
		"execution_id": j.execution.ID,
		"batch_id":     j.execution.BatchID,
		"server_id":    j.server.ID,
		"script_id":    j.script.ID,
		"status":       string(finalStatus),
		"exit_code":    exitCode,
	})

	j.Deps.Logger.Info().
		Uint64("execution_id", j.execution.ID).
		Str("status", string(finalStatus)).
		Msg("ExecuteScript job completed")

	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

// Failed handles job failure
func (j *ExecuteScriptJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Uint64("execution_id", j.Payload.ExecutionID).
		Str("script_id", j.Payload.ScriptID).
		Str("server_id", j.Payload.ServerID).
		Msg("ExecuteScript job failed")

	errMsg := err.Error()

	// Update execution status to failed
	if updateErr := j.Deps.Repos.Execution().UpdateFields(ctx, j.Payload.ExecutionID, map[string]any{
		"status":      models.ExecutionStatusFailed,
		"output":      errMsg,
		"finished_at": time.Now(),
	}); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update execution status on failure")
	}

	// Broadcast failure event
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "script.execution.completed", map[string]any{
		"execution_id": j.Payload.ExecutionID,
		"server_id":    j.Payload.ServerID,
		"script_id":    j.Payload.ScriptID,
		"status":       string(models.ExecutionStatusFailed),
		"error":        errMsg,
	})
}

// NewExecuteScriptTask creates an asynq task for executing a script
func NewExecuteScriptTask(executionID uint64, scriptID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeExecuteScript, ExecuteScriptPayload{
		ExecutionID: executionID,
		ScriptID:    scriptID,
		ServerID:    serverID,
		TeamID:      teamID,
	}, pkgjobs.Dedup("script-exec", fmt.Sprintf("%d", executionID)))
}

// resolveRunAsUser resolves the run-as type to the actual username from the server
// - "root" -> server.RootUsername() (e.g., "root" or provider-specific root user)
// - "local" -> server.GetUsername() (e.g., "launch" or custom username)
func resolveRunAsUser(runAs *scripttypes.RunAsUser, server *servermodels.Server) string {
	if runAs == nil {
		return server.RootUsername()
	}

	switch *runAs {
	case scripttypes.RunAsUserLocal:
		return server.GetUsername()
	case scripttypes.RunAsUserRoot:
		return server.RootUsername()
	default:
		return server.RootUsername()
	}
}
