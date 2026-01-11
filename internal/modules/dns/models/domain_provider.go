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
	ID               string              `gorm:"primaryKey;size:26" json:"id"`
	UserID           string              `gorm:"size:26;not null;index" json:"user_id"`
	TeamID           string              `gorm:"size:26;not null;index" json:"team_id"`
	Profile          string              `gorm:"size:255" json:"profile"`
	Provider         enums.DnsProvider   `gorm:"size:50;not null" json:"provider"`
	Credentials      string              `gorm:"type:text;not null" json:"-"`
	Connected        bool                `gorm:"default:true" json:"connected"`
	AdditionalData   *string             `gorm:"type:json" json:"-"`
	SyncStatus       enums.SyncStatus    `gorm:"size:50" json:"sync_status,omitempty"`
	LastSyncedAt     *time.Time          `json:"last_synced_at,omitempty"`
	SyncErrorMessage *string             `gorm:"size:1000" json:"sync_error_message,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	DeletedAt        gorm.DeletedAt      `gorm:"index" json:"-"`

	// Relations
	Domains []Domain `gorm:"foreignKey:DomainProviderID" json:"domains,omitempty"`
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
