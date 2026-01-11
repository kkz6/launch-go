package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Task represents a task execution record
type Task struct {
	ID         string     `gorm:"primaryKey;size:26" json:"id"`
	ServerID   string     `gorm:"size:26;not null;index" json:"server_id"`
	Type       string     `gorm:"size:255;not null" json:"type"`
	Status     string     `gorm:"size:50;default:'pending'" json:"status"`
	Name       *string    `gorm:"size:255" json:"name,omitempty"`
	User       *string    `gorm:"size:100" json:"user,omitempty"`
	Script     *string    `gorm:"type:text" json:"-"`
	Output     *string    `gorm:"type:longtext" json:"output,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
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

func (t *Task) Duration() time.Duration {
	if t.StartedAt == nil || t.FinishedAt == nil {
		return 0
	}

	return t.FinishedAt.Sub(*t.StartedAt)
}
