package models

import (
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"time"
)

// RegistryLease is a short-lived, deployment-bound GHCR pull credential.
// Queue payloads only contain its opaque ID; token bytes remain encrypted.
type RegistryLease struct {
	basemodels.BaseModel
	TeamID       string                 `gorm:"type:char(26);not null;index" json:"-"`
	DeploymentID string                 `gorm:"type:char(26);not null;index" json:"-"`
	TargetType   string                 `gorm:"type:varchar(32);not null" json:"-"`
	TargetID     string                 `gorm:"type:char(26);not null" json:"-"`
	Package      string                 `gorm:"type:varchar(512);not null" json:"-"`
	Purpose      string                 `gorm:"type:varchar(32);not null" json:"-"`
	Snapshot     string                 `gorm:"type:text;not null" json:"-"`
	Token        dbtype.EncryptedString `gorm:"type:text;not null" json:"-"`
	ExpiresAt    time.Time              `gorm:"not null;index" json:"-"`
}

func (RegistryLease) TableName() string { return "docker_registry_leases" }
