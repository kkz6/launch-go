package models

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Passkey represents a WebAuthn passkey credential for passwordless authentication
type Passkey struct {
	basemodels.BaseModel
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

	// Relations
	User *User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// TableName returns the table name for the Passkey model
func (Passkey) TableName() string {
	return "passkeys"
}

// DisplayName returns a display-friendly name for the passkey
func (p *Passkey) DisplayName() string {
	if p.Name != nil && *p.Name != "" {
		return *p.Name
	}

	if p.CreatedAt != nil {
		return "Passkey created on " + p.CreatedAt.Format("Jan 2, 2006")
	}

	return "Passkey"
}

// ToWebAuthnCredential converts the DB model to a webauthn.Credential
func (p *Passkey) ToWebAuthnCredential() webauthn.Credential {
	credID, _ := base64.RawURLEncoding.DecodeString(p.CredentialID)
	pubKey, _ := base64.RawURLEncoding.DecodeString(p.PublicKey)

	cred := webauthn.Credential{
		ID:              credID,
		PublicKey:       pubKey,
		AttestationType: "",
		Authenticator: webauthn.Authenticator{
			SignCount: uint32(p.SignCount),
		},
	}

	if p.Transports != nil {
		var transports []string
		if err := json.Unmarshal([]byte(*p.Transports), &transports); err == nil {
			for _, t := range transports {
				cred.Transport = append(cred.Transport, protocol.AuthenticatorTransport(t))
			}
		}
	}

	return cred
}

// UpdateUsage updates sign count and last used timestamp
func (p *Passkey) UpdateUsage(signCount uint32) {
	p.SignCount = int(signCount)
	now := time.Now()
	p.LastUsedAt = &now
}

// PasskeyFromCredential creates a Passkey model from a WebAuthn credential
func PasskeyFromCredential(userID string, cred *webauthn.Credential, name *string) *Passkey {
	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	pubKey := base64.RawURLEncoding.EncodeToString(cred.PublicKey)

	var transports *string
	if len(cred.Transport) > 0 {
		ts := make([]string, len(cred.Transport))
		for i, t := range cred.Transport {
			ts[i] = string(t)
		}

		data, _ := json.Marshal(ts)
		s := string(data)
		transports = &s
	}

	var aaguid *string
	aaguidStr := base64.RawURLEncoding.EncodeToString(cred.Authenticator.AAGUID)
	if aaguidStr != "" {
		aaguid = &aaguidStr
	}

	return &Passkey{
		UserID:       userID,
		Name:         name,
		CredentialID: credID,
		PublicKey:    pubKey,
		SignCount:    int(cred.Authenticator.SignCount),
		AAGUID:       aaguid,
		Transports:   transports,
		Type:         "public-key",
	}
}
