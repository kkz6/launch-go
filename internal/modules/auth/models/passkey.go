package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Passkey represents a WebAuthn passkey credential for passwordless authentication
type Passkey struct {
	ID              string     `gorm:"type:char(26);primaryKey" json:"id"`
	UserID          string     `gorm:"column:user_id;type:char(26);not null;index:idx_passkeys_user_created,priority:1" json:"user_id"`
	Name            *string    `gorm:"type:varchar(255)" json:"name,omitempty"`
	CredentialID    string     `gorm:"column:credential_id;type:varchar(255);not null;uniqueIndex" json:"credential_id"`
	PublicKey       string     `gorm:"column:public_key;type:text;not null" json:"-"`
	SignCount       int        `gorm:"column:sign_count;type:int;not null;default:0" json:"sign_count"`
	AAGUID          *string    `gorm:"type:varchar(255)" json:"aaguid,omitempty"`
	Transports      *string    `gorm:"type:json" json:"transports,omitempty"`
	Type            string     `gorm:"type:varchar(255);not null;default:'public-key'" json:"type"`
	AttestationData *string    `gorm:"column:attestation_data;type:json" json:"attestation_data,omitempty"`
	LastUsedAt      *time.Time `gorm:"column:last_used_at;type:timestamp null" json:"last_used_at,omitempty"`
	CreatedAt       *time.Time `gorm:"type:timestamp null;index:idx_passkeys_user_created,priority:2" json:"created_at,omitempty"`
	UpdatedAt       *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	User *User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// TableName returns the table name for the Passkey model
func (p *Passkey) TableName() string {
	return "passkeys"
}

// BeforeCreate is a GORM hook that sets the ID if not provided
func (p *Passkey) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = utils.NewULID()
	}

	return nil
}
