package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// TaskBuilder provides a fluent API for building asynq tasks with type-safe payloads.
// It reduces boilerplate by allowing pre-configuration of task options.
//
// Usage:
//
//	// Define a builder for a task type
//	var DeployBuilder = jobs.NewTaskBuilder[DeployPayload](TypeDeploy)
//
//	// Build tasks with options
//	task, err := DeployBuilder.
//	    WithDeduplication("deploy", deploymentID).
//	    Build(DeployPayload{SiteID: siteID, DeploymentID: deploymentID})
type TaskBuilder[P any] struct {
	taskType string
	opts     []asynq.Option
}

// NewTaskBuilder creates a new TaskBuilder for the given task type.
// The generic parameter P specifies the payload type.
func NewTaskBuilder[P any](taskType string) *TaskBuilder[P] {
	return &TaskBuilder[P]{
		taskType: taskType,
		opts:     []asynq.Option{asynq.MaxRetry(DefaultMaxRetry)},
	}
}

// WithTaskID adds a task ID for deduplication.
// Tasks with the same ID will be deduplicated by the queue.
func (b *TaskBuilder[P]) WithTaskID(id string) *TaskBuilder[P] {
	return &TaskBuilder[P]{
		taskType: b.taskType,
		opts:     append(b.opts, asynq.TaskID(id)),
	}
}

// WithDeduplication adds a formatted task ID for deduplication.
// The ID is formatted as "prefix:ids[0]:ids[1]:..."
func (b *TaskBuilder[P]) WithDeduplication(prefix string, ids ...string) *TaskBuilder[P] {
	id := prefix
	for _, part := range ids {
		id += ":" + part
	}
	return b.WithTaskID(id)
}

// WithRetry sets the maximum number of retries for the task.
func (b *TaskBuilder[P]) WithRetry(n int) *TaskBuilder[P] {
	return &TaskBuilder[P]{
		taskType: b.taskType,
		opts:     append(b.opts, asynq.MaxRetry(n)),
	}
}

// WithQueue specifies which queue the task should be enqueued to.
func (b *TaskBuilder[P]) WithQueue(queue string) *TaskBuilder[P] {
	return &TaskBuilder[P]{
		taskType: b.taskType,
		opts:     append(b.opts, asynq.Queue(queue)),
	}
}

// WithOptions adds additional asynq options to the task.
func (b *TaskBuilder[P]) WithOptions(opts ...asynq.Option) *TaskBuilder[P] {
	return &TaskBuilder[P]{
		taskType: b.taskType,
		opts:     append(b.opts, opts...),
	}
}

// Build creates an asynq.Task with the given payload.
func (b *TaskBuilder[P]) Build(payload P) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(b.taskType, data, b.opts...), nil
}

// MustBuild creates an asynq.Task, panicking on error.
// Use only when the payload is known to be valid at compile time.
func (b *TaskBuilder[P]) MustBuild(payload P) *asynq.Task {
	task, err := b.Build(payload)
	if err != nil {
		panic(err)
	}
	return task
}

// TaskType returns the task type string.
func (b *TaskBuilder[P]) TaskType() string {
	return b.taskType
}
