package models

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// PasswordResetToken represents a password reset request
type PasswordResetToken struct {
	Email     string    `gorm:"primaryKey;size:255" json:"email"`
	Token     string    `gorm:"size:255;not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName returns the table name for the PasswordResetToken model
func (p *PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// IsExpired checks if the token has expired (default 60 minutes)
func (p *PasswordResetToken) IsExpired() bool {
	return time.Since(p.CreatedAt) > 60*time.Minute
}

// GenerateToken creates a new random token for password reset
func GenerateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}
