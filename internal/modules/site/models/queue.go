package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Queue represents a supervisor queue worker for a site
type Queue struct {
	ID                        string     `gorm:"type:char(26);primaryKey" json:"id"`
	SiteID                    string     `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	UserID                    string     `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Directory                 *string    `gorm:"type:varchar(255)" json:"directory,omitempty"`
	Command                   string     `gorm:"type:text;not null" json:"command"`
	User                      string     `gorm:"type:varchar(255);not null" json:"user"`
	AutoStart                 bool       `gorm:"column:auto_start;default:true" json:"auto_start"`
	AutoRestart               bool       `gorm:"column:auto_restart;default:true" json:"auto_restart"`
	NumProcs                  int        `gorm:"column:numprocs;default:8" json:"numprocs"`
	RedirectStderr            bool       `gorm:"column:redirect_stderr;default:true" json:"redirect_stderr"`
	StopWaitSeconds           int        `gorm:"column:stop_wait_seconds;default:10" json:"stop_wait_seconds"`
	StopSignal                string     `gorm:"column:stop_signal;type:varchar(255);not null" json:"stop_signal"`
	QueueConnection           string     `gorm:"column:queue_connection;type:varchar(255);not null" json:"queue_connection"`
	Environment               *string    `gorm:"type:varchar(255)" json:"environment,omitempty"`
	QueueName                 string     `gorm:"column:queue;type:varchar(255);not null" json:"queue"`
	MaxSecondsPerJob          *int       `gorm:"column:max_seconds_per_job" json:"max_seconds_per_job,omitempty"`
	MaxTries                  *int       `gorm:"column:max_tries" json:"max_tries,omitempty"`
	RestSecondsOnEmpty        *int       `gorm:"column:rest_seconds_on_empty" json:"rest_seconds_on_empty,omitempty"`
	FailedJobDelaySeconds     *int       `gorm:"column:failed_job_delay_seconds" json:"failed_job_delay_seconds,omitempty"`
	MaxMemory                 *int       `gorm:"column:max_memory" json:"max_memory,omitempty"`
	RunOnMaintenance          bool       `gorm:"column:run_on_maintenance;default:false" json:"run_on_maintenance"`
	RunWithListen             bool       `gorm:"column:run_with_listen;default:false" json:"run_with_listen"`
	LastStatusCheck           *time.Time `gorm:"column:last_status_check;type:timestamp null" json:"last_status_check,omitempty"`
	Running                   bool       `gorm:"default:false" json:"running"`
	Info                      *string    `gorm:"type:json" json:"info,omitempty"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"-"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
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

	if q.MaxTries != nil && *q.MaxTries > 0 {
		cmd += fmt.Sprintf(" --tries=%d", *q.MaxTries)
	}

	if q.Environment != nil && *q.Environment != "" {
		cmd += fmt.Sprintf(" --env=%s", *q.Environment)
	}

	if q.RestSecondsOnEmpty != nil {
		cmd += fmt.Sprintf(" --sleep=%d", *q.RestSecondsOnEmpty)
	}

	if q.MaxSecondsPerJob != nil {
		cmd += fmt.Sprintf(" --timeout=%d", *q.MaxSecondsPerJob)
	}

	if q.FailedJobDelaySeconds != nil && *q.FailedJobDelaySeconds > 0 {
		cmd += fmt.Sprintf(" --backoff=%d", *q.FailedJobDelaySeconds)
	}

	if q.RunOnMaintenance {
		cmd += " --force"
	}

	if q.MaxMemory != nil && *q.MaxMemory > 0 {
		cmd += fmt.Sprintf(" --memory=%d", *q.MaxMemory)
	}

	return cmd
}

// IsInstalled returns true if the queue is installed
func (q *Queue) IsInstalled() bool {
	return q.InstalledAt != nil
}
