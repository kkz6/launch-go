package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

// TrackTaskInBackground wraps a task to run in the background with callback URLs
type TrackTaskInBackground struct {
	taskrunner.BaseTask
	actualTask  taskrunner.Task
	finishedURL string
	failedURL   string
	timeoutURL  string
	eof         string
}

// NewTrackTaskInBackground creates a new TrackTaskInBackground task
func NewTrackTaskInBackground(actualTask taskrunner.Task, finishedURL, failedURL, timeoutURL string) *TrackTaskInBackground {
	task := &TrackTaskInBackground{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "track-task-in-background",
			TemplateName: "server/track-task-in-background",
			TaskTimeout:  actualTask.Timeout() + 30*time.Second,
		},
		actualTask:  actualTask,
		finishedURL: finishedURL,
		failedURL:   failedURL,
		timeoutURL:  timeoutURL,
		eof:         generateEOF(),
	}

	return task
}

// Data returns the template data
func (t *TrackTaskInBackground) Data() map[string]interface{} {
	actualScript, _ := t.actualTask.Script()

	return map[string]interface{}{
		"ActualTask":        t.actualTask,
		"ActualScript":      actualScript,
		"ActualTaskTimeout": int(t.actualTask.Timeout().Seconds()),
		"FinishedURL":       t.finishedURL,
		"FailedURL":         t.failedURL,
		"TimeoutURL":        t.timeoutURL,
		"EOF":               t.eof,
		"HasCallback":       t.actualTask.(*taskrunner.BaseTask).GetCallbackURL() != "",
	}
}

// ActualTask returns the wrapped task
func (t *TrackTaskInBackground) ActualTask() taskrunner.Task {
	return t.actualTask
}

// FinishedURL returns the callback URL for successful completion
func (t *TrackTaskInBackground) FinishedURL() string {
	return t.finishedURL
}

// FailedURL returns the callback URL for failure
func (t *TrackTaskInBackground) FailedURL() string {
	return t.failedURL
}

// TimeoutURL returns the callback URL for timeout
func (t *TrackTaskInBackground) TimeoutURL() string {
	return t.timeoutURL
}

// EOF returns the unique end-of-file marker
func (t *TrackTaskInBackground) EOF() string {
	return t.eof
}

// Timeout returns the timeout including additional time for background processing
func (t *TrackTaskInBackground) Timeout() time.Duration {
	return t.actualTask.Timeout() + 30*time.Second
}

// generateEOF generates a unique EOF marker
func generateEOF() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "LAUNCH-TASK-RUNNER-DEFAULT"
	}

	return "LAUNCH-TASK-RUNNER-" + strings.ToUpper(hex.EncodeToString(bytes))
}
