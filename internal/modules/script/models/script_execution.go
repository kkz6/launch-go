package models

import (
	"time"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// ExecutionStatus represents the status of a script execution
type ExecutionStatus string

const (
	ExecutionStatusPending  ExecutionStatus = "pending"
	ExecutionStatusRunning  ExecutionStatus = "running"
	ExecutionStatusFinished ExecutionStatus = "finished"
	ExecutionStatusFailed   ExecutionStatus = "failed"
)

// ScriptExecution represents a single execution of a script on a server
type ScriptExecution struct {
	ID         uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID   string          `gorm:"column:script_id;type:char(26);not null;index" json:"script_id"`
	ServerID   string          `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	BatchID    *string         `gorm:"column:batch_id;type:char(26);index" json:"batch_id,omitempty"`
	User       *string         `gorm:"type:varchar(255)" json:"user,omitempty"`
	Status     ExecutionStatus `gorm:"type:varchar(50);not null;default:pending" json:"status"`
	ExitCode   *int            `gorm:"column:exit_code" json:"exit_code,omitempty"`
	Output     *string         `gorm:"type:longtext" json:"output,omitempty"`
	StartedAt  *time.Time      `gorm:"column:started_at;type:timestamp null" json:"started_at,omitempty"`
	FinishedAt *time.Time      `gorm:"column:finished_at;type:timestamp null" json:"finished_at,omitempty"`
	CreatedAt  *time.Time      `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt  *time.Time      `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Script *Script               `gorm:"foreignKey:ScriptID;references:ID" json:"script,omitempty"`
	Server *servermodels.Server  `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (ScriptExecution) TableName() string {
	return "script_executions"
}

// IsComplete returns true if the execution has finished (success or failure)
func (e *ScriptExecution) IsComplete() bool {
	return e.Status == ExecutionStatusFinished || e.Status == ExecutionStatusFailed
}
