package dto

import (
	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// CreateDNSRecordRequest represents a request to create a DNS record
type CreateDNSRecordRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=255"`
	Value    string `json:"value" validate:"required,min=1"`
	Type     string `json:"type" validate:"required,oneof=A AAAA CNAME MX TXT SRV CAA NS"`
	TTL      int    `json:"ttl" validate:"omitempty,min=1"`
	Priority *int   `json:"priority" validate:"omitempty,min=0"`
	Weight   *int   `json:"weight" validate:"omitempty,min=0"`
	Port     *int   `json:"port" validate:"omitempty,min=1,max=65535"`
	Flags    *int   `json:"flags" validate:"omitempty,min=0"`
	Tag      string `json:"tag" validate:"omitempty,max=255"`
	Comment  string `json:"comment" validate:"omitempty,max=1000"`
	Proxied  *bool  `json:"proxied"`
}

// ToModel converts the request to a DNSRecord model
func (r *CreateDNSRecordRequest) ToModel(domainID string) *models.DNSRecord {
	record := &models.DNSRecord{
		DomainID: domainID,
		Type:     enums.RecordType(r.Type),
		Name:     r.Name,
		Value:    r.Value,
		TTL:      r.TTL,
		Priority: r.Priority,
		Weight:   r.Weight,
		Port:     r.Port,
		Flags:    r.Flags,
		Proxied:  r.Proxied,
	}

	if r.TTL == 0 {
		record.TTL = 3600
	}

	if r.Tag != "" {
		record.Tag = &r.Tag
	}

	if r.Comment != "" {
		record.Comment = &r.Comment
	}

	return record
}

// UpdateDNSRecordRequest represents a request to update a DNS record
type UpdateDNSRecordRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=255"`
	Value    string `json:"value" validate:"required,min=1"`
	Type     string `json:"type" validate:"required,oneof=A AAAA CNAME MX TXT SRV CAA"`
	TTL      int    `json:"ttl" validate:"omitempty,min=1"`
	Priority *int   `json:"priority" validate:"omitempty,min=0"`
	Weight   *int   `json:"weight" validate:"omitempty,min=0"`
	Port     *int   `json:"port" validate:"omitempty,min=1,max=65535"`
	Flags    *int   `json:"flags" validate:"omitempty,min=0"`
	Tag      string `json:"tag" validate:"omitempty,max=255"`
	Comment  string `json:"comment" validate:"omitempty,max=1000"`
	Proxied  *bool  `json:"proxied"`
}

// ApplyToModel applies the update request to an existing DNSRecord
func (r *UpdateDNSRecordRequest) ApplyToModel(record *models.DNSRecord) {
	record.Name = r.Name
	record.Value = r.Value
	record.Type = enums.RecordType(r.Type)
	record.Priority = r.Priority
	record.Weight = r.Weight
	record.Port = r.Port
	record.Flags = r.Flags
	record.Proxied = r.Proxied

	if r.TTL > 0 {
		record.TTL = r.TTL
	}

	if r.Tag != "" {
		record.Tag = &r.Tag
	} else {
		record.Tag = nil
	}

	if r.Comment != "" {
		record.Comment = &r.Comment
	} else {
		record.Comment = nil
	}
}

// DNSRecordResponse represents a DNS record in API responses
type DNSRecordResponse struct {
	ID         string  `json:"id"`
	DomainID   string  `json:"domain_id"`
	ProviderID string  `json:"provider_id"`
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	Value      string  `json:"value"`
	TTL        int     `json:"ttl"`
	Priority   *int    `json:"priority,omitempty"`
	Tag        *string `json:"tag,omitempty"`
	Weight     *int    `json:"weight,omitempty"`
	Port       *int    `json:"port,omitempty"`
	Flags      *int    `json:"flags,omitempty"`
	Comment    *string `json:"comment,omitempty"`
	Proxied    *bool   `json:"proxied,omitempty"`
	IsEditable bool    `json:"is_editable"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// ToDNSRecordResponse converts a DNSRecord to DNSRecordResponse
func ToDNSRecordResponse(r *models.DNSRecord) DNSRecordResponse {
	return DNSRecordResponse{
		ID:         r.ID,
		DomainID:   r.DomainID,
		ProviderID: r.ProviderID,
		Type:       r.Type.String(),
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
		IsEditable: r.IsEditable(),
		CreatedAt:  pkgdto.FormatTimeOrEmpty(r.CreatedAt),
		UpdatedAt:  pkgdto.FormatTimeOrEmpty(r.UpdatedAt),
	}
}
