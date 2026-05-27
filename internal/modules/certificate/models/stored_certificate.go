package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// StoredCertificate is a team-scoped, reusable TLS certificate +
// private key pair. Picked from a dropdown wherever HTTPS is
// configured (PHP sites, docker domains). Mirrors the shape of
// registry_credentials / source_controls / storage_providers.
//
// `Certificate` is the leaf-PEM (often including chain); never
// secret on its own, kept as plain TEXT for indexing and audit.
// `PrivateKey` is encrypted at rest via dbtype.EncryptedString and
// is the only field that's `json:"-"` — everything else can be
// safely exposed in API responses.
//
// Parsed metadata (domains / not_before / not_after / issuer / serial
// / fingerprint) is server-derived on save by the service layer; the
// user never supplies it. See services/parser.go (Phase 2).
//
// Soft-delete uses partial unique indexes (WHERE deleted_at IS NULL)
// at the DB level (migration 0046), so a removed row's name and
// fingerprint can be reused later.
type StoredCertificate struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel

	TeamID string  `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID *string `gorm:"column:user_id;type:char(26)" json:"user_id,omitempty"`

	Name  string  `gorm:"type:varchar(255);not null" json:"name"`
	Notes *string `gorm:"type:text" json:"notes,omitempty"`

	Certificate string                 `gorm:"type:text;not null" json:"certificate"`
	PrivateKey  dbtype.EncryptedString `gorm:"column:private_key;type:text;not null" json:"-"`

	Domains           dbtype.JSONStringSlice `gorm:"type:jsonb;not null;default:'[]'::jsonb" json:"domains"`
	CommonName        *string                `gorm:"column:common_name;type:varchar(255)" json:"common_name,omitempty"`
	Issuer            *string                `gorm:"type:varchar(255)" json:"issuer,omitempty"`
	NotBefore         time.Time              `gorm:"column:not_before;type:timestamptz;not null" json:"not_before"`
	NotAfter          time.Time              `gorm:"column:not_after;type:timestamptz;not null" json:"not_after"`
	SerialNumber      *string                `gorm:"column:serial_number;type:varchar(255)" json:"serial_number,omitempty"`
	FingerprintSHA256 *string                `gorm:"column:fingerprint_sha256;type:varchar(64)" json:"fingerprint_sha256,omitempty"`
}

func (StoredCertificate) TableName() string { return "stored_certificates" }
