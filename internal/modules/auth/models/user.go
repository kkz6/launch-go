package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// User represents an authenticated user in the system
type User struct {
	ID                      string         `gorm:"primaryKey;size:26" json:"id"`
	Name                    string         `gorm:"size:255;not null" json:"name"`
	Email                   string         `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password                string         `gorm:"size:255;not null" json:"-"`
	EmailVerifiedAt         *time.Time     `json:"email_verified_at,omitempty"`
	RememberToken           *string        `gorm:"size:100" json:"-"`
	CurrentTeamID           *string        `gorm:"size:26" json:"current_team_id,omitempty"`
	ProfilePhotoPath        *string        `gorm:"size:2048" json:"profile_photo_path,omitempty"`
	TwoFactorSecret         *string        `gorm:"size:255" json:"-"`
	TwoFactorConfirmedAt    *time.Time     `json:"-"`
	TwoFactorRecoveryCodes  *string        `gorm:"type:text" json:"-"`
	Timezone                string         `gorm:"size:50;default:'UTC'" json:"timezone"`
	Onboarded               bool           `gorm:"default:false" json:"onboarded"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
	DeletedAt               gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Teams       []Team `gorm:"many2many:team_members;" json:"teams,omitempty"`
	CurrentTeam *Team  `gorm:"foreignKey:CurrentTeamID" json:"current_team,omitempty"`
	OwnedTeams  []Team `gorm:"foreignKey:OwnerID" json:"owned_teams,omitempty"`
}

// TableName returns the table name for the User model
func (u *User) TableName() string {
	return "users"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = utils.NewULID()
	}

	return nil
}

// HasVerifiedEmail checks if the user has verified their email
func (u *User) HasVerifiedEmail() bool {
	return u.EmailVerifiedAt != nil
}

// HasEnabledTwoFactorAuthentication checks if 2FA is enabled
func (u *User) HasEnabledTwoFactorAuthentication() bool {
	return u.TwoFactorSecret != nil && u.TwoFactorConfirmedAt != nil
}

// TwoFactorEnabled returns whether 2FA is enabled (for JSON serialization)
func (u *User) TwoFactorEnabled() bool {
	return u.HasEnabledTwoFactorAuthentication()
}

// ProfilePhotoURL returns the user's profile photo URL or a default gravatar
func (u *User) ProfilePhotoURL() string {
	if u.ProfilePhotoPath != nil && *u.ProfilePhotoPath != "" {
		return *u.ProfilePhotoPath
	}

	return u.DefaultProfilePhotoURL()
}

// DefaultProfilePhotoURL generates a default avatar URL using UI Avatars
func (u *User) DefaultProfilePhotoURL() string {
	name := u.Name
	if name == "" {
		name = "U"
	}

	return "https://ui-avatars.com/api/?name=" + name + "&color=7F9CF5&background=EBF4FF"
}

// OwnsTeam checks if the user owns the given team
func (u *User) OwnsTeam(team *Team) bool {
	return u.ID == team.OwnerID
}

// BelongsToTeam checks if the user belongs to the given team
func (u *User) BelongsToTeam(team *Team) bool {
	if u.OwnsTeam(team) {
		return true
	}

	for _, t := range u.Teams {
		if t.ID == team.ID {
			return true
		}
	}

	return false
}
