package models

import (
	"encoding/json"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Domain represents a domain managed by a DNS provider
type Domain struct {
	basemodels.BaseModel
	UserID           string  `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID           *string `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	DomainProviderID string  `gorm:"column:domain_provider_id;type:char(26);not null;index" json:"domain_provider_id"`
	Label            string  `gorm:"type:varchar(255);not null" json:"label"`
	Address          string  `gorm:"type:varchar(255);not null" json:"address"`
	ProviderID       string  `gorm:"column:provider_id;type:varchar(255);not null" json:"provider_id"`
	AdditionalData   *string `gorm:"column:additional_data;type:json" json:"-"`

	// Relations
	Provider *DomainProvider `gorm:"foreignKey:DomainProviderID;references:ID" json:"provider,omitempty"`
	Records  []DnsRecord     `gorm:"foreignKey:DomainID;references:ID" json:"records,omitempty"`
}

// TableName specifies the table name for Domain
func (Domain) TableName() string {
	return "domains"
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
