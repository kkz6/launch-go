package dns

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// DomainProvider represents a DNS provider configuration
type DomainProvider struct {
	ID               string         `gorm:"primaryKey;size:26" json:"id"`
	UserID           string         `gorm:"size:26;not null;index" json:"user_id"`
	TeamID           string         `gorm:"size:26;not null;index" json:"team_id"`
	Profile          string         `gorm:"size:255" json:"profile"`
	Provider         DnsProvider    `gorm:"size:50;not null" json:"provider"`
	Credentials      string         `gorm:"type:text;not null" json:"-"`
	Connected        bool           `gorm:"default:true" json:"connected"`
	AdditionalData   *string        `gorm:"type:json" json:"-"`
	SyncStatus       SyncStatus     `gorm:"size:50" json:"sync_status,omitempty"`
	LastSyncedAt     *time.Time     `json:"last_synced_at,omitempty"`
	SyncErrorMessage *string        `gorm:"size:1000" json:"sync_error_message,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

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

// DnsRecord represents a DNS record for a domain
type DnsRecord struct {
	ID         string         `gorm:"primaryKey;size:26" json:"id"`
	DomainID   string         `gorm:"size:26;not null;index" json:"domain_id"`
	ProviderID string         `gorm:"size:255;not null" json:"provider_id"`
	Type       RecordType     `gorm:"size:10;not null" json:"type"`
	Name       string         `gorm:"size:255;not null" json:"name"`
	Value      string         `gorm:"size:4096;not null" json:"value"`
	TTL        int            `gorm:"not null;default:3600" json:"ttl"`
	Priority   *int           `json:"priority,omitempty"`
	Tag        *string        `gorm:"size:255" json:"tag,omitempty"`
	Weight     *int           `json:"weight,omitempty"`
	Port       *int           `json:"port,omitempty"`
	Flags      *int           `json:"flags,omitempty"`
	Comment    *string        `gorm:"size:1000" json:"comment,omitempty"`
	Proxied    *bool          `json:"proxied,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Domain *Domain `gorm:"foreignKey:DomainID" json:"domain,omitempty"`
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
	return r.Type != RecordTypeNS && r.Type != RecordTypeSOA
}

// IsDeletable returns true if this record type can be deleted by users
func (r *DnsRecord) IsDeletable() bool {
	return r.Type != RecordTypeNS && r.Type != RecordTypeSOA
}

// ToProviderRecord converts DnsRecord to providers.DnsRecord for provider operations
func (r *DnsRecord) ToProviderRecord() *providers.DnsRecord {
	return &providers.DnsRecord{
		ID:         r.ID,
		ProviderID: r.ProviderID,
		Type:       r.Type,
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
