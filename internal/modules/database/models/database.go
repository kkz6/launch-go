package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Database represents a database on a server
type Database struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	basemodels.ServerScoped
	basemodels.TeamScoped
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// Relations
	Users []DatabaseUser `gorm:"many2many:database_database_user;" json:"users,omitempty"`
}

func (Database) TableName() string {
	return "databases"
}
