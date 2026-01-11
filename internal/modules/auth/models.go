package auth

import (
	"crypto/rand"
	"encoding/base64"
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

// Team represents a team/organization in the system
type Team struct {
	ID                   string         `gorm:"primaryKey;size:26" json:"id"`
	Name                 string         `gorm:"size:255;not null" json:"name"`
	OwnerID              string         `gorm:"size:26;not null;index" json:"owner_id"`
	PersonalTeam         bool           `gorm:"default:false" json:"personal_team"`
	ImagePath            *string        `gorm:"size:2048" json:"image_path,omitempty"`
	RequiresSubscription bool           `gorm:"default:false" json:"requires_subscription"`
	TrialEndsAt          *time.Time     `json:"trial_ends_at,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Owner       *User            `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Members     []User           `gorm:"many2many:team_members;" json:"members,omitempty"`
	Invitations []TeamInvitation `gorm:"foreignKey:TeamID" json:"invitations,omitempty"`
}

// TableName returns the table name for the Team model
func (t *Team) TableName() string {
	return "teams"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}

	return nil
}

// ImageURL returns the team's image URL or a default avatar
func (t *Team) ImageURL() string {
	if t.ImagePath != nil && *t.ImagePath != "" {
		return *t.ImagePath
	}

	return t.DefaultImageURL()
}

// DefaultImageURL generates a default avatar URL using UI Avatars
func (t *Team) DefaultImageURL() string {
	name := t.Name
	if name == "" {
		name = "T"
	}

	return "https://ui-avatars.com/api/?name=" + name + "&color=7F9CF5&background=EBF4FF"
}

// HasUser checks if a user belongs to the team
func (t *Team) HasUser(user *User) bool {
	if user.ID == t.OwnerID {
		return true
	}

	for _, member := range t.Members {
		if member.ID == user.ID {
			return true
		}
	}

	return false
}

// TeamMember represents the pivot table for team-user relationships
type TeamMember struct {
	ID        string    `gorm:"primaryKey;size:26" json:"id"`
	TeamID    string    `gorm:"size:26;uniqueIndex:idx_team_user;not null" json:"team_id"`
	UserID    string    `gorm:"size:26;uniqueIndex:idx_team_user;not null" json:"user_id"`
	Role      string    `gorm:"size:50;default:'member'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for the TeamMember model
func (tm *TeamMember) TableName() string {
	return "team_members"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (tm *TeamMember) BeforeCreate(tx *gorm.DB) error {
	if tm.ID == "" {
		tm.ID = utils.NewULID()
	}

	return nil
}

// TeamInvitation represents a pending team invitation
type TeamInvitation struct {
	ID        string    `gorm:"primaryKey;size:26" json:"id"`
	TeamID    string    `gorm:"size:26;not null;index" json:"team_id"`
	Email     string    `gorm:"size:255;not null" json:"email"`
	Role      string    `gorm:"size:50;default:'member'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
}

// TableName returns the table name for the TeamInvitation model
func (ti *TeamInvitation) TableName() string {
	return "team_invitations"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (ti *TeamInvitation) BeforeCreate(tx *gorm.DB) error {
	if ti.ID == "" {
		ti.ID = utils.NewULID()
	}

	return nil
}

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

// TeamRole represents valid team roles
type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleMember TeamRole = "member"
)

// IsValid checks if the role is valid
func (r TeamRole) IsValid() bool {
	switch r {
	case TeamRoleOwner, TeamRoleAdmin, TeamRoleMember:
		return true
	}

	return false
}

// String returns the string representation of the role
func (r TeamRole) String() string {
	return string(r)
}
