package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// DeviceAuthorization stores only hashes of the terminal's secret and user code.
// A credential is created only when an approved grant is consumed.
type DeviceAuthorization struct {
	basemodels.BaseModel
	DeviceCodeHash        string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`
	UserCodeHash          string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`
	UserID                *string    `gorm:"type:char(26)" json:"-"`
	Status                string     `gorm:"type:varchar(20);not null" json:"-"`
	ExpiresAt             time.Time  `gorm:"not null;index" json:"-"`
	LastPolledAt          *time.Time `json:"-"`
	AuthorizationAttempts int        `gorm:"not null;default:0" json:"-"`
}

// DeviceAuthorizationLimit provides an atomic fixed-window counter shared by
// all API instances, including requests for invalid user codes.
type DeviceAuthorizationLimit struct {
	ID        string    `gorm:"type:varchar(64);primaryKey"`
	Attempts  int       `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
}
