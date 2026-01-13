package models

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Daemon represents a background process managed by supervisor
type Daemon struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID        string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	User            string     `gorm:"type:varchar(255);not null" json:"user"`
	Directory       *string    `gorm:"type:varchar(255)" json:"directory,omitempty"`
	Command         string     `gorm:"type:longtext;not null" json:"command"`
	Processes       int        `gorm:"type:int;not null;default:1" json:"processes"`
	StopWaitSeconds int        `gorm:"column:stop_wait_seconds;type:int;not null;default:10" json:"stop_wait_seconds"`
	StopSignal      string     `gorm:"column:stop_signal;type:varchar(255);not null" json:"stop_signal"`
	LastStatusCheck *time.Time `gorm:"column:last_status_check;type:timestamp null" json:"last_status_check,omitempty"`
	Running         bool       `gorm:"type:tinyint(1);not null;default:0" json:"running"`
	Info            *string    `gorm:"type:json" json:"-"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (d *Daemon) BeforeCreate(tx *gorm.DB) error {
	if err := d.BaseModel.BeforeCreate(tx); err != nil {
		return err
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

// ToSupervisorConfig generates the supervisor configuration file contents
func (d *Daemon) ToSupervisorConfig() string {
	dir := ""
	if d.Directory != nil {
		dir = *d.Directory
	}

	config := fmt.Sprintf(`[program:%s]
process_name=%%(program_name)s_%%(process_num)02d
command=%s
autostart=true
autorestart=true
stopasgroup=true
killasgroup=true
user=%s
numprocs=%d
redirect_stderr=true
stdout_logfile=%s
stderr_logfile=%s
stopwaitsecs=%d
stopsignal=%s
`, d.ProgramName(), d.Command, d.User, d.Processes, d.GetLogPath(), d.GetErrorLogPath(), d.StopWaitSeconds, d.StopSignal)

	if dir != "" {
		config += fmt.Sprintf("directory=%s\n", dir)
	}

	return config
}
