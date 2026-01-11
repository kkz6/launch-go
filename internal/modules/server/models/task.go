package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Task represents a task execution record
type Task struct {
	ID        string     `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name      string     `gorm:"type:varchar(255);not null" json:"name"`
	User      string     `gorm:"type:varchar(255);not null" json:"user"`
	Type      string     `gorm:"type:varchar(255);not null" json:"type"`
	Instance  *string    `gorm:"type:longtext" json:"instance,omitempty"`
	Script    string     `gorm:"type:longtext;not null" json:"-"`
	Timeout   int        `gorm:"type:int;not null" json:"timeout"`
	Status    string     `gorm:"type:varchar(255);not null" json:"status"`
	Output    *string    `gorm:"type:longtext" json:"output,omitempty"`
	ExitCode  *int       `gorm:"column:exit_code;type:int" json:"exit_code,omitempty"`
	CreatedAt *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}

	if t.Status == "" {
		t.Status = "pending"
	}

	return nil
}

func (t *Task) TableName() string {
	return "tasks"
}

func (t *Task) IsSuccessful() bool {
	return t.ExitCode != nil && *t.ExitCode == 0
}

func (t *Task) IsPending() bool {
	return t.Status == "pending"
}

func (t *Task) IsRunning() bool {
	return t.Status == "running"
}

func (t *Task) IsFinished() bool {
	return t.Status == "finished" || t.Status == "failed"
}
