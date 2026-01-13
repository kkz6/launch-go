package taskrunner

import "encoding/json"

// TaskWithCompletion is implemented by tasks that need to dispatch
// a job when completed. This is the simple approach - just define
// what job should run on completion.
//
// Usage:
//
//	type InstallPhpTask struct {
//	    BaseTask
//	    ServerID   string
//	    PhpVersion string
//	}
//
//	func (t *InstallPhpTask) CompletionJobType() string {
//	    return "server:php-installed"
//	}
//
//	func (t *InstallPhpTask) CompletionPayload() interface{} {
//	    return map[string]string{
//	        "server_id": t.ServerID,
//	        "php_version": t.PhpVersion,
//	    }
//	}
//
// Then when running:
//
//	runner.NewRunner(server, task).
//	    AsRoot().
//	    TrackInBackground().  // Will auto-detect CompletionJobType
//	    Dispatch(ctx)
type TaskWithCompletion interface {
	Task

	// CompletionJobType returns the asynq job type to dispatch on completion.
	// Return empty string to skip job dispatch.
	CompletionJobType() string

	// CompletionPayload returns the payload for the completion job.
	// This will be JSON serialized.
	CompletionPayload() interface{}
}

// CompletionConfig stores what to do when a task completes.
// This is serialized to the task's Instance field.
type CompletionConfig struct {
	// OnFinished is the job to dispatch when task finishes successfully
	OnFinished *JobRef `json:"on_finished,omitempty"`

	// OnFailed is the job to dispatch when task fails
	OnFailed *JobRef `json:"on_failed,omitempty"`

	// OnTimeout is the job to dispatch when task times out
	OnTimeout *JobRef `json:"on_timeout,omitempty"`
}

// JobRef references an asynq job to dispatch
type JobRef struct {
	Type    string          `json:"type"`    // Asynq job type (e.g., "server:php-installed")
	Payload json.RawMessage `json:"payload"` // JSON payload for the job
}

// NewJobRef creates a JobRef from a type and payload
func NewJobRef(jobType string, payload interface{}) (*JobRef, error) {
	if jobType == "" {
		return nil, nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &JobRef{
		Type:    jobType,
		Payload: data,
	}, nil
}

// MarshalCompletionConfig serializes completion config for storage
func MarshalCompletionConfig(config *CompletionConfig) (string, error) {
	if config == nil {
		return "", nil
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// UnmarshalCompletionConfig deserializes completion config from storage
func UnmarshalCompletionConfig(data string) (*CompletionConfig, error) {
	if data == "" {
		return nil, nil
	}

	var config CompletionConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ExtractCompletionConfig extracts completion config from a task
// if it implements TaskWithCompletion
func ExtractCompletionConfig(task Task) (*CompletionConfig, error) {
	completionTask, ok := task.(TaskWithCompletion)
	if !ok {
		return nil, nil
	}

	jobType := completionTask.CompletionJobType()
	if jobType == "" {
		return nil, nil
	}

	jobRef, err := NewJobRef(jobType, completionTask.CompletionPayload())
	if err != nil {
		return nil, err
	}

	// By default, only dispatch on success
	// Tasks can override by implementing more specific interfaces
	return &CompletionConfig{
		OnFinished: jobRef,
	}, nil
}
