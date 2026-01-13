package models

import (
	"fmt"

	"gorm.io/gorm"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Task represents a task execution record
type Task struct {
	basemodels.BaseModel
	ServerID string                     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name     string                     `gorm:"type:varchar(255);not null" json:"name"`
	User     string                     `gorm:"type:varchar(255);not null" json:"user"`
	Type     string                     `gorm:"type:varchar(255);not null" json:"type"`
	Instance *string                    `gorm:"type:longtext" json:"instance,omitempty"`
	Script   string                     `gorm:"type:longtext;not null" json:"-"`
	Timeout  int                        `gorm:"type:int;not null" json:"timeout"`
	Status   string                     `gorm:"type:varchar(255);not null" json:"status"`
	Output   basemodels.EncryptedString `gorm:"type:longtext" json:"-"`
	ExitCode *int                       `gorm:"column:exit_code;type:int" json:"exit_code,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if err := t.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if t.Status == "" {
		t.Status = "pending"
	}

	return nil
}

func (Task) TableName() string {
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

// GetLogPath returns the path to the task log file
// Requires Server to be preloaded to determine root username
func (t *Task) GetLogPath() string {
	rootUser := "root"
	if t.Server != nil {
		rootUser = t.Server.RootUsername()
	}

	var scriptPath string
	if t.User == rootUser {
		// Use root's script path
		if rootUser == "root" {
			scriptPath = "/root/.launch-tasks"
		} else {
			scriptPath = fmt.Sprintf("/%s/.launch-tasks", rootUser)
		}
	} else {
		// Use the task user's script path
		if t.User == "root" || t.User == "ubuntu" {
			scriptPath = fmt.Sprintf("/%s/.launch-tasks", t.User)
		} else {
			scriptPath = fmt.Sprintf("/home/%s/.launch-tasks", t.User)
		}
	}

	return fmt.Sprintf("%s/task-%s.log", scriptPath, t.ID)
}
