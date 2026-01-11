package models

import (
	"time"
)

// DatabaseDatabaseUser represents the many-to-many relationship between Database and DatabaseUser
type DatabaseDatabaseUser struct {
	DatabaseID     string    `gorm:"primaryKey;size:26" json:"database_id"`
	DatabaseUserID string    `gorm:"primaryKey;size:26" json:"database_user_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (DatabaseDatabaseUser) TableName() string {
	return "database_database_user"
}
