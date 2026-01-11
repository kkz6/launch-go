package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// DnsRecord represents a DNS record for a domain
type DnsRecord struct {
	ID         string           `gorm:"type:char(26);primaryKey" json:"id"`
	DomainID   string           `gorm:"column:domain_id;type:char(26);not null;index" json:"domain_id"`
	ProviderID string           `gorm:"column:provider_id;type:varchar(255);not null" json:"provider_id"`
	Type       enums.RecordType `gorm:"type:varchar(255);not null" json:"type"`
	Name       string           `gorm:"type:varchar(255);not null" json:"name"`
	Value      string           `gorm:"type:varchar(255);not null" json:"value"`
	TTL        int              `gorm:"type:int;not null" json:"ttl"`
	Priority   *int             `gorm:"type:int" json:"priority,omitempty"`
	Tag        *string          `gorm:"type:varchar(255)" json:"tag,omitempty"`
	Weight     *int             `gorm:"type:int" json:"weight,omitempty"`
	Port       *int             `gorm:"type:int" json:"port,omitempty"`
	Flags      *int             `gorm:"type:int" json:"flags,omitempty"`
	Comment    *string          `gorm:"type:varchar(255)" json:"comment,omitempty"`
	Proxied    *bool            `gorm:"type:boolean" json:"proxied,omitempty"`
	CreatedAt  *time.Time       `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt  *time.Time       `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Domain *Domain `gorm:"foreignKey:DomainID;references:ID" json:"domain,omitempty"`
}

// TableName specifies the table name for DnsRecord
func (DnsRecord) TableName() string {
	return "dns_records"
}

// BeforeCreate hook to generate ULID
func (r *DnsRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}

	return nil
}

// IsEditable returns true if this record type can be edited by users
func (r *DnsRecord) IsEditable() bool {
	return r.Type != enums.RecordTypeNS && r.Type != enums.RecordTypeSOA
}

// IsDeletable returns true if this record type can be deleted by users
func (r *DnsRecord) IsDeletable() bool {
	return r.Type != enums.RecordTypeNS && r.Type != enums.RecordTypeSOA
}

// ToProviderRecord converts DnsRecord to providers.DnsRecord for provider operations
func (r *DnsRecord) ToProviderRecord() *providers.DnsRecord {
	return &providers.DnsRecord{
		ID:         r.ID,
		ProviderID: r.ProviderID,
		Type:       providers.RecordType(r.Type),
		Name:       r.Name,
		Value:      r.Value,
		TTL:        r.TTL,
		Priority:   r.Priority,
		Tag:        r.Tag,
		Weight:     r.Weight,
		Port:       r.Port,
		Flags:      r.Flags,
		Comment:    r.Comment,
		Proxied:    r.Proxied,
	}
}

// ProviderRecord is an alias to providers.ProviderRecord for convenience
type ProviderRecord = providers.ProviderRecord
