package models

import (
	"fmt"

	"gorm.io/gorm"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Cron represents a scheduled cron job on a server
type Cron struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID   string                     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	SiteID     *string                    `gorm:"column:site_id;type:char(26);index" json:"site_id,omitempty"`
	User       string                     `gorm:"type:varchar(255);not null" json:"user"`
	Expression string                     `gorm:"type:varchar(255);not null" json:"expression"`
	Command    basemodels.EncryptedString `gorm:"type:longtext;not null" json:"-"`
	Frequency  string                     `gorm:"type:varchar(255);not null" json:"frequency"`
	Hidden     bool                       `gorm:"type:tinyint(1);not null;default:0" json:"hidden"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (c *Cron) BeforeCreate(tx *gorm.DB) error {
	if err := c.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if c.User == "" {
		c.User = "root"
	}

	return nil
}

func (c *Cron) TableName() string {
	return "crons"
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

// ToCronFileContents generates the cron file contents
func (c *Cron) ToCronFileContents() string {
	// Format: expression user command >> logfile 2>&1
	return fmt.Sprintf("%s %s %s >> %s 2>&1\n", c.Expression, c.User, c.Command.String(), c.GetLogPath())
}

// GetCommand returns the decrypted command string
func (c *Cron) GetCommand() string {
	return c.Command.String()
}
