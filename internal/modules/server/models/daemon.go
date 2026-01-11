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
	ID                        string     `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	User                      string     `gorm:"type:varchar(255);not null" json:"user"`
	Directory                 *string    `gorm:"type:varchar(255)" json:"directory,omitempty"`
	Command                   string     `gorm:"type:longtext;not null" json:"command"`
	Processes                 int        `gorm:"type:int;not null;default:1" json:"processes"`
	StopWaitSeconds           int        `gorm:"column:stop_wait_seconds;type:int;not null;default:10" json:"stop_wait_seconds"`
	StopSignal                string     `gorm:"column:stop_signal;type:varchar(255);not null" json:"stop_signal"`
	LastStatusCheck           *time.Time `gorm:"column:last_status_check;type:timestamp null" json:"last_status_check,omitempty"`
	Running                   bool       `gorm:"type:tinyint(1);not null;default:0" json:"running"`
	Info                      *string    `gorm:"type:json" json:"-"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"-"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
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

// Path returns the path to the supervisor config file
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

// GetLogPath returns the path to the output log file
func (d *Daemon) GetLogPath() string {
	if d.User == "root" {
		return fmt.Sprintf("/root/daemon-%s.log", d.ID)
	}
	return fmt.Sprintf("/home/%s/daemon-%s.log", d.User, d.ID)
}

// GetErrorLogPath returns the path to the error log file
func (d *Daemon) GetErrorLogPath() string {
	if d.User == "root" {
		return fmt.Sprintf("/root/daemon-%s.err", d.ID)
	}
	return fmt.Sprintf("/home/%s/daemon-%s.err", d.User, d.ID)
}

// ProgramName returns the supervisor program name
func (d *Daemon) ProgramName() string {
	return fmt.Sprintf("daemon-%s", d.ID)
}
