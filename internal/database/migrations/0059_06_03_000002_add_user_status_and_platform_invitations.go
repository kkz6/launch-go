package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0059_06_03_000002_add_user_status_and_platform_invitations",
		Name:      "Add users.status, create platform_invitations",
		Timestamp: time.Date(2026, 6, 3, 0, 0, 2, 0, time.UTC),
		Up:        addUserStatusAndPlatformInvitationsUp,
	})
}

// platformInvitationMigration is a pending platform-level invite with a trial
// window. FK on invited_by -> users is intentionally omitted so the invite
// record survives staff churn (mirrors impersonation_sessions).
type platformInvitationMigration struct {
	ID          string     `gorm:"type:char(26);primaryKey"`
	Email       string     `gorm:"column:email;type:varchar(255);not null;index"`
	Token       string     `gorm:"column:token;type:varchar(64);not null;uniqueIndex"`
	TrialEndsAt time.Time  `gorm:"column:trial_ends_at;type:timestamp;not null"`
	InvitedBy   string     `gorm:"column:invited_by;type:char(26);not null"`
	AcceptedAt  *time.Time `gorm:"column:accepted_at;type:timestamp"`
	ExpiresAt   time.Time  `gorm:"column:expires_at;type:timestamp;not null"`
	CreatedAt   *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;type:timestamp"`
}

func (platformInvitationMigration) TableName() string { return "platform_invitations" }

func addUserStatusAndPlatformInvitationsUp(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE users ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active'",
	).Error; err != nil {
		return err
	}

	return db.Migrator().CreateTable(&platformInvitationMigration{})
}
