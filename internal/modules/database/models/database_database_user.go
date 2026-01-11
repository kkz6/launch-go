package models

import (
	"time"
)

// DatabaseDatabaseUser represents the many-to-many relationship between Database and DatabaseUser
type DatabaseDatabaseUser struct {
	DatabaseID     string     `gorm:"column:database_id;type:char(26);not null;primaryKey" json:"database_id"`
	DatabaseUserID string     `gorm:"column:database_user_id;type:char(26);not null;primaryKey" json:"database_user_id"`
	CreatedAt      *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt      *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`
}

func (DatabaseDatabaseUser) TableName() string {
	return "database_database_user"
}
