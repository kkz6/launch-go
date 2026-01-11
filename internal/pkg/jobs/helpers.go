package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// UnmarshalPayload unmarshals asynq task payload with generics
// Usage: payload, err := jobs.UnmarshalPayload[MyPayload](t)
func UnmarshalPayload[T any](t *asynq.Task) (T, error) {
	var payload T
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return payload, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return payload, nil
}

// NewTask creates an asynq task from a typed payload
// Usage: task, err := jobs.NewTask("my:task", MyPayload{...})
func NewTask[T any](taskType string, payload T) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(taskType, data), nil
}

// MustNewTask creates an asynq task, panicking on error (for static payloads)
func MustNewTask[T any](taskType string, payload T) *asynq.Task {
	task, err := NewTask(taskType, payload)
	if err != nil {
		panic(err)
	}
	return task
}
