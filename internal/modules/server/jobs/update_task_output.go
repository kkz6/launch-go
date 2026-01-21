package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

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
	pkgjobs.BaseJob[*JobContext, UpdateTaskOutputPayload]
}

// NewUpdateTaskOutputJob creates a new UpdateTaskOutputJob
func NewUpdateTaskOutputJob(ctx *JobContext, payload UpdateTaskOutputPayload) *UpdateTaskOutputJob {
	return &UpdateTaskOutputJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the update task output job
func (j *UpdateTaskOutputJob) Handle(ctx context.Context) error {
	task, err := j.Ctx.Repos.Task().FindByID(ctx, j.Payload.TaskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	j.Ctx.LogInfo("Updating task output",
		"task_id", task.ID,
		"server_id", j.Payload.ServerID,
	)

	var newOutput string
	if j.Payload.Append && !task.Output.IsEmpty() {
		// Append to existing output
		newOutput = task.Output.String() + j.Payload.Output
	} else {
		// Replace output
		newOutput = j.Payload.Output
	}

	// Update the task output
	if err := j.Ctx.Repos.Task().UpdateOutput(ctx, task.ID, newOutput); err != nil {
		return fmt.Errorf("failed to update task output: %w", err)
	}

	j.Ctx.LogInfo("Task output updated",
		"task_id", task.ID,
	)

	return nil
}

// Failed handles job failure
func (j *UpdateTaskOutputJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to update task output",
		"task_id", j.Payload.TaskID,
	)
}

// NewUpdateTaskOutputTask creates an update task output task
func NewUpdateTaskOutputTask(taskID, serverID, output string, shouldAppend bool) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateTaskOutput, UpdateTaskOutputPayload{
		TaskID:   taskID,
		ServerID: serverID,
		Output:   output,
		Append:   shouldAppend,
	})
}
