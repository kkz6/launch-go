package dto

import "time"

// AdminFailure is a single normalized failure surfaced by the back-office
// failures monitor. It folds provision failures, failed/timed-out tasks and
// failed deployments into one shape so the UI can render a unified feed.
type AdminFailure struct {
	Kind     string     `json:"kind"` // "provision" | "task" | "deployment"
	ID       string     `json:"id"`
	Title    string     `json:"title"`               // human label, e.g. server name / task type / site id
	TeamID   string     `json:"team_id,omitempty"`   //
	ServerID string     `json:"server_id,omitempty"` //
	When     *time.Time `json:"when,omitempty"`      // updated_at / created_at
	Error    string     `json:"error"`               // short error text
	Detail   string     `json:"detail,omitempty"`    // truncated output (~2KB max)
}

// AdminFailuresResponse is the response payload for GET /admin/failures.
type AdminFailuresResponse struct {
	Failures []AdminFailure `json:"failures"`
	Caveat   string         `json:"caveat"` // notes asynq job failures are not captured (Redis-only)
}
