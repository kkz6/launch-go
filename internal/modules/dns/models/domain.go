package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Domain represents a domain managed by a DNS provider
type Domain struct {
	ID               string         `gorm:"primaryKey;size:26" json:"id"`
	UserID           string         `gorm:"size:26;not null;index" json:"user_id"`
	TeamID           string         `gorm:"size:26;not null;index" json:"team_id"`
	DomainProviderID string         `gorm:"size:26;not null;index" json:"domain_provider_id"`
	ProviderID       string         `gorm:"size:255;not null" json:"provider_id"`
	Label            string         `gorm:"size:255;not null" json:"label"`
	Address          string         `gorm:"size:255;not null" json:"address"`
	AdditionalData   *string        `gorm:"type:json" json:"-"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Provider *DomainProvider `gorm:"foreignKey:DomainProviderID" json:"provider,omitempty"`
	Records  []DnsRecord     `gorm:"foreignKey:DomainID" json:"records,omitempty"`
}

// TableName specifies the table name for Domain
func (Domain) TableName() string {
	return "domains"
}

// BeforeCreate hook to generate ULID
func (d *Domain) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}

	return nil
}

// GetAdditionalData returns additional data as a map
func (d *Domain) GetAdditionalData() (map[string]interface{}, error) {
	if d.AdditionalData == nil {
		return nil, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*d.AdditionalData), &data); err != nil {
		return nil, err
	}

	return data, nil
}

// SetAdditionalData sets additional data from a map
func (d *Domain) SetAdditionalData(data map[string]interface{}) error {
	if data == nil {
		d.AdditionalData = nil
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	s := string(jsonData)
	d.AdditionalData = &s

	return nil
}
