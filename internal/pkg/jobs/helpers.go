package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// DefaultMaxRetry is the default number of retries for jobs (0 = no retry, run only once)
const DefaultMaxRetry = 0

// UnmarshalPayload unmarshals asynq task payload with generics
// Usage: payload, err := jobs.UnmarshalPayload[MyPayload](t)
func UnmarshalPayload[T any](t *asynq.Task) (T, error) {
	var payload T
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return payload, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return payload, nil
}

// NewTask creates an asynq task from a typed payload with default max retry of 1
// Usage: task, err := jobs.NewTask("my:task", MyPayload{...})
func NewTask[T any](taskType string, payload T, opts ...asynq.Option) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	// Prepend default max retry, allowing callers to override
	opts = append([]asynq.Option{asynq.MaxRetry(DefaultMaxRetry)}, opts...)
	return asynq.NewTask(taskType, data, opts...), nil
}

// NewTaskWithRetry creates an asynq task with a specific max retry count
// Usage: task, err := jobs.NewTaskWithRetry("my:task", MyPayload{...}, 3)
func NewTaskWithRetry[T any](taskType string, payload T, maxRetry int, opts ...asynq.Option) (*asynq.Task, error) {
	opts = append([]asynq.Option{asynq.MaxRetry(maxRetry)}, opts...)
	return NewTask(taskType, payload, opts...)
}

// MustNewTask creates an asynq task, panicking on error (for static payloads)
func MustNewTask[T any](taskType string, payload T, opts ...asynq.Option) *asynq.Task {
	task, err := NewTask(taskType, payload, opts...)
	if err != nil {
		panic(err)
	}
	return task
}
