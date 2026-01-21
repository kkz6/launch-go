package models

import (
	"time"
)

// PasswordResetToken represents a password reset request
// Note: Uses email as primary key, no ULID
type PasswordResetToken struct {
	Email     string     `gorm:"type:varchar(255);primaryKey" json:"email"`
	Token     string     `gorm:"type:varchar(255);not null" json:"-"`
	CreatedAt *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
}

// TableName returns the table name for the PasswordResetToken model
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// IsExpired checks if the token has expired (default 60 minutes)
func (p *PasswordResetToken) IsExpired() bool {
	if p.CreatedAt == nil {
		return true
	}

	return time.Since(*p.CreatedAt) > 60*time.Minute
}
