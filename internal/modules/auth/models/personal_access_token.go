package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// PersonalAccessToken represents a user's API token (polymorphic relation)
type PersonalAccessToken struct {
	ID            string         `gorm:"type:char(26);primaryKey" json:"id"`
	TokenableType string         `gorm:"column:tokenable_type;type:varchar(255);not null;index:personal_access_tokens_tokenable_type_tokenable_id_index,priority:1" json:"tokenable_type"`
	TokenableID   uint64         `gorm:"column:tokenable_id;type:bigint unsigned;not null;index:personal_access_tokens_tokenable_type_tokenable_id_index,priority:2" json:"tokenable_id"`
	Name          string         `gorm:"type:varchar(255);not null" json:"name"`
	Token         string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	Abilities     *string        `gorm:"type:text" json:"abilities,omitempty"`
	LastUsedAt    *time.Time     `gorm:"column:last_used_at;type:timestamp null" json:"last_used_at,omitempty"`
	ExpiresAt     *time.Time     `gorm:"column:expires_at;type:timestamp null" json:"expires_at,omitempty"`
	CreatedAt     *time.Time     `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt     *time.Time     `gorm:"type:timestamp null" json:"updated_at,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
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
