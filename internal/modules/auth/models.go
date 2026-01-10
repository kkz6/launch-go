package auth

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

type User struct {
	ID              string         `gorm:"primaryKey;size:26" json:"id"`
	Name            string         `gorm:"size:255;not null" json:"name"`
	Email           string         `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password        string         `gorm:"size:255;not null" json:"-"`
	EmailVerifiedAt *time.Time     `json:"email_verified_at,omitempty"`
	TwoFactorSecret *string        `gorm:"size:255" json:"-"`
	CurrentTeamID   *string        `gorm:"size:26" json:"current_team_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Teams       []Team       `gorm:"many2many:team_members;" json:"teams,omitempty"`
	CurrentTeam *Team        `gorm:"foreignKey:CurrentTeamID" json:"current_team,omitempty"`
	OwnedTeams  []Team       `gorm:"foreignKey:OwnerID" json:"owned_teams,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = utils.NewULID()
	}
	return nil
}

type Team struct {
	ID           string         `gorm:"primaryKey;size:26" json:"id"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	OwnerID      string         `gorm:"size:26;not null" json:"owner_id"`
	PersonalTeam bool           `gorm:"default:false" json:"personal_team"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Owner   *User  `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Members []User `gorm:"many2many:team_members;" json:"members,omitempty"`
}

func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}
	return nil
}

type TeamMember struct {
	TeamID    string    `gorm:"primaryKey;size:26"`
	UserID    string    `gorm:"primaryKey;size:26"`
	Role      string    `gorm:"size:50;default:'member'"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

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
}

func (t *PersonalAccessToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}
	return nil
}
