package models

import (
	"gorm.io/gorm"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// DatabaseUser represents a database user on a server
type DatabaseUser struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID string  `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name     string  `gorm:"type:varchar(255);not null" json:"name"`
	Password *string `gorm:"type:longtext" json:"-"`
	Host     string  `gorm:"type:varchar(255);not null;default:localhost" json:"host"`

	// Relations
	Databases []Database `gorm:"many2many:database_database_user;" json:"databases,omitempty"`
}

func (DatabaseUser) TableName() string {
	return "database_users"
}

// BeforeCreate extends the base BeforeCreate to also set default Host
func (u *DatabaseUser) BeforeCreate(tx *gorm.DB) error {
	// Call embedded BaseModel's BeforeCreate for ULID generation
	if err := u.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if u.Host == "" {
		u.Host = "localhost"
	}
	return nil
}
