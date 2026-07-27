package dto

import "time"

// ActiveAction is a navigation-independent representation of work currently
// running for a team. The client uses the target fields to reopen its logs.
type ActiveAction struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Label       string     `json:"label"`
	Description string     `json:"description,omitempty"`
	ServerID    string     `json:"server_id"`
	ProjectID   string     `json:"project_id,omitempty"`
	TargetType  string     `json:"target_type"`
	TargetID    string     `json:"target_id"`
	TaskID      *string    `json:"task_id,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
