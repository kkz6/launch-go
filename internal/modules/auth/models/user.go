package models

import (
	"net/url"
	"time"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// User represents an authenticated user in the system
type User struct {
	basemodels.BaseModel
	Name                   string                `gorm:"type:varchar(255);not null" json:"name"`
	Email                  string                `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	EmailVerifiedAt        *time.Time            `gorm:"type:timestamp null" json:"email_verified_at,omitempty"`
	Password               string                `gorm:"type:varchar(255);not null" json:"-"`
	TwoFactorSecret        *string               `gorm:"type:text" json:"-"`
	TwoFactorRecoveryCodes *string               `gorm:"type:text" json:"-"`
	TwoFactorConfirmedAt   *time.Time            `gorm:"type:timestamp null" json:"-"`
	RememberToken          *string               `gorm:"type:varchar(100)" json:"-"`
	CurrentTeamID          *string               `gorm:"column:current_team_id;type:char(26)" json:"current_team_id,omitempty"`
	ProfilePhotoPath       *string               `gorm:"column:profile_photo_path;type:varchar(2048)" json:"profile_photo_path,omitempty"`
	Timezone               *string               `gorm:"type:varchar(255);default:'UTC'" json:"timezone,omitempty"`
	Locale                 *string               `gorm:"type:varchar(10)" json:"locale"`
	Onboarded              bool                  `gorm:"type:tinyint(1);not null;default:0" json:"onboarded"`
	StaffRole              *stafftypes.StaffRole `gorm:"column:staff_role;type:varchar(20)" json:"staff_role,omitempty"`
	Status                 authtypes.UserStatus  `gorm:"column:status;type:varchar(20);default:active" json:"status"`

	// Relations
	Teams       []Team `gorm:"many2many:team_user;" json:"teams,omitempty"`
	CurrentTeam *Team  `gorm:"foreignKey:CurrentTeamID;references:ID" json:"current_team,omitempty"`
	OwnedTeams  []Team `gorm:"foreignKey:UserID;references:ID" json:"owned_teams,omitempty"`
}

// TableName returns the table name for the User model
func (User) TableName() string {
	return "users"
}

// HasVerifiedEmail checks if the user has verified their email
func (u *User) HasVerifiedEmail() bool {
	return u.EmailVerifiedAt != nil
}

// IsStaff reports whether the user has any back-office staff access.
func (u *User) IsStaff() bool {
	return u.StaffRole != nil && u.StaffRole.IsValid()
}

// HasStaffRole reports whether the user's staff tier is at least minRole.
func (u *User) HasStaffRole(minRole stafftypes.StaffRole) bool {
	return u.StaffRole != nil && u.StaffRole.Level() >= minRole.Level()
}

// IsSuspended reports whether the user account is suspended.
func (u *User) IsSuspended() bool {
	return u.Status == authtypes.UserStatusSuspended
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

	return "https://ui-avatars.com/api/?name=" + url.QueryEscape(name) + "&color=7F9CF5&background=EBF4FF"
}

// GetTimezone returns the user's timezone or UTC if not set
func (u *User) GetTimezone() string {
	if u.Timezone != nil && *u.Timezone != "" {
		return *u.Timezone
	}

	return "UTC"
}

// OwnsTeam checks if the user owns the given team
func (u *User) OwnsTeam(team *Team) bool {
	return u.ID == team.UserID
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
