package models

import "time"

// ServerRestart retains the latest restart outcome across page loads and worker restarts.
type ServerRestart struct {
	ServerID    string     `gorm:"primaryKey" json:"-"`
	OperationID string     `json:"-"`
	Status      string     `json:"status"`
	BootID      string     `json:"-"`
	RequestedAt *time.Time `json:"requested_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DeadlineAt  *time.Time `json:"-"`
	CheckAfter  *time.Time `json:"-"`
	CanVerify   bool       `gorm:"-" json:"can_verify"`
	Message     string     `json:"message,omitempty"`
}

func (ServerRestart) TableName() string { return "server_restarts" }

func (r ServerRestart) Active() bool {
	return r.Status == "queued" || r.Status == "restarting" || r.Status == "verifying"
}
