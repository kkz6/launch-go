package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// RunCommandJob handles running commands on a site
type RunCommandJob struct {
	SiteJobBase
	Payload RunCommandPayload
}

// Type returns the job type
func (j *RunCommandJob) Type() string {
	return TypeRunCommand
}

// Handle executes the run command job
func (j *RunCommandJob) Handle(ctx context.Context) error {
	// Get the command from database (with user preloaded)
	command, err := j.Ctx.CommandRepo.FindByID(ctx, j.Payload.CommandID)
	if err != nil {
		return fmt.Errorf("failed to find command: %w", err)
	}

	// Get the site with server info
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get the server for SSH connection
	server, err := j.Ctx.ServerRepo.FindServerByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update command status to running
	command.Status = enums.CommandStatusRunning
	if err := j.Ctx.CommandRepo.UpdateFields(ctx, command.ID, map[string]any{
		"status": enums.CommandStatusRunning,
	}); err != nil {
		j.LogError(err, "Failed to update command status", "command_id", command.ID)
	}

	// Broadcast that command is running with full command data
	j.BroadcastSiteEvent(site.ID, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})

	// Also broadcast to server channel for server-wide listeners
	j.BroadcastToServer(server.ID, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})

	// Create the task
	task := tasks.RunCommand(tasks.RunCommandConfig{
		Command: command.Command,
		Site:    site,
	})

	// Run the task on the server as the site user
	result, err := j.RunTaskOnServer(server, task).
		AsUser(site.User).
		Dispatch(ctx)

	// Update command with result
	if result != nil {
		output := result.GetOutput()
		exitCode := result.GetExitCode()
		command.Output = &output
		command.ExitCode = &exitCode

		if result.IsSuccessful() {
			command.Status = enums.CommandStatusFinished
		} else {
			command.Status = enums.CommandStatusFailed
		}
	} else if err != nil {
		command.Status = enums.CommandStatusFailed
		errMsg := err.Error()
		command.Output = &errMsg
	}

	// Persist the updates
	if updateErr := j.Ctx.CommandRepo.UpdateFields(ctx, command.ID, map[string]any{
		"status":    command.Status,
		"output":    command.Output,
		"exit_code": command.ExitCode,
	}); updateErr != nil {
		j.LogError(updateErr, "Failed to update command result", "command_id", command.ID)
	}

	// Broadcast completion with full command data
	j.BroadcastSiteEvent(site.ID, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})

	// Also broadcast to server channel
	j.BroadcastToServer(server.ID, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})

	j.LogInfo("Command executed",
		"site_id", site.ID,
		"command_id", command.ID,
		"status", command.Status,
	)

	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

// Failed handles job failure
func (j *RunCommandJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to run command",
		"site_id", j.Payload.SiteID,
		"command_id", j.Payload.CommandID,
	)

	errMsg := err.Error()

	// Update command status to failed
	if updateErr := j.Ctx.CommandRepo.UpdateFields(ctx, j.Payload.CommandID, map[string]any{
		"status": enums.CommandStatusFailed,
		"output": errMsg,
	}); updateErr != nil {
		j.LogError(updateErr, "Failed to update command status on failure")
	}

	// Get the updated command to broadcast
	command, findErr := j.Ctx.CommandRepo.FindByID(ctx, j.Payload.CommandID)
	if findErr != nil {
		j.LogError(findErr, "Failed to find command for broadcast")
		return
	}

	// Broadcast failure with full command data
	j.BroadcastSiteEvent(j.Payload.SiteID, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})
}

// NewRunCommandTaskFromAsynq creates a RunCommand asynq task (for backward compatibility)
func NewRunCommandTaskFromAsynq(siteID, commandID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRunCommand, RunCommandPayload{
		SiteID:    siteID,
		CommandID: commandID,
	})
}
