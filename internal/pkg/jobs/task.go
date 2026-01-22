package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// DefaultMaxRetry is the default number of retries for jobs (0 = no retry, run only once)
const DefaultMaxRetry = 0

// Task creates an asynq task with the given type and payload.
// Uses DefaultMaxRetry (0) by default - jobs run once without retry.
func Task[P any](jobType string, payload P, opts ...asynq.Option) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	opts = append([]asynq.Option{asynq.MaxRetry(DefaultMaxRetry)}, opts...)
	return asynq.NewTask(jobType, data, opts...), nil
}

// TaskWithID creates a task with a specific ID for deduplication.
// Tasks with the same ID will be deduplicated by the queue.
func TaskWithID[P any](jobType string, payload P, taskID string, opts ...asynq.Option) (*asynq.Task, error) {
	opts = append(opts, asynq.TaskID(taskID))
	return Task(jobType, payload, opts...)
}

// Dedup creates a deduplication ID from parts: "prefix:part1:part2:..."
func Dedup(prefix string, parts ...string) string {
	id := prefix
	for _, p := range parts {
		id += ":" + p
	}
	return id
}

// UnmarshalPayload unmarshals an asynq task payload into a typed struct.
func UnmarshalPayload[T any](t *asynq.Task) (T, error) {
	var payload T
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return payload, fmt.Errorf("unmarshal payload: %w", err)
	}
	return payload, nil
}
