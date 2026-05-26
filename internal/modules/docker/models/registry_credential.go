package models

import (
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// RegistryCredential is a saved docker-image registry login that can
// be attached to an application or a compose stack. Same usage shape
// as source_controls and storage_providers (team-scoped, picked from
// a dropdown at workload-create time).
//
// Username + password are BOTH encrypted at rest. Username is short
// + cheap to encrypt, and it can leak useful info to an attacker
// (the registry account / org), so we wrap it too. `json:"-"` keeps
// both off any response that serializes the model directly — the
// service layer decrypts and re-emits a display label for the
// picker (e.g. "ghcr.io — kkz6").
//
// RegistryURL is nullable: empty → Docker Hub. The deploy script
// runs `docker login` (no host arg) for Docker Hub vs
// `docker login <registry_url>` for everything else.
//
// Per-team name uniqueness (live rows only) enforced by migration
// 0037's index — service layer surfaces a 409 with a friendly
// message before that constraint trips.
//
// Soft-delete: deleted_at is part of the unique index so a removed
// row's name can be reused later.
type RegistryCredential struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	// TeamID scopes the credential to a team — every team member can
	// pick it when creating a workload.
	TeamID string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	// UserID is the creator. Recorded for audit only; not used for
	// authorisation (any team member can use any team credential,
	// same model source_controls follows).
	UserID *string `gorm:"column:user_id;type:char(26)" json:"user_id,omitempty"`
	// Name is the display label — what shows up in the dropdown
	// when picking on application/compose create.
	Name string `gorm:"type:varchar(255);not null" json:"name"`
	// RegistryURL: empty = Docker Hub (the deploy script omits the
	// host argument to `docker login` in that case). Non-empty:
	// the literal registry host, e.g. "ghcr.io".
	RegistryURL *string `gorm:"column:registry_url;type:varchar(255)" json:"registry_url,omitempty"`
	// Username encrypted at rest. Read-on-demand only.
	Username dbtype.EncryptedString `gorm:"column:username;type:longtext;not null" json:"-"`
	// Password encrypted at rest. NEVER serialized to JSON.
	Password dbtype.EncryptedString `gorm:"column:password;type:longtext;not null" json:"-"`
}

// TableName pins the table name (don't auto-pluralize to
// "registry_credentials" via GORM's reflection; declare it
// explicitly to match the migration).
func (RegistryCredential) TableName() string { return "registry_credentials" }
