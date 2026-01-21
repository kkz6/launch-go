package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Domain represents a domain managed by a DNS provider
type Domain struct {
	basemodels.BaseModel
	basemodels.UserScopedModel
	basemodels.TeamScopedModel
	DomainProviderID string             `gorm:"column:domain_provider_id;type:char(26);not null;index" json:"domain_provider_id"`
	Label            string             `gorm:"type:varchar(255);not null" json:"label"`
	Address          string             `gorm:"type:varchar(255);not null" json:"address"`
	ProviderID       string             `gorm:"column:provider_id;type:varchar(255);not null" json:"provider_id"`
	AdditionalData   basemodels.JSONMap `gorm:"column:additional_data;type:json" json:"-"`

	// Relations
	Provider *DomainProvider `gorm:"foreignKey:DomainProviderID;references:ID" json:"provider,omitempty"`
	Records  []DNSRecord     `gorm:"foreignKey:DomainID;references:ID" json:"records,omitempty"`
}

// TableName specifies the table name for Domain
func (Domain) TableName() string {
	return "domains"
}
