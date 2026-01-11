package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// DomainProvider represents a DNS provider configuration
type DomainProvider struct {
	ID               string            `gorm:"type:char(26);primaryKey" json:"id"`
	UserID           string            `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID           *string           `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Profile          *string           `gorm:"type:varchar(255)" json:"profile,omitempty"`
	Provider         enums.DnsProvider `gorm:"type:varchar(255);not null" json:"provider"`
	Credentials      string            `gorm:"type:longtext;not null" json:"-"`
	Connected        bool              `gorm:"default:true" json:"connected"`
	AdditionalData   *string           `gorm:"column:additional_data;type:json" json:"-"`
	SyncStatus       enums.SyncStatus  `gorm:"column:sync_status;type:varchar(255);not null;default:idle" json:"sync_status"`
	LastSyncedAt     *time.Time        `gorm:"column:last_synced_at;type:timestamp null" json:"last_synced_at,omitempty"`
	SyncErrorMessage *string           `gorm:"column:sync_error_message;type:text" json:"sync_error_message,omitempty"`
	CreatedAt        *time.Time        `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt        *time.Time        `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Domains []Domain `gorm:"foreignKey:DomainProviderID;references:ID" json:"domains,omitempty"`
}

// TableName specifies the table name for DomainProvider
func (DomainProvider) TableName() string {
	return "domain_providers"
}

// BeforeCreate hook to generate ULID
func (dp *DomainProvider) BeforeCreate(tx *gorm.DB) error {
	if dp.ID == "" {
		dp.ID = utils.NewULID()
	}

	return nil
}

// GetCredentials decrypts and returns the credentials as a map
func (dp *DomainProvider) GetCredentials() (map[string]string, error) {
	var creds map[string]string
	if err := json.Unmarshal([]byte(dp.Credentials), &creds); err != nil {
		return nil, err
	}

	return creds, nil
}

// SetCredentials encrypts and stores the credentials
func (dp *DomainProvider) SetCredentials(creds map[string]string) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	dp.Credentials = string(data)

	return nil
}

// GetAdditionalData returns additional data as a map
func (dp *DomainProvider) GetAdditionalData() (map[string]interface{}, error) {
	if dp.AdditionalData == nil {
		return nil, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*dp.AdditionalData), &data); err != nil {
		return nil, err
	}

	return data, nil
}

// SetAdditionalData sets additional data from a map
func (dp *DomainProvider) SetAdditionalData(data map[string]interface{}) error {
	if data == nil {
		dp.AdditionalData = nil
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	s := string(jsonData)
	dp.AdditionalData = &s

	return nil
}

// ProviderLabel returns a human-readable label for the provider
func (dp *DomainProvider) ProviderLabel() string {
	return dp.Provider.Label()
}
