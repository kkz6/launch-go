package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
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
	Deps    *JobDeps
	Payload RunCommandPayload

	// Model fields for Failed() callback
	site    *models.Site
	server  *servermodels.Server
	command *models.Command
}

func NewRunCommandJob(p RunCommandPayload) pkgjobs.Handler {
	return &RunCommandJob{Deps: deps, Payload: p}
}

// Handle executes the run command job
func (j *RunCommandJob) Handle(ctx context.Context) error {
	var err error

	// Get the command from database (with user preloaded)
	j.command, err = j.Deps.Repos.Command().FindByID(ctx, j.Payload.CommandID)
	if err != nil {
		return fmt.Errorf("failed to find command: %w", err)
	}

	// Get the site with server info
	j.site, err = j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get the server for SSH connection
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update command status to running
	j.command.Status = sitetypes.CommandStatusRunning
	if err := j.Deps.Repos.Command().UpdateFields(ctx, j.command.ID, map[string]any{
		"status": sitetypes.CommandStatusRunning,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Str("command_id", j.command.ID).Msg("Failed to update command status")
	}

	j.broadcastCommandUpdate()

	// Create the task
	task := tasks.RunCommand(tasks.RunCommandConfig{
		Command: j.command.Command,
		Site:    j.site,
	})

	// Run the task on the server as the site user
	result, err := j.Deps.RunTask(j.server, task).
		AsUser(j.site.User).
		Dispatch(ctx)

	// Update command with result
	if result != nil {
		output := result.GetOutput()
		exitCode := result.GetExitCode()
		j.command.Output = &output
		j.command.ExitCode = &exitCode

		if result.IsSuccessful() {
			j.command.Status = sitetypes.CommandStatusFinished
		} else {
			j.command.Status = sitetypes.CommandStatusFailed
		}
	} else if err != nil {
		j.command.Status = sitetypes.CommandStatusFailed
		errMsg := err.Error()
		j.command.Output = &errMsg
	}

	// Persist the updates
	if updateErr := j.Deps.Repos.Command().UpdateFields(ctx, j.command.ID, map[string]any{
		"status":    j.command.Status,
		"output":    j.command.Output,
		"exit_code": j.command.ExitCode,
	}); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Str("command_id", j.command.ID).Msg("Failed to update command result")
	}

	j.broadcastCommandUpdate()

	j.Deps.Logger.Info().
		Str("site_id", j.site.ID).
		Str("command_id", j.command.ID).
		Str("status", string(j.command.Status)).
		Msg("Command executed")

	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

// Failed handles job failure
func (j *RunCommandJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("command_id", j.Payload.CommandID).
		Msg("Failed to run command")

	errMsg := err.Error()

	// Update command status to failed
	if updateErr := j.Deps.Repos.Command().UpdateFields(ctx, j.Payload.CommandID, map[string]any{
		"status": sitetypes.CommandStatusFailed,
		"output": errMsg,
	}); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update command status on failure")
	}

	// Get the updated command to broadcast
	command, findErr := j.Deps.Repos.Command().FindByID(ctx, j.Payload.CommandID)
	if findErr != nil {
		j.Deps.Logger.Error().Err(findErr).Msg("Failed to find command for broadcast")
		return
	}

	j.command = command
	j.broadcastCommandUpdate()
}

func (j *RunCommandJob) broadcastCommandUpdate() {
	if j.command == nil {
		return
	}

	j.Deps.BroadcastToTeam(j.command.TeamID, "command.updated", commandEventData(j.command))
}

func commandEventData(command *models.Command) map[string]any {
	return map[string]any{
		"command_id": command.ID,
		"site_id":    command.SiteID,
		"status":     string(command.Status),
		"command":    dto.ToCommandResponse(command),
	}
}

// NewRunCommandTask creates a run command job
func NewRunCommandTask(siteID, commandID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRunCommand, RunCommandPayload{
		SiteID:    siteID,
		CommandID: commandID,
	})
}
