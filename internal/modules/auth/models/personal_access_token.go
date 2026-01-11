package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// PersonalAccessToken represents a user's API token
type PersonalAccessToken struct {
	ID            string         `gorm:"primaryKey;size:26" json:"id"`
	UserID        string         `gorm:"size:26;not null;index" json:"user_id"`
	Name          string         `gorm:"size:255;not null" json:"name"`
	Token         string         `gorm:"size:64;uniqueIndex;not null" json:"-"`
	Abilities     string         `gorm:"type:text" json:"abilities,omitempty"`
	LastUsedAt    *time.Time     `json:"last_used_at,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for the PersonalAccessToken model
func (t *PersonalAccessToken) TableName() string {
	return "personal_access_tokens"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (t *PersonalAccessToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}

	return nil
}

// IsExpired checks if the token has expired
func (t *PersonalAccessToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}

	return time.Now().After(*t.ExpiresAt)
}
