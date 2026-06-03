package models

import "time"

type PlatformInvitation struct {
	ID          string     `gorm:"type:char(26);primaryKey" json:"id"`
	Email       string     `gorm:"column:email;type:varchar(255);not null" json:"email"`
	Token       string     `gorm:"column:token;type:varchar(64);not null" json:"-"`
	TrialEndsAt time.Time  `gorm:"column:trial_ends_at;type:timestamp;not null" json:"trial_ends_at"`
	InvitedBy   string     `gorm:"column:invited_by;type:char(26);not null" json:"invited_by"`
	AcceptedAt  *time.Time `gorm:"column:accepted_at;type:timestamp" json:"accepted_at,omitempty"`
	ExpiresAt   time.Time  `gorm:"column:expires_at;type:timestamp;not null" json:"expires_at"`
	CreatedAt   *time.Time `gorm:"column:created_at;type:timestamp" json:"created_at,omitempty"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;type:timestamp" json:"updated_at,omitempty"`
}

func (PlatformInvitation) TableName() string { return "platform_invitations" }

// IsExpired reports whether the invite is past its expiry.
func (p *PlatformInvitation) IsExpired(now time.Time) bool { return now.After(p.ExpiresAt) }

// IsAccepted reports whether the invite has already been consumed.
func (p *PlatformInvitation) IsAccepted() bool { return p.AcceptedAt != nil }
