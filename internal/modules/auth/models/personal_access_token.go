package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// PersonalAccessToken represents a user's API token (polymorphic relation)
type PersonalAccessToken struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	TokenableType string     `gorm:"column:tokenable_type;type:varchar(255);not null;index:personal_access_tokens_tokenable_type_tokenable_id_index,priority:1" json:"tokenable_type"`
	TokenableID   uint64     `gorm:"column:tokenable_id;type:bigint unsigned;not null;index:personal_access_tokens_tokenable_type_tokenable_id_index,priority:2" json:"tokenable_id"`
	Name          string     `gorm:"type:varchar(255);not null" json:"name"`
	Token         string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	Abilities     *string    `gorm:"type:text" json:"abilities,omitempty"`
	LastUsedAt    *time.Time `gorm:"column:last_used_at;type:timestamp null" json:"last_used_at,omitempty"`
	ExpiresAt     *time.Time `gorm:"column:expires_at;type:timestamp null" json:"expires_at,omitempty"`
}

// TableName returns the table name for the PersonalAccessToken model
func (t *PersonalAccessToken) TableName() string {
	return "personal_access_tokens"
}

// IsExpired checks if the token has expired
func (t *PersonalAccessToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}

	return time.Now().After(*t.ExpiresAt)
}
