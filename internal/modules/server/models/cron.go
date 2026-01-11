package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Cron represents a scheduled cron job on a server
type Cron struct {
	ID                        string     `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	SiteID                    *string    `gorm:"column:site_id;type:char(26);index" json:"site_id,omitempty"`
	User                      string     `gorm:"type:varchar(255);not null" json:"user"`
	Expression                string     `gorm:"type:varchar(255);not null" json:"expression"`
	Command                   string     `gorm:"type:longtext;not null" json:"command"`
	Frequency                 string     `gorm:"type:varchar(255);not null" json:"frequency"`
	Hidden                    bool       `gorm:"type:tinyint(1);not null;default:0" json:"hidden"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"-"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (c *Cron) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = utils.NewULID()
	}

	if c.User == "" {
		c.User = "root"
	}

	return nil
}

func (c *Cron) TableName() string {
	return "crons"
}

func (c *Cron) IsInstalled() bool {
	return c.InstalledAt != nil
}

// Path returns the path to the cron file
func (c *Cron) Path() string {
	return fmt.Sprintf("/etc/cron.d/cron-%s", c.ID)
}

// GetLogPath returns the path to the cron log file
func (c *Cron) GetLogPath() string {
	if c.User == "root" {
		return fmt.Sprintf("/root/cron-%s.log", c.ID)
	}
	return fmt.Sprintf("/home/%s/cron-%s.log", c.User, c.ID)
}
