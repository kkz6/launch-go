package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// DatabaseUser represents a database user on a server
type DatabaseUser struct {
	ID                        string     `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name                      string     `gorm:"type:varchar(255);not null" json:"name"`
	Password                  *string    `gorm:"type:longtext;serializer:encrypted" json:"-"`
	Host                      string     `gorm:"type:varchar(255);not null;default:localhost" json:"host"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"-"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Databases []Database `gorm:"many2many:database_database_user" json:"databases,omitempty"`
}

func (d *DatabaseUser) TableName() string {
	return "database_users"
}

func (d *DatabaseUser) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}
	return nil
}

// IsInstalled returns true if the user is installed
func (d *DatabaseUser) IsInstalled() bool {
	return d.InstalledAt != nil
}
