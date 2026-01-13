package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// DomainProvider represents a DNS provider configuration
type DomainProvider struct {
	basemodels.BaseModel
	UserID           string                   `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID           *string                  `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Profile          *string                  `gorm:"type:varchar(255)" json:"profile,omitempty"`
	Provider         enums.DnsProvider        `gorm:"type:varchar(255);not null" json:"provider"`
	Credentials      basemodels.EncryptedJSONStringMap `gorm:"type:longtext;not null" json:"-"`
	Connected        bool                     `gorm:"default:true" json:"connected"`
	AdditionalData   basemodels.JSONMap       `gorm:"column:additional_data;type:json" json:"-"`
	SyncStatus       enums.SyncStatus         `gorm:"column:sync_status;type:varchar(255);not null;default:idle" json:"sync_status"`
	LastSyncedAt     *time.Time               `gorm:"column:last_synced_at;type:timestamp null" json:"last_synced_at,omitempty"`
	SyncErrorMessage *string                  `gorm:"column:sync_error_message;type:text" json:"sync_error_message,omitempty"`

	// Relations
	Domains []Domain `gorm:"foreignKey:DomainProviderID;references:ID" json:"domains,omitempty"`
}

// TableName specifies the table name for DomainProvider
func (DomainProvider) TableName() string {
	return "domain_providers"
}

// ProviderLabel returns a human-readable label for the provider
func (dp *DomainProvider) ProviderLabel() string {
	return dp.Provider.Label()
}
