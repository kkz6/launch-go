package models

import (
	"fmt"

	"gorm.io/gorm"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Task represents a task execution record
type Task struct {
	basemodels.BaseModel
	basemodels.ServerScoped
	Name     string                 `gorm:"type:varchar(255);not null" json:"name"`
	User     string                 `gorm:"type:varchar(255);not null" json:"user"`
	Type     string                 `gorm:"type:varchar(255);not null" json:"type"`
	Instance dbtype.EncryptedString `gorm:"type:longtext" json:"instance,omitempty"`
	Script   dbtype.EncryptedString `gorm:"type:longtext" json:"-"`
	Timeout  int                    `gorm:"type:int;not null" json:"timeout"`
	Status   string                 `gorm:"type:varchar(255);not null" json:"status"`
	Output   dbtype.EncryptedString `gorm:"type:longtext" json:"-"`
	ExitCode *int                   `gorm:"column:exit_code;type:int" json:"exit_code,omitempty"`

	// SiteID scopes a task to a site on top of its server, so site-scoped
	// work (a PHP runtime switch, say) can be routed to the site in the UI
	// instead of showing as a bare server action.
	SiteID *string `gorm:"column:site_id;type:char(26);index" json:"site_id,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if err := t.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	basemodels.SetDefaultStatus(&t.Status, string(servertypes.TaskStatusPending))

	return nil
}

func (Task) TableName() string {
	return "tasks"
}

func (t *Task) IsSuccessful() bool {
	return t.ExitCode != nil && *t.ExitCode == 0
}

func (t *Task) IsPending() bool {
	return t.Status == string(servertypes.TaskStatusPending)
}

func (t *Task) IsRunning() bool {
	return t.Status == string(servertypes.TaskStatusRunning)
}

func (t *Task) IsFinished() bool {
	return t.Status == string(servertypes.TaskStatusFinished) || t.Status == string(servertypes.TaskStatusFailed)
}

// BroadcastData returns the standard broadcast payload for task events.
func (t *Task) BroadcastData(output string) map[string]interface{} {
	data := map[string]interface{}{
		"task_id":   t.ID,
		"server_id": t.ServerID,
		"name":      t.Name,
		"status":    t.Status,
		"user":      t.User,
	}
	if t.ExitCode != nil {
		data["exit_code"] = *t.ExitCode
	}
	if output != "" {
		data["output"] = output
	}
	return data
}

// GetLogPath returns the path to the task log file.
func (t *Task) GetLogPath() string {
	server := t.Server
	if server == nil {
		server = &Server{}
	}

	return fmt.Sprintf("%s/task-%s.log", server.GetScriptPath(t.User), t.ID)
}
