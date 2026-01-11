package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Cron represents a scheduled cron job on a server
type Cron struct {
	ID                        string     `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string     `gorm:"size:26;not null;index" json:"server_id"`
	SiteID                    *string    `gorm:"size:26;index" json:"site_id,omitempty"`
	User                      string     `gorm:"size:100;not null;default:'root'" json:"user"`
	Expression                string     `gorm:"size:100;not null" json:"expression"`
	Command                   string     `gorm:"type:text;not null" json:"command"`
	Frequency                 *string    `gorm:"size:100" json:"frequency,omitempty"`
	Hidden                    bool       `gorm:"default:false" json:"hidden"`
	InstalledAt               *time.Time `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `json:"-"`
	UninstallationFailedAt    *time.Time `json:"-"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
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

func (c *Cron) Path() string {
	return fmt.Sprintf("/etc/cron.d/cron-%s", c.ID)
}

func (c *Cron) LogPath(workingDirectory string) string {
	if c.User == "root" {
		return fmt.Sprintf("/root/%s/cron-%s.log", workingDirectory, c.ID)
	}

	return fmt.Sprintf("/home/%s/%s/cron-%s.log", c.User, workingDirectory, c.ID)
}
