package taskrunner

import "time"

// TaskResult contains the execution result
type TaskResult struct {
	TaskID     string
	Output     string
	ExitCode   int
	Duration   time.Duration
	TimedOut   bool
	Error      error
	FinishedAt time.Time
}

// IsSuccessful returns true if the task completed successfully
func (r *TaskResult) IsSuccessful() bool {
	return r.ExitCode == 0 && !r.TimedOut && r.Error == nil
}

// IsFailed returns true if the task failed
func (r *TaskResult) IsFailed() bool {
	return r.ExitCode != 0 || r.Error != nil
}

// IsTimedOut returns true if the task timed out
func (r *TaskResult) IsTimedOut() bool {
	return r.TimedOut
}
