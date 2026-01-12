package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Database represents a database on a server
type Database struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID string `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name     string `gorm:"type:varchar(255);not null" json:"name"`

	// Relations
	Users []DatabaseUser `gorm:"many2many:database_database_user;" json:"users,omitempty"`
}

func (Database) TableName() string {
	return "databases"
}
