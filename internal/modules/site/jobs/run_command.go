package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRunCommand = "site:run_command"

// RunCommandPayload holds data for running a command on a site
type RunCommandPayload struct {
	SiteID    string `json:"site_id"`
	CommandID string `json:"command_id"`
}

// RunCommandJob handles running commands on a site
type RunCommandJob struct {
	ctx     *JobContext
	Payload RunCommandPayload
}

// NewRunCommandJob creates a new RunCommandJob instance
func NewRunCommandJob(ctx *JobContext, payload RunCommandPayload) *RunCommandJob {
	return &RunCommandJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the run command job
func (j *RunCommandJob) Handle(ctx context.Context) error {
	// Get the command from database (with user preloaded)
	command, err := j.ctx.CommandRepo.FindByID(ctx, j.Payload.CommandID)
	if err != nil {
		return fmt.Errorf("failed to find command: %w", err)
	}

	// Get the site with server info
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get the server for SSH connection
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update command status to running
	command.Status = enums.CommandStatusRunning
	if err := j.ctx.CommandRepo.UpdateFields(ctx, command.ID, map[string]any{
		"status": enums.CommandStatusRunning,
	}); err != nil {
		j.ctx.LogError(err, "Failed to update command status", "command_id", command.ID)
	}

	// Broadcast that command is running with full command data
	j.ctx.BroadcastServerEvent(server, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})

	// Create the task
	task := tasks.RunCommand(tasks.RunCommandConfig{
		Command: command.Command,
		Site:    site,
	})

	// Run the task on the server as the site user
	result, err := j.ctx.RunTaskOnServer(server, task).
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
	if updateErr := j.ctx.CommandRepo.UpdateFields(ctx, command.ID, map[string]any{
		"status":    command.Status,
		"output":    command.Output,
		"exit_code": command.ExitCode,
	}); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update command result", "command_id", command.ID)
	}

	// Broadcast completion with full command data
	j.ctx.BroadcastServerEvent(server, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})

	j.ctx.LogInfo("Command executed",
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
	j.ctx.LogError(err, "Failed to run command",
		"site_id", j.Payload.SiteID,
		"command_id", j.Payload.CommandID,
	)

	errMsg := err.Error()

	// Update command status to failed
	if updateErr := j.ctx.CommandRepo.UpdateFields(ctx, j.Payload.CommandID, map[string]any{
		"status": enums.CommandStatusFailed,
		"output": errMsg,
	}); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update command status on failure")
	}

	// Get the updated command to broadcast
	command, findErr := j.ctx.CommandRepo.FindByID(ctx, j.Payload.CommandID)
	if findErr != nil {
		j.ctx.LogError(findErr, "Failed to find command for broadcast")
		return
	}

	// Get site and server for broadcasting
	site, siteErr := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if siteErr != nil {
		j.ctx.LogError(siteErr, "Failed to find site for broadcast")
		return
	}

	server, serverErr := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if serverErr != nil {
		j.ctx.LogError(serverErr, "Failed to find server for broadcast")
		return
	}

	// Broadcast failure with full command data
	j.ctx.BroadcastServerEvent(server, "command.updated", map[string]any{
		"command": dto.ToCommandResponse(command),
	})
}

// NewRunCommandTask creates a run command job
func NewRunCommandTask(siteID, commandID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeRunCommand, RunCommandPayload{
		SiteID:    siteID,
		CommandID: commandID,
	})
}
