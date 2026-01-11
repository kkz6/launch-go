package models

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Daemon represents a background process managed by supervisor
type Daemon struct {
	ID                        string     `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string     `gorm:"size:26;not null;index" json:"server_id"`
	User                      string     `gorm:"size:100;not null;default:'root'" json:"user"`
	Directory                 *string    `gorm:"size:255" json:"directory,omitempty"`
	Command                   string     `gorm:"type:text;not null" json:"command"`
	Processes                 int        `gorm:"default:1" json:"processes"`
	StopWaitSeconds           int        `gorm:"default:10" json:"stop_wait_seconds"`
	StopSignal                *string    `gorm:"size:20" json:"stop_signal,omitempty"`
	InstalledAt               *time.Time `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `json:"-"`
	UninstallationFailedAt    *time.Time `json:"-"`
	LastStatusCheck           *time.Time `json:"last_status_check,omitempty"`
	Running                   bool       `gorm:"default:false" json:"running"`
	Info                      *string    `gorm:"type:json" json:"-"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (d *Daemon) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}

	if d.User == "" {
		d.User = "root"
	}

	if d.Processes == 0 {
		d.Processes = 1
	}

	if d.StopWaitSeconds == 0 {
		d.StopWaitSeconds = 10
	}

	return nil
}

func (d *Daemon) TableName() string {
	return "daemons"
}

func (d *Daemon) IsInstalled() bool {
	return d.InstalledAt != nil
}

func (d *Daemon) Path() string {
	return fmt.Sprintf("/etc/supervisor/conf.d/daemon-%s.conf", d.ID)
}

func (d *Daemon) GetInfo() map[string]interface{} {
	if d.Info == nil {
		return nil
	}

	var info map[string]interface{}
	if err := json.Unmarshal([]byte(*d.Info), &info); err != nil {
		return nil
	}

	return info
}

func (d *Daemon) SetInfo(info map[string]interface{}) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	str := string(data)
	d.Info = &str

	return nil
}
