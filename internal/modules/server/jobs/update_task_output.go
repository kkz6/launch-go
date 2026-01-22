package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateTaskOutput = "server:update_task_output"

// UpdateTaskOutputPayload holds data for task output update
type UpdateTaskOutputPayload struct {
	TaskID   string `json:"task_id"`
	ServerID string `json:"server_id"`
	Output   string `json:"output"`
	Append   bool   `json:"append"`
}

// UpdateTaskOutputJob updates the output of a running task in the database
// This allows streaming task output for long-running tasks
type UpdateTaskOutputJob struct {
	Deps    *JobDeps
	Payload UpdateTaskOutputPayload

	task *models.Task
}

func NewUpdateTaskOutputJob(p UpdateTaskOutputPayload) pkgjobs.Handler {
	return &UpdateTaskOutputJob{Deps: deps, Payload: p}
}

// Handle executes the update task output job
func (j *UpdateTaskOutputJob) Handle(ctx context.Context) error {
	var err error
	j.task, err = j.Deps.Repos.Task().FindByID(ctx, j.Payload.TaskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	j.Deps.Logger.Info().
		Str("task_id", j.task.ID).
		Str("server_id", j.Payload.ServerID).
		Msg("Updating task output")

	var newOutput string
	if j.Payload.Append && !j.task.Output.IsEmpty() {
		// Append to existing output
		newOutput = j.task.Output.String() + j.Payload.Output
	} else {
		// Replace output
		newOutput = j.Payload.Output
	}

	// Update the task output
	if err := j.Deps.Repos.Task().UpdateOutput(ctx, j.task.ID, newOutput); err != nil {
		return fmt.Errorf("failed to update task output: %w", err)
	}

	j.Deps.Logger.Info().
		Str("task_id", j.task.ID).
		Msg("Task output updated")

	return nil
}

// Failed handles job failure
func (j *UpdateTaskOutputJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("task_id", j.Payload.TaskID).
		Msg("Failed to update task output")
}

// NewUpdateTaskOutputTask creates an update task output task
func NewUpdateTaskOutputTask(taskID, serverID, output string, shouldAppend bool) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUpdateTaskOutput, UpdateTaskOutputPayload{
		TaskID:   taskID,
		ServerID: serverID,
		Output:   output,
		Append:   shouldAppend,
	})
}
