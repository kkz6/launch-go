package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Queue represents a supervisor queue worker for a site
type Queue struct {
	ID                        string         `gorm:"primaryKey;size:26" json:"id"`
	SiteID                    string         `gorm:"size:26;not null;index" json:"site_id"`
	ServerID                  string         `gorm:"size:26;not null;index" json:"server_id"`
	UserID                    string         `gorm:"size:26;not null;index" json:"user_id"`
	Name                      string         `gorm:"size:255" json:"name"`
	Directory                 string         `gorm:"size:500" json:"directory"`
	Command                   string         `gorm:"type:text" json:"command"`
	User                      string         `gorm:"size:100" json:"user"`
	AutoStart                 bool           `gorm:"default:true" json:"auto_start"`
	AutoRestart               bool           `gorm:"default:true" json:"auto_restart"`
	NumProcs                  int            `gorm:"default:1" json:"numprocs"`
	RedirectStderr            bool           `gorm:"default:true" json:"redirect_stderr"`
	StopWaitSeconds           int            `gorm:"default:10" json:"stop_wait_seconds"`
	StopSignal                string         `gorm:"size:20;default:'TERM'" json:"stop_signal"`
	QueueConnection           string         `gorm:"size:50;default:'database'" json:"queue_connection"`
	Environment               *string        `gorm:"size:100" json:"environment,omitempty"`
	QueueName                 string         `gorm:"size:100;default:'default'" json:"queue"`
	MaxSecondsPerJob          int            `gorm:"default:60" json:"max_seconds_per_job"`
	MaxTries                  int            `gorm:"default:3" json:"max_tries"`
	RestSecondsOnEmpty        int            `gorm:"default:10" json:"rest_seconds_on_empty"`
	FailedJobDelaySeconds     int            `gorm:"default:3" json:"failed_job_delay_seconds"`
	MaxMemory                 int            `gorm:"default:128" json:"max_memory"`
	RunOnMaintenance          bool           `gorm:"default:false" json:"run_on_maintenance"`
	RunWithListen             bool           `gorm:"default:false" json:"run_with_listen"`
	Running                   bool           `gorm:"default:false" json:"running"`
	Info                      string         `gorm:"type:json" json:"info,omitempty"`
	LastStatusCheck           *time.Time     `json:"last_status_check,omitempty"`
	InstalledAt               *time.Time     `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time     `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time     `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time     `json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 time.Time      `json:"created_at"`
	UpdatedAt                 time.Time      `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (q *Queue) TableName() string {
	return "queues"
}

func (q *Queue) BeforeCreate(tx *gorm.DB) error {
	if q.ID == "" {
		q.ID = utils.NewULID()
	}

	return nil
}

// GetPath returns the path to the supervisor config file
func (q *Queue) GetPath() string {
	return fmt.Sprintf("/etc/supervisor/conf.d/daemon-%s.conf", q.ID)
}

// ErrorLogPath returns the path to the error log file
func (q *Queue) ErrorLogPath(workingDirectory string) string {
	if q.User == "root" {
		return fmt.Sprintf("/root/%s/daemon-%s.err", workingDirectory, q.ID)
	}

	return fmt.Sprintf("/home/%s/%s/daemon-%s.err", q.User, workingDirectory, q.ID)
}

// OutputLogPath returns the path to the output log file
func (q *Queue) OutputLogPath(workingDirectory string) string {
	if q.User == "root" {
		return fmt.Sprintf("/root/%s/daemon-%s.log", workingDirectory, q.ID)
	}

	return fmt.Sprintf("/home/%s/%s/daemon-%s.log", q.User, workingDirectory, q.ID)
}

// BuildCommand builds the artisan queue command
func (q *Queue) BuildCommand() string {
	run := "work"
	if q.RunWithListen {
		run = "listen"
	}

	cmd := fmt.Sprintf("php artisan queue:%s %s --queue=%s", run, q.QueueConnection, q.QueueName)

	if q.MaxTries > 0 {
		cmd += fmt.Sprintf(" --tries=%d", q.MaxTries)
	}

	if q.Environment != nil && *q.Environment != "" {
		cmd += fmt.Sprintf(" --env=%s", *q.Environment)
	}

	cmd += fmt.Sprintf(" --sleep=%d", q.RestSecondsOnEmpty)
	cmd += fmt.Sprintf(" --timeout=%d", q.MaxSecondsPerJob)

	if q.FailedJobDelaySeconds > 0 {
		cmd += fmt.Sprintf(" --backoff=%d", q.FailedJobDelaySeconds)
	}

	if q.RunOnMaintenance {
		cmd += " --force"
	}

	if q.MaxMemory > 0 {
		cmd += fmt.Sprintf(" --memory=%d", q.MaxMemory)
	}

	return cmd
}

// IsInstalled returns true if the queue is installed
func (q *Queue) IsInstalled() bool {
	return q.InstalledAt != nil
}
